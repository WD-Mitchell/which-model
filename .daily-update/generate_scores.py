#!/usr/bin/env python3
"""Generate higher-is-better model scores and task-category composites."""

from __future__ import annotations

import argparse
import csv
import re
import sys
import unicodedata
from decimal import Decimal, InvalidOperation, ROUND_HALF_UP
from pathlib import Path
from typing import Iterable, Mapping


# The source files are executable directly as well as imported by the tests.
SCRIPT_DIR = Path(__file__).resolve().parent
if str(SCRIPT_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPT_DIR))

from model_config import (  # noqa: E402
    DEFAULT_BENCHMARK_CONFIG_PATH,
    load_benchmark_config,
)
from model_types import UpdateError, clean_model_name  # noqa: E402


REPOSITORY_ROOT = Path(__file__).resolve().parents[1]
DEFAULT_INPUT = REPOSITORY_ROOT / "data/available_model_raw_values.csv"
DEFAULT_OUTPUT = REPOSITORY_ROOT / "data/available_model_scores.csv"

IDENTITY_COLUMNS = ("model", "reasoning")
CORE_METRICS = {
    "intelligence_index": True,
    "time_per_intelligence_index_task_seconds": False,
    "cost_per_intelligence_index_task_usd": False,
    "median_end_to_end_response_time_seconds": False,
    "artificial_analysis_coding_index": True,
    "artificial_analysis_agentic_index": True,
}
BENCHMARK_COLUMN_PREFIX = "benchmark:"
# Every measurement is independently nullable, including the three core axes.
OPTIONAL_METRICS = set(CORE_METRICS)
NULLABLE_METRICS = set(CORE_METRICS)
CORE_INPUT_COLUMNS = (*IDENTITY_COLUMNS, *CORE_METRICS)
SCORE_QUANTUM = Decimal("1")
ONE_HUNDRED = Decimal("100")

# These are deliberately score columns rather than coverage columns. A blank
# means the source data did not meet that category's evidence threshold.
CATEGORY_SCORE_COLUMNS = (
    "reasoning_score",
    "knowledge_score",
    "research_score",
    "planning_capability_score",
    "instruction_following_score",
    "software_engineering_score",
    "ui_visual_score",
    "agentic_tools_score",
    "finance_score",
    "evidence_capture_score",
    "security_score",
    "data_ml_score",
)
CATEGORY_GROUPS = {
    "reasoning_score": "reasoning",
    "knowledge_score": "knowledge",
    "research_score": "research",
    "instruction_following_score": "instruction_following",
    "software_engineering_score": "software_engineering",
    "ui_visual_score": "ui_visual",
    "agentic_tools_score": "agentic_tools",
    "finance_score": "finance",
    "evidence_capture_score": "evidence_capture",
    "security_score": "security",
    "data_ml_score": "data_ml",
}

# A composite with one populated benchmark is too fragile to use as a task
# signal. Security has only two currently published candidates, so one is the
# minimum that can produce a useful early signal; all broader composites need
# two independent pieces of evidence.
CATEGORY_MINIMUM_EVIDENCE = {
    "reasoning_score": 2,
    "knowledge_score": 2,
    "research_score": 2,
    "instruction_following_score": 2,
    "software_engineering_score": 2,
    "ui_visual_score": 2,
    "agentic_tools_score": 2,
    "finance_score": 2,
    "evidence_capture_score": 2,
    "security_score": 1,
    "data_ml_score": 2,
}


class InputError(ValueError):
    """Raised when the raw CSV cannot be scored safely."""


def _benchmark_key(value: str) -> str:
    """Return a stable key used to deduplicate benchmark aliases/variants."""

    normalized = unicodedata.normalize("NFKC", value).casefold()
    normalized = normalized.replace("’", "'").replace("`", "'")
    compact = "".join(character for character in normalized if character.isalnum())
    # These are known models.dev aliases/variants. They are one evidence
    # source, not separate votes in a category mean.
    return {
        "financeagent": "financeagent",
        "gdpval": "gdpval",
        "gdpvalaa": "gdpval",
        "humanityslastexam": "humanityslastexam",
        "artificialanalysiscodingindex": "artificialanalysiscodingindex",
        "artificialanalysiscodingagentindex": "artificialanalysiscodingagentindex",
    }.get(compact, compact)


