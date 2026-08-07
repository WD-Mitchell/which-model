"""Dynamic CSV validation, merge, rendering, and atomic backup replacement."""

from __future__ import annotations

import csv
import io
import os
import tempfile
from dataclasses import replace
from datetime import datetime, timezone
from decimal import Decimal, InvalidOperation, ROUND_HALF_UP
from pathlib import Path
from typing import Sequence

from model_types import (
    BENCHMARK_COLUMN_PREFIX,
    CORE_CSV_COLUMNS,
    ModelFamily,
    RawRow,
    UpdateError,
    clean_model_name,
)


RAW_METRIC_COLUMNS = {
    "intelligence_index": "intelligence",
    "time_per_intelligence_index_task_seconds": "time_seconds",
    "cost_per_intelligence_index_task_usd": "cost",
    "median_end_to_end_response_time_seconds": "median_response_time",
    "artificial_analysis_coding_index": "coding_index",
    "artificial_analysis_agentic_index": "agentic_index",
}
LEGACY_ROW_ATTRIBUTES = ("coding", "reasoning_performance", "agentic", "math")
NONNEGATIVE_RAW_COLUMNS = {
    "time_per_intelligence_index_task_seconds",
    "cost_per_intelligence_index_task_usd",
    "median_end_to_end_response_time_seconds",
}


def _decimal(value: object, *, field: str) -> Decimal:
    if isinstance(value, bool): raise UpdateError(f"{field} must be numeric")
    try: number = Decimal(str(value))
    except (InvalidOperation, ValueError) as error:
        raise UpdateError(f"{field} must be numeric") from error
    if not number.is_finite(): raise UpdateError(f"{field} must be finite")
    return number


def validate_complete_rows(rows: Sequence[RawRow]) -> None:
    if not rows: raise UpdateError("complete dataset contains no model rows")
    identities: set[tuple[str, str]] = set()
    for row in rows:
        identity = (row.model, row.reasoning)
        if identity in identities:
            raise UpdateError(f"duplicate model/reasoning row: {row.model} / {row.reasoning}")
        identities.add(identity)
        values = {
            "intelligence_index": row.intelligence,
            "time_per_intelligence_index_task_seconds": row.time_seconds,
            "cost_per_intelligence_index_task_usd": row.cost,
            "median_end_to_end_response_time_seconds": row.median_response_time,
            "artificial_analysis_coding_index": row.coding_index,
            "artificial_analysis_agentic_index": row.agentic_index,
        }
        for field, value in values.items():
            if value is not None and not value.is_finite():
                raise UpdateError(f"nonfinite {field} for {row.model} / {row.reasoning}")
            if value is not None and field in NONNEGATIVE_RAW_COLUMNS and value < 0:
                raise UpdateError(f"negative required metric for {row.model} / {row.reasoning}")
        for name, value in row.benchmark_values.items():
            if not name or name != name.strip():
                raise UpdateError(f"invalid benchmark name for {row.model} / {row.reasoning}")
            if value is not None and not value.is_finite():
                raise UpdateError(f"nonfinite benchmark {name!r} for {row.model} / {row.reasoning}")


