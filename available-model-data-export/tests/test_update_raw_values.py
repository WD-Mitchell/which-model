from __future__ import annotations

import email.message
import http.server
import importlib.util
import io
import json
import os
import ssl
import sys
import tempfile
import threading
import unittest
import urllib.error
import urllib.request
from datetime import datetime, timezone
from decimal import Decimal
from pathlib import Path
from unittest import mock


REPOSITORY_ROOT = Path(__file__).resolve().parent.parent
SCRIPT = (
    REPOSITORY_ROOT
    / ".github/workflows/update_available_model_data/update_raw_values.py"
)
SPEC = importlib.util.spec_from_file_location("update_raw_values", SCRIPT)
assert SPEC is not None and SPEC.loader is not None
updater = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = updater
SPEC.loader.exec_module(updater)

TEST_MODEL_FAMILIES = (
    updater.ModelFamily("GPT-5.6 Sol", "gpt-5-6-sol"),
    updater.ModelFamily("GPT-5.6 Terra", "gpt-5-6-terra"),
    updater.ModelFamily("GPT-5.6 Luna", "gpt-5-6-luna"),
    updater.ModelFamily("Claude Opus 5", "claude-opus-5"),
    updater.ModelFamily("Claude Sonnet 5", "claude-sonnet-5"),
    updater.ModelFamily("Claude Fable 5", "claude-fable-5"),
    updater.ModelFamily("Gemini 3.6 Flash", "gemini-3-6-flash"),
)
TEST_BASE_REASONING = {
    "GPT-5.6 Sol": "max",
    "GPT-5.6 Terra": "max",
    "GPT-5.6 Luna": "max",
    "Claude Opus 5": "max",
    "Claude Sonnet 5": "max",
    "Claude Fable 5": "max",
    "Gemini 3.6 Flash": "high",
}
CONFIG_PREFIX = (
    '[benchmark_selection]\ngroups = ["test"]\nbenchmarks = []\n\n'
    '[benchmark_groups.test]\nbenchmarks = ["SWE-Bench Verified"]\n\n'
)


def model(
    family: updater.ModelFamily,
    *,
    slug: str | None = None,
    name: str | None = None,
    intelligence: Decimal | None = Decimal("42.04"),
    cost: Decimal = Decimal("0.125"),
    median_response_time: Decimal | None = Decimal("6.5"),
    coding_index: Decimal | None = Decimal("37.25"),
    agentic_index: Decimal | None = Decimal("28.75"),
    reasoning_model: bool | None = None,
    include_cost_data: bool = True,
) -> dict[str, object]:
    default_configuration = {
        "Claude Fable 5": "Adaptive Reasoning, Max Effort, Opus 4.8 Fallback",
        "Gemini 3.6 Flash": "high",
    }.get(family.name, "max")
    result: dict[str, object] = {
        "name": (
            f"{family.name} ({default_configuration})" if name is None else name
        ),
        "slug": family.base_slug if slug is None else slug,
        "evaluations": {
            "artificial_analysis_intelligence_index": intelligence,
            "artificial_analysis_coding_index": coding_index,
            "artificial_analysis_agentic_index": agentic_index,
        },
        "performance": {
            "median_end_to_end_response_time_seconds": median_response_time
        },
    }
    if include_cost_data:
        result["artificial_analysis_intelligence_index_cost"] = {
            "cost_per_task": {"total_cost": cost}
        }
    if reasoning_model is not None:
        result["reasoning_model"] = reasoning_model
    return result


def page(
    slug: str, seconds: str | None = "12.5", cost: str | None = None
) -> str:
    time_fragment = (
        ""
        if seconds is None
        else ',\\"intelligenceIndexTimePerTask\\":' + seconds
    )
    cost_fragment = (
        ""
        if cost is None
        else ',\\"intelligenceIndexCostPerTask\\":{\\"cost\\":{\\"total\\":'
        + cost
        + "}}"
    )
    return (
        '<script>payload={\\"currentModel\\":{\\"slug\\":\\"'
        + slug
        + '\\"'
        + time_fragment
        + cost_fragment
        + ',\\"other\\":true}}</script>'
    )


def complete_current_rows() -> list[updater.RawRow]:
    return [
        updater.RawRow(
            family.name,
            TEST_BASE_REASONING[family.name],
            Decimal("1.1") + index,
            Decimal("20") + index,
            Decimal("0.10") + Decimal(index) / 100,
            Decimal("61") + index,
            Decimal("62") + index,
            Decimal("63") + index,
            Decimal("64") + index,
            Decimal("10") + index,
            Decimal("30") + index,
            Decimal("40") + index,
            benchmark_values={
                "SWE-Bench Verified": Decimal("61") + index,
                "Terminal-Bench": Decimal("63") + index,
            },
        )
        for index, family in enumerate(TEST_MODEL_FAMILIES)
    ]


def api_response() -> str:
    return json.dumps(
        {
            "pagination": {"page": 1, "total_pages": 1, "has_more": False},
            "data": [model(family) for family in TEST_MODEL_FAMILIES],
        },
        default=str,
    )


def models_dev_response() -> str:
    provider_families = {
        "anthropic": TEST_MODEL_FAMILIES[3:6],
        "openai": TEST_MODEL_FAMILIES[:3],
        "github-copilot": TEST_MODEL_FAMILIES[6:],
    }
    providers = {
            provider: {
                "id": provider,
                "models": {
                    family.base_slug: {
                        "id": family.base_slug,
                        "name": family.name,
                        "reasoning": True,
                        "reasoning_options": [
                            {
                                "type": "effort",
                                "values": [
                                    TEST_BASE_REASONING[family.name].split(" ", 1)[0]
                                ],
                            }
                        ],
                        "base_model": f"fixture/{family.base_slug}",
                    }
                    for family in families
                },
            }
            for provider, families in provider_families.items()
        }
    generic_models = {
        f"fixture/{family.base_slug}": {
            "id": f"fixture/{family.base_slug}",
            "name": family.name,
            "benchmarks": [],
        }
        for family in TEST_MODEL_FAMILIES
    }
    return json.dumps(
        {"providers": providers, "models": generic_models}
    )


def models_dev_provider_response() -> str:
    return json.dumps(json.loads(models_dev_response())["providers"])


def models_dev_benchmark_response() -> str:
    return json.dumps(json.loads(models_dev_response())["models"])


def catalogue_model(
    model_id: str,
    name: str,
    *,
    efforts: list[str] | None = None,
    reasoning: bool = True,
    status: str | None = None,
) -> dict[str, object]:
    result: dict[str, object] = {
        "id": model_id,
        "name": name,
        "reasoning": reasoning,
    }
    if efforts is not None:
        result["reasoning_options"] = [
            {"type": "effort", "values": efforts},
            {"type": "budget_tokens", "min": 256, "max": 8192},
        ]
    if status is not None:
        result["status"] = status
    return result


class RecordingClient:
    def __init__(self, responses: dict[str, str]) -> None:
        self.responses = responses
        self.calls: list[tuple[str, dict[str, str], str]] = []

    def get_text(
        self, url: str, *, headers: dict[str, str] | None = None, purpose: str
    ) -> str:
        self.calls.append((url, dict(headers or {}), purpose))
        if url not in self.responses:
            raise updater.UpdateError(f"unexpected URL: {url}")
        return self.responses[url]


class FakeResponse:
    def __init__(self, body: bytes) -> None:
        self.body = body
        self.headers = email.message.Message()
        self.headers["Content-Type"] = "application/json; charset=utf-8"

    def __enter__(self) -> "FakeResponse":
        return self

    def __exit__(self, *args: object) -> None:
        return None

    def read(self) -> bytes:
        return self.body


