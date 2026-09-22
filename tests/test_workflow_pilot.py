import importlib.util
from pathlib import Path
import unittest


RUNNER = Path(__file__).resolve().parents[1] / "scripts/run_workflow_pilot.py"
SPEC = importlib.util.spec_from_file_location("workflow_pilot", RUNNER)
workflow_pilot = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(workflow_pilot)


class WorkflowPilotTest(unittest.TestCase):
    def test_observed_tool_items_deduplicates_lifecycle_events_and_keeps_file_changes(self):
        events = [
            {"type": "item.started", "item": {"id": "command", "type": "command_execution", "status": "in_progress"}},
            {"type": "item.completed", "item": {"id": "command", "type": "command_execution", "status": "completed"}},
            {"type": "item.started", "item": {"id": "change", "type": "file_change", "status": "in_progress"}},
            {"type": "item.completed", "item": {"id": "change", "type": "file_change", "status": "completed"}},
            {"type": "item.started", "item": {"id": "patch", "type": "apply_patch", "status": "in_progress"}},
            {"type": "item.completed", "item": {"id": "patch", "type": "apply_patch", "status": "completed"}},
        ]

        observed = workflow_pilot.observed_tool_items(events)

        self.assertEqual([event["item"]["id"] for event in observed], ["command", "change", "patch"])
        self.assertTrue(all(event["item"]["status"] == "completed" for event in observed))

    def test_treatment_primer_has_exact_read_form_and_discourages_repeat_discovery(self):
        prompt = workflow_pilot.task_prompt({"prompt": "Find the answer."}, "ordinary_tools_plus_repoctx", Path("/tmp/repoctx"))

        self.assertIn("read -root repository -file PATH[:START:END]", prompt)
        self.assertIn("If discover evidence answers the task, use it and stop.", prompt)
        self.assertIn("Only when more context is needed", prompt)


if __name__ == "__main__":
    unittest.main()