def parse_existing_csv(content: bytes, *, source: Path) -> list[RawRow]:
    try: text = content.decode("utf-8")
    except UnicodeDecodeError as error:
        raise UpdateError(f"existing raw CSV is not valid UTF-8: {source}") from error
    reader = csv.DictReader(io.StringIO(text, newline=""))
    fields = reader.fieldnames or []
    if fields[: len(CORE_CSV_COLUMNS)] != list(CORE_CSV_COLUMNS):
        raise UpdateError(
            "existing raw CSV has unexpected core columns: expected "
            f"{','.join(CORE_CSV_COLUMNS)}, got {','.join(fields)}"
        )
    extras = fields[len(CORE_CSV_COLUMNS) :]
    if any(not column.startswith(BENCHMARK_COLUMN_PREFIX) or not column.removeprefix(BENCHMARK_COLUMN_PREFIX) for column in extras):
        raise UpdateError("existing raw CSV has invalid dynamic benchmark columns")
    names = tuple(column.removeprefix(BENCHMARK_COLUMN_PREFIX) for column in extras)
    if len(names) != len(set(names)):
        raise UpdateError("existing raw CSV has duplicate benchmark columns")
    rows: list[RawRow] = []
    raw_identities: set[tuple[str, str]] = set()
    for number, raw in enumerate(reader, start=2):
        if None in raw: raise UpdateError(f"existing raw CSV row {number} has extra cells")
        if any(raw[column] is None for column in fields):
            raise UpdateError(f"existing raw CSV row {number} has missing cells")
        raw_model, reasoning = raw["model"].strip(), raw["reasoning"].strip()
        model = clean_model_name(raw_model)
        if not model or not reasoning:
            raise UpdateError(f"existing raw CSV row {number} has a blank identity")
        raw_identity = (raw_model, reasoning)
        if raw_identity in raw_identities:
            raise UpdateError(f"existing raw CSV has duplicate identity: {model} / {reasoning}")
        raw_identities.add(raw_identity)
        values: dict[str, Decimal | None] = {}
        for column, attribute in RAW_METRIC_COLUMNS.items():
            value = None if not raw[column].strip() else _decimal(raw[column], field=f"existing raw CSV row {number} {column}")
            if value is not None and column in NONNEGATIVE_RAW_COLUMNS and value < 0:
                raise UpdateError(f"existing raw CSV row {number} {column} must not be negative")
            values[attribute] = value
        benchmark_values = {
            name: None if not raw[column].strip() else _decimal(raw[column], field=f"existing raw CSV row {number} {column}")
            for column, name in zip(extras, names, strict=True)
        }
        rows.append(RawRow(model=model, reasoning=reasoning, benchmark_values=benchmark_values, **values))
    if not rows: raise UpdateError("existing raw CSV contains no data rows")
    return _collapse_default_reasoning(rows)


def _collapse_default_reasoning(rows: Sequence[RawRow]) -> list[RawRow]:
    """Normalize names/default effort and merge duplicate evidence rows."""
    grouped: dict[tuple[str, str], list[RawRow]] = {}
    order: list[tuple[str, str]] = []
    for row in rows:
        model = clean_model_name(row.model)
        if not model:
            raise UpdateError("model name is blank after removing annotations")
        normalized_row = replace(row, model=model)
        identity = (model, "high" if row.reasoning == "default" else row.reasoning)
        if identity not in grouped:
            grouped[identity] = []
            order.append(identity)
        grouped[identity].append(normalized_row)

    result: list[RawRow] = []
    for identity in order:
        candidates = grouped[identity]
        explicit = next(
            (candidate for candidate in candidates if candidate.reasoning != "default"),
            None,
        )
        base = explicit or candidates[0]
        replacements: dict[str, object] = {}
        for attribute in (*RAW_METRIC_COLUMNS.values(), *LEGACY_ROW_ATTRIBUTES):
            if getattr(base, attribute) is not None:
                continue
            for candidate in candidates:
                value = getattr(candidate, attribute)
                if value is not None:
                    replacements[attribute] = value
                    break
        benchmark_names = {
            name for candidate in candidates for name in candidate.benchmark_values
        }
        benchmarks: dict[str, Decimal | None] = {}
        for name in benchmark_names:
            values = [candidate.benchmark_values.get(name) for candidate in candidates]
            populated = [value for value in values if value is not None]
            benchmarks[name] = max(populated) if populated else None
        result.append(
            replace(
                base,
                model=identity[0],
                reasoning=identity[1],
                benchmark_values=benchmarks,
                authoritative_benchmarks=frozenset(
                    name
                    for candidate in candidates
                    for name in candidate.authoritative_benchmarks
                ),
                **replacements,
            )
        )
    return result


def merge_rows(fresh_rows: Sequence[RawRow], current_rows: Sequence[RawRow]) -> list[RawRow]:
    fresh_rows = _collapse_default_reasoning(fresh_rows)
    current_rows = _collapse_default_reasoning(current_rows)
    current_by_identity = {(row.model, row.reasoning): row for row in current_rows}
    merged: list[RawRow] = []
    for fresh in fresh_rows:
        current = current_by_identity.get((fresh.model, fresh.reasoning)); replacements = {}
        if current:
            for attribute in (*RAW_METRIC_COLUMNS.values(), *LEGACY_ROW_ATTRIBUTES):
                value = getattr(fresh, attribute)
                replacements[attribute] = value if value is not None else getattr(current, attribute)
        benchmarks = dict(fresh.benchmark_values)
        if current:
            for name, value in benchmarks.items():
                if value is None and name not in fresh.authoritative_benchmarks:
                    benchmarks[name] = current.benchmark_values.get(name)
        merged.append(replace(fresh, benchmark_values=benchmarks, **replacements))
    return merged


