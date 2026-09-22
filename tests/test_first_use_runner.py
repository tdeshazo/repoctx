import importlib.util
from pathlib import Path
import unittest


RUNNER = Path(__file__).resolve().parents[1] / "scripts/run_first_use.py"
SPEC = importlib.util.spec_from_file_location("first_use_runner", RUNNER)
first_use_runner = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(first_use_runner)


class FirstUseRunnerTest(unittest.TestCase):
    def test_export_excludes_evaluation_and_generated_boundaries_but_keeps_roadmap(self):
        self.assertTrue(first_use_runner.excluded_source_path("evals/first-use/README.md"))
        self.assertTrue(first_use_runner.excluded_source_path("scripts/run_first_use.py"))
        self.assertTrue(first_use_runner.excluded_source_path("docs/reports/m4-13-workflow-pilot.md"))
        self.assertTrue(first_use_runner.excluded_source_path("build/lib/repoctx"))
        self.assertFalse(first_use_runner.excluded_source_path("ROADMAP.md"))
        self.assertFalse(first_use_runner.excluded_source_path("skills/repoctx/SKILL.md"))
        self.assertFalse(first_use_runner.excluded_source_path("internal/discovery/discover.go"))

    def test_treatment_points_to_actual_skill_without_a_handcrafted_workflow(self):
        task = {"prompt": "Find the source evidence."}
        treatment = first_use_runner.task_prompt(task, "ordinary_tools_plus_repoctx", Path("/pinned/repoctx"))
        baseline = first_use_runner.task_prompt(task, "ordinary_tools", Path("/pinned/repoctx"))

        self.assertIn("repository/skills/repoctx/SKILL.md", treatment)
        self.assertIn("/pinned/repoctx", treatment)
        self.assertNotIn("begin with", treatment.lower())
        self.assertIn("Do not invoke repoctx", baseline)


if __name__ == "__main__":
    unittest.main()
