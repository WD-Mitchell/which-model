#!/usr/bin/env python3
"""Refresh provider, benchmark, and Artificial Analysis API metrics.

The default refresh uses the Artificial Analysis V2 API only. The legacy public
model-page collector remains an explicit opt-in source for compatibility, but
the scheduled workflow never enables it.
"""

from __future__ import annotations

import argparse
import json
import os
import sys
from dataclasses import replace
from datetime import datetime
from pathlib import Path
from typing import Callable, Mapping, Sequence

# These scripts are executable directly as well as importable by repository tests.
SCRIPT_DIR = Path(__file__).resolve().parent
if str(SCRIPT_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPT_DIR))

from get_aa_api_values import (  # noqa: E402
    AA_BENCHMARK_FIELDS,
    API_KEY_NAME,
    API_URL,
    DEFAULT_ENV_PATH,
    MAX_PAGES,
    discover_models,
    fetch_all_models,
    load_api_key,
    load_named_secret,
    match_provider_models,
)
from get_aa_page_values import (  # noqa: E402
    MODEL_PAGE_URL,
    extract_public_page_metrics,
    extract_time_for_slug,
    fetch_public_page_metrics,
)
from get_benchmarks import (  # noqa: E402
    BENCHMARK_HARNESS_PRIORITY,
    MODELS_DEV_BENCHMARK_URL,
    extract_benchmarks,
    fetch_and_attach_benchmarks,
    parse_and_attach_benchmarks,
    _effort as _benchmark_effort,
)
from get_provider_models import (  # noqa: E402
    MODELS_DEV_PROVIDER_URL,
    fetch_provider_models,
    parse_provider_models,
)
from model_config import (  # noqa: E402
    DEFAULT_BENCHMARK_CONFIG_PATH,
    DEFAULT_PROVIDER_CONFIG_PATH,
    load_benchmark_config,
    load_provider_config,
    resolve_provider_ids,
)
from csv_store import (  # noqa: E402
    merge_partial_refresh,
    merge_rows,
    parse_existing_csv,
    render_csv,
    replace_with_backup,
    validate_complete_rows,
)
from http_client import (  # noqa: E402
    HttpClient,
    RejectRedirectHandler,
    REQUEST_TIMEOUT_SECONDS,
    _retry_delay,
)
from model_types import (  # noqa: E402
    BENCHMARK_COLUMN_PREFIX,
    CORE_CSV_COLUMNS,
    EFFORT_ORDER,
    BenchmarkConfiguration,
    ModelFamily,
    ProviderConfiguration,
    ProviderMatchResult,
    ProviderModel,
    PublicPageMetrics,
    RawRow,
    SelectedModel,
    UpdateError,
)


REPOSITORY_ROOT = Path(__file__).resolve().parents[3]
DEFAULT_OUTPUT_PATH = REPOSITORY_ROOT / "available_model_raw_values.csv"
# Keep the legacy page collector available for explicit one-off backfills. It is
# deliberately absent from the default additions set and scheduled workflow.
OPTIONAL_SOURCES = frozenset({"aa_page"})

# Source-specific compatibility aliases retained for importers of the former
# monolith. New code should import the focused collectors directly.
extract_selected_models_dev_benchmarks = extract_benchmarks


def parse_models_dev_catalogue(
    text: str,
    providers: Sequence[str],
    exclusions_by_provider: Mapping[str, Sequence[str]],
    benchmark_names: Sequence[str] = (),
    *,
    reporter: Callable[[str], None] | None = None,
) -> dict[str, list[ProviderModel]]:
    try:
        payload = json.loads(text)
    except json.JSONDecodeError as error:
        raise UpdateError("models.dev catalogues returned invalid JSON") from error
    if not isinstance(payload, dict) or not isinstance(payload.get("providers"), dict) or not isinstance(payload.get("models"), dict):
        raise UpdateError("models.dev payload must contain models and providers mappings")
    available = parse_provider_models(
        json.dumps(payload["providers"]), providers, exclusions_by_provider,
        reporter=reporter,
    )
    return parse_and_attach_benchmarks(
        json.dumps(payload["models"]), available, benchmark_names, reporter=reporter,
    )


def fetch_models_dev_catalogue(
    client: HttpClient,
    providers: Sequence[str],
    exclusions_by_provider: Mapping[str, Sequence[str]],
    benchmark_names: Sequence[str] = (),
    *,
    reporter: Callable[[str], None] | None = None,
) -> dict[str, list[ProviderModel]]:
    available = fetch_provider_models(
        client, providers, exclusions_by_provider, reporter=reporter
    )
    return fetch_and_attach_benchmarks(
        client, available, benchmark_names, reporter=reporter
    )


