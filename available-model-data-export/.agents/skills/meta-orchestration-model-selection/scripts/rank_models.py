#!/usr/bin/env python3
"""Rank exact model/reasoning rows from the generated score CSV."""

from __future__ import annotations

import argparse
import csv
import json
import sys
from dataclasses import dataclass
from decimal import Decimal, InvalidOperation
from pathlib import Path
from typing import Iterable, Mapping, Sequence


REPOSITORY_ROOT = Path(__file__).resolve().parents[4]
DEFAULT_SCORE_PATH = (
    REPOSITORY_ROOT / ".centree-agentic-framework/available_model_scores.csv"
)

TIER1_COLUMNS = {
    "intelligence": "intelligence_index_score",
    "cost": "cost_per_intelligence_index_task_usd_score",
    "speed": "median_end_to_end_response_time_seconds_score",
}
CATEGORY_NAMES = (
    "reasoning",
    "knowledge",
    "research",
    "planning_capability",
    "instruction_following",
    "software_engineering",
    "ui_visual",
    "agentic_tools",
    "finance",
    "evidence_capture",
    "security",
    "data_ml",
)
CATEGORY_COLUMNS = {name: f"{name}_score" for name in CATEGORY_NAMES}


class RankingError(ValueError):
    """Raised when scores, weights, or availability cannot be used safely."""


@dataclass(frozen=True)
class Profile:
    name: str
    tier1_share: Decimal
    tier2_share: Decimal
    tier1_weights: Mapping[str, Decimal]
    tier2_weights: Mapping[str, Decimal]


def _decimal(value: object, *, field: str) -> Decimal:
    if isinstance(value, bool):
        raise RankingError(f"{field} must be numeric")
    try:
        number = Decimal(str(value))
    except (InvalidOperation, ValueError) as error:
        raise RankingError(f"{field} must be numeric") from error
    if not number.is_finite():
        raise RankingError(f"{field} must be finite")
    return number


def _weights(values: Mapping[str, object], *, tier: str) -> dict[str, Decimal]:
    result: dict[str, Decimal] = {}
    for name, value in values.items():
        if not isinstance(name, str) or not name.strip():
            raise RankingError(f"{tier} weight names must be non-blank strings")
        number = _decimal(value, field=f"{tier} weight {name}")
        if number < 0 or number > 5:
            raise RankingError(f"{tier} weight {name} must be between 0 and 5")
        result[name] = number
    return result


def validate_profile(profile: Profile) -> Profile:
    if profile.tier1_share <= 0 or profile.tier2_share < 0:
        raise RankingError("tier 1 share must be positive and tier 2 share cannot be negative")
    if profile.tier1_share + profile.tier2_share != 100:
        raise RankingError("tier 1 and tier 2 shares must sum to 100")
    if set(profile.tier1_weights) != set(TIER1_COLUMNS):
        missing = sorted(set(TIER1_COLUMNS) - set(profile.tier1_weights))
        unknown = sorted(set(profile.tier1_weights) - set(TIER1_COLUMNS))
        details = []
        if missing:
            details.append("missing " + ", ".join(missing))
        if unknown:
            details.append("unknown " + ", ".join(unknown))
        raise RankingError("tier 1 weights must include intelligence, cost, and speed (" + "; ".join(details) + ")")
    for name, weight in profile.tier1_weights.items():
        if weight <= 0 or weight > 5:
            raise RankingError(f"tier 1 weight {name} must be greater than 0 and at most 5")
    unknown_categories = sorted(set(profile.tier2_weights) - set(CATEGORY_NAMES))
    if unknown_categories:
        raise RankingError("unknown tier 2 categories: " + ", ".join(unknown_categories))
    for name, weight in profile.tier2_weights.items():
        if weight <= 0 or weight > 5:
            raise RankingError(f"tier 2 weight {name} must be greater than 0 and at most 5")
    return profile


def _profile(
    name: str,
    tier1_share: int,
    tier2_share: int,
    tier1: Mapping[str, int],
    tier2: Mapping[str, int],
) -> Profile:
    return validate_profile(
        Profile(
            name,
            Decimal(tier1_share),
            Decimal(tier2_share),
            {key: Decimal(value) for key, value in tier1.items()},
            {key: Decimal(value) for key, value in tier2.items()},
        )
    )


