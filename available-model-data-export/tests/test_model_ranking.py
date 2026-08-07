from __future__ import annotations

import json
import subprocess
import sys
import tempfile
import unittest
from decimal import Decimal
from pathlib import Path


ROOT = Path(__file__).resolve().parent.parent
SCRIPT = ROOT / ".agents/skills/meta-orchestration-model-selection/scripts/rank_models.py"
TOOLS = ROOT / ".github/workflows/update_available_model_data"
sys.path.insert(0, str(SCRIPT.parent))
sys.path.insert(0, str(TOOLS))

import rank_models  # noqa: E402
import generate_scores  # noqa: E402


def row(
    model: str,
    reasoning: str,
    *,
    intelligence: str = "80",
    cost: str = "80",
    speed: str = "80",
    **categories: str,
) -> dict[str, str]:
    result = {
        "model": model,
        "reasoning": reasoning,
        "intelligence_index_score": intelligence,
        "cost_per_intelligence_index_task_usd_score": cost,
        "median_end_to_end_response_time_seconds_score": speed,
    }
    result.update({f"{name}_score": value for name, value in categories.items()})
    return result


class ModelRankingTests(unittest.TestCase):
    def test_all_profiles_have_positive_mandatory_tier_one_weights(self) -> None:
        for profile in rank_models.PROFILES.values():
            self.assertEqual(set(profile.tier1_weights), set(rank_models.TIER1_COLUMNS))
            self.assertTrue(all(value > 0 for value in profile.tier1_weights.values()))

    def test_planning_profile_has_exact_rebalanced_composite_weights(self) -> None:
        profile = rank_models.PROFILES["planning"]
        self.assertEqual(profile.tier2_weights, {"planning_capability": Decimal("5")})
        composite = generate_scores._category_score(
            {
                "reasoning_score": "100",
                "knowledge_score": "80",
                "agentic_tools_score": "60",
                "research_score": "40",
            },
            "planning_capability_score",
            {},
        )
        self.assertEqual(composite, "80")

    def test_orchestration_profile_has_researched_weights_without_double_counting(self) -> None:
        profile = rank_models.PROFILES["orchestration"]
        self.assertEqual(profile.tier1_share, Decimal("60"))
        self.assertEqual(profile.tier2_share, Decimal("40"))
        self.assertEqual(
            profile.tier1_weights,
            {
                "intelligence": Decimal("5"),
                "cost": Decimal("5"),
                "speed": Decimal("4"),
            },
        )
        self.assertEqual(
            profile.tier2_weights,
            {
                "planning_capability": Decimal("5"),
                "instruction_following": Decimal("5"),
            },
        )
        self.assertTrue(
            {"reasoning", "knowledge", "agentic_tools", "research"}.isdisjoint(
                profile.tier2_weights
            )
        )

    def test_alias_variants_are_not_counted_twice_in_a_category(self) -> None:
        composite = generate_scores._category_score(
            {
                "benchmark:Finance Agent": "50",
                "benchmark:FinanceAgent": "100",
                "benchmark:GDPval": "80",
                "benchmark:GDPval-AA": "0",
            },
            "finance_score",
            {"finance": ("Finance Agent", "FinanceAgent", "GDPval", "GDPval-AA")},
        )
        self.assertEqual(composite, "65")

    def test_missing_tier_one_excludes_a_row_without_imputation(self) -> None:
        result = rank_models.rank_models(
            [
                row("Good", "high", software_engineering="90"),
                row("Incomplete", "high", speed="", software_engineering="100"),
            ],
            rank_models.PROFILES["balanced_implementation"],
        )
        self.assertEqual(result["recommendation"]["model"], "Good")
        self.assertEqual(result["excluded"][0]["reasons"], ["missing_tier1:speed"])

    def test_missing_tier_two_warns_and_uses_tier_one(self) -> None:
        result = rank_models.rank_models(
            [row("Only Tier One", "medium")],
            rank_models.PROFILES["simple_action_execution"],
        )
        recommendation = result["recommendation"]
        self.assertEqual(recommendation["total_score"], recommendation["tier1_score"])
        self.assertEqual(recommendation["tier2_contribution"], 0.0)
        self.assertTrue(any("no optional task-category" in warning for warning in recommendation["warnings"]))

    def test_optional_values_are_weighted_and_top_n_is_deterministic(self) -> None:
        profile = rank_models.PROFILES["balanced_implementation"]
        result = rank_models.rank_models(
            [
                row("Beta", "high", intelligence="90", software_engineering="90", instruction_following="90", agentic_tools="90"),
                row("Alpha", "high", intelligence="90", software_engineering="90", instruction_following="90", agentic_tools="90"),
            ],
            profile,
            top_n=1,
        )
        self.assertEqual(result["candidate_count"], 2)
        self.assertEqual(result["recommendation"]["model"], "Alpha")
        self.assertEqual(result["alternatives"], [])

    def test_live_availability_filters_exact_identity_without_substitution(self) -> None:
        result = rank_models.rank_models(
            [
                row("Model A", "high"),
                row("Model A", "low"),
                row("Model B", "high"),
            ],
            rank_models.PROFILES["simple_implementation"],
            available={("Model A", "low")},
        )
        self.assertEqual(
            (result["recommendation"]["model"], result["recommendation"]["reasoning"]),
            ("Model A", "low"),
        )
        self.assertEqual(len(result["excluded"]), 2)
        self.assertTrue(all("not_live_available" in item["reasons"] for item in result["excluded"]))

    def test_custom_json_and_repeated_weights_require_tier_one(self) -> None:
        profile = rank_models.profile_from_json(
            json.dumps(
                {
                    "tier1_share": 70,
                    "tier2_share": 30,
                    "tier1_weights": {"intelligence": 5, "cost": 1, "speed": 1},
                    "tier2_weights": {"research": 5},
                }
            )
        )
        self.assertEqual(profile.tier2_weights, {"research": Decimal("5")})
        nested = rank_models.profile_from_json(
            json.dumps(
                {
                    "tier1": {"share": 70, "intelligence": 5, "cost": 1, "speed": 1},
                    "tier2": {"share": 30, "research": 5},
                }
            )
        )
        self.assertEqual(nested.tier1_share, Decimal("70"))
        self.assertEqual(nested.tier2_weights, {"research": Decimal("5")})
        with self.assertRaisesRegex(rank_models.RankingError, "tier 1 weights must include"):
            rank_models.profile_from_json('{"tier1_weights":{"intelligence":5}}')

    def test_cli_returns_machine_readable_recommendation(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "scores.csv"
            path.write_text(
                "model,reasoning,intelligence_index_score,cost_per_intelligence_index_task_usd_score,median_end_to_end_response_time_seconds_score,software_engineering_score,instruction_following_score,agentic_tools_score\n"
                "Model A,medium,90,90,90,90,90,90\n"
                "Model B,medium,80,80,80,80,80,80\n",
                encoding="utf-8",
            )
            result = subprocess.run(
                [sys.executable, str(SCRIPT), "--scores", str(path), "--profile", "balanced_implementation"],
                capture_output=True,
                text=True,
                check=False,
            )
        self.assertEqual(result.returncode, 0, result.stderr)
        payload = json.loads(result.stdout)
        self.assertEqual(payload["recommendation"]["model"], "Model A")


if __name__ == "__main__":
    unittest.main()
