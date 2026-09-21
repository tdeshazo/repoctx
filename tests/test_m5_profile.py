"""Deterministic M5 complete-path profile tests."""

import importlib.util
from pathlib import Path
import tempfile
import unittest


PROJECT = Path(__file__).resolve().parents[1]


def load_module():
    spec = importlib.util.spec_from_file_location(
        "m5_profile", PROJECT / "scripts/m5_profile.py"
    )
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


PROFILE = load_module()


class M5ProfileTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.path = Path(self.temp.name) / "bench.txt"

    def write_results(self, samples=10):
        config = PROFILE.load_config()
        lines = ["goos: linux", "goarch: amd64", "cpu: test cpu"]
        for files in config["sizes"]:
            for group, stages in config["stages"].items():
                title = group.title()
                for stage_index, stage in enumerate(stages, 1):
                    for sample in range(samples):
                        duration = files * stage_index * 100 + sample
                        context = " 2048 context-bytes" if group == "context" else ""
                        lines.append(
                            f"BenchmarkM5{title}Path/files={files}/{stage}-8 3 "
                            f"{duration} ns/op {files * 20} artifact-bytes{context} "
                            f"{files:.1f} files {files * 10} source-bytes "
                            f"{files * stage_index} B/op {stage_index} allocs/op"
                        )
        self.path.write_text("\n".join(lines) + "\n", encoding="utf-8")

    def test_report_requires_and_summarizes_every_stage(self):
        self.write_results()
        report = PROFILE.build_report([self.path])
        self.assertEqual(report["samples_per_stage"], 10)
        self.assertEqual([item["files"] for item in report["sizes"]], [10, 100, 1000])
        self.assertEqual(report["sizes"][-1]["context-bytes"], 2048)
        self.assertGreater(report["sizes"][-1]["complete_path_p95_component_sum_ns"],
                           report["sizes"][-1]["complete_path_median_ns"])
        self.assertEqual(report["largest_atomic_bottleneck"]["stage"],
                         "source_verification")
        self.assertEqual(report["target"]["canonical_semantic_output"], "identical")

    def test_incomplete_sample_set_is_rejected(self):
        self.write_results(samples=9)
        with self.assertRaisesRegex(ValueError, "has 9 samples, want 10"):
            PROFILE.build_report([self.path])

    def test_nearest_rank_uses_observed_p95(self):
        self.assertEqual(PROFILE.nearest_rank(list(range(1, 11)), 0.95), 10)


if __name__ == "__main__":
    unittest.main()
