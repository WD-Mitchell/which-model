from __future__ import annotations

import json
import io
import subprocess
import sys
import tempfile
import unittest
from decimal import Decimal
from pathlib import Path
from unittest import mock


ROOT = Path(__file__).resolve().parent.parent
TOOLS = ROOT / ".github/workflows/update_available_model_data"
sys.path.insert(0, str(TOOLS))

import get_aa_api_values as aa_api
import get_aa_page_values as aa_page
import get_benchmarks as benchmarks
import get_provider_models as providers
import generate_scores as scores
import model_config
import update_raw_values as updater
from model_types import ModelFamily, ProviderModel, SelectedModel, UpdateError


class RecordingClient:
    def __init__(self, responses: dict[str, str]) -> None:
        self.responses = responses
        self.calls: list[tuple[str, dict[str, str]]] = []

    def get_text(
        self, url: str, *, headers: dict[str, str] | None = None, purpose: str
    ) -> str:
        self.calls.append((url, dict(headers or {})))
        if url not in self.responses:
            raise UpdateError(f"unexpected endpoint: {url}")
        return self.responses[url]


class SourceBoundaryTests(unittest.TestCase):
    def test_source_urls_are_distinct_and_current(self) -> None:
        self.assertEqual(providers.MODELS_DEV_PROVIDER_URL, "https://models.dev/api.json")
        self.assertEqual(benchmarks.MODELS_DEV_BENCHMARK_URL, "https://models.dev/models.json")
        self.assertNotEqual(providers.MODELS_DEV_PROVIDER_URL, benchmarks.MODELS_DEV_BENCHMARK_URL)

    def test_provider_collection_is_one_unauthenticated_request(self) -> None:
        payload = {
            "openai": {
                "id": "openai",
                "models": {
                    "gpt": {
                        "id": "gpt",
                        "name": "GPT",
                        "reasoning": True,
                        "reasoning_options": [
                            {"type": "effort", "values": ["low", "high"]},
                            {"type": "budget_tokens", "min": 1, "max": 2},
                        ],
                    }
                },
            }
        }
        client = RecordingClient({providers.MODELS_DEV_PROVIDER_URL: json.dumps(payload)})
        result = providers.fetch_provider_models(client, ["openai"], {"openai": ()})
        self.assertEqual(result["openai"][0].reasoning_levels, ("low", "high"))
        self.assertEqual(client.calls, [(providers.MODELS_DEV_PROVIDER_URL, {})])

    def test_benchmark_collection_is_one_unauthenticated_request(self) -> None:
        payload = {
            "openai/gpt": {
                "id": "openai/gpt",
                "name": "GPT",
                "benchmarks": [
                    {"name": "SWE-Bench Pro", "score": 52.4, "variant": "reasoning effort xhigh"}
                ],
            }
        }
        client = RecordingClient({benchmarks.MODELS_DEV_BENCHMARK_URL: json.dumps(payload)})
        result = benchmarks.fetch_and_attach_benchmarks(
            client,
            {"openai": [ProviderModel("openai", "gpt", "GPT", ("high", "xhigh"))]},
            ["SWE-Bench Pro"],
        )
        self.assertEqual(
            dict(dict(result["openai"][0].benchmark_overrides)["xhigh"])["SWE-Bench Pro"],
            Decimal("52.4"),
        )
        self.assertEqual(client.calls, [(benchmarks.MODELS_DEV_BENCHMARK_URL, {})])

    def test_benchmark_model_name_matching_ignores_provider_annotations(self) -> None:
        payload = {
            "vendor/claude-opus": {
                "id": "vendor/claude-opus",
                "name": "Claude Opus 4.5 (latest)",
                "benchmarks": [
                    {"name": "SWE-Bench Pro", "score": 52.4}
                ],
            }
        }
        result = benchmarks.parse_and_attach_benchmarks(
            json.dumps(payload),
            {
                "anthropic": [
                    ProviderModel(
                        "anthropic",
                        "provider-opus",
                        "Claude Opus 4.5 [claude-opus-4-5-20251101]",
                        ("high",),
                    )
                ]
            },
            ["SWE-Bench Pro"],
        )
        self.assertEqual(
            result["anthropic"][0].canonical_id,
            "vendor/claude-opus",
        )

    def test_benchmark_cli_requests_only_models_json_without_authentication(self) -> None:
        payload = {
            "openai/gpt": {
                "id": "openai/gpt",
                "name": "GPT",
                "benchmarks": [{"name": "SWE-Bench Pro", "score": 51.25}],
            }
        }
        client = RecordingClient({
            benchmarks.MODELS_DEV_BENCHMARK_URL: json.dumps(payload)
        })
        with tempfile.TemporaryDirectory() as directory:
            config = Path(directory) / "benchmarks.toml"
            config.write_text(
                '[benchmark_selection]\n'
                'groups = ["coding"]\n'
                'benchmarks = []\n\n'
                '[benchmark_groups.coding]\n'
                'benchmarks = ["SWE-Bench Pro"]\n',
                encoding="utf-8",
            )
            output = io.StringIO()
            with mock.patch.object(benchmarks, "HttpClient", return_value=client), mock.patch(
                "sys.stdout", output
            ):
                status = benchmarks.main(["--benchmark-config", str(config)])
        self.assertEqual(status, 0)
        self.assertEqual(client.calls, [(benchmarks.MODELS_DEV_BENCHMARK_URL, {})])
        self.assertEqual(
            json.loads(output.getvalue())["openai/gpt"]["benchmarks"],
            {"SWE-Bench Pro": "51.25"},
        )

    def test_artificial_analysis_is_the_only_authenticated_source(self) -> None:
        response = json.dumps({
            "pagination": {"page": 1, "total_pages": 1, "has_more": False},
            "data": [],
        })
        client = RecordingClient({f"{aa_api.API_URL}?page=1": response})
        self.assertEqual(aa_api.fetch_all_models(client, "secret"), [])
        self.assertEqual(client.calls, [(f"{aa_api.API_URL}?page=1", {"x-api-key": "secret"})])

    def test_artificial_analysis_falls_back_to_free_endpoint_only_when_forbidden(self) -> None:
        response = json.dumps({
            "pagination": {"page": 1, "total_pages": 1, "has_more": False},
            "data": [],
        })

        class ForbiddenThenFree(RecordingClient):
            def get_text(
                self, url: str, *, headers: dict[str, str] | None = None, purpose: str
            ) -> str:
                self.calls.append((url, dict(headers or {})))
                if url == f"{aa_api.API_URL}?page=1":
                    raise UpdateError("failed to fetch Artificial Analysis model list page 1: API access was forbidden for this key (HTTP 403)")
                if url == f"{aa_api.FREE_API_URL}?page=1":
                    return response
                raise UpdateError(f"unexpected endpoint: {url}")

        client = ForbiddenThenFree({})
        self.assertEqual(aa_api.fetch_all_models(client, "secret"), [])
        self.assertEqual(
            client.calls,
            [
                (f"{aa_api.API_URL}?page=1", {"x-api-key": "secret"}),
                (f"{aa_api.FREE_API_URL}?page=1", {"x-api-key": "secret"}),
            ],
        )

    def test_api_cli_requests_only_authenticated_v2_api(self) -> None:
        response = json.dumps({
            "pagination": {"page": 1, "total_pages": 1, "has_more": False},
            "data": [],
        })
        client = RecordingClient({f"{aa_api.API_URL}?page=1": response})
        with tempfile.TemporaryDirectory() as directory:
            env_file = Path(directory) / ".env"
            env_file.write_text("ARTIFICIAL_ANALYSIS_API=test-secret\n", encoding="utf-8")
            output = io.StringIO()
            with mock.patch.object(aa_api, "HttpClient", return_value=client), mock.patch(
                "sys.stdout", output
            ):
                status = aa_api.main(["--env-file", str(env_file)])
        self.assertEqual(status, 0)
        self.assertEqual(
            client.calls,
            [(f"{aa_api.API_URL}?page=1", {"x-api-key": "test-secret"})],
        )
        self.assertNotIn("test-secret", output.getvalue())

    def test_page_cli_requests_only_public_slug_pages_without_headers(self) -> None:
        slug = "example-model"
        page = (
            '<script>payload={\\"currentModel\\":{\\"slug\\":\\"example-model\\",'
            '\\"intelligenceIndexTimePerTask\\":12.5,\\"other\\":true}}</script>'
        )
        url = aa_page.MODEL_PAGE_URL.format(slug=slug)
        client = RecordingClient({url: page})
        output = io.StringIO()
        with mock.patch.object(aa_page, "HttpClient", return_value=client), mock.patch(
            "sys.stdout", output
        ):
            status = aa_page.main(["--slug", slug])
        self.assertEqual(status, 0)
        self.assertEqual(client.calls, [(url, {})])
        self.assertEqual(
            json.loads(output.getvalue())[slug][
                "time_per_intelligence_index_task_seconds"
            ],
            "12.5",
        )

    def test_default_row_collection_never_calls_public_page_collector(self) -> None:
        selected = SelectedModel(
            ModelFamily("Example", "example"), "high", "example-high",
            Decimal("50"), Decimal("0.5"),
        )
        with mock.patch.object(
            updater,
            "fetch_public_page_metrics",
            side_effect=AssertionError("page collector called on default path"),
        ):
            rows = updater.collect_rows(RecordingClient({}), [selected])
        self.assertEqual(rows[0].time_seconds, None)
        self.assertEqual(rows[0].cost, Decimal("0.5"))

    def test_malformed_payloads_fail_at_their_own_boundary(self) -> None:
        with self.assertRaisesRegex(UpdateError, "provider catalogue"):
            providers.parse_provider_models("[]", ["openai"], {"openai": ()})
        with self.assertRaisesRegex(UpdateError, "benchmark catalogue"):
            benchmarks.parse_and_attach_benchmarks("[]", {"openai": []}, ["SWE"])

    def test_each_source_script_has_a_direct_json_cli(self) -> None:
        for script in (
            "get_provider_models.py",
            "get_benchmarks.py",
            "get_aa_api_values.py",
            "get_aa_page_values.py",
            "update_raw_values.py",
            "generate_scores.py",
        ):
            with self.subTest(script=script):
                result = subprocess.run(
                    [sys.executable, str(TOOLS / script), "--help"],
                    cwd=ROOT,
                    text=True,
                    capture_output=True,
                    check=False,
                )
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertIn("usage:", result.stdout)

    def test_workflow_references_existing_renamed_entry_points(self) -> None:
        workflow = (ROOT / ".github/workflows/update-available-model-data.yml").read_text(
            encoding="utf-8"
        )
        expected = (
            ".github/workflows/update_available_model_data/update_raw_values.py",
            ".github/workflows/update_available_model_data/generate_scores.py",
        )
        for relative in expected:
            self.assertIn(f"python3 {relative}", workflow)
            self.assertTrue((ROOT / relative).is_file())
        self.assertNotIn("python3 scripts/", workflow)
        self.assertNotIn("--add aa_page", workflow)
        self.assertIn(
            "git add -- available_model_raw_values.csv "
            ".centree-agentic-framework/available_model_scores.csv",
            workflow,
        )

    def test_score_csv_is_tracked_scope_while_user_memory_remains_ignored(self) -> None:
        score = ".centree-agentic-framework/available_model_scores.csv"
        score_result = subprocess.run(
            ["git", "check-ignore", "-q", score], cwd=ROOT, check=False
        )
        memory_result = subprocess.run(
            [
                "git",
                "check-ignore",
                "-q",
                ".centree-agentic-framework/user-memory/ignore-probe.md",
            ],
            cwd=ROOT,
            check=False,
        )
        self.assertEqual(score_result.returncode, 1)
        self.assertEqual(memory_result.returncode, 0)

    def test_old_scripts_tree_and_long_entry_point_names_are_absent(self) -> None:
        self.assertFalse((ROOT / "scripts").exists())
        old_names = {
            "get_models_dev_provider_models.py",
            "get_models_dev_benchmarks.py",
            "get_artificial_analysis_api_values.py",
            "get_artificial_analysis_page_values.py",
            "update_available_model_raw_values.py",
            "generate_available_model_scores.py",
        }
        self.assertTrue(old_names.isdisjoint(path.name for path in TOOLS.iterdir()))

    def test_default_paths_resolve_to_repository_root(self) -> None:
        self.assertEqual(model_config.DEFAULT_BENCHMARK_CONFIG_PATH, ROOT / "benchmarks.toml")
        self.assertEqual(model_config.DEFAULT_PROVIDER_CONFIG_PATH, ROOT / "providers.toml")
        self.assertEqual(aa_api.DEFAULT_ENV_PATH, ROOT / ".env")
        self.assertEqual(updater.DEFAULT_OUTPUT_PATH, ROOT / "available_model_raw_values.csv")
        self.assertEqual(scores.DEFAULT_INPUT, ROOT / "available_model_raw_values.csv")
        self.assertEqual(
            scores.DEFAULT_OUTPUT,
            ROOT / ".centree-agentic-framework/available_model_scores.csv",
        )

    def test_artificial_analysis_modules_are_source_pure_and_independent(self) -> None:
        api_source = (TOOLS / "get_aa_api_values.py").read_text(encoding="utf-8")
        page_source = (TOOLS / "get_aa_page_values.py").read_text(encoding="utf-8")
        self.assertNotIn("get_models_dev", api_source)
        self.assertNotIn("https://models.dev", api_source)
        self.assertNotIn("get_aa_page_values", api_source)
        self.assertNotIn("MODEL_PAGE_URL", api_source)
        self.assertNotIn("get_aa_api_values", page_source)
        self.assertNotIn("API_KEY", page_source)
        self.assertNotIn("DEFAULT_ENV", page_source)
        self.assertNotIn("x-api-key", page_source)

    def test_benchmark_module_has_no_provider_collector_dependency(self) -> None:
        source = (TOOLS / "get_benchmarks.py").read_text(encoding="utf-8")
        self.assertNotIn("get_provider_models", source)
        self.assertNotIn("MODELS_DEV_PROVIDER_URL", source)


if __name__ == "__main__":
    unittest.main()
