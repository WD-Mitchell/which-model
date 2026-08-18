#!/usr/bin/env python3
"""Attach models.dev benchmarks, resolving duplicate evidence by maximum score."""

from __future__ import annotations

import argparse
import json
import re
import sys
from dataclasses import replace
from decimal import Decimal, InvalidOperation
from pathlib import Path
from typing import Callable, Mapping, Sequence

from model_config import (
    DEFAULT_BENCHMARK_CONFIG_PATH,
    load_benchmark_config,
)
from http_client import HttpClient
from model_types import ProviderModel, UpdateError, clean_model_name


MODELS_DEV_BENCHMARK_URL = "https://models.dev/models.json"
BENCHMARK_HARNESS_PRIORITY = (
    "Codex", "Claude Code", "Terminus-2", "Mini-SWE-Agent", "Cursor CLI"
)
# Retained as an import-compatible constant for older callers.  Resolution is
# now intentionally source-agnostic and uses the highest numeric value.

def _number(value: object, *, field: str) -> Decimal:
    if isinstance(value, bool):
        raise UpdateError(f"{field} must be numeric")
    try:
        number = Decimal(str(value))
    except (InvalidOperation, ValueError) as error:
        raise UpdateError(f"{field} must be numeric") from error
    if not number.is_finite():
        raise UpdateError(f"{field} must be finite")
    return number


def _effort(record: Mapping[str, object]) -> str | None:
    variant = record.get("variant")
    if not isinstance(variant, str):
        return None
    normalized = variant.casefold().replace("_", " ").replace("-", " ").strip()
    efforts = r"minimal|low|medium|high|xhigh|max"
    suffix = r"(?:, (?:context compaction|with tools))?"
    match = re.fullmatch(
        rf"(?P<effort>{efforts})(?: effort| reasoning)?{suffix}", normalized
    ) or re.fullmatch(
        rf"reasoning effort (?P<effort>none|{efforts}){suffix}", normalized
    )
    if match is None:
        return None
    return "default" if match.group("effort") == "none" else match.group("effort")


def _version(value: object, *, field: str) -> tuple[int, ...]:
    if value is None:
        return (0,)
    if not isinstance(value, str):
        raise UpdateError(f"{field} has an invalid version")
    normalized = value.strip()
    if re.fullmatch(r"v?\d+(?:\.\d+)*", normalized):
        return tuple(int(part) for part in normalized.removeprefix("v").split("."))
    month = re.fullmatch(
        r"(?P<month>January|February|March|April|May|June|July|August|"
        r"September|October|November|December)\s+(?P<year>\d{4})",
        normalized,
        re.IGNORECASE,
    )
    if month:
        month_number = (
            "january february march april may june july august september october "
            "november december"
        ).split().index(month.group("month").casefold()) + 1
        return int(month.group("year")), month_number
    raise UpdateError(f"{field} has an invalid version")


def _resolve(records: Sequence[Mapping[str, object]], *, field: str) -> Decimal:
    """Resolve duplicate benchmark evidence using the highest numeric score."""
    scores = [
        _number(record.get("score"), field=f"{field} score")
        for record in records
    ]
    if not scores:
        raise UpdateError(f"{field} has no scores")
    return max(scores)


