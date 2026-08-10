#!/usr/bin/env python3
"""Collect and match authenticated Artificial Analysis V2 metrics and benchmarks."""

from __future__ import annotations

import argparse
import json
import os
import re
import sys
import urllib.parse
from dataclasses import replace
from decimal import Decimal, InvalidOperation
from pathlib import Path
from typing import Mapping, Sequence

from http_client import HttpClient
from model_types import (
    EFFORT_ORDER, ModelFamily, ProviderMatchResult, ProviderModel,
    SelectedModel, UpdateError, clean_model_name,
)


REPOSITORY_ROOT = Path(__file__).resolve().parents[3]
DEFAULT_ENV_PATH = REPOSITORY_ROOT / ".env"
# The authenticated endpoint includes the per-benchmark evaluation fields.
# Free-key access may be limited to the headline fields, so the collector
# falls back to the documented free endpoint on an explicit HTTP 403.
API_URL = "https://artificialanalysis.ai/api/v2/language/models"
FREE_API_URL = "https://artificialanalysis.ai/api/v2/language/models/free"
API_KEY_NAME = "ARTIFICIAL_ANALYSIS_API"
MAX_PAGES = 100

# (API evaluation field, generated benchmark column, value is a 0..1 fraction)
# The direct AA indexes are already on a 0..100 scale; the per-evaluation API
# fields are proportions in the V2 response and are converted to percentages.
AA_BENCHMARK_FIELDS = (
    ("artificial_analysis_coding_index", "Artificial Analysis Coding Index", False),
    ("artificial_analysis_agentic_index", "Artificial Analysis Coding Agent Index", False),
    ("tau_banking", "τ3 Banking", True),
    ("tau3_banking", "τ3 Banking", True),
    ("tau2_banking", "τ3 Banking", True),
    ("terminalbench_v2_1", "Terminal-Bench", True),
    ("terminalbench_hard", "Terminal-Bench Hard", True),
    ("scicode", "SciCode", True),
    ("ifbench", "IFBench", True),
    ("ifeval", "IFEval", True),
    ("hle", "Humanity's Last Exam", True),
    ("gpqa_diamond", "GPQA Diamond", True),
    ("mmmu_pro", "MMMU Pro", True),
    ("gdpval_aa_normalized", "GDPval-AA", True),
)


def load_named_secret(
    name: str, environ: Mapping[str, str] | None = None,
    env_path: Path = DEFAULT_ENV_PATH,
) -> str | None:
    environment = os.environ if environ is None else environ
    if environment.get(name, "").strip():
        return environment[name].strip()
    if not env_path.exists():
        return None
    try:
        lines = env_path.read_text(encoding="utf-8").splitlines()
    except OSError as error:
        raise UpdateError(f"cannot read {env_path}: {error}") from error
    matches: list[str] = []
    for line in lines:
        stripped = line.strip()
        if not stripped or stripped.startswith("#") or "=" not in stripped:
            continue
        field, value = stripped.split("=", 1)
        if field.strip() != name:
            continue
        value = value.strip()
        if len(value) >= 2 and value[0] == value[-1] and value[0] in {"'", '"'}:
            value = value[1:-1]
        matches.append(value)
    if len(matches) > 1:
        raise UpdateError(f"{env_path} defines {name} more than once")
    return matches[0] if matches and matches[0] else None


def load_api_key(
    environ: Mapping[str, str] | None = None, env_path: Path = DEFAULT_ENV_PATH
) -> str:
    value = load_named_secret(API_KEY_NAME, environ, env_path)
    if not value:
        raise UpdateError(
            f"missing {API_KEY_NAME}; set it in the environment or in {env_path}"
        )
    return value


def _decimal(value: object, *, field: str, allow_null: bool = False) -> Decimal | None:
    if value is None and allow_null:
        return None
    if isinstance(value, bool):
        raise UpdateError(f"{field} must be numeric")
    try:
        number = Decimal(str(value))
    except (InvalidOperation, ValueError) as error:
        raise UpdateError(f"{field} must be numeric") from error
    if not number.is_finite():
        raise UpdateError(f"{field} must be finite")
    return number


