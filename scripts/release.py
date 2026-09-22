#!/usr/bin/env python3
"""Validate release metadata or build an unsigned Linux amd64 release set."""

from __future__ import annotations

import argparse
import gzip
import hashlib
import io
import json
import os
import platform
import re
import shutil
import subprocess
import sys
import tarfile
import tempfile
from pathlib import Path
from typing import Any, Sequence


ROOT = Path(__file__).resolve().parents[1]
VERSION_PATTERN = re.compile(r"^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$")
REQUIRED_DOCUMENTS = (
    "CHANGELOG.md",
    "CONTRIBUTING.md",
    "LICENSE",
    "NOTICE",
    "README.md",
    "SECURITY.md",
    "THIRD_PARTY_NOTICES.md",
    "docs/COMPATIBILITY.md",
    "docs/M0_BASELINE.md",
    "docs/RELEASING.md",
)
SCHEMA_CONTRACTS = {
    "ir": ("docs/ir.schema.json", "v"),
    "context": ("docs/context.schema.json", "version"),
    "discovery": ("docs/discovery.schema.json", "version"),
    "artifacts": ("docs/artifacts.schema.json", "version"),
    "obligations": ("docs/obligations.schema.json", "version"),
}


class ReleaseError(RuntimeError):
    """Describe a release validation or packaging failure."""


def run(
    command: Sequence[str],
    *,
    cwd: Path = ROOT,
    env: dict[str, str] | None = None,
) -> subprocess.CompletedProcess[str]:
    """Run a release command and retain bounded diagnostics on failure."""
    completed = subprocess.run(
        list(command),
        cwd=cwd,
        env=env,
        capture_output=True,
        text=True,
        timeout=900,
    )
    if completed.returncode != 0:
        detail = (completed.stderr or completed.stdout)[-12000:]
        raise ReleaseError(f"{' '.join(command)} failed ({completed.returncode}):\n{detail}")
    return completed


def sha256(path: Path) -> str:
    """Return the lowercase SHA-256 digest of a file."""
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for block in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def package_version(root: Path = ROOT) -> str:
    """Read the project version without adding a TOML dependency for Python 3.9."""
    pyproject = (root / "pyproject.toml").read_text(encoding="utf-8")
    project = pyproject.split("[project]", 1)
    if len(project) != 2:
        raise ReleaseError("pyproject.toml has no [project] table")
    match = re.search(r'^version\s*=\s*"([^"]+)"\s*$', project[1], re.MULTILINE)
    if match is None or VERSION_PATTERN.fullmatch(match.group(1)) is None:
        raise ReleaseError("pyproject.toml project.version must be plain SemVer")
    return match.group(1)


def go_requirements(root: Path = ROOT) -> dict[str, str]:
    """Return all explicitly required Go modules and versions."""
    requirements: dict[str, str] = {}
    for line in (root / "go.mod").read_text(encoding="utf-8").splitlines():
        fields = line.strip().split()
        if len(fields) >= 3 and fields[0] == "require":
            module, version = fields[1], fields[2]
        elif len(fields) >= 2:
            module, version = fields[0], fields[1]
        else:
            continue
        if module.startswith("github.com/") or module.startswith("gopkg.in/"):
            requirements[module] = version
    return requirements


def schema_version(path: Path, property_name: str) -> str:
    """Read the current top-level contract identifier from a JSON Schema."""
    schema = json.loads(path.read_text(encoding="utf-8"))
    try:
        value = schema["properties"][property_name]["const"]
    except (KeyError, TypeError) as exc:
        raise ReleaseError(f"{path} has no properties.{property_name}.const") from exc
    if not isinstance(value, str) or not value:
        raise ReleaseError(f"{path} has an invalid contract identifier")
    return value


def parse_help_commands(help_text: str) -> set[str]:
    """Extract public commands from the executable's Usage block."""
    commands = set(re.findall(r"^  repoctx ([a-z][a-z0-9-]*)\b", help_text, re.MULTILINE))
    commands.add("help")
    return commands


def parse_usage_commands(usage_text: str) -> set[str]:
    """Extract top-level command declarations from the Usage specification."""
    return set(re.findall(r'^cmd\s+"([a-z][a-z0-9-]*)"', usage_text, re.MULTILINE))