def extract_benchmarks(
    model_id: str, model: Mapping[str, object], selected_names: Sequence[str]
) -> tuple[
    tuple[tuple[str, Decimal], ...],
    tuple[tuple[str, tuple[tuple[str, Decimal], ...]], ...],
]:
    records = model.get("benchmarks", [])
    if not isinstance(records, list):
        raise UpdateError(f"models.dev model {model_id!r} has invalid benchmarks")
    by_name: dict[str, list[Mapping[str, object]]] = {name: [] for name in selected_names}
    for index, record in enumerate(records):
        if not isinstance(record, dict) or not isinstance(record.get("name"), str):
            raise UpdateError(f"models.dev model {model_id!r} benchmark {index} is invalid")
        if record["name"] in by_name:
            by_name[record["name"]].append(record)
    defaults: list[tuple[str, Decimal]] = []
    overrides: dict[str, list[tuple[str, Decimal]]] = {}
    for name in selected_names:
        scoped: dict[str | None, list[Mapping[str, object]]] = {}
        for record in by_name[name]:
            scoped.setdefault(_effort(record), []).append(record)
        for effort, candidates in scoped.items():
            # Validate every version, but deliberately do not discard older
            # records or prefer a harness.  The collection policy is simple:
            # when the same benchmark has multiple numeric values, retain the
            # highest value, regardless of source metadata.
            for record in candidates:
                _version(record.get("version"), field=f"models.dev {model_id!r} {name!r}")
            score = _resolve(
                candidates,
                field=f"models.dev model {model_id!r} benchmark {name!r}"
                + (f" effort {effort}" if effort else ""),
            )
            (defaults if effort is None else overrides.setdefault(effort, [])).append(
                (name, score)
            )
    return tuple(defaults), tuple(
        (effort, tuple(values)) for effort, values in sorted(overrides.items())
    )


def parse_and_attach_benchmarks(
    text: str,
    provider_models: Mapping[str, Sequence[ProviderModel]],
    benchmark_names: Sequence[str],
    *,
    reporter: Callable[[str], None] | None = None,
) -> dict[str, list[ProviderModel]]:
    try:
        models = json.loads(text)
    except json.JSONDecodeError as error:
        raise UpdateError("models.dev benchmark catalogue returned invalid JSON") from error
    if not isinstance(models, dict) or not models:
        raise UpdateError("models.dev benchmark catalogue must be a non-empty model mapping")
    metadata: dict[str, tuple[str, Mapping[str, object]]] = {}
    by_suffix: dict[str, list[str]] = {}
    by_name: dict[str, list[str]] = {}
    available: set[str] = set()
    for canonical_id, model in models.items():
        if (
            not isinstance(canonical_id, str) or not canonical_id
            or not isinstance(model, dict) or model.get("id") != canonical_id
            or not isinstance(model.get("name"), str) or not model["name"].strip()
        ):
            raise UpdateError(f"models.dev has invalid generic model {canonical_id!r}")
        cleaned_name = clean_model_name(model["name"])
        if not cleaned_name:
            raise UpdateError(f"models.dev generic model {canonical_id!r} has an invalid name")
        metadata[canonical_id] = (cleaned_name, model)
        by_suffix.setdefault(canonical_id.split("/", 1)[-1], []).append(canonical_id)
        by_name.setdefault(cleaned_name, []).append(canonical_id)
        records = model.get("benchmarks", [])
        if not isinstance(records, list):
            raise UpdateError(f"models.dev model {canonical_id!r} has invalid benchmarks")
        available.update(
            record["name"] for record in records
            if isinstance(record, dict) and isinstance(record.get("name"), str)
        )
    missing = [name for name in benchmark_names if name not in available]
    if reporter and missing:
        reporter(
            "selected benchmarks absent from current models.dev catalogue: "
            + ", ".join(missing)
        )
    result: dict[str, list[ProviderModel]] = {}
    for provider, entries in provider_models.items():
        attached: list[ProviderModel] = []
        for entry in entries:
            canonical = entry.canonical_id
            if canonical is not None and canonical not in metadata:
                raise UpdateError(
                    f"models.dev {provider} model {entry.model_id!r} references unknown base_model {canonical!r}"
                )
            if canonical is None:
                direct = f"{provider}/{entry.model_id}"
                if direct in metadata:
                    canonical = direct
                else:
                    candidates = list(
                        dict.fromkeys(
                            [
                                *by_suffix.get(entry.model_id, []),
                                *by_name.get(clean_model_name(entry.display_name), []),
                            ]
                        )
                    )
                    canonical = candidates[0] if candidates else None
            candidate_ids = (
                [canonical]
                if canonical is not None
                else []
            )
            if entry.canonical_id is None and canonical is not None:
                # A normalized name may identify dated/latest snapshots in
                # addition to the first canonical match.  Merge all of their
                # selected benchmark evidence instead of treating the
                # annotations as an ambiguity.
                candidate_ids = list(
                    dict.fromkeys(
                        [
                            *by_suffix.get(entry.model_id, []),
                            *by_name.get(clean_model_name(entry.display_name), []),
                        ]
                    )
                ) or [canonical]
            default_values: dict[str, Decimal] = {}
            override_values: dict[str, dict[str, Decimal]] = {}
            for candidate_id in candidate_ids:
                defaults, overrides = extract_benchmarks(
                    candidate_id, metadata[candidate_id][1], benchmark_names
                )
                for name, value in defaults:
                    previous = default_values.get(name)
                    if previous is None or value > previous:
                        default_values[name] = value
                for effort, values in overrides:
                    target = override_values.setdefault(effort, {})
                    for name, value in values:
                        previous = target.get(name)
                        if previous is None or value > previous:
                            target[name] = value
            values = tuple(default_values.items())
            overrides = tuple(
                (effort, tuple(values.items()))
                for effort, values in sorted(override_values.items())
            )
            attached.append(
                replace(
                    entry, canonical_id=canonical, benchmarks=values,
                    benchmark_overrides=overrides,
                )
            )
        result[provider] = attached
    return result