def _fetch_all_models_from_url(
    client: HttpClient, api_key: str, api_url: str,
) -> list[dict[str, object]]:
    result: list[dict[str, object]] = []
    page = 1
    while True:
        url = f"{api_url}?{urllib.parse.urlencode({'page': page})}"
        text = client.get_text(
            url, headers={"x-api-key": api_key},
            purpose=f"Artificial Analysis model list page {page}",
        )
        try:
            payload = json.loads(text, parse_float=Decimal)
        except (json.JSONDecodeError, TypeError) as error:
            raise UpdateError(f"model list page {page} returned invalid JSON") from error
        if not isinstance(payload, dict) or not isinstance(payload.get("data"), list):
            raise UpdateError(f"model list page {page} has an invalid data envelope")
        pagination = payload.get("pagination")
        if not isinstance(pagination, dict):
            raise UpdateError(f"model list page {page} has no valid pagination object")
        returned, more, total = (
            pagination.get("page"), pagination.get("has_more"), pagination.get("total_pages")
        )
        if returned != page or not isinstance(more, bool) or not isinstance(total, int):
            raise UpdateError(f"model list page {page} has invalid pagination values")
        if total < page or total > MAX_PAGES:
            raise UpdateError(f"model list reports invalid total_pages={total}")
        entries = payload["data"]
        if not all(isinstance(item, dict) for item in entries):
            raise UpdateError(f"model list page {page} contains a malformed model entry")
        result.extend(entries)
        if not more:
            if page != total:
                raise UpdateError("model list pagination ended before total_pages")
            return result
        if page >= total:
            raise UpdateError("model list pagination has_more exceeds total_pages")
        page += 1


def fetch_all_models(client: HttpClient, api_key: str) -> list[dict[str, object]]:
    """Fetch full AA evaluations, falling back only when the key lacks access."""
    try:
        return _fetch_all_models_from_url(client, api_key, API_URL)
    except UpdateError as error:
        # A rate limit, malformed response, or transport error must not be
        # hidden by a second request.  Only a documented access restriction is
        # safe to retry against the free endpoint.
        if "HTTP 403" not in str(error):
            raise
        return _fetch_all_models_from_url(client, api_key, FREE_API_URL)


def _tokens(value: str) -> tuple[str, ...]:
    return tuple(re.findall(r"[a-z0-9]+", value.casefold().replace("+", " plus ").replace(".", " ")))


def _name_parts(name: str) -> tuple[str, str | None]:
    match = re.fullmatch(r"(.+)\s+\(([^()]*)\)", name.strip())
    if match is None:
        return clean_model_name(name), None
    configuration = match.group(2).strip()
    if not re.search(
        r"\b(?:minimal|low|medium|high|xhigh|extra[- ]high|max(?:imum)?|thinking|adaptive|non[- ]reasoning|reasoning|effort)\b",
        configuration, re.IGNORECASE,
    ):
        return clean_model_name(name), None
    return clean_model_name(match.group(1)), configuration


def _provider_keys(model: ProviderModel) -> tuple[tuple[str, ...], ...]:
    identifier = re.sub(r"-picker\Z", "", model.model_id)
    display_name = clean_model_name(model.display_name)
    keys = tuple(
        dict.fromkeys(
            _tokens(value)
            for value in (display_name, identifier)
            if value
        )
    )
    if not keys or any(not key for key in keys):
        raise UpdateError(f"{model.provider} returned unmatchable model identity {model.model_id!r}")
    return keys


def _root_slug(slug: str) -> str:
    """Return a family root shared by dated/latest provider snapshots."""

    root = re.sub(r"-(?:\d{8}|\d{4}-\d{2}-\d{2})\Z", "", slug)
    return re.sub(r"-latest\Z", "", root, flags=re.IGNORECASE)


def _reasoning(configuration: str, *, slug: str) -> str | None:
    normalized = configuration.casefold().replace("_", "-")
    if "non-reasoning" in normalized or "non reasoning" in normalized:
        return None
    aliases = (
        (r"\b(?:xhigh|extra[- ]high)\b", "xhigh"),
        (r"\bminimal(?: effort)?\b", "minimal"), (r"\blow(?: effort)?\b", "low"),
        (r"\bmedium(?: effort)?\b", "medium"), (r"\bhigh(?: effort)?\b", "high"),
        (r"\bmax(?:imum)?(?: effort)?\b", "max"),
    )
    labels = [label for pattern, label in aliases if re.search(pattern, normalized)]
    if len(set(labels)) > 1:
        raise UpdateError(f"{slug} has ambiguous reasoning configuration {configuration!r}")
    if labels:
        label = labels[0]
        fallback = re.search(r"\b([a-z]+\s+\d+(?:\.\d+)?)\s+fallback\b", normalized)
        if fallback:
            words = fallback.group(1).split()
            label += f" ({words[0].capitalize()} {words[1]} fallback)"
        return label
    if re.search(r"\bthinking\b", normalized) or normalized.strip() == "reasoning":
        return "thinking"
    if re.search(r"\badaptive(?: reasoning)?\b", normalized):
        return "adaptive"
    raise UpdateError(f"{slug} has unrecognized reasoning configuration {configuration!r}")


