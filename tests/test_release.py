from __future__ import annotations

import importlib.util
import io
import tarfile
import tempfile
from pathlib import Path
from unittest import TestCase


ROOT = Path(__file__).resolve().parents[1]
SPEC = importlib.util.spec_from_file_location("repoctx_release", ROOT / "scripts" / "release.py")
assert SPEC is not None and SPEC.loader is not None
release = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(release)


class ReleaseTests(TestCase):
    def test_package_version_is_plain_semver(self) -> None:
        self.assertRegex(release.package_version(), release.VERSION_PATTERN)

    def test_go_requirements_have_notices(self) -> None:
        notices = (ROOT / "THIRD_PARTY_NOTICES.md").read_text(encoding="utf-8")
        for module, version in release.go_requirements().items():
            self.assertIn(f"`{module}` | `{version}`", notices)

    def test_cli_and_usage_commands_match(self) -> None:
        usage = (ROOT / "repoctx.usage.kdl").read_text(encoding="utf-8")
        help_text = "\n".join(
            f"  repoctx {command}"
            for command in release.parse_usage_commands(usage)
            if command != "help"
        )
        self.assertEqual(
            release.parse_help_commands(help_text),
            release.parse_usage_commands(usage),
        )

    def test_archive_is_reproducible_and_normalized(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            source = root / "repoctx"
            source.write_bytes(b"binary")
            notice = root / "NOTICE"
            notice.write_text("notice\n", encoding="utf-8")
            first = root / "first.tar.gz"
            second = root / "second.tar.gz"
            files = (("repoctx", source), ("NOTICE", notice))
            release.deterministic_archive(first, "repoctx-1.2.3-linux-amd64", files)
            release.deterministic_archive(second, "repoctx-1.2.3-linux-amd64", files)
            self.assertEqual(first.read_bytes(), second.read_bytes())

    def test_sdist_normalization_removes_varying_tar_metadata(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            outputs = []
            for index, timestamp in enumerate((10, 20)):
                source = root / f"source-{index}.tar.gz"
                with tarfile.open(source, mode="w:gz") as archive:
                    directory = tarfile.TarInfo("repoctx-1.2.3")
                    directory.type = tarfile.DIRTYPE
                    directory.mtime = timestamp
                    archive.addfile(directory)
                    data = b"content\n"
                    member = tarfile.TarInfo("repoctx-1.2.3/README.md")
                    member.size = len(data)
                    member.mtime = timestamp
                    archive.addfile(member, io.BytesIO(data))
                output = root / f"normalized-{index}.tar.gz"
                release.normalize_sdist(source, output)
                outputs.append(output)

            self.assertEqual(outputs[0].read_bytes(), outputs[1].read_bytes())

    def test_release_output_must_be_outside_repository(self) -> None:
        self.assertFalse(release.outside_root(ROOT / "dist"))
        self.assertTrue(release.outside_root(Path(tempfile.gettempdir()) / "repoctx-release-test"))


if __name__ == "__main__":
    import unittest

    unittest.main()
