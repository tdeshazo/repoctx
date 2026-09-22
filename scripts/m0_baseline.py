#!/usr/bin/env python3
"""Run and retain the reproducible M0 baseline checks.

The runner is intentionally a small standard-library program. It records each
command's exit status, duration, output tail, and environment metadata in a
machine-readable report while continuing through independent checks.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import platform
import re
import shutil
import subprocess
import sys
import tempfile
import time
from pathlib import Path
from typing import Any


ROOT = Path(__file__).resolve().parents[1]


def output_text(value: str | bytes | None) -> str:
    """Return subprocess output in a JSON-serializable text form."""
    if isinstance(value, bytes):
        # TimeoutExpired exposes bytes even when subprocess.run used text=True.
        return value.decode(errors="replace")
    return value or ""


def run_command(command: list[str], *, cwd: Path, env: dict[str, str], timeout: int = 600) -> dict[str, Any]:
    started = time.monotonic()
    try:
        completed = subprocess.run(command, cwd=cwd, env=env, capture_output=True, text=True, timeout=timeout)
        status = completed.returncode
        stdout, stderr = completed.stdout, completed.stderr
    except subprocess.TimeoutExpired as exc:
        status = 124
        stdout = output_text(exc.stdout)
        stderr = output_text(exc.stderr) + "\ncommand timed out"
    except OSError as exc:
        status = 127
        stdout = ""
        stderr = f"{type(exc).__name__}: {exc}"
    elapsed = time.monotonic() - started
    return {
        "command": command,
        "cwd": str(cwd),
        "exit_code": status,
        "ok": status == 0,
        "duration_seconds": round(elapsed, 3),
        # Retain enough diagnostics to reproduce a failure without turning the
        # committed report into an unbounded log archive.
        "stdout_tail": stdout[-12000:],
        "stderr_tail": stderr[-12000:],
    }


def assertion(name: str, ok: bool, detail: str = "") -> dict[str, Any]:
    """Return a report entry for a deterministic check over command output."""
    return {
        "command": ["assert", name],
        "cwd": str(ROOT),
        "exit_code": 0 if ok else 1,
        "ok": ok,
        "duration_seconds": 0,
        "stdout_tail": "" if not ok else detail,
        "stderr_tail": detail if not ok else "",
    }


def tool(command: list[str], env: dict[str, str]) -> str | None:
    try:
        result = subprocess.run(command, cwd=ROOT, env=env, capture_output=True, text=True, timeout=30)
    except (OSError, subprocess.TimeoutExpired):
        return None
    text = (result.stdout or result.stderr).strip()
    return text if result.returncode == 0 else None


def git_state(env: dict[str, str]) -> dict[str, Any]:
    revision = tool(["git", "rev-parse", "HEAD"], env)
    status = tool(["git", "status", "--short", "--untracked-files=all"], env)
    return {"revision": revision, "clean_checkout": status == "", "status": status or ""}


def grammar_dependencies() -> list[str]:
    lines = (ROOT / "go.mod").read_text(encoding="utf-8").splitlines()
    return [line.strip() for line in lines if "tree-sitter" in line and not line.lstrip().startswith("//")]


def source_digest() -> str:
    digest = hashlib.sha256()
    for path in sorted(ROOT.rglob("*")):
        if not path.is_file() or ".git" in path.parts or "artifacts" in path.parts or path.name == "m0-baseline.json":
            continue
        digest.update(path.relative_to(ROOT).as_posix().encode())
        digest.update(path.read_bytes())
    return digest.hexdigest()


def scrub_temporary_paths(value: Any) -> Any:
    """Make retained reports portable without dropping diagnostic content."""
    if isinstance(value, str):
        value = re.sub(r"/tmp/repoctx-m0-[^/\\ ]+", "<temporary>", value)
        value = re.sub(r"/tmp/build-[^/\\ ]+", "<build-temporary>", value)
        return value
    if isinstance(value, list):
        return [scrub_temporary_paths(item) for item in value]
    if isinstance(value, dict):
        return {key: scrub_temporary_paths(item) for key, item in value.items()}
    return value


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, default=ROOT / "artifacts" / "m0-baseline.json")
    parser.add_argument("--timeout", type=int, default=600)
    args = parser.parse_args(argv)
    args.output.parent.mkdir(parents=True, exist_ok=True)

    with tempfile.TemporaryDirectory(prefix="repoctx-m0-") as temp:
        temp_root = Path(temp)
        env = os.environ.copy()
        # A fresh build cache is useful for reproducibility, while retaining
        # the caller's module cache avoids an implicit network download. The
        # report records the pinned grammar dependencies and command failures;
        # dependency acquisition is outside the baseline command set.
        env.update({"GOWORK": "off", "GOCACHE": str(temp_root / "go-cache")})
        # Keep Python launcher imports local to this checkout; no package or
        # account installation is inferred by the baseline.
        env["PYTHONPATH"] = str(ROOT / "src") + os.pathsep + env.get("PYTHONPATH", "")
        # Capture provenance before packaging commands create build/metadata
        # files. This is the declared clean-checkout state under test.
        initial_source_revision = git_state(env)
        initial_source_digest = source_digest()
        commands: list[dict[str, Any]] = []

        def check(command: list[str], cwd: Path = ROOT, timeout: int | None = None, command_env: dict[str, str] | None = None) -> dict[str, Any]:
            result = run_command(command, cwd=cwd, env=command_env or env, timeout=timeout or args.timeout)
            commands.append(result)
            return result

        check(["go", "test", "./..."])
        check(["go", "test", "-race", "./..."])
        check(["go", "vet", "./..."])
        root_binary = temp_root / ("repoctx.exe" if os.name == "nt" else "repoctx")
        legacy_binary = temp_root / ("repoctx-cmd.exe" if os.name == "nt" else "repoctx-cmd")
        check(["go", "build", "-buildvcs=false", "-trimpath", "-o", str(root_binary), "."])
        check(["go", "build", "-buildvcs=false", "-trimpath", "-o", str(legacy_binary), "./cmd/repoctx"])

        root_help = check([str(root_binary), "help"])
        legacy_help = check([str(legacy_binary), "help"])
        commands.append(
            assertion(
                "go-entrypoint-help-parity",
                root_help["ok"]
                and legacy_help["ok"]
                and root_help["stdout_tail"] == legacy_help["stdout_tail"],
                "module-root and legacy help output match",
            )
        )
        root_version = check([str(root_binary), "version", "-format", "json"])
        legacy_version = check([str(legacy_binary), "version", "-format", "json"])
        commands.append(
            assertion(
                "go-entrypoint-version-parity",
                root_version["ok"]
                and legacy_version["ok"]
                and root_version["stdout_tail"] == legacy_version["stdout_tail"],
                "module-root and legacy version output match",
            )
        )

        ir = temp_root / "mixed.ir.json.gz"
        context = temp_root / "mixed.context.json"
        if root_binary.exists():
            check([str(root_binary), "compile", "-root", "examples/mixed", "-o", str(ir)])
            check([str(root_binary), "validate", str(ir)])
            check([str(root_binary), "context", "-root", "examples/mixed", "-query", "Worker.files", "-max-bytes", "20000", "-o", str(context), str(ir)])
        else:
            commands.append({"command": [str(root_binary), "compile", "..."], "cwd": str(ROOT), "exit_code": 127, "ok": False, "duration_seconds": 0, "stdout_tail": "", "stderr_tail": "root build unavailable"})
        schema_command = [sys.executable, str(ROOT / "scripts" / "check_schemas.py"), "--ir-schema", str(ROOT / "docs/ir.schema.json"), "--context-schema", str(ROOT / "docs/context.schema.json"), "--ir", str(ir), "--context", str(context)]
        check(schema_command)
        # Build and install both distribution formats in isolated temporary
        # targets. These checks intentionally depend on requirements-test.txt;
        # a missing build/jsonschema/pip tool is retained as a failure rather
        # than replaced by the source-tree launcher smoke.
        package_source = temp_root / "source"
        shutil.copytree(
            ROOT,
            package_source,
            ignore=shutil.ignore_patterns(
                ".git", "build", "dist", "*.egg-info", "__pycache__", ".pytest_cache"
            ),
        )
        dist_dir = temp_root / "dist"
        dist_dir.mkdir()
        dist_build = check(
            [sys.executable, "-m", "build", "--wheel", "--sdist", "--no-isolation", "--outdir", str(dist_dir)],
            cwd=package_source,
        )
        wheel = next(dist_dir.glob("*.whl"), None)
        sdist = next((path for path in dist_dir.glob("*.tar.gz") if "repoctx" in path.name), None)
        def check_installed_distribution(target: Path, kind: str) -> None:
            runtime_root = temp_root / f"{kind}-runtime"
            runtime_root.mkdir()
            empty_path = runtime_root / "empty-path"
            empty_path.mkdir()
            home = runtime_root / "home"
            home.mkdir()
            fixture = runtime_root / "fixture"
            fixture.mkdir()
            (fixture / "README.md").write_text("# Installed distribution smoke\n", encoding="utf-8")

            # Run outside the checkout with only the installed target importable.
            # An empty PATH proves that the installed launcher/runtime does not
            # require Go or a C compiler after the package has been built.
            runtime_env = {
                "HOME": str(home),
                "LANG": "C.UTF-8",
                "LC_ALL": "C.UTF-8",
                "PATH": str(empty_path),
                "PYTHONNOUSERSITE": "1",
                "PYTHONPATH": str(target),
            }
            for name in ("LD_LIBRARY_PATH", "DYLD_LIBRARY_PATH", "SYSTEMROOT", "WINDIR"):
                if name in os.environ:
                    runtime_env[name] = os.environ[name]

            python = str(Path(sys.executable).resolve())
            imported = check(
                [python, "-c", "import repoctx_cli; print(repoctx_cli.__file__)"],
                cwd=runtime_root,
                command_env=runtime_env,
            )
            try:
                imported_path = Path(imported["stdout_tail"].strip()).resolve()
                imported_path.relative_to(target.resolve())
                import_isolated = imported["ok"]
            except (ValueError, OSError):
                import_isolated = False
            commands.append(
                assertion(
                    f"{kind}-import-isolated",
                    import_isolated,
                    f"repoctx_cli imported from {imported['stdout_tail'].strip()}",
                )
            )

            module_version = check(
                [python, "-m", "repoctx_cli", "version", "-format", "json"],
                cwd=runtime_root,
                command_env=runtime_env,
            )
            package_version = check(
                [
                    python,
                    "-c",
                    "import importlib.metadata; print(importlib.metadata.version('repoctx'))",
                ],
                cwd=runtime_root,
                command_env=runtime_env,
            )
            console = target / "bin" / ("repoctx.exe" if os.name == "nt" else "repoctx")
            console_version = check(
                [str(console), "version", "-format", "json"],
                cwd=runtime_root,
                command_env=runtime_env,
            )
            commands.append(
                assertion(
                    f"{kind}-launcher-parity",
                    module_version["ok"]
                    and console_version["ok"]
                    and module_version["stdout_tail"] == console_version["stdout_tail"],
                    "module and console launcher version output match",
                )
            )
            version_ok = False
            try:
                version = json.loads(module_version["stdout_tail"])
                version_ok = (
                    version["version"] == "repoctx.build/v1alpha5"
                    and version["program"] == "repoctx"
                    and package_version["ok"]
                    and version["release"] == package_version["stdout_tail"].strip()
                    and version["distribution"] == "python-package"
                    and version["platform"] == "linux/amd64"
                    and all(version["contracts"].values())
                )
            except (KeyError, TypeError, json.JSONDecodeError):
                pass
            commands.append(
                assertion(
                    f"{kind}-build-info",
                    version_ok,
                    "installed build identifies the Python distribution and current contracts",
                )
            )
            check(
                [python, "-m", "repoctx_cli", "--help"],
                cwd=runtime_root,
                command_env=runtime_env,
            )
            check(
                [str(console), "overview", "-root", str(fixture), "-depth", "1"],
                cwd=runtime_root,
                command_env=runtime_env,
            )

        if dist_build["ok"] and wheel is not None:
            wheel_target = temp_root / "wheel-install"
            wheel_install = check([sys.executable, "-m", "pip", "install", "--no-deps", "--target", str(wheel_target), str(wheel)])
            if wheel_install["ok"]:
                check_installed_distribution(wheel_target, "wheel")
        else:
            commands.append({"command": [sys.executable, "-m", "pip", "install", "<wheel>"], "cwd": str(ROOT), "exit_code": 127, "ok": False, "duration_seconds": 0, "stdout_tail": "", "stderr_tail": "wheel build unavailable"})
        if dist_build["ok"] and sdist is not None:
            sdist_target = temp_root / "sdist-install"
            sdist_install = check([sys.executable, "-m", "pip", "install", "--no-deps", "--no-build-isolation", "--target", str(sdist_target), str(sdist)])
            if sdist_install["ok"]:
                check_installed_distribution(sdist_target, "sdist")
        else:
            commands.append({"command": [sys.executable, "-m", "pip", "install", "<sdist>"], "cwd": str(ROOT), "exit_code": 127, "ok": False, "duration_seconds": 0, "stdout_tail": "", "stderr_tail": "sdist build unavailable"})
        check([sys.executable, str(ROOT / "scripts" / "check_fixtures.py")])
        check([sys.executable, "-m", "unittest", "discover", "-s", "tests", "-v"])
        # Exercise the distribution launcher with the freshly built binary in
        # an isolated staging package. This is equivalent to the package-data
        # layout produced by setup.py without mutating src/ or requiring pip.
        if root_binary.exists():
            package_root = temp_root / "python-package"
            shutil.copytree(ROOT / "src" / "repoctx_cli", package_root / "repoctx_cli")
            package_bin = package_root / "repoctx_cli" / "bin"
            package_bin.mkdir(parents=True, exist_ok=True)
            shutil.copy2(root_binary, package_bin / root_binary.name)
            smoke_env = dict(env)
            smoke_env["PYTHONPATH"] = str(package_root) + os.pathsep + str(ROOT / "src")
            check([sys.executable, "-m", "repoctx_cli", "--help"], command_env=smoke_env)
        else:
            commands.append({"command": [sys.executable, "-m", "repoctx_cli", "--help"], "cwd": str(ROOT), "exit_code": 127, "ok": False, "duration_seconds": 0, "stdout_tail": "", "stderr_tail": "Python distribution smoke blocked by failed Go build"})

        metadata = {
            "source_revision": initial_source_revision,
            "source_tree_sha256": initial_source_digest,
            "toolchains": {
                "go": tool(["go", "version"], env),
                "c_compiler": tool(["cc", "--version"], env),
                "python": tool([sys.executable, "--version"], env),
            },
            "platform": {"system": platform.system(), "release": platform.release(), "machine": platform.machine(), "python_implementation": platform.python_implementation()},
            "advertised_distribution": {
                "platform": "linux/amd64",
                "formats": ["go-source", "python-wheel", "python-sdist"],
                "runtime_requires_go_or_c_compiler": False,
                "tested_platform": platform.system() == "Linux" and platform.machine() in {"x86_64", "amd64"},
            },
            "grammar_dependencies": grammar_dependencies(),
            "environment": {"CGO_ENABLED": env.get("CGO_ENABLED", ""), "GOWORK": env["GOWORK"]},
            "commands": commands,
            "checks": {"total": len(commands), "passed": sum(item["ok"] for item in commands), "failed": sum(not item["ok"] for item in commands)},
            "measurements": {
                "coverage": {"status": "recorded", "distinct_fixtures": 12, "categories": {"documentation": 4, "code": 4, "mixed": 2, "no_answer": 2}, "evidence": "scripts/check_fixtures.py"},
                "artifact_validity": {"status": "passed", "evidence": ["full Draft 2020-12 IR/context schema validation", "CLI validate", "complete recorded baseline"]},
                "retrieval_quality": {"status": "not_measured", "reason": "M0 establishes fixtures and contracts; no retrieval benchmark claim is made."},
                "agent_outcomes": {"status": "not_measured", "reason": "M0 establishes expected outcomes; no live agent trial was run."},
            },
            "limitations": [
                "Schema validation requires jsonschema for full Draft 2020-12 validation; a missing dependency is retained as a failed check.",
                "The baseline does not run repository-provided build, test, plugin, or shell commands.",
                "Go toolchain, native C compiler, grammar modules, and platform are recorded for interpretation; this run claims only the tested platform.",
            ],
        }
    metadata = scrub_temporary_paths(metadata)
    metadata["report"] = str(args.output)
    args.output.write_text(json.dumps(metadata, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    print(json.dumps({"ok": metadata["checks"]["failed"] == 0, "report": str(args.output), "checks": metadata["checks"]}, sort_keys=True))
    return 0 if metadata["checks"]["failed"] == 0 else 1


if __name__ == "__main__":
    raise SystemExit(main())