def check_repository(root: Path = ROOT) -> dict[str, Any]:
    """Validate release metadata, dependencies, schemas, skill, and CLI parity."""
    missing = [path for path in REQUIRED_DOCUMENTS if not (root / path).is_file()]
    if missing:
        raise ReleaseError(f"missing release documents: {', '.join(missing)}")

    version = package_version(root)
    changelog = (root / "CHANGELOG.md").read_text(encoding="utf-8")
    if re.search(rf"^## \[{re.escape(version)}\] - \d{{4}}-\d{{2}}-\d{{2}}$", changelog, re.MULTILINE) is None:
        raise ReleaseError(f"CHANGELOG.md has no dated {version} release")

    readme = (root / "README.md").read_text(encoding="utf-8")
    manifest = (root / "MANIFEST.in").read_text(encoding="utf-8")
    for path in ("CHANGELOG.md", "CONTRIBUTING.md", "SECURITY.md", "THIRD_PARTY_NOTICES.md", "docs/RELEASING.md"):
        if path not in readme:
            raise ReleaseError(f"README.md does not link {path}")
    for path in (
        "CHANGELOG.md",
        "CONTRIBUTING.md",
        "LICENSE",
        "NOTICE",
        "SECURITY.md",
        "THIRD_PARTY_NOTICES.md",
    ):
        if f"include {path}" not in manifest:
            raise ReleaseError(f"MANIFEST.in does not include {path}")
    for directory in ("docs", "scripts", "skills", "tests"):
        if f"graft {directory}" not in manifest:
            raise ReleaseError(f"MANIFEST.in does not graft {directory}")

    notices = (root / "THIRD_PARTY_NOTICES.md").read_text(encoding="utf-8")
    requirements = go_requirements(root)
    missing_notices = [
        f"{module} {release}"
        for module, release in requirements.items()
        if f"`{module}` | `{release}`" not in notices
    ]
    if missing_notices:
        raise ReleaseError(f"missing dependency notices: {', '.join(missing_notices)}")

    with tempfile.TemporaryDirectory(prefix="repoctx-release-check-") as temp:
        binary = Path(temp) / "repoctx"
        env = os.environ.copy()
        env.update({"CGO_ENABLED": "1", "GOCACHE": str(Path(temp) / "go-cache"), "GOWORK": "off"})
        run(["go", "mod", "verify"], cwd=root, env=env)
        vuln = shutil.which("govulncheck")
        if vuln is None:
            raise ReleaseError("govulncheck is required for release validation")
        run([vuln, "./..."], cwd=root, env=env)
        run(["go", "build", "-buildvcs=false", "-trimpath", "-o", str(binary), "."], cwd=root, env=env)
        help_result = run([str(binary), "help"], cwd=root)
        version_result = run([str(binary), "version", "-format", "json"], cwd=root)

    usage_commands = parse_usage_commands((root / "repoctx.usage.kdl").read_text(encoding="utf-8"))
    help_commands = parse_help_commands(help_result.stdout + help_result.stderr)
    if usage_commands != help_commands:
        raise ReleaseError(
            f"CLI/Usage command drift: executable={sorted(help_commands)} usage={sorted(usage_commands)}"
        )

    build_info = json.loads(version_result.stdout)
    contracts = build_info.get("contracts", {})
    for name, (relative_path, property_name) in SCHEMA_CONTRACTS.items():
        actual = schema_version(root / relative_path, property_name)
        if contracts.get(name) != actual:
            raise ReleaseError(f"{relative_path}={actual}, executable {name}={contracts.get(name)}")

    compatibility = (root / "docs/COMPATIBILITY.md").read_text(encoding="utf-8")
    for name, contract in contracts.items():
        if contract not in compatibility:
            raise ReleaseError(f"compatibility documentation omits {name}={contract}")

    skill = (root / "skills/repoctx/SKILL.md").read_text(encoding="utf-8")
    for command in ("version", "discover", "compile", "validate", "context"):
        if f"repoctx {command}" not in skill:
            raise ReleaseError(f"vendored skill omits the repoctx {command} preflight/workflow")

    for report in ("m6-01-evidence.md", "m6-02-evidence.md", "m6-03-evidence.md"):
        if not (root / "docs" / "reports" / report).is_file():
            raise ReleaseError(f"missing current release evidence: {report}")

    return {
        "version": version,
        "commands": sorted(help_commands),
        "contracts": contracts,
        "go_requirements": requirements,
        "vulnerability_scan": "passed",
    }