PROFILES = {
    "simple_implementation": _profile(
        "simple_implementation", 80, 20, {"intelligence": 1, "cost": 5, "speed": 5}, {"instruction_following": 5}
    ),
    "simple_action_execution": _profile(
        "simple_action_execution", 65, 35, {"intelligence": 1, "cost": 5, "speed": 5},
        {"instruction_following": 5, "evidence_capture": 5, "agentic_tools": 3, "software_engineering": 2},
    ),
    "balanced_implementation": _profile(
        "balanced_implementation", 70, 30, {"intelligence": 3, "cost": 3, "speed": 3},
        {"software_engineering": 5, "instruction_following": 3, "agentic_tools": 2},
    ),
    "complex_implementation": _profile(
        "complex_implementation", 60, 40, {"intelligence": 5, "cost": 1, "speed": 1},
        {"software_engineering": 5, "planning_capability": 4, "instruction_following": 2},
    ),
    "ui_ux": _profile(
        "ui_ux", 60, 40, {"intelligence": 3, "cost": 2, "speed": 3},
        {"ui_visual": 5, "software_engineering": 4, "instruction_following": 3, "evidence_capture": 2},
    ),
    "complex_action_execution": _profile(
        "complex_action_execution", 60, 40, {"intelligence": 4, "cost": 2, "speed": 2},
        {"agentic_tools": 5, "instruction_following": 4, "evidence_capture": 2},
    ),
    "financial_work": _profile(
        "financial_work", 60, 40, {"intelligence": 5, "cost": 1, "speed": 2},
        {"finance": 5, "knowledge": 4, "reasoning": 4, "research": 3, "instruction_following": 3},
    ),
    "research": _profile(
        "research", 60, 40, {"intelligence": 4, "cost": 2, "speed": 2},
        {"research": 5, "knowledge": 4, "reasoning": 3, "instruction_following": 2, "agentic_tools": 2},
    ),
    "planning": _profile(
        "planning", 60, 40, {"intelligence": 5, "cost": 1, "speed": 1},
        {"planning_capability": 5},
    ),
    "orchestration": _profile(
        "orchestration", 60, 40, {"intelligence": 5, "cost": 5, "speed": 4},
        {
            "planning_capability": 5,
            "instruction_following": 5,
        },
    ),
    "review": _profile(
        "review", 65, 35, {"intelligence": 4, "cost": 3, "speed": 2},
        {"instruction_following": 5, "software_engineering": 4, "reasoning": 4, "security": 3, "evidence_capture": 2},
    ),
}


def load_score_rows(path: Path) -> list[dict[str, str]]:
    try:
        source = path.open("r", encoding="utf-8", newline="")
    except OSError as error:
        raise RankingError(f"cannot read score CSV {path}: {error}") from error
    with source:
        reader = csv.DictReader(source)
        fields = reader.fieldnames or []
        required = {"model", "reasoning", *TIER1_COLUMNS.values()}
        missing = sorted(required - set(fields))
        if missing:
            raise RankingError("score CSV is missing required columns: " + ", ".join(missing))
        rows: list[dict[str, str]] = []
        identities: set[tuple[str, str]] = set()
        for number, raw in enumerate(reader, start=2):
            if None in raw:
                raise RankingError(f"score CSV row {number} has extra cells")
            model, reasoning = (raw.get("model") or "").strip(), (raw.get("reasoning") or "").strip()
            if not model or not reasoning:
                raise RankingError(f"score CSV row {number} has a blank model/reasoning identity")
            identity = (model, reasoning)
            if identity in identities:
                raise RankingError(f"score CSV has duplicate identity: {model} / {reasoning}")
            identities.add(identity)
            for column in (*TIER1_COLUMNS.values(), *CATEGORY_COLUMNS.values()):
                value = (raw.get(column) or "").strip()
                if not value:
                    continue
                number_value = _decimal(value, field=f"score CSV row {number} {column}")
                if number_value < 0 or number_value > 100:
                    raise RankingError(f"score CSV row {number} {column} must be between 0 and 100")
                raw[column] = str(number_value)
            rows.append(dict(raw))
    if not rows:
        raise RankingError("score CSV contains no model rows")
    return rows