def collect_rows(
    client: HttpClient,
    selected: Sequence[SelectedModel],
    additions: set[str] | frozenset[str] = frozenset(),
    benchmark_names: Sequence[str] = (),
) -> list[RawRow]:
    unknown = set(additions) - OPTIONAL_SOURCES
    if unknown:
        raise UpdateError("unknown optional sources: " + ", ".join(sorted(unknown)))
    if not selected:
        raise UpdateError("no matched Artificial Analysis models were selected")
    rows: list[RawRow] = []
    for model in selected:
        page_metrics = PublicPageMetrics(None, None)
        if "aa_page" in additions and model.slug is not None:
            page_metrics = fetch_public_page_metrics(
                client, model.slug, require_fallback_cost=model.cost is None
            )
        rows.append(
            RawRow(
                model.family.name,
                model.reasoning,
                model.intelligence,
                page_metrics.time_seconds,
                model.cost if model.cost is not None else page_metrics.fallback_cost,
                median_response_time=model.median_response_time,
                coding_index=model.coding_index,
                agentic_index=model.agentic_index,
                benchmark_values={
                    name: dict(model.benchmarks).get(name) for name in benchmark_names
                },
                authoritative_benchmarks=model.benchmark_clears,
            )
        )
    return rows


def update(
    output_path: Path = DEFAULT_OUTPUT_PATH,
    env_path: Path = DEFAULT_ENV_PATH,
    environ: Mapping[str, str] | None = None,
    client: HttpClient | None = None,
    now: datetime | None = None,
    additions: set[str] | frozenset[str] = frozenset(),
    providers: Sequence[str] | None = None,
    benchmark_config_path: Path = DEFAULT_BENCHMARK_CONFIG_PATH,
    provider_config_path: Path = DEFAULT_PROVIDER_CONFIG_PATH,
    reporter: Callable[[str], None] | None = None,
) -> Path:
    if not output_path.is_file():
        raise UpdateError(f"existing raw CSV not found: {output_path}")
    original = output_path.read_bytes()
    current_rows = parse_existing_csv(original, source=output_path)
    benchmark_config = load_benchmark_config(benchmark_config_path)
    provider_config = load_provider_config(provider_config_path)
    selected_providers = resolve_provider_ids(provider_config, providers)
    http_client = HttpClient() if client is None else client
    discovered = fetch_provider_models(
        http_client,
        selected_providers,
        {
            provider: provider_config.excluded_models_by_provider[provider]
            for provider in selected_providers
        },
        reporter=reporter,
    )
    benchmarked = fetch_and_attach_benchmarks(
        http_client,
        discovered,
        benchmark_config.selected_benchmarks,
        reporter=reporter,
    )
    match = match_provider_models(
        fetch_all_models(http_client, load_api_key(environ, env_path)),
        benchmarked,
    )
    if reporter is not None:
        for provider in selected_providers:
            unmatched = match.unmatched_by_provider[provider]
            matched = match.discovered_by_provider[provider] - len(unmatched)
            reporter(
                f"provider {provider}: discovered={match.discovered_by_provider[provider]} "
                f"matched={matched} unmatched={len(unmatched)}"
            )
            if unmatched:
                shown = ", ".join(unmatched[:20])
                reporter(
                    f"provider {provider} unmatched model IDs: {shown}"
                    + (" ..." if len(unmatched) > 20 else "")
                )
    fresh_rows = collect_rows(
        http_client,
        match.selected,
        additions=additions,
        benchmark_names=benchmark_config.selected_benchmarks,
    )
    preserve_unselected = providers is not None and set(selected_providers) != set(
        provider_config.excluded_models_by_provider
    )
    rows = merge_partial_refresh(
        fresh_rows,
        current_rows,
        match.families,
        preserve_unselected=preserve_unselected,
    )
    validate_complete_rows(rows)
    return replace_with_backup(
        output_path,
        render_csv(rows, benchmark_config.selected_benchmarks),
        now=now,
        expected_original=original,
    )


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, default=DEFAULT_OUTPUT_PATH)
    parser.add_argument("--env-file", type=Path, default=DEFAULT_ENV_PATH)
    parser.add_argument("--benchmark-config", type=Path, default=DEFAULT_BENCHMARK_CONFIG_PATH)
    parser.add_argument("--provider-config", type=Path, default=DEFAULT_PROVIDER_CONFIG_PATH)
    parser.add_argument("--provider", action="append", default=None)
    parser.add_argument("--list-providers", action="store_true")
    parser.add_argument("--add", action="append", choices=sorted(OPTIONAL_SOURCES), default=[])
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    try:
        if args.list_providers:
            configured = load_provider_config(args.provider_config)
            for provider, exclusions in configured.excluded_models_by_provider.items():
                print(f"{provider}: excluded_models={', '.join(exclusions) or 'none'}")
            return 0
        backup = update(
            args.output,
            args.env_file,
            additions=set(args.add),
            providers=args.provider,
            benchmark_config_path=args.benchmark_config,
            provider_config_path=args.provider_config,
            reporter=print,
        )
    except UpdateError as error:
        print(f"error: {error}", file=sys.stderr)
        return 1
    print(f"updated {args.output}; backup: {backup}")
    print(
        ".centree-agentic-framework/available_model_scores.csv was not changed; "
        "regenerate it separately if desired"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