def git_state(root: Path = ROOT) -> tuple[str, bool]:
    """Return the exact Git revision and whether the worktree is clean."""
    revision = run(["git", "rev-parse", "HEAD"], cwd=root).stdout.strip()
    status = run(["git", "status", "--porcelain", "--untracked-files=all"], cwd=root).stdout
    return revision, status == ""


def outside_root(path: Path, root: Path = ROOT) -> bool:
    """Report whether path is outside root without requiring Python 3.9 APIs."""
    try:
        path.resolve().relative_to(root.resolve())
    except ValueError:
        return True
    return False


def deterministic_archive(output: Path, prefix: str, files: Sequence[tuple[str, Path]]) -> None:
    """Write a gzip-compressed tar archive with normalized metadata."""
    tar_bytes = io.BytesIO()
    with tarfile.open(fileobj=tar_bytes, mode="w", format=tarfile.PAX_FORMAT) as archive:
        for name, source in sorted(files):
            data = source.read_bytes()
            info = tarfile.TarInfo(f"{prefix}/{name}")
            info.size = len(data)
            info.mode = 0o755 if name == "repoctx" else 0o644
            info.mtime = 0
            info.uid = 0
            info.gid = 0
            info.uname = ""
            info.gname = ""
            archive.addfile(info, io.BytesIO(data))
    with output.open("wb") as raw:
        with gzip.GzipFile(filename="", mode="wb", fileobj=raw, mtime=0) as compressed:
            compressed.write(tar_bytes.getvalue())


def copy_source(root: Path, destination: Path) -> None:
    """Copy release sources without local build and VCS artifacts."""
    shutil.copytree(
        root,
        destination,
        ignore=shutil.ignore_patterns(
            ".git", "build", "dist", "*.egg-info", "__pycache__", ".pytest_cache"
        ),
    )


def tool_version(command: Sequence[str]) -> str:
    """Return the first non-empty version-output line for provenance."""
    completed = run(command)
    text = completed.stdout or completed.stderr
    return next((line.strip() for line in text.splitlines() if line.strip()), "unknown")