def parse_number(value: str, *, column: str, row_number: int) -> Decimal | None:
    stripped = value.strip()
    if not stripped:
        if column in NULLABLE_METRICS or column.startswith(BENCHMARK_COLUMN_PREFIX):
            return None
        raise InputError(f"row {row_number}: {column} must not be blank")

    try:
        number = Decimal(stripped)
    except InvalidOperation as error:
        raise InputError(
            f"row {row_number}: {column} must be numeric, got {value!r}"
        ) from error

    if not number.is_finite():
        raise InputError(f"row {row_number}: {column} must be finite, got {value!r}")
    if column in {
        "time_per_intelligence_index_task_seconds",
        "cost_per_intelligence_index_task_usd",
        "median_end_to_end_response_time_seconds",
    } and number < 0:
        raise InputError(
            f"row {row_number}: {column} must not be negative, got {value!r}"
        )
    return number


def _merge_input_rows(
    rows: Iterable[dict[str, str | Decimal | None]],
) -> list[dict[str, str | Decimal | None]]:
    """Normalize annotated names and merge duplicate model/effort rows.

    Older raw snapshots can contain the same model more than once because a
    provider appended a dated ID or ``(latest)`` to its display name.  Treat
    those annotations as identity-neutral here as well as in the collector so
    score generation remains stable if it is run before a refresh.
    """

    grouped: dict[tuple[str, str], dict[str, str | Decimal | None]] = {}
    order: list[tuple[str, str]] = []
    for row in rows:
        model = clean_model_name(str(row["model"]))
        if not model:
            raise InputError("model name is blank after removing annotations")
        reasoning_value = str(row["reasoning"])
        reasoning = "high" if reasoning_value == "default" else reasoning_value
        identity = (model, reasoning)
        if identity not in grouped:
            grouped[identity] = {
                **row,
                "model": model,
                "reasoning": reasoning,
            }
            order.append(identity)
            continue
        target = grouped[identity]
        for column, value in row.items():
            if column in IDENTITY_COLUMNS or value is None:
                continue
            current = target[column]
            if current is None:
                target[column] = value
            elif (
                column.startswith(BENCHMARK_COLUMN_PREFIX)
                and isinstance(current, Decimal)
                and isinstance(value, Decimal)
            ):
                target[column] = max(current, value)
    return [grouped[identity] for identity in order]


def read_rows(
    path: Path,
) -> tuple[list[dict[str, str | Decimal | None]], dict[str, bool], set[str]]:
    try:
        source = path.open("r", encoding="utf-8", newline="")
    except OSError as error:
        raise InputError(f"cannot read {path}: {error}") from error

    with source:
        reader = csv.DictReader(source)
        fieldnames = reader.fieldnames or []
        if fieldnames[: len(CORE_INPUT_COLUMNS)] != list(CORE_INPUT_COLUMNS):
            raise InputError(
                "unexpected core columns: expected "
                f"{','.join(CORE_INPUT_COLUMNS)}, got {','.join(fieldnames)}"
            )
        benchmark_metrics = fieldnames[len(CORE_INPUT_COLUMNS) :]
        if any(
            not column.startswith(BENCHMARK_COLUMN_PREFIX)
            or not column[len(BENCHMARK_COLUMN_PREFIX) :]
            for column in benchmark_metrics
        ) or len(benchmark_metrics) != len(set(benchmark_metrics)):
            raise InputError("invalid or duplicate dynamic benchmark columns")
        metrics = {**CORE_METRICS, **{column: True for column in benchmark_metrics}}
        optional_metrics = {*OPTIONAL_METRICS, *benchmark_metrics}

        rows: list[dict[str, str | Decimal | None]] = []
        for row_number, raw_row in enumerate(reader, start=2):
            if None in raw_row:
                raise InputError(f"row {row_number}: too many values")
            if any(raw_row[column] is None for column in fieldnames):
                raise InputError(f"row {row_number}: too few values")

            row: dict[str, str | Decimal | None] = {}
            for column in IDENTITY_COLUMNS:
                value = raw_row[column].strip()
                if not value:
                    raise InputError(f"row {row_number}: {column} must not be blank")
                row[column] = value
            for column in metrics:
                row[column] = parse_number(
                    raw_row[column], column=column, row_number=row_number
                )
            rows.append(row)

    if not rows:
        raise InputError("input contains no data rows")
    return _merge_input_rows(rows), metrics, optional_metrics