def merge_partial_refresh(
    fresh_rows: Sequence[RawRow], current_rows: Sequence[RawRow],
    refreshed_families: Sequence[ModelFamily], *, preserve_unselected: bool,
) -> list[RawRow]:
    current_rows = _collapse_default_reasoning(current_rows)
    merged = merge_rows(fresh_rows, current_rows)
    if not preserve_unselected: return merged
    names = {family.name for family in refreshed_families}
    return [*merged, *(row for row in current_rows if row.model not in names)]


def _format(value: Decimal, quantum: str) -> str:
    return str(value.quantize(Decimal(quantum), rounding=ROUND_HALF_UP))


def render_csv(rows: Sequence[RawRow], benchmark_names: Sequence[str] | None = None) -> bytes:
    if benchmark_names is None:
        benchmark_names = tuple(dict.fromkeys(name for row in rows for name in row.benchmark_values))
    fields = (*CORE_CSV_COLUMNS, *(f"{BENCHMARK_COLUMN_PREFIX}{name}" for name in benchmark_names))
    output = io.StringIO(newline=""); writer = csv.DictWriter(output, fieldnames=fields, lineterminator="\n")
    writer.writeheader()
    for row in rows:
        values = {
            "model": row.model, "reasoning": row.reasoning,
            "intelligence_index": "" if row.intelligence is None else _format(row.intelligence, "0.1"),
            "time_per_intelligence_index_task_seconds": "" if row.time_seconds is None else _format(row.time_seconds, "1"),
            "cost_per_intelligence_index_task_usd": "" if row.cost is None else _format(row.cost, "0.01"),
            "median_end_to_end_response_time_seconds": "" if row.median_response_time is None else _format(row.median_response_time, "1"),
            "artificial_analysis_coding_index": "" if row.coding_index is None else _format(row.coding_index, "0.1"),
            "artificial_analysis_agentic_index": "" if row.agentic_index is None else _format(row.agentic_index, "0.1"),
        }
        values.update({
            f"{BENCHMARK_COLUMN_PREFIX}{name}": "" if row.benchmark_values.get(name) is None else _format(row.benchmark_values[name], "0.1")
            for name in benchmark_names
        })
        writer.writerow(values)
    return output.getvalue().encode("utf-8")


def _backup_path(path: Path, now: datetime) -> Path:
    stamp = now.astimezone(timezone.utc).strftime("%Y%m%dT%H%M%S%fZ")
    candidate = path.with_name(f"{path.name}.{stamp}.bak"); suffix = 1
    while candidate.exists():
        candidate = path.with_name(f"{path.name}.{stamp}.{suffix}.bak"); suffix += 1
    return candidate


def replace_with_backup(
    output_path: Path, content: bytes, *, now: datetime | None = None,
    expected_original: bytes | None = None,
) -> Path:
    if not output_path.is_file(): raise UpdateError(f"existing raw CSV not found: {output_path}")
    original = output_path.read_bytes()
    if expected_original is not None and original != expected_original:
        raise UpdateError(f"{output_path} changed while model data was being collected")
    backup = _backup_path(output_path, datetime.now(timezone.utc) if now is None else now)
    temporary_name: str | None = None
    try:
        with tempfile.NamedTemporaryFile(mode="wb", dir=output_path.parent, prefix=f".{output_path.name}.", delete=False) as temporary:
            temporary.write(content); temporary.flush(); os.fsync(temporary.fileno()); temporary_name = temporary.name
        if output_path.read_bytes() != original:
            raise UpdateError(f"{output_path} changed while model data was being collected")
        with backup.open("xb") as target:
            target.write(original); target.flush(); os.fsync(target.fileno())
        if output_path.read_bytes() != original:
            backup.unlink(); raise UpdateError(f"{output_path} changed while model data was being collected")
        os.replace(temporary_name, output_path); temporary_name = None; return backup
    except OSError as error:
        raise UpdateError(f"failed to replace {output_path}: {error}") from error
    finally:
        if temporary_name is not None:
            try: Path(temporary_name).unlink()
            except FileNotFoundError: pass
