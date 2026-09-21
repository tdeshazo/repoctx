"""Predeclared M4 decision-rule tests."""

import importlib.util
from pathlib import Path
import unittest


PROJECT = Path(__file__).resolve().parents[1]


def load(name, relative):
    spec = importlib.util.spec_from_file_location(name, PROJECT / relative)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


DECISIONS = load("m4_decisions", "scripts/m4_decisions.py")


def interval(estimate, lower, upper):
    return {"estimate": estimate, "lower": lower, "upper": upper}


def decision_input(comparison, intervals, paired=33, eligible=33, missing=0):
    rules = DECISIONS.validate_config()
    return {
        "version": "repoctx.m4.decision-input/v1alpha1",
        "run_lock_sha256": "a" * 64,
        "accounting_report_sha256": "b" * 64,
        "scored_results_sha256": "c" * 64,
        "comparison": comparison,
        "eligible_tasks": eligible,
        "paired_tasks": paired,
        "missing_fraction": missing,
        "uncertainty": rules["uncertainty"],
        "intervals": intervals,
    }


class M4DecisionTests(unittest.TestCase):
    def test_rules_are_frozen_and_manual_conditions_are_descriptive(self):
        rules = DECISIONS.validate_config()
        indexed = {item["id"]: item for item in rules["comparisons"]}
        self.assertEqual(indexed["artifact_aware_vs_repoctx_graph"]["decision"],
                         "descriptive_only")
        self.assertEqual(indexed["human_oracle_upper_bound"]["attribution"], "human_gold")
        self.assertEqual(indexed["persisted_file_reference_vs_inline"]["track"],
                         "persistence")
        self.assertEqual(rules["sample_rules"]["underpowered"],
                         "inconclusive; never equivalence or non-regression")

    def test_end_to_end_efficiency_requires_quality_and_practical_threshold(self):
        value = decision_input("end_to_end_repoctx_graph_vs_ordinary", {
            "verified_task_success_absolute_delta": interval(0.01, -0.02, 0.04),
            "input_plus_output_tokens_relative_reduction": interval(0.22, 0.17, 0.28),
            "tool_calls_relative_reduction": interval(0.27, 0.21, 0.34),
            "trial_wall_ms_relative_reduction": interval(0.10, 0.02, 0.18),
        })
        result = DECISIONS.assess(value)
        self.assertEqual(result["overall"], "inconclusive")
        self.assertEqual(result["claims"]["input_plus_output_tokens_relative_reduction"],
                         "supported")
        self.assertEqual(result["claims"]["tool_calls_relative_reduction"], "supported")
        self.assertEqual(result["claims"]["trial_wall_ms_relative_reduction"],
                         "inconclusive")
        value["intervals"]["trial_wall_ms_relative_reduction"] = interval(0.20, 0.16, 0.25)
        self.assertEqual(DECISIONS.assess(value)["overall"], "supported")

    def test_quality_harm_blocks_efficiency_claims(self):
        value = decision_input("end_to_end_repoctx_graph_vs_ordinary", {
            "verified_task_success_absolute_delta": interval(-0.08, -0.11, -0.06),
            "input_plus_output_tokens_relative_reduction": interval(0.40, 0.35, 0.45),
            "tool_calls_relative_reduction": interval(0.40, 0.35, 0.45),
            "trial_wall_ms_relative_reduction": interval(0.40, 0.35, 0.45),
        })
        result = DECISIONS.assess(value)
        self.assertEqual(result["overall"], "quality_harm")
        self.assertTrue(all(state == "quality_harm" for name, state in result["claims"].items()
                            if name != "verified_task_success_absolute_delta"))

    def test_underpowered_and_crossing_intervals_are_inconclusive(self):
        intervals = {
            "required_span_recall_absolute_delta": interval(0.04, -0.01, 0.08),
        }
        underpowered = decision_input("evidence_graph_vs_no_graph", intervals,
                                      paired=12, eligible=12)
        self.assertEqual(DECISIONS.assess(underpowered)["overall"],
                         "inconclusive_underpowered")
        crossing = decision_input("evidence_graph_vs_no_graph", intervals)
        self.assertEqual(DECISIONS.assess(crossing)["overall"], "inconclusive")
        missing = decision_input("evidence_graph_vs_no_graph", intervals,
                                 paired=30, eligible=33, missing=3 / 33)
        self.assertEqual(DECISIONS.assess(missing)["overall"], "inconclusive_missing")

    def test_threshold_not_met_is_not_equivalence(self):
        value = decision_input("evidence_graph_vs_no_graph", {
            "required_span_recall_absolute_delta": interval(0.0, -0.01, 0.02),
        })
        result = DECISIONS.assess(value)
        self.assertEqual(result["overall"], "threshold_not_met")
        self.assertNotIn("equivalent", str(result))

    def test_manual_comparison_never_becomes_repoctx_claim(self):
        value = decision_input("artifact_aware_vs_repoctx_graph", {
            "required_span_recall_absolute_delta": interval(0.50, 0.40, 0.60),
        }, paired=11, eligible=11)
        result = DECISIONS.assess(value)
        self.assertEqual(result["overall"], "descriptive_only")
        self.assertEqual(result["attribution"],
                         "repository_declarations_and_harness_selection")
        self.assertEqual(result["sources"]["run_lock_sha256"], "a" * 64)

    def test_persistence_comparison_reuses_quality_and_efficiency_guards(self):
        value = decision_input("persisted_file_reference_vs_inline", {
            "verified_task_success_absolute_delta": interval(0.0, -0.02, 0.03),
            "input_plus_output_tokens_relative_reduction": interval(0.20, 0.16, 0.25),
            "tool_calls_relative_reduction": interval(0.25, 0.21, 0.30),
            "trial_wall_ms_relative_reduction": interval(0.20, 0.16, 0.25),
        })
        result = DECISIONS.assess(value)
        self.assertEqual(result["overall"], "supported")
        self.assertEqual(result["attribution"], "repoctx_optional_file_reference_adapter")


if __name__ == "__main__":
    unittest.main()