def _normalise_reasoning_level(level: str) -> str:
    """Treat a provider/API default configuration as the high-effort row."""
    return "high" if level == "default" else level


def _aa_benchmarks(
    slug: str, evaluations: Mapping[str, object],
) -> tuple[tuple[str, Decimal], ...]:
    values: dict[str, Decimal] = {}
    for field, name, fraction in AA_BENCHMARK_FIELDS:
        if field not in evaluations:
            continue
        value = _decimal(
            evaluations[field], field=f"{slug} {field}", allow_null=True
        )
        if value is None:
            continue
        if fraction and Decimal("0") <= value <= Decimal("1"):
            value *= Decimal("100")
        previous = values.get(name)
        if previous is None or value > previous:
            values[name] = value
    return tuple(values.items())


def _highest_benchmark_values(
    *sources: Mapping[str, Decimal],
) -> dict[str, Decimal]:
    """Merge benchmark evidence, keeping the highest value for each name."""
    result: dict[str, Decimal] = {}
    for source in sources:
        for name, value in source.items():
            previous = result.get(name)
            if previous is None or value > previous:
                result[name] = value
    return result


def _selected(item: dict[str, object], family: ModelFamily, reasoning: str) -> SelectedModel:
    slug, evaluations, performance = item.get("slug"), item.get("evaluations"), item.get("performance")
    if not isinstance(slug, str):
        raise UpdateError("Artificial Analysis model is missing slug")
    if not isinstance(evaluations, dict):
        raise UpdateError(f"{slug} is missing evaluations data")
    if not isinstance(performance, dict):
        raise UpdateError(f"{slug} is missing performance data")
    fields = (
        "artificial_analysis_intelligence_index", "artificial_analysis_coding_index",
        "artificial_analysis_agentic_index",
    )
    for field in fields:
        if field not in evaluations:
            raise UpdateError(f"{slug} evaluations is missing {field}")
    response_field = "median_end_to_end_response_time_seconds"
    if response_field not in performance:
        raise UpdateError(f"{slug} performance is missing {response_field}")
    values = [
        _decimal(evaluations[field], field=f"{slug} {field}", allow_null=True)
        for field in fields
    ]
    response = _decimal(performance[response_field], field=f"{slug} {response_field}", allow_null=True)
    if response is not None and response < 0:
        raise UpdateError(f"{slug} median end-to-end response time must not be negative")
    cost_field = "artificial_analysis_intelligence_index_cost"
    if cost_field not in item:
        raise UpdateError(f"{slug} is missing {cost_field}")
    cost_data, cost = item[cost_field], None
    if cost_data is not None:
        if not isinstance(cost_data, dict):
            raise UpdateError(f"{slug} has malformed cost data")
        if "cost_per_task" not in cost_data:
            raise UpdateError(f"{slug} cost data is missing cost_per_task")
        task = cost_data["cost_per_task"]
        if task is not None:
            if not isinstance(task, dict):
                raise UpdateError(f"{slug} has malformed cost_per_task")
            if "total_cost" not in task:
                raise UpdateError(f"{slug} cost_per_task is missing total_cost")
            cost = _decimal(task["total_cost"], field=f"{slug} cost per task", allow_null=True)
    if cost is not None and cost < 0:
        raise UpdateError(f"{slug} cost per task must not be negative")
    return SelectedModel(
        family, reasoning, slug, values[0], cost, response, values[1], values[2],
        benchmarks=_aa_benchmarks(slug, evaluations),
    )


