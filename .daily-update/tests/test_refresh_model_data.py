from __future__ import annotations

import importlib.util
import json
import tempfile
import unittest
from pathlib import Path

SCRIPT = Path(__file__).resolve().parent.parent / "refresh-model-data.py"
SPEC = importlib.util.spec_from_file_location("refresh_model_data", SCRIPT)
assert SPEC is not None and SPEC.loader is not None
refresh_model_data = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(refresh_model_data)


class RefreshModelDataTests(unittest.TestCase):
    def test_default_output_is_the_checked_in_raw_csv(self) -> None:
        self.assertEqual(
            refresh_model_data.DEFAULT_OUTPUT_PATH,
            refresh_model_data.REPOSITORY_ROOT
            / "data"
            / "available_model_raw_values.csv",
        )
        self.assertTrue(refresh_model_data.DEFAULT_OUTPUT_PATH.is_file())

    def test_discovers_all_providers_in_stable_order(self) -> None:
        payload = {
            "zeta": {"id": "zeta", "models": {}},
            "alpha": {"id": "alpha", "models": {}},
        }
        self.assertEqual(
            refresh_model_data.discover_provider_ids(json.dumps(payload)),
            ("alpha", "zeta"),
        )

    def test_discovers_all_benchmarks_deduplicated_in_stable_order(self) -> None:
        payload = {
            "vendor/a": {
                "benchmarks": [{"name": "SWE-bench"}, {"name": "GPQA"}]
            },
            "vendor/b": {
                "benchmarks": [{"name": "GPQA"}, {"name": "Terminal-Bench"}]
            },
        }
        names = refresh_model_data.discover_benchmark_names(json.dumps(payload))
        self.assertTrue(
            {"GPQA", "SWE-bench", "Terminal-Bench"}.issubset(names)
        )
        self.assertIn("Artificial Analysis Coding Agent Index", names)
        self.assertIn("GDPval-AA", names)

    def test_all_provider_parser_accepts_null_default_effort(self) -> None:
        payload = {
            "provider": {
                "id": "provider",
                "models": {
                    "model": {
                        "id": "model",
                        "name": "Model",
                        "reasoning": True,
                        "reasoning_options": [
                            {"type": "effort", "values": [None, "low", "high"]}
                        ],
                    }
                },
            }
        }
        parsed = refresh_model_data.collector.parse_provider_models(
            json.dumps(payload), ("provider",), {}
        )
        self.assertEqual(parsed["provider"][0].reasoning_levels, ("high", "low"))

    def test_generated_configs_select_every_discovered_item(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            provider_path, benchmark_path = refresh_model_data._write_discovered_configs(
                Path(directory),
                ("provider/one", "provider-two"),
                ("Benchmark A", "Benchmark B"),
            )
            providers = refresh_model_data.collector.load_provider_config(provider_path)
            benchmarks = refresh_model_data.collector.load_benchmark_config(benchmark_path)
        self.assertEqual(
            tuple(providers.excluded_models_by_provider),
            ("provider/one", "provider-two"),
        )
        self.assertEqual(benchmarks.selected_benchmarks, ("Benchmark A", "Benchmark B"))


if __name__ == "__main__":
    unittest.main()