def _identity(value: str) -> tuple[str, str]:
    candidate = value.strip()
    for separator in ("|", "::", ",", "/"):
        if separator in candidate:
            model, reasoning = candidate.rsplit(separator, 1)
            model, reasoning = model.strip(), reasoning.strip()
            if model and reasoning:
                return model, reasoning
    raise RankingError(
        f"availability identity {value!r} must use model|reasoning, model::reasoning, model,reasoning, or model/reasoning"
    )


def _availability_values(path: Path) -> set[tuple[str, str]]:
    try:
        text = path.read_text(encoding="utf-8")
    except OSError as error:
        raise RankingError(f"cannot read availability list {path}: {error}") from error
    try:
        payload = json.loads(text)
    except json.JSONDecodeError:
        payload = None
    if payload is not None:
        if not isinstance(payload, list):
            raise RankingError("availability JSON must be a list")
        result: set[tuple[str, str]] = set()
        for item in payload:
            if isinstance(item, str):
                result.add(_identity(item))
            elif isinstance(item, dict) and isinstance(item.get("model"), str) and isinstance(item.get("reasoning"), str):
                result.add((item["model"].strip(), item["reasoning"].strip()))
            elif isinstance(item, list) and len(item) == 2 and all(isinstance(part, str) for part in item):
                result.add((item[0].strip(), item[1].strip()))
            else:
                raise RankingError(f"invalid availability entry: {item!r}")
        return result

    result = set()
    for line_number, line in enumerate(text.splitlines(), start=1):
        stripped = line.strip()
        if not stripped or stripped.startswith("#"):
            continue
        if line_number == 1 and stripped.casefold().replace(" ", "") in {
            "model,reasoning",
            "model|reasoning",
        }:
            continue
        result.add(_identity(stripped))
    if not result:
        raise RankingError(f"availability list {path} contains no identities")
    return result


def parse_availability(paths: Sequence[Path], identities: Sequence[str]) -> set[tuple[str, str]] | None:
    if not paths and not identities:
        return None
    result: set[tuple[str, str]] = set()
    for path in paths:
        result.update(_availability_values(path))
    for value in identities:
        result.add(_identity(value))
    if not result:
        raise RankingError("availability filter was supplied but contains no identities")
    return result


def _parse_assignment(value: str, *, kind: str) -> tuple[str, Decimal]:
    if "=" not in value:
        raise RankingError(f"{kind} must use name=value")
    name, raw_value = value.split("=", 1)
    name = name.strip()
    if not name:
        raise RankingError(f"{kind} name must not be blank")
    return name, _decimal(raw_value.strip(), field=f"{kind} {name}")


def profile_from_json(text: str) -> Profile:
    try:
        payload = json.loads(text)
    except json.JSONDecodeError as error:
        raise RankingError(f"weights JSON is invalid: {error}") from error
    if not isinstance(payload, dict):
        raise RankingError("weights JSON must be an object")
    tier1 = payload.get("tier1")
    tier2 = payload.get("tier2")
    if isinstance(tier1, dict) and "weights" in tier1:
        tier1_weights = tier1.get("weights")
        tier1_share = tier1.get("share", payload.get("tier1_share", 100))
    elif isinstance(tier1, dict):
        tier1_weights = {key: value for key, value in tier1.items() if key != "share"}
        tier1_share = tier1.get("share", payload.get("tier1_share", 100))
    else:
        tier1_weights = payload.get("tier1_weights", {})
        tier1_share = payload.get("tier1_share", 100)
    if isinstance(tier2, dict) and "weights" in tier2:
        tier2_weights = tier2.get("weights")
        tier2_share = tier2.get("share", payload.get("tier2_share", 0))
    elif isinstance(tier2, dict):
        tier2_weights = {key: value for key, value in tier2.items() if key != "share"}
        tier2_share = tier2.get("share", payload.get("tier2_share", 0))
    else:
        tier2_weights = payload.get("tier2_weights", {})
        tier2_share = payload.get("tier2_share", 0)
    if not isinstance(tier1_weights, dict) or not isinstance(tier2_weights, dict):
        raise RankingError("weights JSON tier1/tier2 weights must be objects")
    return validate_profile(
        Profile(
            "custom",
            _decimal(tier1_share, field="tier1 share"),
            _decimal(tier2_share, field="tier2 share"),
            _weights(tier1_weights, tier="tier 1"),
            _weights(tier2_weights, tier="tier 2"),
        )
    )