class UpdateAvailableModelRawValuesTests(unittest.TestCase):
    def test_checked_in_provider_config_has_exact_verified_memberships(self) -> None:
        provider_config = updater.load_provider_config()
        benchmark_config = updater.load_benchmark_config()
        self.assertTrue(
            updater.DEFAULT_BENCHMARK_CONFIG_PATH.read_text(encoding="utf-8").startswith(
                "[benchmark_selection]"
            )
        )
        self.assertEqual(set(updater.DEFAULT_PROVIDER_CONFIG_PATH.read_text(encoding="utf-8").splitlines()[0:1]), {"[providers.anthropic]"})
        self.assertIn("software_engineering", benchmark_config.benchmark_groups)
        self.assertIn("finance", benchmark_config.benchmark_groups)
        for group in (
            "reasoning",
            "knowledge",
            "research",
            "instruction_following",
            "agentic_tools",
            "evidence_capture",
            "ui_visual",
            "security",
            "data_ml",
            "finance",
        ):
            self.assertIn(group, benchmark_config.benchmark_groups)
            self.assertTrue(
                set(benchmark_config.benchmark_groups[group]).issubset(
                    set(benchmark_config.selected_benchmarks)
                )
            )
        self.assertEqual(
            benchmark_config.selected_benchmarks[: len(
                benchmark_config.benchmark_groups["software_engineering"]
            )],
            benchmark_config.benchmark_groups["software_engineering"],
        )
        self.assertEqual(
            list(provider_config.excluded_models_by_provider),
            ["anthropic", "openai", "github-copilot"],
        )
        self.assertEqual(
            provider_config.excluded_models_by_provider["github-copilot"],
            ("grok-4.5",),
        )
        self.assertTrue(
            all(
                not provider_config.excluded_models_by_provider[provider]
                for provider in ("anthropic", "openai")
            )
        )

    def test_benchmark_groups_custom_direct_order_and_deduplication(self) -> None:
        content = (
            '[benchmark_selection]\ngroups = ["custom", "second"]\n'
            'benchmarks = ["Direct, τ³", "Shared"]\n\n'
            '[benchmark_groups.custom]\nbenchmarks = ["First", "Shared"]\n\n'
            '[benchmark_groups.second]\nbenchmarks = ["Shared", "Last"]\n'
        )
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "config.toml"
            path.write_text(content, encoding="utf-8")
            config = updater.load_benchmark_config(path)
        self.assertEqual(
            config.selected_benchmarks,
            ("First", "Shared", "Last", "Direct, τ³"),
        )

    def test_benchmark_config_rejects_unknown_groups_and_invalid_entries(self) -> None:
        cases = (
            CONFIG_PREFIX + "[providers.gateway]\nexcluded_models = []\n",
            CONFIG_PREFIX.replace('groups = ["test"]', 'groups = ["missing"]'),
            CONFIG_PREFIX.replace(
                'benchmarks = ["SWE-Bench Verified"]',
                'benchmarks = ["", "SWE-Bench Verified"]',
            ),
            CONFIG_PREFIX.replace(
                'benchmarks = ["SWE-Bench Verified"]',
                'benchmarks = ["SWE-Bench Verified", "SWE-Bench Verified"]',
            ),
            CONFIG_PREFIX.replace(
                '[benchmark_groups.test]',
                '[benchmark_groups.test]\nunknown = true',
            ),
        )
        for content in cases:
            with self.subTest(content=content), tempfile.TemporaryDirectory() as directory:
                path = Path(directory) / "config.toml"
                path.write_text(content, encoding="utf-8")
                with self.assertRaises(updater.UpdateError):
                    updater.load_benchmark_config(path)

    def test_missing_configs_report_their_independent_owners(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            missing = Path(directory) / "missing.toml"
            with self.assertRaisesRegex(updater.UpdateError, "cannot read benchmark config"):
                updater.load_benchmark_config(missing)
            with self.assertRaisesRegex(updater.UpdateError, "cannot read provider config"):
                updater.load_provider_config(missing)

    def test_provider_union_is_deduplicated_and_deterministic(self) -> None:
        config = updater.load_provider_config()
        expected = ("anthropic", "openai", "github-copilot")
        self.assertEqual(updater.resolve_provider_ids(config), expected)
        self.assertEqual(
            updater.resolve_provider_ids(
                config,
                ["github-copilot", "anthropic", "github-copilot", "openai"],
            ),
            ("github-copilot", "anthropic", "openai"),
        )
        with self.assertRaisesRegex(updater.UpdateError, "unknown access providers"):
            updater.resolve_provider_ids(config, ["codex"])
        with self.assertRaisesRegex(updater.UpdateError, "at least one"):
            updater.resolve_provider_ids(config, [])

    def test_missing_or_empty_exclusions_mean_all_known_model_families(self) -> None:
        cases = (
            "[providers.everything]\n",
            "[providers.everything]\nexcluded_models = []\n",
        )
        for content in cases:
            with self.subTest(content=content), tempfile.TemporaryDirectory() as directory:
                path = Path(directory) / "providers.toml"
                path.write_text(content, encoding="utf-8")
                config = updater.load_provider_config(path)
                self.assertEqual(
                    config.excluded_models_by_provider["everything"],
                    (),
                )
                self.assertEqual(
                    updater.resolve_provider_ids(config),
                    ("everything",),
                )

    def test_provider_sections_themselves_determine_default_selection(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "providers.toml"
            path.write_text(
                "[providers.claude_only]\n"
                "excluded_models = [\n"
                '  "gpt-5-6-sol", "gpt-5-6-terra", "gpt-5-6-luna",\n'
                '  "gemini-3-6-flash",\n'
                "]\n",
                encoding="utf-8",
            )
            config = updater.load_provider_config(path)
        self.assertEqual(
            updater.resolve_provider_ids(config),
            ("claude_only",),
        )

    def test_provider_config_rejects_invalid_shape_keys_and_exclusions(self) -> None:
        cases = {
            "wrong root": "[other.example]\n",
            "empty providers": "[providers]\n",
            "provider not table": '[providers]\nanthropic = "bad"\n',
            "unknown key": "[providers.anthropic]\nmodels = []\n",
            "wrong exclusion type": (
                '[providers.anthropic]\nexcluded_models = "gpt-5-6-sol"\n'
            ),
            "non-string exclusion": (
                "[providers.anthropic]\nexcluded_models = [1]\n"
            ),
            "duplicate exclusion": (
                "[providers.anthropic]\n"
                'excluded_models = ["gpt-5-6-sol", "gpt-5-6-sol"]\n'
            ),
            "unknown top-level key": (
                "version = 1\n[providers.anthropic]\n"
            ),
        }
        for name, content in cases.items():
            with self.subTest(name=name), tempfile.TemporaryDirectory() as directory:
                path = Path(directory) / "providers.toml"
                path.write_text(content, encoding="utf-8")
                with self.assertRaises(updater.UpdateError):
                    updater.load_provider_config(path)

    def test_list_providers_uses_config_without_update_or_network(self) -> None:
        output = io.StringIO()
        with mock.patch.object(updater, "update") as update, mock.patch(
            "sys.stdout", output
        ):
            self.assertEqual(
                updater.main(
                    ["--list-providers", "--benchmark-config", "/missing/benchmarks.toml"]
                ),
                0,
            )
        update.assert_not_called()
        lines = output.getvalue().splitlines()
        self.assertEqual([line.split(":", 1)[0] for line in lines], [
            "anthropic",
            "openai",
            "github-copilot",
        ])

    def test_models_dev_generic_provider_efforts_exclusions_and_deprecated(self) -> None:
        providers = {
            "future/provider": {
                "id": "future/provider",
                "models": {
                    "nova:1": catalogue_model(
                        "nova:1", "Nova 1", efforts=["low", "high"]
                    ),
                    "plain.model": catalogue_model(
                        "plain.model", "Plain Model", reasoning=False
                    ),
                    "old": catalogue_model(
                        "old", "Old Model", status="deprecated"
                    ),
                },
            }
        }
        payload = {
            "providers": providers,
            "models": {
                "fixture/dummy": {
                    "id": "fixture/dummy", "name": "Dummy", "benchmarks": []
                }
            },
        }
        reports: list[str] = []
        discovered = updater.parse_models_dev_catalogue(
            json.dumps(payload),
            ["future/provider"],
            {"future/provider": ["plain.model", "no-longer-listed"]},
            reporter=reports.append,
        )
        self.assertEqual(
            discovered,
            {
                "future/provider": [
                    updater.ProviderModel(
                        "future/provider", "nova:1", "Nova 1", ("low", "high")
                    )
                ]
            },
        )
        self.assertTrue(any("no-longer-listed" in report for report in reports))
        self.assertFalse(any("budget" in level for level in discovered["future/provider"][0].reasoning_levels))

    def test_models_dev_sources_are_each_fetched_once_without_credentials(self) -> None:
        client = RecordingClient({
            updater.MODELS_DEV_PROVIDER_URL: models_dev_provider_response(),
            updater.MODELS_DEV_BENCHMARK_URL: models_dev_benchmark_response(),
        })
        discovered = updater.fetch_models_dev_catalogue(
            client,
            ["anthropic", "openai", "github-copilot"],
            {"anthropic": (), "openai": (), "github-copilot": ()},
        )
        self.assertEqual(len(client.calls), 2)
        self.assertEqual(
            [call[0] for call in client.calls],
            [updater.MODELS_DEV_PROVIDER_URL, updater.MODELS_DEV_BENCHMARK_URL],
        )
        self.assertTrue(all(call[1] == {} for call in client.calls))
        self.assertTrue(all(discovered.values()))

    def test_models_dev_warns_when_selected_benchmark_is_absent(self) -> None:
        reports: list[str] = []
        updater.parse_models_dev_catalogue(
            models_dev_response(),
            ["openai"],
            {"openai": ()},
            ["No Longer Published"],
            reporter=reports.append,
        )
        self.assertTrue(any("No Longer Published" in report for report in reports))

    def test_models_dev_rejects_unknown_provider_and_malformed_records(self) -> None:
        valid_providers = {
            "provider": {
                "id": "provider",
                "models": {"nova": catalogue_model("nova", "Nova")},
            }
        }
        generic = {
            "fixture/dummy": {
                "id": "fixture/dummy", "name": "Dummy", "benchmarks": []
            }
        }
        valid = {"providers": valid_providers, "models": generic}
        cases = (
            ("not-json", "invalid JSON"),
            (json.dumps([]), "models and providers"),
            (json.dumps(valid), "no valid provider"),
            (
                json.dumps({
                    "providers": {"provider": {"id": "wrong", "models": {}}},
                    "models": generic,
                }),
                "mismatched id",
            ),
            (
                json.dumps({
                    "providers": {"provider": {"id": "provider", "models": {}}},
                    "models": generic,
                }),
                "no valid models mapping",
            ),
            (
                json.dumps(
                    {
                        "providers": {
                            "provider": {
                                "id": "provider",
                                "models": {"nova": catalogue_model("different", "Nova")},
                            }
                        },
                        "models": generic,
                    }
                ),
                "invalid record",
            ),
        )
        for content, expected in cases:
            providers = ["missing"] if expected == "no valid provider" else ["provider"]
            with self.subTest(expected=expected), self.assertRaisesRegex(
                updater.UpdateError, expected
            ):
                updater.parse_models_dev_catalogue(content, providers, {})

    def test_models_dev_rejects_invalid_effort_options(self) -> None:
        invalid_options = (
            "bad",
            [{"type": "effort", "values": []}],
            [{"type": "effort", "values": ["high", "high"]}],
            [{"type": "effort", "values": ["turbo"]}],
            [
                {"type": "effort", "values": ["high"]},
                {"type": "effort", "values": ["low"]},
            ],
        )
        for options in invalid_options:
            model_record = catalogue_model("nova", "Nova")
            model_record["reasoning_options"] = options
            payload = {
                "providers": {
                    "provider": {
                        "id": "provider",
                        "models": {"nova": model_record},
                    }
                },
                "models": {
                    "fixture/dummy": {
                        "id": "fixture/dummy", "name": "Dummy", "benchmarks": []
                    }
                },
            }
            with self.subTest(options=options), self.assertRaises(updater.UpdateError):
                updater.parse_models_dev_catalogue(
                    json.dumps(payload), ["provider"], {"provider": ()}
                )

    def test_model_name_annotations_are_removed_and_duplicate_provider_names_merge(self) -> None:
        payload = {
            "provider": {
                "id": "provider",
                "models": {
                    "opus-dated": catalogue_model(
                        "opus-dated",
                        "Claude Opus 4.5 [claude-opus-4-5-20251101]",
                        efforts=["high"],
                    ),
                    "opus-latest": catalogue_model(
                        "opus-latest",
                        "Claude Opus 4.5 (latest)",
                        efforts=["low"],
                    ),
                },
            }
        }
        discovered = updater.parse_provider_models(
            json.dumps(payload), ["provider"], {"provider": ()}
        )
        self.assertEqual(
            [model.display_name for model in discovered["provider"]],
            ["Claude Opus 4.5", "Claude Opus 4.5"],
        )
        matched = updater.match_provider_models([], discovered)
        self.assertEqual(
            [family.name for family in matched.families], ["Claude Opus 4.5"]
        )
        self.assertEqual(
            [(model.family.name, model.reasoning) for model in matched.selected],
            [("Claude Opus 4.5", "low"), ("Claude Opus 4.5", "high")],
        )

    def test_existing_annotated_names_are_normalized_and_merged(self) -> None:
        content = updater.render_csv(
            [
                updater.RawRow(
                    "Claude Opus 4.5 [claude-opus-4-5-20251101]",
                    "default",
                    Decimal("10"),
                    None,
                    None,
                    benchmark_values={"Humanity's Last Exam": Decimal("61")},
                ),
                updater.RawRow(
                    "Claude Opus 4.5 (latest)",
                    "high",
                    Decimal("11"),
                    None,
                    None,
                    benchmark_values={"Humanity's Last Exam": Decimal("64")},
                ),
            ],
            ["Humanity's Last Exam"],
        )
        rows = updater.parse_existing_csv(content, source=Path("fixture.csv"))
        self.assertEqual(
            [(row.model, row.reasoning) for row in rows],
            [("Claude Opus 4.5", "high")],
        )
        self.assertEqual(rows[0].intelligence, Decimal("11"))
        self.assertEqual(
            rows[0].benchmark_values["Humanity's Last Exam"], Decimal("64")
        )

    def test_models_dev_extracts_only_exact_target_benchmarks(self) -> None:
        defaults, overrides = updater.extract_selected_models_dev_benchmarks(
            "vendor/nova",
            {
                "benchmarks": [
                    {"name": "SWE-Bench Pro", "score": 99},
                    {"name": "SWE-Bench Verified", "score": 76.44},
                    {"name": "GPQA", "score": 80},
                    {"name": "GPQA Diamond", "score": 88.4},
                    {"name": "Terminal-Bench", "version": "2.0", "score": 50},
                    {"name": "Terminal-Bench", "version": "2.1", "score": 52.5},
                    {"name": "Terminal-Bench 2.1", "score": 100},
                    {"name": "AIME", "version": "2025", "score": 100},
                    {"name": "AIME", "version": "2026", "score": 93.33},
                ]
            },
            ["SWE-Bench Verified", "GPQA Diamond", "Terminal-Bench", "AIME"],
        )
        self.assertEqual(
            dict(defaults),
            {
                "SWE-Bench Verified": Decimal("76.44"),
                "GPQA Diamond": Decimal("88.4"),
                "Terminal-Bench": Decimal("52.5"),
                "AIME": Decimal("100"),
            },
        )
        self.assertEqual(overrides, ())

    def test_models_dev_rejects_malformed_and_conflicting_target_benchmarks(self) -> None:
        cases = (
            ([{"name": "GPQA Diamond", "score": "bad"}], "must be numeric"),
            ([{"name": "Terminal-Bench", "version": "latest", "score": 50}], "invalid version"),
        )
        for benchmarks, expected in cases:
            with self.subTest(expected=expected), self.assertRaisesRegex(
                updater.UpdateError, expected
            ):
                updater.extract_selected_models_dev_benchmarks(
                    "vendor/nova",
                    {"benchmarks": benchmarks},
                    [benchmarks[0]["name"]],
                )

        defaults, overrides = updater.extract_selected_models_dev_benchmarks(
            "vendor/nova",
            {
                "benchmarks": [
                    {"name": "Terminal-Bench", "version": "2.1", "harness": "one", "score": 50},
                    {"name": "Terminal-Bench", "version": "2.1", "harness": "two", "score": 51},
                ]
            },
            ["Terminal-Bench"],
        )
        self.assertEqual(dict(defaults)["Terminal-Bench"], Decimal("51"))
        self.assertEqual(overrides, ())

    def test_models_dev_prefers_with_tools_by_keeping_the_highest_value(self) -> None:
        defaults, overrides = updater.extract_selected_models_dev_benchmarks(
            "anthropic/claude-fable-5",
            {
                "benchmarks": [
                    {"name": "Humanity's Last Exam", "variant": "no tools", "score": 59},
                    {"name": "Humanity's Last Exam", "variant": "with tools", "score": 64.5},
                ]
            },
            ["Humanity's Last Exam"],
        )
        self.assertEqual(dict(defaults), {"Humanity's Last Exam": Decimal("64.5")})
        self.assertEqual(overrides, ())

    def test_models_dev_accepts_month_year_benchmark_versions(self) -> None:
        defaults, overrides = updater.extract_selected_models_dev_benchmarks(
            "vendor/nova",
            {
                "benchmarks": [
                    {"name": "HMMT", "version": "November 2025", "score": 94.4},
                    {"name": "HMMT", "version": "February 2026", "score": 92.5},
                ]
            },
            ["HMMT"],
        )
        self.assertEqual(dict(defaults), {"HMMT": Decimal("94.4")})
        self.assertEqual(overrides, ())

    def test_models_dev_base_model_association_populates_every_effort_row(self) -> None:
        payload = {
            "providers": {
                "gateway": {
                    "id": "gateway",
                    "models": {
                        "nova-alias": {
                            **catalogue_model(
                                "nova-alias", "Nova", efforts=["low", "high"]
                            ),
                            "base_model": "vendor/nova",
                        }
                    },
                }
            },
            "models": {
                "vendor/nova": {
                    "id": "vendor/nova",
                    "name": "Nova",
                    "benchmarks": [
                        {"name": "SWE-Bench Verified", "score": 77}
                    ],
                }
            },
        }
        discovered = updater.parse_models_dev_catalogue(
            json.dumps(payload),
            ["gateway"],
            {"gateway": ()},
            ["SWE-Bench Verified"],
        )
        provider_model = discovered["gateway"][0]
        self.assertEqual(provider_model.canonical_id, "vendor/nova")
        self.assertEqual(dict(provider_model.benchmarks)["SWE-Bench Verified"], Decimal("77"))

        family = updater.ModelFamily("Nova", "nova")
        matched = updater.match_provider_models(
            [
                model(family, name="Nova (low)", slug="nova-low"),
                model(family, name="Nova (high)", slug="nova-high"),
            ],
            discovered,
        )
        rows = updater.collect_rows(
            RecordingClient({}),
            matched.selected,
            benchmark_names=["SWE-Bench Verified"],
        )
        self.assertEqual(
            [row.benchmark_values["SWE-Bench Verified"] for row in rows],
            [None, Decimal("77")],
        )

    def test_selected_benchmarks_remain_separate_and_scope_effort(self) -> None:
        defaults, overrides = updater.extract_selected_models_dev_benchmarks(
            "vendor/nova",
            {
                "benchmarks": [
                    {"name": "One", "score": 10},
                    {"name": "Two", "score": 90},
                    {
                        "name": "Terminal-Bench",
                        "score": 60,
                        "version": "2.1",
                        "harness": "Terminus-2",
                    },
                    {
                        "name": "Terminal-Bench",
                        "score": 70,
                        "version": "2.1",
                        "variant": "medium",
                        "harness": "Cursor CLI",
                    },
                    {
                        "name": "Terminal-Bench",
                        "score": 75,
                        "version": "2.1",
                        "variant": "medium",
                        "harness": "Claude Code",
                    },
                ]
            },
            ["One", "Two", "Terminal-Bench"],
        )
        self.assertEqual(
            dict(defaults),
            {"One": Decimal("10"), "Two": Decimal("90"), "Terminal-Bench": Decimal("60")},
        )
        self.assertEqual(
            dict(dict(overrides)["medium"])["Terminal-Bench"], Decimal("75")
        )

    def test_live_benchmark_effort_forms_are_bounded_and_explicit(self) -> None:
        expected = {
            "high": "high",
            "high effort": "high",
            "high reasoning": "high",
            "max effort, context compaction": "max",
            "max effort, with tools": "max",
            "reasoning effort xhigh": "xhigh",
            "reasoning effort none": "default",
        }
        for variant, effort in expected.items():
            with self.subTest(variant=variant):
                self.assertEqual(
                    updater._benchmark_effort({"variant": variant}), effort
                )
        for variant in ("tool use high", "high quality", "reasoning xhigh"):
            with self.subTest(variant=variant):
                self.assertIsNone(updater._benchmark_effort({"variant": variant}))

    def test_reasoning_effort_xhigh_benchmark_scopes_only_to_xhigh_row(self) -> None:
        defaults, overrides = updater.extract_selected_models_dev_benchmarks(
            "openai/gpt-5.4-nano",
            {
                "benchmarks": [
                    {
                        "name": "SWE-Bench Pro",
                        "score": 52.4,
                        "variant": "reasoning effort xhigh",
                    }
                ]
            },
            ["SWE-Bench Pro"],
        )
        self.assertEqual(defaults, ())
        self.assertEqual(
            dict(dict(overrides)["xhigh"])["SWE-Bench Pro"], Decimal("52.4")
        )

        family = updater.ModelFamily("GPT-5.4 Nano", "gpt-5-4-nano")
        provider_model = updater.ProviderModel(
            "openai",
            "gpt-5.4-nano",
            "GPT-5.4 Nano",
            ("default", "low", "medium", "high", "xhigh"),
            "openai/gpt-5.4-nano",
            defaults,
            overrides,
        )
        match = updater.match_provider_models([], {"openai": [provider_model]})
        rows = updater.collect_rows(
            RecordingClient({}),
            match.selected,
            benchmark_names=["SWE-Bench Pro"],
        )
        self.assertEqual(
            {
                row.reasoning: row.benchmark_values["SWE-Bench Pro"]
                for row in rows
            },
            {
                "low": None,
                "medium": None,
                "high": None,
                "xhigh": Decimal("52.4"),
            },
        )
        current_rows = [
            updater.replace(
                row,
                benchmark_values={"SWE-Bench Pro": Decimal("52.4")},
                authoritative_benchmarks=frozenset(),
            )
            for row in rows
        ]
        merged = updater.merge_rows(rows, current_rows)
        self.assertEqual(
            {
                row.reasoning: row.benchmark_values["SWE-Bench Pro"]
                for row in merged
            },
            {
                "low": None,
                "medium": None,
                "high": None,
                "xhigh": Decimal("52.4"),
            },
        )

    def test_default_reasoning_is_merged_into_high_benchmark_row(self) -> None:
        provider_model = updater.ProviderModel(
            "anthropic",
            "example",
            "Example",
            ("default", "high"),
            benchmarks=(("Humanity's Last Exam", Decimal("60")),),
            benchmark_overrides=(
                ("high", (("Humanity's Last Exam", Decimal("65")),)),
            ),
        )
        matched = updater.match_provider_models([], {"anthropic": [provider_model]})
        rows = updater.collect_rows(
            RecordingClient({}), matched.selected,
            benchmark_names=["Humanity's Last Exam"],
        )
        self.assertEqual([(row.model, row.reasoning) for row in rows], [("Example", "high")])
        self.assertEqual(
            rows[0].benchmark_values["Humanity's Last Exam"], Decimal("65")
        )

    def test_default_and_high_fresh_rows_collapse_to_high_with_maximum_benchmark(self) -> None:
        fresh = [
            updater.RawRow(
                "Example", "default", Decimal("1"), None, None,
                benchmark_values={"Humanity's Last Exam": Decimal("60")},
            ),
            updater.RawRow(
                "Example", "high", Decimal("2"), None, None,
                benchmark_values={"Humanity's Last Exam": Decimal("65")},
            ),
        ]
        merged = updater.merge_rows(fresh, [])
        self.assertEqual([(row.model, row.reasoning) for row in merged], [("Example", "high")])
        self.assertEqual(
            merged[0].benchmark_values["Humanity's Last Exam"], Decimal("65")
        )

    def test_dynamic_benchmark_merge_preserves_selected_cells_and_drops_removed(self) -> None:
        current = updater.RawRow(
            "Example",
            "high",
            Decimal("1"),
            None,
            None,
            benchmark_values={"Keep, τ³": Decimal("42"), "Remove": Decimal("99")},
        )
        fresh = updater.replace(
            current,
            benchmark_values={"Keep, τ³": None, "New": None},
        )
        merged = updater.merge_rows([fresh], [current])
        self.assertEqual(
            merged[0].benchmark_values,
            {"Keep, τ³": Decimal("42"), "New": None},
        )
        rendered = updater.render_csv(merged, ["Keep, τ³", "New"])
        parsed = updater.parse_existing_csv(rendered, source=Path("fixture.csv"))
        self.assertEqual(parsed[0].benchmark_values, merged[0].benchmark_values)

    def test_new_benchmark_value_can_correct_a_higher_existing_cell(self) -> None:
        current = updater.RawRow(
            "Example", "high", Decimal("1"), None, None,
            benchmark_values={"Humanity's Last Exam": Decimal("90")},
        )
        fresh = updater.replace(
            current,
            benchmark_values={"Humanity's Last Exam": Decimal("50")},
        )
        merged = updater.merge_rows([fresh], [current])
        self.assertEqual(
            merged[0].benchmark_values["Humanity's Last Exam"], Decimal("50")
        )

    def test_dynamic_match_includes_unmatched_efforts_and_deduplicates_overlap(self) -> None:
        nova = updater.ModelFamily("Nova 1", "nova-1")
        api_models = [
            model(nova, name="Nova 1 (low)", slug="nova-1-low"),
            model(nova, name="Nova 1 (high)", slug="nova-1-high"),
        ]
        providers = {
            "one": [
                updater.ProviderModel("one", "nova-1", "Nova 1", ("low",)),
                updater.ProviderModel("one", "mystery", "Mystery", ("medium",)),
            ],
            "two": [
                updater.ProviderModel("two", "nova-1", "Nova 1", ("high",)),
            ],
        }
        result = updater.match_provider_models(api_models, providers)
        identities = [(item.family.name, item.reasoning) for item in result.selected]
        self.assertEqual(
            identities,
            [("Mystery", "medium"), ("Nova 1", "low"), ("Nova 1", "high")],
        )
        mystery = result.selected[0]
        self.assertIsNone(mystery.slug)
        self.assertIsNone(mystery.intelligence)
        rows = updater.collect_rows(RecordingClient({}), result.selected)
        self.assertEqual([(row.model, row.reasoning) for row in rows], identities)

    def test_all_provider_bridge_merges_duplicate_claude_identity(self) -> None:
        dated = updater.ProviderModel(
            "anthropic",
            "claude-haiku-4-5-20251001",
            "Claude Haiku 4.5 20251001",
            ("high",),
        )
        alias = updater.ProviderModel(
            "gateway",
            "claude-haiku-4-5",
            "Claude Haiku 4.5",
            ("low",),
        )
        bridge = updater.ProviderModel(
            "replicate",
            "claude-haiku-4-5-20251001",
            "Claude Haiku 4.5",
            ("medium",),
        )

        matched = updater.match_provider_models(
            [], {"anthropic": [dated], "gateway": [alias], "replicate": [bridge]}
        )

        self.assertEqual(len(matched.families), 1)
        self.assertEqual(
            [(item.family.name, item.reasoning) for item in matched.selected],
            [
                ("Claude Haiku 4.5 20251001", "low"),
                ("Claude Haiku 4.5 20251001", "medium"),
                ("Claude Haiku 4.5 20251001", "high"),
            ],
        )

    def test_provider_display_name_wins_conflicting_identifier_match(self) -> None:
        alpha = updater.ModelFamily("Alpha", "alpha")
        beta = updater.ModelFamily("Beta", "beta")
        matched = updater.match_provider_models(
            [model(alpha), model(beta)],
            {"provider": [updater.ProviderModel("provider", "beta", "Alpha", ("high",))]},
        )
        self.assertEqual(
            [(item.family.name, item.reasoning) for item in matched.selected],
            [("Alpha", "high")],
        )

    def test_provider_bridge_uses_current_display_family_to_merge_conflicts(self) -> None:
        alpha = updater.ModelFamily("Alpha", "alpha")
        beta = updater.ModelFamily("Beta", "beta")
        matched = updater.match_provider_models(
            [model(alpha), model(beta)],
            {
                "first": [updater.ProviderModel("first", "alpha-dated", "Alpha", ("high",))],
                "second": [updater.ProviderModel("second", "beta", "Beta", ("low",))],
                "bridge": [updater.ProviderModel("bridge", "beta", "Alpha", ("medium",))],
            },
        )
        self.assertEqual({family.name for family in matched.families}, {"Alpha"})
        self.assertEqual(
            {(item.family.name, item.reasoning) for item in matched.selected},
            {("Alpha", "low"), ("Alpha", "medium"), ("Alpha", "high")},
        )

    def test_unknown_provider_fails_before_network_backup_or_replacement(self) -> None:
        class NeverClient:
            def __init__(self) -> None:
                self.calls = 0

            def get_text(self, *args: object, **kwargs: object) -> str:
                self.calls += 1
                raise AssertionError("network must not be called")

        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "available_model_raw_values.csv"
            original = updater.render_csv(complete_current_rows())
            output.write_bytes(original)
            client = NeverClient()
            with self.assertRaisesRegex(updater.UpdateError, "unknown access providers: codex"):
                updater.update(
                    output,
                    Path(directory) / ".env",
                    {"ARTIFICIAL_ANALYSIS_API": "secret-key"},
                    client,
                    providers=["codex"],
                )
            self.assertEqual(client.calls, 0)
            self.assertEqual(output.read_bytes(), original)
            self.assertEqual(list(output.parent.glob(f"{output.name}.*.bak")), [])

    def test_invalid_provider_config_fails_before_secret_network_or_backup(self) -> None:
        class NeverClient:
            def __init__(self) -> None:
                self.calls = 0

            def get_text(self, *args: object, **kwargs: object) -> str:
                self.calls += 1
                raise AssertionError("network must not be called")

        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "available_model_raw_values.csv"
            config = Path(directory) / "providers.toml"
            original = updater.render_csv(complete_current_rows())
            output.write_bytes(original)
            config.write_text(
                '[providers.anthropic]\nexcluded_models = [""]\n',
                encoding="utf-8",
            )
            client = NeverClient()
            with self.assertRaisesRegex(updater.UpdateError, "invalid excluded_models"):
                updater.update(
                    output,
                    Path(directory) / ".env",
                    {},
                    client,
                    provider_config_path=config,
                )
            self.assertEqual(client.calls, 0)
            self.assertEqual(output.read_bytes(), original)
            self.assertEqual(list(output.parent.glob(f"{output.name}.*.bak")), [])

    def test_default_tls_context_preserves_certificate_and_hostname_verification(self) -> None:
        client = updater.HttpClient()
        self.assertEqual(client.tls_context.verify_mode, ssl.CERT_REQUIRED)
        self.assertTrue(client.tls_context.check_hostname)
        if hasattr(ssl, "VERIFY_X509_STRICT"):
            self.assertFalse(client.tls_context.verify_flags & ssl.VERIFY_X509_STRICT)

    def test_default_network_path_passes_verified_tls_context(self) -> None:
        client = updater.HttpClient(max_retries=0)
        with mock.patch.object(
            client.network_opener, "open", return_value=FakeResponse(b"ok")
        ) as open_request:
            self.assertEqual(
                client.get_text("https://example.test", purpose="TLS test"),
                "ok",
            )
        _, keyword_arguments = open_request.call_args
        self.assertEqual(keyword_arguments["timeout"], updater.REQUEST_TIMEOUT_SECONDS)
        https_handler = next(
            handler
            for handler in client.network_opener.handlers
            if isinstance(handler, urllib.request.HTTPSHandler)
        )
        self.assertIs(https_handler._context, client.tls_context)
        self.assertTrue(
            any(
                isinstance(handler, updater.RejectRedirectHandler)
                for handler in client.network_opener.handlers
            )
        )

    def test_authenticated_cross_origin_redirect_is_rejected_without_forwarding_key(self) -> None:
        target_requests: list[str | None] = []

        class QuietHandler(http.server.BaseHTTPRequestHandler):
            def log_message(self, format: str, *args: object) -> None:
                return None

        class TargetHandler(QuietHandler):
            def do_GET(self) -> None:
                target_requests.append(self.headers.get("x-api-key"))
                self.send_response(200)
                self.end_headers()
                self.wfile.write(b"target")

        target_server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), TargetHandler)
        target_port = target_server.server_address[1]

        class RedirectHandler(QuietHandler):
            def do_GET(self) -> None:
                self.send_response(302)
                self.send_header("Location", f"http://127.0.0.1:{target_port}/target")
                self.end_headers()

        redirect_server = http.server.ThreadingHTTPServer(
            ("127.0.0.1", 0), RedirectHandler
        )
        redirect_port = redirect_server.server_address[1]
        threads = [
            threading.Thread(target=target_server.serve_forever, daemon=True),
            threading.Thread(target=redirect_server.serve_forever, daemon=True),
        ]
        for thread in threads:
            thread.start()
        try:
            client = updater.HttpClient(max_retries=0)
            with self.assertRaisesRegex(
                updater.UpdateError, r"redirects are not allowed \(HTTP 302\)"
            ):
                client.get_text(
                    f"http://127.0.0.1:{redirect_port}/source",
                    headers={"x-api-key": "must-not-be-forwarded"},
                    purpose="redirect regression test",
                )
        finally:
            redirect_server.shutdown()
            target_server.shutdown()
            redirect_server.server_close()
            target_server.server_close()
            for thread in threads:
                thread.join(timeout=2)
        self.assertEqual(target_requests, [])

    def test_environment_key_takes_precedence_over_dotenv(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            env_path = Path(directory) / ".env"
            env_path.write_text("ARTIFICIAL_ANALYSIS_API=dotenv-key\n", encoding="utf-8")
            self.assertEqual(
                updater.load_api_key(
                    {"ARTIFICIAL_ANALYSIS_API": "environment-key"}, env_path
                ),
                "environment-key",
            )

    def test_dotenv_key_is_used_as_safe_fallback(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            env_path = Path(directory) / ".env"
            env_path.write_text(
                "IGNORED=value\nARTIFICIAL_ANALYSIS_API='dotenv-key'\n",
                encoding="utf-8",
            )
            self.assertEqual(updater.load_api_key({}, env_path), "dotenv-key")

    def test_missing_key_has_clear_error(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            env_path = Path(directory) / ".env"
            env_path.write_text("ARTIFICIAL_ANALYSIS_API=\n", encoding="utf-8")
            with self.assertRaisesRegex(updater.UpdateError, "missing ARTIFICIAL_ANALYSIS_API"):
                updater.load_api_key({}, env_path)

    def test_model_list_uses_api_header_and_follows_pagination(self) -> None:
        page_one_url = f"{updater.API_URL}?page=1"
        page_two_url = f"{updater.API_URL}?page=2"
        client = RecordingClient(
            {
                page_one_url: json.dumps(
                    {
                        "pagination": {
                            "page": 1,
                            "total_pages": 2,
                            "has_more": True,
                        },
                        "data": [{"slug": "one"}],
                    }
                ),
                page_two_url: json.dumps(
                    {
                        "pagination": {
                            "page": 2,
                            "total_pages": 2,
                            "has_more": False,
                        },
                        "data": [{"slug": "two"}],
                    }
                ),
            }
        )
        result = updater.fetch_all_models(client, "secret-key")
        self.assertEqual([item["slug"] for item in result], ["one", "two"])
        self.assertEqual([call[1] for call in client.calls], [
            {"x-api-key": "secret-key"},
            {"x-api-key": "secret-key"},
        ])

    def test_discovers_reasoning_variants_orders_them_and_preserves_null_intelligence(self) -> None:
        families = TEST_MODEL_FAMILIES
        api_models = [model(family) for family in reversed(families)]
        sol = families[0]
        sonnet = families[4]
        api_models.extend(
            [
                model(
                    sol,
                    slug=f"{sol.base_slug}-low",
                    name=f"{sol.name} (low)",
                    intelligence=None,
                ),
                model(
                    sol,
                    slug=f"{sol.base_slug}-non-reasoning",
                    name=f"{sol.name} (Non-reasoning)",
                    reasoning_model=False,
                ),
                model(
                    sonnet,
                    slug=f"{sonnet.base_slug}-high",
                    name=f"{sonnet.name} (Adaptive Reasoning, High Effort)",
                ),
            ]
        )
        selected = updater.discover_models(api_models, families=families)
        sol_models = [item for item in selected if item.family == sol]
        self.assertEqual([item.reasoning for item in sol_models], ["low", "max"])
        self.assertIsNone(sol_models[0].intelligence)
        self.assertNotIn("non reasoning", [item.reasoning for item in selected])
        self.assertEqual(
            next(item.reasoning for item in selected if item.family.name == "Claude Fable 5"),
            "max (Opus 4.8 fallback)",
        )
        self.assertEqual(
            next(item.reasoning for item in selected if item.family.name == "Gemini 3.6 Flash"),
            "high",
        )
        self.assertEqual(
            {item.family.name for item in selected if item.slug == item.family.base_slug},
            {family.name for family in families},
        )

    def test_extracts_optional_artificial_analysis_metrics_and_preserves_nulls(self) -> None:
        family = TEST_MODEL_FAMILIES[0]
        published, unpublished = updater.discover_models(
            [
                model(
                    family,
                    slug=f"{family.base_slug}-low",
                    name=f"{family.name} (low)",
                    median_response_time=Decimal("12.5"),
                    coding_index=Decimal("44.45"),
                    agentic_index=Decimal("33.35"),
                ),
                model(
                    family,
                    median_response_time=None,
                    coding_index=None,
                    agentic_index=None,
                ),
            ],
            families=(family,),
        )
        self.assertEqual(published.median_response_time, Decimal("12.5"))
        self.assertEqual(published.coding_index, Decimal("44.45"))
        self.assertEqual(published.agentic_index, Decimal("33.35"))
        self.assertIsNone(unpublished.median_response_time)
        self.assertIsNone(unpublished.coding_index)
        self.assertIsNone(unpublished.agentic_index)

    def test_artificial_analysis_benchmarks_merge_with_models_dev_using_highest_value(self) -> None:
        family = TEST_MODEL_FAMILIES[0]
        aa_item = model(
            family,
            name=f"{family.name} (high)",
            slug=f"{family.base_slug}-high",
        )
        aa_item["evaluations"].update(
            {
                "hle": Decimal("0.70"),
                "gpqa_diamond": Decimal("0.80"),
                "terminalbench_v2_1": Decimal("0.65"),
            }
        )
        selected = updater.discover_models([aa_item], families=(family,))[0]
        self.assertEqual(
            dict(selected.benchmarks)["Humanity's Last Exam"], Decimal("70.00")
        )
        self.assertEqual(dict(selected.benchmarks)["GPQA Diamond"], Decimal("80.00"))

        provider_model = updater.ProviderModel(
            "anthropic",
            family.base_slug,
            family.name,
            ("high",),
            f"anthropic/{family.base_slug}",
            (("Humanity's Last Exam", Decimal("64.5")),),
        )
        matched = updater.match_provider_models(
            [aa_item], {"anthropic": [provider_model]}
        )
        row = updater.collect_rows(
            RecordingClient({}), matched.selected,
            benchmark_names=["Humanity's Last Exam"],
        )[0]
        self.assertEqual(row.benchmark_values["Humanity's Last Exam"], Decimal("70.00"))

        higher_models_dev = updater.ProviderModel(
            "anthropic",
            family.base_slug,
            family.name,
            ("high",),
            f"anthropic/{family.base_slug}",
            (("Humanity's Last Exam", Decimal("75")),),
        )
        higher = updater.match_provider_models(
            [aa_item], {"anthropic": [higher_models_dev]}
        )
        higher_row = updater.collect_rows(
            RecordingClient({}), higher.selected,
            benchmark_names=["Humanity's Last Exam"],
        )[0]
        self.assertEqual(
            higher_row.benchmark_values["Humanity's Last Exam"], Decimal("75")
        )

    def test_annotated_artificial_analysis_snapshots_merge_into_one_family(self) -> None:
        api_models = [
            {
                **model(
                    TEST_MODEL_FAMILIES[3],
                    name="Claude Opus 4.5 [claude-opus-4-5-20251101]",
                    slug="claude-opus-4-5-20251101",
                ),
                "evaluations": {
                    **model(
                        TEST_MODEL_FAMILIES[3],
                        name="Claude Opus 4.5 [claude-opus-4-5-20251101]",
                        slug="claude-opus-4-5-20251101",
                    )["evaluations"],
                    "hle": Decimal("0.61"),
                },
            },
            {
                **model(
                    TEST_MODEL_FAMILIES[3],
                    name="Claude Opus 4.5 (latest)",
                    slug="claude-opus-4-5-latest",
                ),
                "evaluations": {
                    **model(
                        TEST_MODEL_FAMILIES[3],
                        name="Claude Opus 4.5 (latest)",
                        slug="claude-opus-4-5-latest",
                    )["evaluations"],
                    "hle": Decimal("0.64"),
                },
            },
        ]
        provider = updater.ProviderModel(
            "anthropic",
            "claude-opus-4-5",
            "Claude Opus 4.5 [claude-opus-4-5-20251101]",
            ("high",),
        )
        matched = updater.match_provider_models(api_models, {"anthropic": [provider]})
        self.assertEqual(
            [(item.family.name, item.reasoning) for item in matched.selected],
            [("Claude Opus 4.5", "high")],
        )
        self.assertEqual(
            dict(matched.selected[0].benchmarks)["Humanity's Last Exam"],
            Decimal("64.00"),
        )

    def test_required_artificial_analysis_metric_keys_distinguish_missing_from_null(self) -> None:
        family = TEST_MODEL_FAMILIES[0]
        fields = (
            ("evaluations", "artificial_analysis_coding_index"),
            ("evaluations", "artificial_analysis_agentic_index"),
            ("performance", "median_end_to_end_response_time_seconds"),
        )
        for envelope, field in fields:
            with self.subTest(envelope=envelope, field=field):
                missing = model(family)
                del missing[envelope][field]
                with self.assertRaisesRegex(updater.UpdateError, "is missing"):
                    updater.discover_models([missing], families=(family,))

                explicit_null = model(family)
                explicit_null[envelope][field] = None
                selected = updater.discover_models(
                    [explicit_null], families=(family,)
                )[0]
                value = {
                    "artificial_analysis_coding_index": selected.coding_index,
                    "artificial_analysis_agentic_index": selected.agentic_index,
                    "median_end_to_end_response_time_seconds": selected.median_response_time,
                }[field]
                self.assertIsNone(value)

    def test_intelligence_and_cost_schema_distinguishes_missing_from_explicit_null(self) -> None:
        family = TEST_MODEL_FAMILIES[0]
        cost_field = "artificial_analysis_intelligence_index_cost"
        intelligence_field = "artificial_analysis_intelligence_index"

        missing_intelligence = model(family)
        del missing_intelligence["evaluations"][intelligence_field]
        with self.assertRaisesRegex(updater.UpdateError, "is missing"):
            updater.discover_models([missing_intelligence], families=(family,))
        self.assertIsNone(
            updater.discover_models(
                [model(family, intelligence=None)], families=(family,)
            )[0].intelligence
        )

        missing_outer_cost = model(family, include_cost_data=False)
        with self.assertRaisesRegex(updater.UpdateError, "is missing"):
            updater.discover_models([missing_outer_cost], families=(family,))
        null_outer_cost = model(family)
        null_outer_cost[cost_field] = None
        self.assertIsNone(
            updater.discover_models([null_outer_cost], families=(family,))[0].cost
        )

        missing_cost_per_task = model(family)
        missing_cost_per_task[cost_field] = {}
        with self.assertRaisesRegex(updater.UpdateError, "missing cost_per_task"):
            updater.discover_models([missing_cost_per_task], families=(family,))
        null_cost_per_task = model(family)
        null_cost_per_task[cost_field] = {"cost_per_task": None}
        self.assertIsNone(
            updater.discover_models([null_cost_per_task], families=(family,))[0].cost
        )
        malformed_cost_per_task = model(family)
        malformed_cost_per_task[cost_field] = {"cost_per_task": []}
        with self.assertRaisesRegex(updater.UpdateError, "malformed cost_per_task"):
            updater.discover_models([malformed_cost_per_task], families=(family,))

        missing_total_cost = model(family)
        missing_total_cost[cost_field] = {"cost_per_task": {}}
        with self.assertRaisesRegex(updater.UpdateError, "missing total_cost"):
            updater.discover_models([missing_total_cost], families=(family,))
        self.assertIsNone(
            updater.discover_models(
                [model(family, cost=None)], families=(family,)
            )[0].cost
        )
        malformed_total_cost = model(family)
        malformed_total_cost[cost_field] = {
            "cost_per_task": {"total_cost": "not-a-number"}
        }
        with self.assertRaisesRegex(updater.UpdateError, "must be numeric"):
            updater.discover_models([malformed_total_cost], families=(family,))

    def test_rejects_malformed_artificial_analysis_metric_envelopes_and_numbers(self) -> None:
        family = TEST_MODEL_FAMILIES[0]
        malformed_cases: list[tuple[str, object, str]] = [
            ("evaluations", None, "missing evaluations data"),
            ("performance", None, "missing performance data"),
        ]
        for field, value, expected in malformed_cases:
            with self.subTest(field=field):
                item = model(family)
                item[field] = value
                with self.assertRaisesRegex(updater.UpdateError, expected):
                    updater.discover_models([item], families=(family,))

        for field, value in (
            ("artificial_analysis_coding_index", "not-a-number"),
            ("artificial_analysis_agentic_index", "NaN"),
        ):
            with self.subTest(field=field):
                item = model(family)
                item["evaluations"][field] = value
                with self.assertRaises(updater.UpdateError):
                    updater.discover_models([item], families=(family,))

        for value in ("not-a-number", "Infinity", Decimal("-0.1")):
            with self.subTest(response_time=value):
                item = model(family)
                item["performance"]["median_end_to_end_response_time_seconds"] = value
                with self.assertRaises(updater.UpdateError):
                    updater.discover_models([item], families=(family,))

    def test_rendering_uses_required_round_half_up_precision(self) -> None:
        content = updater.render_csv(
            [
                updater.RawRow(
                    model="Example",
                    reasoning="high",
                    intelligence=Decimal("49.45"),
                    time_seconds=Decimal("12.5"),
                    cost=Decimal("0.125"),
                    median_response_time=Decimal("12.5"),
                    coding_index=Decimal("44.45"),
                    agentic_index=Decimal("33.35"),
                    benchmark_values={"SWE-Bench, τ³": Decimal("76.44")},
                ),
                updater.RawRow(
                    "Unpublished",
                    "low",
                    None,
                    Decimal("1.4"),
                    Decimal("1.004"),
                ),
                updater.RawRow(
                    "Missing task metrics",
                    "medium",
                    Decimal("10"),
                    None,
                    None,
                ),
            ],
            ["SWE-Bench, τ³"],
        ).decode("utf-8")
        self.assertIn(
            'Example,high,49.5,13,0.13,13,44.5,33.4,76.4\n',
            content,
        )
        self.assertIn("Unpublished,low,,1,1.00,,,,\n", content)
        self.assertIn("Missing task metrics,medium,10.0,,,,,,\n", content)

    def test_public_page_time_is_bound_to_exact_slug(self) -> None:
        self.assertEqual(
            updater.extract_time_for_slug(page("exact-slug", "123.6688"), "exact-slug"),
            Decimal("123.6688"),
        )
        with self.assertRaisesRegex(updater.UpdateError, "slug mismatch"):
            updater.extract_time_for_slug(page("different-slug"), "expected-slug")
        with self.assertRaisesRegex(updater.UpdateError, "ambiguous currentModel"):
            updater.extract_time_for_slug(page("exact") + page("exact"), "exact")
        self.assertIsNone(updater.extract_time_for_slug(page("exact", "null"), "exact"))
        self.assertIsNone(updater.extract_time_for_slug(page("exact", None), "exact"))

    def test_api_cost_takes_precedence_and_page_fallback_is_exact_slug_bound(self) -> None:
        family = TEST_MODEL_FAMILIES[0]
        api_cost_model = updater.discover_models(
            [model(family, cost=Decimal("0.75"))], families=(family,)
        )[0]
        page_metrics = updater.extract_public_page_metrics(
            page(family.base_slug, cost="-99"), family.base_slug
        )
        self.assertEqual(api_cost_model.cost, Decimal("0.75"))
        self.assertIsNone(page_metrics.fallback_cost)

        fallback_item = model(family)
        fallback_item["artificial_analysis_intelligence_index_cost"] = None
        fallback_model = updater.discover_models(
            [fallback_item], families=(family,)
        )[0]
        self.assertIsNone(fallback_model.cost)
        fallback = updater.extract_public_page_metrics(
            page(family.base_slug, cost="0.69115"),
            family.base_slug,
            require_fallback_cost=True,
        )
        self.assertEqual(fallback.fallback_cost, Decimal("0.69115"))
        with self.assertRaisesRegex(updater.UpdateError, "slug mismatch"):
            updater.extract_public_page_metrics(
                page("different", cost="0.5"),
                family.base_slug,
                require_fallback_cost=True,
            )

    def test_page_fallback_cost_rejects_missing_malformed_negative_and_ambiguous_data(self) -> None:
        slug = "exact"
        self.assertIsNone(
            updater.extract_public_page_metrics(
                page(slug), slug, require_fallback_cost=True
            ).fallback_cost
        )
        self.assertIsNone(
            updater.extract_public_page_metrics(
                page(slug, cost="null"), slug, require_fallback_cost=True
            ).fallback_cost
        )
        for source, expected in (
            (page(slug, cost="not-a-number"), "invalid currentModel JSON"),
            (page(slug, cost="-0.1"), "must not be negative"),
            (
                page(slug, cost="0.1").replace(
                    ',\\"other\\":true',
                    ',\\"intelligenceIndexCostPerTask\\":{\\"cost\\":{\\"total\\":0.2}},\\"other\\":true',
                ),
                "duplicate intelligenceIndexCostPerTask data",
            ),
        ):
            with self.subTest(expected=expected), self.assertRaisesRegex(
                updater.UpdateError, expected
            ):
                updater.extract_public_page_metrics(
                    source, slug, require_fallback_cost=True
                )

    def test_exact_current_model_never_reads_later_unrelated_task_metrics(self) -> None:
        source = page("exact", None) + (
            '<script>other={"slug":"gemini-3-5-flash",'
            '"intelligenceIndexTimePerTask":170.90715786446634,'
            '"intelligenceIndexCostPerTask":{"cost":{"total":0.6911525063081332}}}'
            "</script>"
        )
        metrics = updater.extract_public_page_metrics(
            source, "exact", require_fallback_cost=True
        )
        self.assertIsNone(metrics.time_seconds)
        self.assertIsNone(metrics.fallback_cost)

    def test_collect_rows_fetches_only_opted_in_public_pages_without_api_key(self) -> None:
        api_models = [model(family) for family in TEST_MODEL_FAMILIES]
        selected = updater.discover_models(api_models, families=TEST_MODEL_FAMILIES)
        responses = {}
        for family in TEST_MODEL_FAMILIES:
            responses[updater.MODEL_PAGE_URL.format(slug=family.base_slug)] = page(
                family.base_slug
            )
        client = RecordingClient(responses)
        rows = updater.collect_rows(
            client, selected, additions={"aa_page"}
        )
        self.assertEqual(len(rows), len(TEST_MODEL_FAMILIES))
        page_calls = [call for call in client.calls if "/models/" in call[0] and "/api/" not in call[0]]
        self.assertTrue(page_calls)
        self.assertTrue(all(headers == {} for _, headers, _ in page_calls))

    def test_collect_rows_calls_exactly_the_requested_optional_source(self) -> None:
        api_models = [model(family) for family in TEST_MODEL_FAMILIES]
        selected = updater.discover_models(api_models, families=TEST_MODEL_FAMILIES)
        page_urls = {
            updater.MODEL_PAGE_URL.format(slug=family.base_slug): page(family.base_slug)
            for family in TEST_MODEL_FAMILIES
        }
        cases = ((set(), False), ({"aa_page"}, True))
        for additions, expects_pages in cases:
            with self.subTest(additions=additions):
                responses: dict[str, str] = {}
                if expects_pages:
                    responses.update(page_urls)
                client = RecordingClient(responses)
                rows = updater.collect_rows(
                    client, selected, additions=additions
                )
                called_urls = {url for url, _, _ in client.calls}
                expected_urls: set[str] = set()
                if expects_pages:
                    expected_urls.update(page_urls)
                self.assertEqual(called_urls, expected_urls)
                self.assertTrue(
                    all(
                        headers == {}
                        for url, headers, _ in client.calls
                    )
                )
                self.assertTrue(
                    all(
                        (row.time_seconds is not None) == expects_pages
                        for row in rows
                    )
                )
                self.assertTrue(all(row.coding is None for row in rows))

    def test_merge_fresh_non_null_values_including_zero_win_for_every_metric(self) -> None:
        current = updater.RawRow(
            "Example", "high", *(Decimal(str(value)) for value in range(1, 11))
        )
        fresh = updater.RawRow(
            "Example", "high", *(Decimal("0") for _ in range(10))
        )
        self.assertEqual(updater.merge_rows([fresh], [current]), [fresh])

    def test_merge_fresh_null_values_preserve_every_current_metric(self) -> None:
        current = updater.RawRow(
            "Example", "high", *(Decimal(str(value)) for value in range(1, 11))
        )
        fresh = updater.RawRow("Example", "high", *(None for _ in range(10)))
        self.assertEqual(updater.merge_rows([fresh], [current]), [current])

    def test_merge_is_independent_per_cell_within_a_mixed_row(self) -> None:
        current = updater.RawRow(
            "Example", "high", *(Decimal(str(value)) for value in range(1, 11))
        )
        fresh = updater.RawRow(
            "Example",
            "high",
            Decimal("101"),
            None,
            Decimal("0"),
            None,
            Decimal("105"),
            None,
            Decimal("107"),
            None,
            Decimal("109"),
            None,
        )
        merged = updater.merge_rows([fresh], [current])[0]
        self.assertEqual(
            (
                merged.intelligence,
                merged.time_seconds,
                merged.cost,
                merged.coding,
                merged.reasoning_performance,
                merged.agentic,
                merged.math,
                merged.median_response_time,
                merged.coding_index,
                merged.agentic_index,
            ),
            (
                Decimal("101"),
                Decimal("2"),
                Decimal("0"),
                Decimal("4"),
                Decimal("105"),
                Decimal("6"),
                Decimal("107"),
                Decimal("8"),
                Decimal("109"),
                Decimal("10"),
            ),
        )

    def test_merge_is_exact_identity_only_and_drops_stale_rows(self) -> None:
        current = [
            updater.RawRow("Example", "high", Decimal("12"), None, None),
            updater.RawRow("Example", "low", Decimal("99"), None, None),
            updater.RawRow("Stale", "high", Decimal("88"), None, None),
        ]
        fresh = [
            updater.RawRow("Example", "high", None, None, None),
            updater.RawRow("New", "medium", None, None, None),
        ]
        merged = updater.merge_rows(fresh, current)
        self.assertEqual(
            [(row.model, row.reasoning) for row in merged],
            [("Example", "high"), ("New", "medium")],
        )
        self.assertEqual(merged[0].intelligence, Decimal("12"))
        self.assertIsNone(merged[1].intelligence)

    def test_default_update_calls_only_v2_and_preserves_omitted_optional_metrics(self) -> None:
        current = complete_current_rows()
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "available_model_raw_values.csv"
            output.write_bytes(updater.render_csv(current))
            client = RecordingClient(
                {
                    updater.MODELS_DEV_PROVIDER_URL: models_dev_provider_response(),
                    updater.MODELS_DEV_BENCHMARK_URL: models_dev_benchmark_response(),
                    f"{updater.API_URL}?page=1": api_response(),
                }
            )
            updater.update(
                output,
                Path(directory) / ".env",
                {"ARTIFICIAL_ANALYSIS_API": "secret-key"},
                client,
            )
            rows = updater.parse_existing_csv(output.read_bytes(), source=output)

        self.assertEqual([url for url, _, _ in client.calls], [
            updater.MODELS_DEV_PROVIDER_URL,
            updater.MODELS_DEV_BENCHMARK_URL,
            f"{updater.API_URL}?page=1",
        ])
        before_by_identity = {(row.model, row.reasoning): row for row in current}
        for after in rows:
            before = before_by_identity[(after.model, after.reasoning)]
            self.assertEqual(after.time_seconds, before.time_seconds)
            self.assertEqual(
                after.benchmark_values["SWE-Bench Verified"],
                before.benchmark_values["SWE-Bench Verified"],
            )
            self.assertEqual(
                after.benchmark_values["Terminal-Bench"],
                before.benchmark_values["Terminal-Bench"],
            )
            self.assertEqual(after.intelligence, Decimal("42.0"))
            self.assertEqual(after.cost, Decimal("0.13"))

    def test_partial_provider_refresh_updates_selected_and_preserves_unselected_rows(self) -> None:
        current = complete_current_rows()
        current.append(
            updater.RawRow(
                "Claude Opus 5", "legacy", Decimal("99"), Decimal("99"), Decimal("9")
            )
        )
        before_by_identity = {(row.model, row.reasoning): row for row in current}
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "available_model_raw_values.csv"
            output.write_bytes(updater.render_csv(current))
            client = RecordingClient(
                {
                    updater.MODELS_DEV_PROVIDER_URL: models_dev_provider_response(),
                    updater.MODELS_DEV_BENCHMARK_URL: models_dev_benchmark_response(),
                    f"{updater.API_URL}?page=1": api_response(),
                }
            )
            updater.update(
                output,
                Path(directory) / ".env",
                {"ARTIFICIAL_ANALYSIS_API": "secret-key"},
                client,
                providers=["anthropic"],
            )
            rows = updater.parse_existing_csv(output.read_bytes(), source=output)

        selected = {"Claude Opus 5", "Claude Sonnet 5", "Claude Fable 5"}
        self.assertNotIn(("Claude Opus 5", "legacy"), {
            (row.model, row.reasoning) for row in rows
        })
        for row in rows:
            before = before_by_identity[(row.model, row.reasoning)]
            if row.model in selected:
                self.assertEqual(row.intelligence, Decimal("42.0"))
                self.assertEqual(row.time_seconds, before.time_seconds)
            else:
                self.assertEqual(row.intelligence, before.intelligence)
                for name, value in before.benchmark_values.items():
                    self.assertEqual(row.benchmark_values[name], value)

    def test_missing_models_dev_benchmarks_preserve_exact_current_values(self) -> None:
        current = complete_current_rows()
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "available_model_raw_values.csv"
            output.write_bytes(updater.render_csv(current))
            client = RecordingClient(
                {
                    updater.MODELS_DEV_PROVIDER_URL: models_dev_provider_response(),
                    updater.MODELS_DEV_BENCHMARK_URL: models_dev_benchmark_response(),
                    f"{updater.API_URL}?page=1": api_response(),
                }
            )
            updater.update(
                output,
                Path(directory) / ".env",
                {"ARTIFICIAL_ANALYSIS_API": "secret-key"},
                client,
            )
            rows = updater.parse_existing_csv(output.read_bytes(), source=output)

        self.assertEqual(
            {url for url, _, _ in client.calls},
            {
                updater.MODELS_DEV_PROVIDER_URL,
                updater.MODELS_DEV_BENCHMARK_URL,
                f"{updater.API_URL}?page=1",
            },
        )
        before_by_identity = {(row.model, row.reasoning): row for row in current}
        for after in rows:
            before = before_by_identity[(after.model, after.reasoning)]
            self.assertEqual(
                after.benchmark_values["SWE-Bench Verified"],
                before.benchmark_values["SWE-Bench Verified"],
            )
            self.assertEqual(
                after.benchmark_values["Terminal-Bench"],
                before.benchmark_values["Terminal-Bench"],
            )

    def test_malformed_current_csv_fails_before_network_or_backup(self) -> None:
        class NeverClient:
            def __init__(self) -> None:
                self.calls = 0

            def get_text(self, *args: object, **kwargs: object) -> str:
                self.calls += 1
                raise AssertionError("network must not be called")

        valid = updater.render_csv(complete_current_rows()).decode("utf-8")
        header, first, *remaining = valid.splitlines()
        cases = {
            "schema": valid.replace("intelligence_index", "wrong_index", 1),
            "extra cell": "\n".join([header, first + ",extra", *remaining]) + "\n",
            "duplicate": "\n".join([header, first, first, *remaining]) + "\n",
            "malformed numeric": valid.replace(",1.1,", ",bad,", 1),
            "nonfinite": valid.replace(",1.1,", ",NaN,", 1),
            "negative": valid.replace(",20,0.10,", ",20,-0.10,", 1),
        }
        for name, content in cases.items():
            with self.subTest(name=name), tempfile.TemporaryDirectory() as directory:
                output = Path(directory) / "available_model_raw_values.csv"
                original = content.encode("utf-8")
                output.write_bytes(original)
                client = NeverClient()
                with self.assertRaises(updater.UpdateError):
                    updater.update(
                        output,
                        Path(directory) / ".env",
                        {"ARTIFICIAL_ANALYSIS_API": "secret-key"},
                        client,
                    )
                self.assertEqual(client.calls, 0)
                self.assertEqual(output.read_bytes(), original)
                self.assertEqual(list(output.parent.glob(f"{output.name}.*.bak")), [])

    def test_concurrent_output_change_fails_without_backup_or_replacement(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "available_model_raw_values.csv"
            output.write_bytes(updater.render_csv(complete_current_rows()))
            changed_rows = complete_current_rows()
            changed_rows[0] = updater.replace(
                changed_rows[0], intelligence=Decimal("77.7")
            )
            changed = updater.render_csv(changed_rows)

            class RacingClient:
                def get_text(
                    self,
                    url: str,
                    *,
                    headers: dict[str, str] | None = None,
                    purpose: str,
                ) -> str:
                    if url == updater.MODELS_DEV_PROVIDER_URL:
                        return models_dev_provider_response()
                    if url == updater.MODELS_DEV_BENCHMARK_URL:
                        return models_dev_benchmark_response()
                    output.write_bytes(changed)
                    return api_response()

            with self.assertRaisesRegex(updater.UpdateError, "changed while"):
                updater.update(
                    output,
                    Path(directory) / ".env",
                    {"ARTIFICIAL_ANALYSIS_API": "secret-key"},
                    RacingClient(),
                )
            self.assertEqual(output.read_bytes(), changed)
            self.assertEqual(list(output.parent.glob(f"{output.name}.*.bak")), [])

    def test_cli_additions_reject_unknown_and_deduplicate_repeats(self) -> None:
        with self.assertRaises(SystemExit):
            updater.parse_args(["--add", "unknown"])
        with mock.patch.object(
            updater, "update", return_value=Path("backup.csv")
        ) as update:
            self.assertEqual(
                updater.main(
                    [
                        "--benchmark-config",
                        "bench.toml",
                        "--provider-config",
                        "providers-custom.toml",
                        "--add",
                        "aa_page",
                        "--add",
                        "aa_page",
                        "--provider",
                        "openai",
                        "--provider",
                        "github-copilot",
                    ]
                ),
                0,
            )
        self.assertEqual(update.call_args.kwargs["additions"], {"aa_page"})
        self.assertEqual(
            update.call_args.kwargs["benchmark_config_path"], Path("bench.toml")
        )
        self.assertEqual(
            update.call_args.kwargs["provider_config_path"],
            Path("providers-custom.toml"),
        )
        self.assertEqual(
            update.call_args.kwargs["providers"], ["openai", "github-copilot"]
        )

    def test_cli_config_flags_remain_local_and_master_workflow_is_config_free(self) -> None:
        args = updater.parse_args(
            [
                "--benchmark-config",
                "bench.toml",
                "--provider-config",
                "providers-custom.toml",
            ]
        )
        self.assertEqual(args.benchmark_config, Path("bench.toml"))
        self.assertEqual(args.provider_config, Path("providers-custom.toml"))
        workflow = (
            REPOSITORY_ROOT.parent / ".github/workflows/refresh-model-data.yml"
        ).read_text(encoding="utf-8")
        self.assertIn("ARTIFICIAL_ANALYSIS_API", workflow)
        self.assertIn("python3 scripts/refresh-model-data.py", workflow)
        self.assertNotIn("--benchmark-config", workflow)
        self.assertNotIn("--provider-config", workflow)
        self.assertNotIn("COPILOT", workflow.upper())
        self.assertNotIn("OPENAI_API_KEY", workflow)
        self.assertNotIn("ANTHROPIC_API_KEY", workflow)
        self.assertEqual(updater.MODELS_DEV_PROVIDER_URL, "https://models.dev/api.json")
        self.assertEqual(updater.MODELS_DEV_BENCHMARK_URL, "https://models.dev/models.json")

    def test_backup_exists_before_atomic_replace_and_preserves_original(self) -> None:
        fixed_time = datetime(2026, 8, 6, 12, 34, 56, 123456, tzinfo=timezone.utc)
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "available_model_raw_values.csv"
            output.write_bytes(b"original")
            real_replace = os.replace

            def checked_replace(source: str, destination: Path) -> None:
                backups = list(output.parent.glob(f"{output.name}.*.bak"))
                self.assertEqual(len(backups), 1)
                self.assertEqual(backups[0].read_bytes(), b"original")
                real_replace(source, destination)

            with mock.patch.object(updater.os, "replace", side_effect=checked_replace):
                backup = updater.replace_with_backup(output, b"replacement", now=fixed_time)
            self.assertEqual(output.read_bytes(), b"replacement")
            self.assertEqual(backup.read_bytes(), b"original")

    def test_backup_name_collision_does_not_overwrite_existing_backup(self) -> None:
        fixed_time = datetime(2026, 8, 6, tzinfo=timezone.utc)
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "available_model_raw_values.csv"
            output.write_bytes(b"original")
            collision = output.with_name(
                "available_model_raw_values.csv.20260806T000000000000Z.bak"
            )
            collision.write_bytes(b"existing backup")
            backup = updater.replace_with_backup(output, b"replacement", now=fixed_time)
            self.assertEqual(collision.read_bytes(), b"existing backup")
            self.assertEqual(backup.name, f"{output.name}.20260806T000000000000Z.1.bak")
            self.assertEqual(backup.read_bytes(), b"original")

    def test_fetch_failure_preserves_original_and_creates_no_backup(self) -> None:
        class FailingClient:
            def get_text(self, *args: object, **kwargs: object) -> str:
                raise updater.UpdateError("simulated fetch failure")

        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "available_model_raw_values.csv"
            original = updater.render_csv(
                [
                    updater.RawRow(
                        "Existing", "low", Decimal("1"), None, Decimal("0.1")
                    )
                ]
            )
            output.write_bytes(original)
            with self.assertRaisesRegex(updater.UpdateError, "simulated fetch failure"):
                updater.update(
                    output,
                    Path(directory) / ".env",
                    {"ARTIFICIAL_ANALYSIS_API": "secret-key"},
                    FailingClient(),
                )
            self.assertEqual(output.read_bytes(), original)
            self.assertEqual(list(output.parent.glob(f"{output.name}.*.bak")), [])

    def test_http_client_retries_rate_limit_without_exposing_key(self) -> None:
        calls: list[urllib.request.Request] = []
        sleeps: list[float] = []
        headers = email.message.Message()
        headers["Retry-After"] = "0"
        responses: list[object] = [
            urllib.error.HTTPError("https://example.test", 429, "limited", headers, None),
            FakeResponse(b"ok"),
        ]

        def opener(request: urllib.request.Request, *, timeout: int) -> object:
            self.assertEqual(timeout, 7)
            calls.append(request)
            response = responses.pop(0)
            if isinstance(response, Exception):
                raise response
            return response

        client = updater.HttpClient(opener=opener, sleeper=sleeps.append, timeout=7, max_retries=1)
        self.assertEqual(
            client.get_text(
                "https://example.test", headers={"x-api-key": "secret-key"}, purpose="test"
            ),
            "ok",
        )
        self.assertEqual(sleeps, [0.0])
        self.assertEqual(calls[0].get_header("X-api-key"), "secret-key")

    def test_http_client_reports_auth_and_persistent_server_errors(self) -> None:
        for status, expected in ((401, "API key was rejected"), (500, "server error persisted")):
            with self.subTest(status=status):
                def opener(request: object, *, timeout: int, status: int = status) -> object:
                    raise urllib.error.HTTPError(
                        "https://example.test", status, "error", email.message.Message(), None
                    )

                client = updater.HttpClient(opener=opener, sleeper=lambda _: None, max_retries=0)
                with self.assertRaisesRegex(updater.UpdateError, expected):
                    client.get_text(
                        "https://example.test",
                        headers={"x-api-key": "secret-key"},
                        purpose="test",
                    )


if __name__ == "__main__":
    unittest.main()