def ranges(
    rows: Iterable[dict[str, str | Decimal | None]],
    metrics: Mapping[str, bool],
    optional_metrics: set[str],
) -> dict[str, tuple[Decimal, Decimal] | None]:
    result: dict[str, tuple[Decimal, Decimal] | None] = {}
    row_list = list(rows)
    for column in metrics:
        values = [row[column] for row in row_list if row[column] is not None]
        if not values:
            if column in optional_metrics:
                result[column] = None
                continue
            raise InputError(f"{column} has no published values")
        numeric_values = [value for value in values if isinstance(value, Decimal)]
        minimum, maximum = min(numeric_values), max(numeric_values)
        if minimum == maximum:
            if column in optional_metrics:
                result[column] = None
                continue
            raise InputError(f"{column} has a degenerate range ({minimum})")
        result[column] = (minimum, maximum)
    return result


def normalized_score(
    value: Decimal, minimum: Decimal, maximum: Decimal, higher_is_better: bool
) -> Decimal:
    numerator = value - minimum if higher_is_better else maximum - value
    return ONE_HUNDRED * numerator / (maximum - minimum)


def score(value: Decimal, minimum: Decimal, maximum: Decimal, higher_is_better: bool) -> str:
    return str(
        normalized_score(value, minimum, maximum, higher_is_better).quantize(
            SCORE_QUANTUM, rounding=ROUND_HALF_UP
        )
    )


def _decimal_score(value: str | Decimal | None) -> Decimal | None:
    if value is None or str(value).strip() == "":
        return None
    try:
        return Decimal(str(value))
    except InvalidOperation as error:  # pragma: no cover - generated values are validated
        raise InputError(f"generated score is not numeric: {value!r}") from error


def _rounded_average(values: Iterable[Decimal]) -> str:
    numbers = tuple(values)
    return str(
        (sum(numbers, Decimal("0")) / Decimal(len(numbers))).quantize(
            SCORE_QUANTUM, rounding=ROUND_HALF_UP
        )
    )


def _source_scores(output_row: Mapping[str, str]) -> dict[str, Decimal]:
    """Collect benchmark and AA-index scores, preferring direct AA columns."""

    result: dict[str, Decimal] = {}

    # The AA index columns are authoritative for these two names. If models.dev
    # also publishes a benchmark column with the same name, it must not count a
    # second time.
    for column, source_name in (
        ("artificial_analysis_coding_index_score", "Artificial Analysis Coding Index"),
        (
            "artificial_analysis_agentic_index_score",
            "Artificial Analysis Coding Agent Index",
        ),
    ):
        value = _decimal_score(output_row.get(column))
        if value is not None:
            result[_benchmark_key(source_name)] = value

    # Dynamic columns are inserted only when no authoritative core metric has
    # already supplied the same canonical source. TOML order is deterministic.
    for column, value in output_row.items():
        if not column.startswith(BENCHMARK_COLUMN_PREFIX):
            continue
        score_value = _decimal_score(value)
        if score_value is None:
            continue
        name = column.removeprefix(BENCHMARK_COLUMN_PREFIX)
        result.setdefault(_benchmark_key(name), score_value)
    return result