def explicit_profile(
    tier1_assignments: Sequence[str],
    tier2_assignments: Sequence[str],
    tier1_share: object,
    tier2_share: object,
) -> Profile:
    tier1 = dict(_parse_assignment(value, kind="tier 1 weight") for value in tier1_assignments)
    tier2 = dict(_parse_assignment(value, kind="tier 2 weight") for value in tier2_assignments)
    return validate_profile(
        Profile(
            "custom",
            _decimal(tier1_share, field="tier1 share"),
            _decimal(tier2_share, field="tier2 share"),
            _weights(tier1, tier="tier 1"),
            _weights(tier2, tier="tier 2"),
        )
    )


def _score(row: Mapping[str, str], column: str) -> Decimal | None:
    value = str(row.get(column, "") or "").strip()
    return None if not value else _decimal(value, field=column)


def rank_models(
    rows: Sequence[Mapping[str, str]],
    profile: Profile,
    *,
    available: set[tuple[str, str]] | None = None,
    top_n: int = 5,
) -> dict[str, object]:
    validate_profile(profile)
    if top_n <= 0:
        raise RankingError("top-n must be positive")

    excluded: list[dict[str, object]] = []
    ranked: list[dict[str, object]] = []
    for row in rows:
        model, reasoning = row["model"], row["reasoning"]
        reasons: list[str] = []
        tier1_values = {
            name: _score(row, column) for name, column in TIER1_COLUMNS.items()
        }
        missing_tier1 = [name for name, value in tier1_values.items() if value is None]
        if missing_tier1:
            reasons.append("missing_tier1:" + ",".join(missing_tier1))
        if reasons:
            excluded.append({"model": model, "reasoning": reasoning, "reasons": reasons})
            continue

        tier1_weight_total = sum(profile.tier1_weights.values(), Decimal("0"))
        tier1_score = sum(
            (tier1_values[name] * profile.tier1_weights[name] for name in TIER1_COLUMNS),
            Decimal("0"),
        ) / tier1_weight_total

        category_values: dict[str, Decimal] = {}
        missing_optional: list[str] = []
        for name in profile.tier2_weights:
            value = _score(row, CATEGORY_COLUMNS[name])
            if value is None:
                missing_optional.append(name)
            else:
                category_values[name] = value
        warnings: list[str] = []
        if missing_optional:
            warnings.append("missing optional category scores: " + ", ".join(missing_optional))

        tier2_score: Decimal | None = None
        if category_values:
            weight_total = sum(
                (profile.tier2_weights[name] for name in category_values), Decimal("0")
            )
            tier2_score = sum(
                (category_values[name] * profile.tier2_weights[name] for name in category_values),
                Decimal("0"),
            ) / weight_total
        elif profile.tier2_weights:
            warnings.append("no optional task-category scores available; Tier 1 score used")

        if tier2_score is None:
            total_score = tier1_score
            tier1_contribution = tier1_score
            tier2_contribution = Decimal("0")
        else:
            tier1_contribution = tier1_score * profile.tier1_share / Decimal("100")
            tier2_contribution = tier2_score * profile.tier2_share / Decimal("100")
            total_score = tier1_contribution + tier2_contribution

        ranked.append(
            {
                "model": model,
                "reasoning": reasoning,
                "total_score": total_score,
                "tier1_score": tier1_score,
                "tier2_score": tier2_score,
                "tier1_contribution": tier1_contribution,
                "tier2_contribution": tier2_contribution,
                "category_scores": category_values,
                "warnings": warnings,
                "_tie_intelligence": tier1_values["intelligence"],
                "_tie_speed": tier1_values["speed"],
                "_tie_cost": tier1_values["cost"],
            }
        )

    # Availability is deliberately the final eligibility filter. Score every
    # complete row first, then remove rows the target harness did not expose.
    if available is not None:
        still_available: list[dict[str, object]] = []
        for candidate in ranked:
            identity = (candidate["model"], candidate["reasoning"])
            if identity in available:
                still_available.append(candidate)
            else:
                excluded.append(
                    {
                        "model": candidate["model"],
                        "reasoning": candidate["reasoning"],
                        "reasons": ["not_live_available"],
                    }
                )
        ranked = still_available

    if not ranked:
        if available is not None:
            raise RankingError("no candidates remain after live model-and-effort availability and Tier 1 filtering")
        raise RankingError("no candidates contain all mandatory Tier 1 scores")

    ranked.sort(
        key=lambda candidate: (
            -candidate["total_score"],
            -candidate["_tie_intelligence"],
            -candidate["tier2_contribution"],
            -candidate["_tie_speed"],
            -candidate["_tie_cost"],
            candidate["model"].casefold(),
            candidate["reasoning"].casefold(),
        )
    )

    def public(candidate: Mapping[str, object]) -> dict[str, object]:
        return {
            key: value
            for key, value in candidate.items()
            if not key.startswith("_")
        }

    selected = [public(candidate) for candidate in ranked[:top_n]]
    return {
        "profile": profile.name,
        "recommendation": selected[0],
        "alternatives": selected[1:],
        "excluded": sorted(excluded, key=lambda item: (str(item["model"]).casefold(), str(item["reasoning"]).casefold())),
        "candidate_count": len(ranked),
        "availability_filter_applied": available is not None,
    }