def fetch_and_attach_benchmarks(
    client: HttpClient,
    provider_models: Mapping[str, Sequence[ProviderModel]],
    benchmark_names: Sequence[str],
    *,
    reporter: Callable[[str], None] | None = None,
) -> dict[str, list[ProviderModel]]:
    text = client.get_text(
        MODELS_DEV_BENCHMARK_URL, purpose="models.dev benchmark catalogue"
    )
    return parse_and_attach_benchmarks(
        text, provider_models, benchmark_names, reporter=reporter
    )


def parse_benchmark_diagnostic(
    text: str, benchmark_names: Sequence[str]
) -> dict[str, dict[str, object]]:
    """Return selected benchmark data keyed by canonical models.dev model ID."""
    try:
        models = json.loads(text)
    except json.JSONDecodeError as error:
        raise UpdateError("models.dev benchmark catalogue returned invalid JSON") from error
    if not isinstance(models, dict) or not models:
        raise UpdateError("models.dev benchmark catalogue must be a non-empty model mapping")
    diagnostic: dict[str, dict[str, object]] = {}
    for canonical_id, model in models.items():
        if (
            not isinstance(canonical_id, str)
            or not isinstance(model, dict)
            or model.get("id") != canonical_id
            or not isinstance(model.get("name"), str)
        ):
            raise UpdateError(f"models.dev has invalid generic model {canonical_id!r}")
        defaults, overrides = extract_benchmarks(
            canonical_id, model, benchmark_names
        )
        if defaults or overrides:
            diagnostic[canonical_id] = {
                "name": model["name"],
                "benchmarks": dict(defaults),
                "effort_overrides": {
                    effort: dict(values) for effort, values in overrides
                },
            }
    return diagnostic


def fetch_benchmark_diagnostic(
    client: HttpClient, benchmark_names: Sequence[str]
) -> dict[str, dict[str, object]]:
    text = client.get_text(
        MODELS_DEV_BENCHMARK_URL, purpose="models.dev benchmark catalogue"
    )
    return parse_benchmark_diagnostic(text, benchmark_names)


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--benchmark-config", type=Path, default=DEFAULT_BENCHMARK_CONFIG_PATH)
    args = parser.parse_args(argv)
    try:
        benchmark_config = load_benchmark_config(args.benchmark_config)
        diagnostic = fetch_benchmark_diagnostic(
            HttpClient(), benchmark_config.selected_benchmarks
        )
    except UpdateError as error:
        print(f"error: {error}", file=sys.stderr)
        return 1
    json.dump(
        diagnostic, sys.stdout, ensure_ascii=False, indent=2, default=str,
    )
    print()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