def build_release(version: str, output_dir: Path, root: Path = ROOT) -> dict[str, Any]:
    """Build an atomic unsigned release directory for Linux amd64."""
    checks = check_repository(root)
    if version != checks["version"] or VERSION_PATTERN.fullmatch(version) is None:
        raise ReleaseError(f"release {version!r} does not match pyproject version {checks['version']!r}")
    if platform.system() != "Linux" or platform.machine() not in {"x86_64", "amd64"}:
        raise ReleaseError("the supported release builder target is Linux amd64")
    if not outside_root(output_dir, root):
        raise ReleaseError("release output must be outside the repository")
    if output_dir.exists():
        raise ReleaseError(f"release output already exists: {output_dir}")

    revision, clean = git_state(root)
    if not clean:
        raise ReleaseError("release builds require a clean Git worktree")

    output_dir.parent.mkdir(parents=True, exist_ok=True)
    work = Path(tempfile.mkdtemp(prefix="repoctx-release-", dir=output_dir.parent))
    staging = work / "output"
    staging.mkdir()
    try:
        env = os.environ.copy()
        env.update({"CGO_ENABLED": "1", "GOCACHE": str(work / "go-cache"), "GOWORK": "off"})
        binary = work / "repoctx"
        ldflags = " ".join(
            (
                f"-X=github.com/tdeshazo/repoctx/internal/cli.buildRelease={version}",
                f"-X=github.com/tdeshazo/repoctx/internal/cli.buildRevision={revision}",
                "-X=github.com/tdeshazo/repoctx/internal/cli.buildModified=false",
                "-X=github.com/tdeshazo/repoctx/internal/cli.buildDistribution=release-archive",
            )
        )
        run(
            ["go", "build", "-buildvcs=false", "-trimpath", "-ldflags", ldflags, "-o", str(binary), "."],
            cwd=root,
            env=env,
        )
        binary_info = json.loads(run([str(binary), "version", "-format", "json"]).stdout)
        if (
            binary_info.get("release") != version
            or binary_info.get("revision") != revision
            or binary_info.get("modified") != "false"
            or binary_info.get("distribution") != "release-archive"
            or binary_info.get("platform") != "linux/amd64"
        ):
            raise ReleaseError(f"release binary build information is inconsistent: {binary_info}")

        archive = staging / f"repoctx-{version}-linux-amd64.tar.gz"
        deterministic_archive(
            archive,
            f"repoctx-{version}-linux-amd64",
            (
                ("repoctx", binary),
                ("LICENSE", root / "LICENSE"),
                ("NOTICE", root / "NOTICE"),
                ("THIRD_PARTY_NOTICES.md", root / "THIRD_PARTY_NOTICES.md"),
            ),
        )

        package_source = work / "source"
        package_output = work / "python-dist"
        copy_source(root, package_source)
        package_output.mkdir()
        run(
            [
                sys.executable,
                "-m",
                "build",
                "--wheel",
                "--sdist",
                "--no-isolation",
                "--outdir",
                str(package_output),
            ],
            cwd=package_source,
            env=env,
        )
        packages = sorted(package_output.iterdir())
        if len(packages) != 2 or not any(path.suffix == ".whl" for path in packages) or not any(path.name.endswith(".tar.gz") for path in packages):
            raise ReleaseError(f"expected one wheel and one sdist, found {[path.name for path in packages]}")
        for package in packages:
            shutil.copy2(package, staging / package.name)

        distributables = sorted(
            path for path in staging.iterdir() if path.is_file()
        )
        materials = []
        for relative in ("go.mod", "go.sum", "pyproject.toml", "setup.py", "MANIFEST.in"):
            source = root / relative
            materials.append({"path": relative, "sha256": sha256(source)})
        provenance = {
            "version": "repoctx.release-provenance/v1alpha1",
            "release": version,
            "revision": revision,
            "source_tree_clean": True,
            "authenticated": False,
            "platform": "linux/amd64",
            "toolchains": {
                "go": tool_version(["go", "version"]),
                "c_compiler": tool_version(["cc", "--version"]),
                "python": tool_version([sys.executable, "--version"]),
                "build_frontend": tool_version([sys.executable, "-m", "build", "--version"]),
            },
            "contracts": binary_info["contracts"],
            "materials": materials,
            "artifacts": [
                {"name": path.name, "sha256": sha256(path), "size": path.stat().st_size}
                for path in distributables
            ],
        }
        provenance_path = staging / f"repoctx-{version}.provenance.json"
        provenance_path.write_text(
            json.dumps(provenance, indent=2, sort_keys=True) + "\n", encoding="utf-8"
        )

        checksums = []
        for path in sorted(staging.iterdir(), key=lambda item: item.name):
            if path.is_file():
                checksums.append(f"{sha256(path)}  {path.name}")
        (staging / "SHA256SUMS").write_text("\n".join(checksums) + "\n", encoding="utf-8")

        os.replace(staging, output_dir)
        result = {
            "ok": True,
            "release": version,
            "revision": revision,
            "output_dir": str(output_dir),
            "artifacts": sorted(path.name for path in output_dir.iterdir()),
        }
    finally:
        shutil.rmtree(work, ignore_errors=True)
    return result


def main(argv: Sequence[str] | None = None) -> int:
    """Run release validation or build an atomic release artifact set."""
    parser = argparse.ArgumentParser(description=__doc__)
    subparsers = parser.add_subparsers(dest="command", required=True)
    subparsers.add_parser("check", help="validate release metadata and synchronized surfaces")
    build = subparsers.add_parser("build", help="build the supported unsigned release artifacts")
    build.add_argument("--version", required=True)
    build.add_argument("--output-dir", required=True, type=Path)
    args = parser.parse_args(argv)
    try:
        if args.command == "check":
            print(json.dumps({"ok": True, **check_repository()}, sort_keys=True))
        else:
            print(json.dumps(build_release(args.version, args.output_dir), sort_keys=True))
    except (OSError, ReleaseError, json.JSONDecodeError) as exc:
        print(f"repoctx release: {exc}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