def _json_safe(value: object) -> object:
    if isinstance(value, Decimal):
        return float(value)
    if isinstance(value, dict):
        return {str(key): _json_safe(item) for key, item in value.items()}
    if isinstance(value, list):
        return [_json_safe(item) for item in value]
    return value


def parse_args(argv: Sequence[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--scores", type=Path, default=DEFAULT_SCORE_PATH)
    parser.add_argument("--profile", choices=sorted(PROFILES), default="balanced_implementation")
    parser.add_argument("--weights-json", help="custom profile JSON; replaces --profile")
    parser.add_argument("--tier1-weight", action="append", default=[], metavar="NAME=VALUE")
    parser.add_argument("--tier2-weight", action="append", default=[], metavar="NAME=VALUE")
    parser.add_argument("--tier1-share", default="100")
    parser.add_argument("--tier2-share", default="0")
    parser.add_argument("--available", type=Path, action="append", default=[], metavar="PATH")
    parser.add_argument("--available-identity", action="append", default=[], metavar="MODEL|REASONING")
    parser.add_argument("--top", type=int, default=5)
    parser.add_argument("--pretty", action="store_true")
    return parser.parse_args(argv)


def main(argv: Sequence[str] | None = None) -> int:
    args = parse_args(argv)
    try:
        explicit = bool(args.weights_json or args.tier1_weight or args.tier2_weight)
        if args.weights_json and (args.tier1_weight or args.tier2_weight):
            raise RankingError("use either --weights-json or repeatable weight arguments, not both")
        if explicit:
            profile = (
                profile_from_json(args.weights_json)
                if args.weights_json
                else explicit_profile(
                    args.tier1_weight,
                    args.tier2_weight,
                    args.tier1_share,
                    args.tier2_share,
                )
            )
        else:
            profile = PROFILES[args.profile]
        result = rank_models(
            load_score_rows(args.scores),
            profile,
            available=parse_availability(args.available, args.available_identity),
            top_n=args.top,
        )
    except RankingError as error:
        print(f"error: {error}", file=sys.stderr)
        return 1
    print(json.dumps(_json_safe(result), ensure_ascii=False, indent=2 if args.pretty else None, sort_keys=False))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
