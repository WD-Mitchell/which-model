#!/usr/bin/env python3
"""Refresh the repository's raw model values without which-model configuration."""

from __future__ import annotations

import argparse
import json
import sys
import tempfile
from pathlib import Path
from typing import Mapping

REPOSITORY_ROOT = Path(__file__).resolve().parents[1]
COLLECTOR_DIR = Path(__file__).resolve().parent
DEFAULT_OUTPUT_PATH = (
    REPOSITORY_ROOT
    / "data"
    / "available_model_raw_values.csv"
)
if str(COLLECTOR_DIR) not in sys.path:
    sys.path.insert(0, str(COLLECTOR_DIR))

import update_raw_values as collector  # noqa: E402


def discover_provider_ids(text: str) -> tuple[str, ...]:
    """Return every provider ID from a models.dev provider catalogue."""
    try:
        payload = json.loads(text)
    except json.JSONDecodeError as error:
        raise collector.UpdateError("models.dev provider catalogue returned invalid JSON") from error
    if not isinstance(payload, dict) or not payload:
        raise collector.UpdateError("models.dev provider catalogue must be a non-empty mapping")
    providers: list[str] = []
    for provider, record in payload.items():
        if not isinstance(provider, str) or not provider or not isinstance(record, dict):
            raise collector.UpdateError(f"models.dev has invalid provider {provider!r}")
        if record.get("id") != provider:
            raise collector.UpdateError(f"models.dev provider {provider!r} has a mismatched id")
        providers.append(provider)
    return tuple(sorted(providers))


def discover_benchmark_names(text: str) -> tuple[str, ...]:
    """Return the union of every models.dev and supported AA benchmark name."""
    names: set[str] = {name for _, name, _ in collector.AA_BENCHMARK_FIELDS}
    try:
        payload = json.loads(text)
    except json.JSONDecodeError as error:
        raise collector.UpdateError("models.dev benchmark catalogue returned invalid JSON") from error
    if not isinstance(payload, dict) or not payload:
        raise collector.UpdateError("models.dev benchmark catalogue must be a non-empty model mapping")
    for model_id, model in payload.items():
        if not isinstance(model_id, str) or not isinstance(model, dict):
            raise collector.UpdateError(f"models.dev has invalid generic model {model_id!r}")
        records = model.get("benchmarks", [])
        if not isinstance(records, list):
            raise collector.UpdateError(f"models.dev model {model_id!r} has invalid benchmarks")
        for index, record in enumerate(records):
            if not isinstance(record, dict) or not isinstance(record.get("name"), str):
                raise collector.UpdateError(
                    f"models.dev model {model_id!r} benchmark {index} is invalid"
                )
            if record["name"]:
                names.add(record["name"])
    if not names:
        raise collector.UpdateError("models.dev benchmark catalogue contains no benchmarks")
    return tuple(sorted(names))


class CachedCatalogueClient:
    """Reuse discovery responses while delegating all other requests."""

    def __init__(self, delegate: collector.HttpClient, cached: Mapping[str, str]) -> None:
        self.delegate = delegate
        self.cached = dict(cached)

    def get_text(self, url: str, **kwargs: object) -> str:
        if url in self.cached:
            return self.cached[url]
        return self.delegate.get_text(url, **kwargs)


def _write_discovered_configs(
    directory: Path, providers: tuple[str, ...], benchmarks: tuple[str, ...]
) -> tuple[Path, Path]:
    provider_path = directory / "providers.toml"
    provider_path.write_text(
        "".join(
            f"[providers.{json.dumps(provider)}]\nexcluded_models = []\n"
            for provider in providers
        ),
        encoding="utf-8",
    )
    benchmark_path = directory / "benchmarks.toml"
    benchmark_path.write_text(
        "[benchmark_selection]\ngroups = []\nbenchmarks = "
        + json.dumps(list(benchmarks), ensure_ascii=False)
        + "\n\n[benchmark_groups.all]\nbenchmarks = "
        + json.dumps(list(benchmarks), ensure_ascii=False)
        + "\n",
        encoding="utf-8",
    )
    return provider_path, benchmark_path


def refresh(output: Path, env_file: Path) -> Path:
    """Discover all providers and benchmarks, then update output atomically."""
    delegate = collector.HttpClient()
    provider_text = delegate.get_text(
        collector.MODELS_DEV_PROVIDER_URL, purpose="models.dev provider model catalogue"
    )
    benchmark_text = delegate.get_text(
        collector.MODELS_DEV_BENCHMARK_URL, purpose="models.dev benchmark catalogue"
    )
    providers = discover_provider_ids(provider_text)
    benchmarks = discover_benchmark_names(benchmark_text)
    cached = CachedCatalogueClient(
        delegate,
        {
            collector.MODELS_DEV_PROVIDER_URL: provider_text,
            collector.MODELS_DEV_BENCHMARK_URL: benchmark_text,
        },
    )
    with tempfile.TemporaryDirectory(prefix="which-model-refresh-") as temp_dir:
        provider_config, benchmark_config = _write_discovered_configs(
            Path(temp_dir), providers, benchmarks
        )
        return collector.update(
            output_path=output,
            env_path=env_file,
            client=cached,
            provider_config_path=provider_config,
            benchmark_config_path=benchmark_config,
            reporter=print,
        )


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--output",
        type=Path,
        default=DEFAULT_OUTPUT_PATH,
    )
    parser.add_argument("--env-file", type=Path, default=REPOSITORY_ROOT / ".env")
    args = parser.parse_args(argv)
    try:
        backup = refresh(args.output, args.env_file)
    except collector.UpdateError as error:
        print(f"error: {error}", file=sys.stderr)
        return 1
    print(f"updated {args.output}; backup: {backup}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