def match_provider_models(
    api_models: Sequence[dict[str, object]],
    provider_models: Mapping[str, Sequence[ProviderModel]],
) -> ProviderMatchResult:
    groups: dict[tuple[str, ...], list[dict[str, object]]] = {}
    names: dict[tuple[str, ...], str] = {}
    for index, item in enumerate(api_models):
        name, slug = item.get("name"), item.get("slug")
        if not isinstance(name, str) or not name.strip() or not isinstance(slug, str) or not slug:
            raise UpdateError(f"Artificial Analysis model entry {index} has invalid identity")
        base, _ = _name_parts(name)
        if not base:
            raise UpdateError(f"Artificial Analysis model entry {index} has an empty model name")
        key = _tokens(base)
        if not key:
            raise UpdateError(f"Artificial Analysis model entry {index} has an unmatchable model name")
        previous = names.setdefault(key, base)
        if previous != base:
            raise UpdateError(f"ambiguous Artificial Analysis family names {previous!r} and {base!r}")
        groups.setdefault(key, []).append(item)

    family_by_key: dict[tuple[str, ...], ModelFamily] = {}
    aa_by_family: dict[str, tuple[SelectedModel, ...]] = {}
    for key, items in groups.items():
        roots: set[str] = set()
        for item in items:
            item_name, slug = item["name"], item["slug"]
            assert isinstance(item_name, str) and isinstance(slug, str)
            _, config = _name_parts(item_name)
            if config is None:
                roots.add(_root_slug(slug))
                continue
            level = _reasoning(config, slug=slug)
            if level is None and slug.endswith("-non-reasoning"):
                roots.add(_root_slug(slug.removesuffix("-non-reasoning")))
                continue
            simple = None if level is None else level.split(" ", 1)[0]
            suffix = f"-{simple}" if simple else ""
            root = slug[: -len(suffix)] if suffix and slug.endswith(suffix) else slug
            roots.add(_root_slug(root))
        if len(roots) != 1:
            continue
        family = ModelFamily(names[key], roots.pop())
        selected_by_level: dict[str, SelectedModel] = {}
        seen_slugs: set[str] = set()
        for item in items:
            name, slug = item["name"], item["slug"]
            assert isinstance(name, str) and isinstance(slug, str)
            _, config = _name_parts(name)
            non_reasoning = (
                item.get("reasoning_model") is False
                or "non-reasoning" in slug.casefold()
                or "non-reasoning" in name.casefold()
            )
            if non_reasoning and config is not None:
                continue
            level = "default" if non_reasoning or config is None else _reasoning(config, slug=slug)
            level = "default" if level is None else level
            if slug in seen_slugs:
                raise UpdateError(f"duplicate Artificial Analysis model slug {slug!r}")
            seen_slugs.add(slug)
            candidate = _selected(item, family, level)
            existing = selected_by_level.get(level)
            if existing is None:
                selected_by_level[level] = candidate
                continue
            merged = _highest_benchmark_values(
                dict(existing.benchmarks), dict(candidate.benchmarks)
            )
            selected_by_level[level] = replace(
                existing,
                intelligence=(
                    existing.intelligence
                    if existing.intelligence is not None
                    else candidate.intelligence
                ),
                cost=(existing.cost if existing.cost is not None else candidate.cost),
                median_response_time=(
                    existing.median_response_time
                    if existing.median_response_time is not None
                    else candidate.median_response_time
                ),
                coding_index=(
                    existing.coding_index
                    if existing.coding_index is not None
                    else candidate.coding_index
                ),
                agentic_index=(
                    existing.agentic_index
                    if existing.agentic_index is not None
                    else candidate.agentic_index
                ),
                benchmarks=tuple(merged.items()),
                benchmark_clears=(
                    existing.benchmark_clears | candidate.benchmark_clears
                ),
            )
        selected = list(selected_by_level.values())
        selected.sort(key=lambda model: (
            EFFORT_ORDER.get(model.reasoning.split(" ", 1)[0], len(EFFORT_ORDER)),
            model.reasoning, model.slug or "",
        ))
        if selected:
            family_by_key[key] = family
            aa_by_family[family.base_slug] = tuple(selected)

    aggregates: list[dict[str, object]] = []
    by_key: dict[tuple[str, ...], int] = {}
    by_canonical: dict[str, int] = {}
    unmatched_by_provider: dict[str, tuple[str, ...]] = {}
    counts: dict[str, int] = {}
    for provider, models in provider_models.items():
        counts[provider] = len(models)
        unmatched: list[str] = []
        for model in sorted(models, key=lambda model: (_provider_keys(model), model.model_id)):
            keys = _provider_keys(model)
            family = next(
                (family_by_key[key] for key in keys if key in family_by_key),
                None,
            )
            if family is None:
                unmatched.append(model.model_id)
            indexes = {by_key[key] for key in keys if key in by_key}
            if model.canonical_id is not None and model.canonical_id in by_canonical:
                indexes.add(by_canonical[model.canonical_id])
            if len(indexes) > 1:
                target_index = min(indexes)
                target = aggregates[target_index]
                winning_family = family or target["family"]
                if winning_family is None:
                    winning_family = next(
                        (
                            aggregates[index]["family"]
                            for index in sorted(indexes - {target_index})
                            if aggregates[index]["family"] is not None
                        ),
                        None,
                    )
                if winning_family is not None:
                    target["family"], target["name"] = winning_family, winning_family.name
                for source_index in sorted(indexes - {target_index}):
                    source = aggregates[source_index]
                    if target["family"] is None and source["family"] is not None:
                        target["family"], target["name"] = source["family"], source["name"]
                    target_levels, source_levels = target["levels"], source["levels"]
                    assert isinstance(target_levels, set) and isinstance(source_levels, set)
                    target_levels.update(source_levels)
                    target_benchmarks, source_benchmarks = target["benchmarks"], source["benchmarks"]
                    assert isinstance(target_benchmarks, dict) and isinstance(source_benchmarks, dict)
                    for name, value in source_benchmarks.items():
                        previous = target_benchmarks.get(name)
                        if previous is None or value > previous:
                            target_benchmarks[name] = value
                    target_overrides, source_overrides = target["overrides"], source["overrides"]
                    assert isinstance(target_overrides, dict) and isinstance(source_overrides, dict)
                    for effort, values in source_overrides.items():
                        merged_values = target_overrides.setdefault(effort, {})
                        for name, value in values.items():
                            previous = merged_values.get(name)
                            if previous is None or value > previous:
                                merged_values[name] = value
                    for key, index in tuple(by_key.items()):
                        if index == source_index:
                            by_key[key] = target_index
                    for key, index in tuple(by_canonical.items()):
                        if index == source_index:
                            by_canonical[key] = target_index
                    aggregates[source_index] = {}
                indexes = {target_index}
            if indexes:
                aggregate = aggregates[next(iter(indexes))]
                if family is not None:
                    aggregate["family"], aggregate["name"] = family, family.name
                index = next(iter(indexes))
            else:
                index = len(aggregates)
                aggregate = {
                    "family": family,
                    "name": family.name if family else clean_model_name(model.display_name),
                    "model_id": model.model_id, "levels": set(),
                    "benchmarks": {}, "overrides": {},
                }
                aggregates.append(aggregate)
            for key in keys:
                by_key[key] = index
            if model.canonical_id is not None:
                by_canonical[model.canonical_id] = index
            aggregate_benchmarks = aggregate["benchmarks"]
            assert isinstance(aggregate_benchmarks, dict)
            for name, value in model.benchmarks:
                previous = aggregate_benchmarks.get(name)
                if previous is None or value > previous:
                    aggregate_benchmarks[name] = value
            aggregate_overrides = aggregate["overrides"]
            assert isinstance(aggregate_overrides, dict)
            for effort, values in model.benchmark_overrides:
                target = aggregate_overrides.setdefault(effort, {})
                for name, value in values:
                    previous = target.get(name)
                    if previous is None or value > previous:
                        target[name] = value
            levels = aggregate["levels"]
            assert isinstance(levels, set)
            levels.update(_normalise_reasoning_level(level) for level in model.reasoning_levels)
        unmatched_by_provider[provider] = tuple(unmatched)
    if not aggregates:
        raise UpdateError("selected models.dev providers have no current models after exclusions")

    families: list[ModelFamily] = []
    selected_models: list[SelectedModel] = []
    for aggregate in aggregates:
        if not aggregate:
            continue
        matched = aggregate["family"]
        family = matched or ModelFamily(str(aggregate["name"]), str(aggregate["model_id"]))
        assert isinstance(family, ModelFamily)
        families.append(family)
        aa_models = aa_by_family.get(matched.base_slug, ()) if isinstance(matched, ModelFamily) else ()
        # A default AA model is evidence for the high row.  If an explicit
        # high model is also present, it is visited first and its benchmark
        # cells are merged with the default using the maximum value.
        aa_levels: dict[str, SelectedModel] = {}
        for candidate in aa_models:
            level = _normalise_reasoning_level(candidate.reasoning)
            previous = aa_levels.get(level)
            if previous is None:
                aa_levels[level] = replace(candidate, reasoning=level)
            else:
                merged = _highest_benchmark_values(
                    dict(previous.benchmarks), dict(candidate.benchmarks)
                )
                aa_levels[level] = replace(
                    previous, benchmarks=tuple(merged.items())
                )
        levels = aggregate["levels"]; benchmarks = aggregate["benchmarks"]; overrides = aggregate["overrides"]
        assert isinstance(levels, set) and isinstance(benchmarks, dict) and isinstance(overrides, dict)
        normalised_overrides: dict[str, dict[str, Decimal]] = {}
        for effort, values in overrides.items():
            target = normalised_overrides.setdefault(
                _normalise_reasoning_level(effort), {}
            )
            for name, value in values.items():
                previous = target.get(name)
                if previous is None or value > previous:
                    target[name] = value
        scoped_names = {name for values in normalised_overrides.values() for name in values}
        for level in sorted(levels, key=lambda value: (EFFORT_ORDER.get(value, len(EFFORT_ORDER)), value)):
            aa_model = aa_levels.get(level)
            if aa_model is None:
                effort_matches = [
                    model for model in aa_levels.values()
                    if model.reasoning.split(" ", 1)[0] == level
                ]
                if len(effort_matches) > 1:
                    raise UpdateError(f"{family.name} effort {level!r} ambiguously matches AA variants")
                aa_model = effort_matches[0] if effort_matches else None
            aa_values = {} if aa_model is None else dict(aa_model.benchmarks)
            values = _highest_benchmark_values(
                benchmarks if level == "high" else {},
                normalised_overrides.get(level, {}),
                aa_values,
            )
            clears = frozenset(scoped_names - set(values))
            if aa_model:
                selected_models.append(replace(
                    aa_model, reasoning=level, benchmarks=tuple(values.items()),
                    benchmark_clears=clears,
                ))
            else:
                selected_models.append(SelectedModel(
                    family, level, None, None, None,
                    benchmarks=tuple(values.items()), benchmark_clears=clears,
                ))
    return ProviderMatchResult(
        tuple(selected_models), tuple(families), unmatched_by_provider,
        counts, {provider: 0 for provider in provider_models},
    )


