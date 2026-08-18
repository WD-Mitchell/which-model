#!/usr/bin/env python3
"""Collect public Artificial Analysis model-page task metrics by exact slug."""

from __future__ import annotations

import argparse
import json
import re
import sys
import urllib.parse
from decimal import Decimal, InvalidOperation
from typing import Mapping, Sequence

from http_client import HttpClient
from model_types import PublicPageMetrics, UpdateError


MODEL_PAGE_URL = "https://artificialanalysis.ai/models/{slug}"


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


def _balanced_object(source: str, opening: int, *, field: str) -> str:
    depth = 0
    in_string = False
    escaped = False
    for position in range(opening, len(source)):
        character = source[position]
        if in_string:
            if escaped:
                escaped = False
            elif character == "\\":
                escaped = True
            elif character == '"':
                in_string = False
            continue
        if character == '"':
            in_string = True
        elif character == "{":
            depth += 1
        elif character == "}":
            depth -= 1
            if depth == 0:
                return source[opening : position + 1]
    raise UpdateError(f"public model page has unterminated {field} JSON")


def extract_public_page_metrics(
    page: str, expected_slug: str, *, require_fallback_cost: bool = False
) -> PublicPageMetrics:
    normalized = page.replace('\\"', '"')
    markers = list(re.finditer(r'"currentModel"\s*:\s*\{', normalized))
    matches: list[Mapping[str, object]] = []
    seen_slugs: list[object] = []
    for marker in markers:
        opening = normalized.find("{", marker.start())

        def reject_duplicate_pairs(
            pairs: list[tuple[str, object]],
        ) -> dict[str, object]:
            result: dict[str, object] = {}
            for key, value in pairs:
                if key in result:
                    raise UpdateError(f"public model page has duplicate {key} data")
                result[key] = value
            return result

        try:
            value = json.loads(
                _balanced_object(normalized, opening, field="currentModel"),
                parse_float=Decimal,
                object_pairs_hook=reject_duplicate_pairs,
            )
        except (json.JSONDecodeError, TypeError) as error:
            raise UpdateError("public model page has invalid currentModel JSON") from error
        if isinstance(value, dict):
            seen_slugs.append(value.get("slug"))
            if value.get("slug") == expected_slug:
                matches.append(value)
    if len(matches) > 1:
        raise UpdateError(
            f"public model page has ambiguous currentModel data for {expected_slug}"
        )
    if not matches:
        if markers:
            raise UpdateError(
                f"public model page currentModel slug mismatch: expected {expected_slug!r}, "
                f"found {seen_slugs!r}"
            )
        return PublicPageMetrics(None, None)
    model = matches[0]
    seconds = _decimal(
        model.get("intelligenceIndexTimePerTask"),
        field=f"{expected_slug} time per task",
        allow_null=True,
    )
    if seconds is not None and seconds < 0:
        raise UpdateError(f"{expected_slug} time per task must not be negative")
    cost: Decimal | None = None
    if require_fallback_cost:
        cost_data = model.get("intelligenceIndexCostPerTask")
        if cost_data is not None:
            if not isinstance(cost_data, dict) or not isinstance(
                cost_data.get("cost"), dict
            ):
                raise UpdateError(
                    f"{expected_slug} has malformed intelligenceIndexCostPerTask"
                )
            cost = _decimal(
                cost_data["cost"].get("total"),
                field=f"{expected_slug} fallback cost",
                allow_null=True,
            )
            if cost is not None and cost < 0:
                raise UpdateError(f"{expected_slug} fallback cost must not be negative")
    return PublicPageMetrics(seconds, cost)


def extract_time_for_slug(page: str, expected_slug: str) -> Decimal | None:
    return extract_public_page_metrics(page, expected_slug).time_seconds


def fetch_public_page_metrics(
    client: HttpClient, slug: str, *, require_fallback_cost: bool = False
) -> PublicPageMetrics:
    page = client.get_text(
        MODEL_PAGE_URL.format(slug=urllib.parse.quote(slug, safe="")),
        purpose=f"public model page for {slug}",
    )
    return extract_public_page_metrics(
        page, slug, require_fallback_cost=require_fallback_cost
    )


def main(argv: Sequence[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--slug", action="append", required=True)
    args = parser.parse_args(argv)
    client = HttpClient()
    try:
        metrics = {
            slug: fetch_public_page_metrics(
                client, slug, require_fallback_cost=True
            )
            for slug in dict.fromkeys(args.slug)
        }
    except UpdateError as error:
        print(f"error: {error}", file=sys.stderr)
        return 1
    json.dump(
        {
            slug: {
                "time_per_intelligence_index_task_seconds": value.time_seconds,
                "fallback_cost_per_intelligence_index_task_usd": value.fallback_cost,
            }
            for slug, value in metrics.items()
        },
        sys.stdout,
        ensure_ascii=False,
        indent=2,
        default=str,
    )
    print()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