def _category_score(
    output_row: Mapping[str, str],
    category_column: str,
    benchmark_groups: Mapping[str, tuple[str, ...]],
) -> str:
    if category_column == "planning_capability_score":
        # Planning is intentionally a fixed composition. Missing components
        # leave the composite blank instead of silently treating them as zero.
        components = (
            ("reasoning_score", Decimal("0.40")),
            ("knowledge_score", Decimal("0.30")),
            ("agentic_tools_score", Decimal("0.20")),
            ("research_score", Decimal("0.10")),
        )
        values = [_decimal_score(output_row.get(column)) for column, _ in components]
        if any(value is None for value in values):
            return ""
        weighted = sum(
            (value * weight for value, (_, weight) in zip(values, components, strict=True)),
            Decimal("0"),
        )
        return str(weighted.quantize(SCORE_QUANTUM, rounding=ROUND_HALF_UP))

    group_id = CATEGORY_GROUPS[category_column]
    names = benchmark_groups.get(group_id, ())
    if not names:
        return ""

    source_scores = _source_scores(output_row)
    values: list[Decimal] = []
    seen: set[str] = set()
    for name in names:
        key = _benchmark_key(name)
        if key in seen:
            continue
        seen.add(key)
        value = source_scores.get(key)
        if value is not None:
            values.append(value)
    if len(values) < CATEGORY_MINIMUM_EVIDENCE[category_column]:
        return ""
    return _rounded_average(values)


def generate(
    input_path: Path,
    output_path: Path,
    benchmark_config_path: Path = DEFAULT_BENCHMARK_CONFIG_PATH,
) -> None:
    rows, metrics, optional_metrics = read_rows(input_path)
    eligible_rows = [
        row
        for row in rows
        if any(row[column] is not None for column in metrics)
    ]
    if not eligible_rows:
        raise InputError(
            "input contains no published metric values"
        )
    metric_ranges = ranges(eligible_rows, metrics, optional_metrics)
    try:
        benchmark_config = load_benchmark_config(benchmark_config_path)
    except UpdateError as error:
        raise InputError(str(error)) from error

    # Keep the source benchmark columns after the derived category scores.
    output_columns = (
        *IDENTITY_COLUMNS,
        *(f"{column}_score" for column in metrics if not column.startswith(BENCHMARK_COLUMN_PREFIX)),
        *CATEGORY_SCORE_COLUMNS,
        *(column for column in metrics if column.startswith(BENCHMARK_COLUMN_PREFIX)),
    )

    output_path.parent.mkdir(parents=True, exist_ok=True)
    try:
        destination = output_path.open("w", encoding="utf-8", newline="")
    except OSError as error:
        raise InputError(f"cannot write {output_path}: {error}") from error

    with destination:
        writer = csv.DictWriter(
            destination, fieldnames=output_columns, lineterminator="\n"
        )
        writer.writeheader()
        for row in eligible_rows:
            output_row: dict[str, str] = {
                column: str(row[column])
                for column in IDENTITY_COLUMNS
            }
            for column, higher_is_better in metrics.items():
                if column.startswith(BENCHMARK_COLUMN_PREFIX):
                    continue
                value = row[column]
                metric_range = metric_ranges[column]
                output_row[f"{column}_score"] = (
                    ""
                    if value is None or metric_range is None
                    else score(value, *metric_range, higher_is_better)
                )
            for column in metrics:
                if not column.startswith(BENCHMARK_COLUMN_PREFIX):
                    continue
                value = row[column]
                metric_range = metric_ranges[column]
                output_row[column] = (
                    ""
                    if value is None or metric_range is None
                    else score(value, *metric_range, True)
                )
            for column in CATEGORY_SCORE_COLUMNS:
                output_row[column] = _category_score(
                    output_row, column, benchmark_config.benchmark_groups
                )
            writer.writerow(output_row)


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--input", type=Path, default=DEFAULT_INPUT)
    parser.add_argument("--output", type=Path, default=DEFAULT_OUTPUT)
    parser.add_argument(
        "--benchmark-config", type=Path, default=DEFAULT_BENCHMARK_CONFIG_PATH
    )
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    try:
        generate(args.input, args.output, args.benchmark_config)
    except (InputError, UpdateError) as error:
        print(f"error: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