def discover_models(
    api_models: Sequence[dict[str, object]], *, families: Sequence[ModelFamily]
) -> list[SelectedModel]:
    fixtures: list[ProviderModel] = []
    for family in families:
        levels: list[str] = []
        for item in api_models:
            name = item.get("name")
            if not isinstance(name, str) or _name_parts(name)[0] != family.name:
                continue
            _, config = _name_parts(name)
            level = "default" if config is None else _reasoning(config, slug=str(item.get("slug", "")))
            if level is not None:
                level = _normalise_reasoning_level(level)
                if level not in levels: levels.append(level)
        fixtures.append(ProviderModel("fixture", family.base_slug, family.name, tuple(levels)))
    return list(match_provider_models(api_models, {"fixture": fixtures}).selected)


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--env-file", type=Path, default=DEFAULT_ENV_PATH)
    args = parser.parse_args(argv)
    try:
        client = HttpClient()
        models = fetch_all_models(client, load_api_key(env_path=args.env_file))
    except UpdateError as error:
        print(f"error: {error}", file=sys.stderr); return 1
    diagnostic: list[dict[str, object]] = []
    for model in models:
        evaluations = model.get("evaluations")
        performance = model.get("performance")
        cost_envelope = model.get("artificial_analysis_intelligence_index_cost")
        task_cost = (
            cost_envelope.get("cost_per_task")
            if isinstance(cost_envelope, dict) else None
        )
        diagnostic.append({
            "name": model.get("name"),
            "slug": model.get("slug"),
            "reasoning_model": model.get("reasoning_model"),
            "intelligence_index": evaluations.get("artificial_analysis_intelligence_index")
            if isinstance(evaluations, dict) else None,
            "coding_index": evaluations.get("artificial_analysis_coding_index")
            if isinstance(evaluations, dict) else None,
            "agentic_index": evaluations.get("artificial_analysis_agentic_index")
            if isinstance(evaluations, dict) else None,
            "median_end_to_end_response_time_seconds": performance.get(
                "median_end_to_end_response_time_seconds"
            ) if isinstance(performance, dict) else None,
            "cost_per_intelligence_index_task_usd": task_cost.get("total_cost")
            if isinstance(task_cost, dict) else None,
        })
    json.dump(
        diagnostic,
        sys.stdout, indent=2, ensure_ascii=False, default=str,
    )
    print(); return 0


if __name__ == "__main__":
    raise SystemExit(main())
