"""Shared immutable model-data contracts."""

from __future__ import annotations

from dataclasses import dataclass, field
from decimal import Decimal


BENCHMARK_COLUMN_PREFIX = "benchmark:"
CORE_CSV_COLUMNS = (
    "model",
    "reasoning",
    "intelligence_index",
    "time_per_intelligence_index_task_seconds",
    "cost_per_intelligence_index_task_usd",
    "median_end_to_end_response_time_seconds",
    "artificial_analysis_coding_index",
    "artificial_analysis_agentic_index",
)
EFFORT_ORDER = {
    name: index
    for index, name in enumerate(("minimal", "low", "medium", "high", "xhigh", "max"))
}
REASONING_LEVELS = {"default", *EFFORT_ORDER}


def clean_model_name(value: str) -> str:
    """Remove annotation groups from a model display name.

    Provider catalogues commonly append dated IDs or freshness labels to a
    display name, for example ``Claude Opus 4.5 [claude-opus-4-5-20251101]``
    and ``Claude Opus 4.5 (latest)``.  Those annotations are not part of the
    model identity, so remove balanced ``()``, ``[]``, and ``{}`` groups (also
    when nested) and normalize the remaining whitespace.
    """

    kept: list[str] = []
    opening = "([{"
    closing = {
        ")": "(",
        "]": "[",
        "}": "{",
    }
    stack: list[str] = []
    for character in value:
        if character in opening:
            stack.append(character)
            continue
        if character in closing:
            # Annotation punctuation is discarded even if a source has a
            # malformed or mismatched closer.  A matched group resumes normal
            # output after its closing delimiter; an unmatched opener simply
            # suppresses the remainder of that malformed annotation.
            if stack and stack[-1] == closing[character]:
                stack.pop()
            continue
        if not stack:
            kept.append(character)
    return " ".join("".join(kept).split())


class UpdateError(RuntimeError):
    """Raised when source data cannot safely replace generated model data."""


@dataclass(frozen=True)
class ModelFamily:
    name: str
    base_slug: str


@dataclass(frozen=True)
class ProviderModel:
    provider: str
    model_id: str
    display_name: str
    reasoning_levels: tuple[str, ...] = ("high",)
    canonical_id: str | None = None
    benchmarks: tuple[tuple[str, Decimal], ...] = ()
    benchmark_overrides: tuple[tuple[str, tuple[tuple[str, Decimal], ...]], ...] = ()


@dataclass(frozen=True)
class ProviderConfiguration:
    excluded_models_by_provider: dict[str, tuple[str, ...]]


@dataclass(frozen=True)
class BenchmarkConfiguration:
    benchmark_groups: dict[str, tuple[str, ...]]
    selected_benchmarks: tuple[str, ...]


@dataclass(frozen=True)
class SelectedModel:
    family: ModelFamily
    reasoning: str
    slug: str | None
    intelligence: Decimal | None
    cost: Decimal | None
    median_response_time: Decimal | None = None
    coding_index: Decimal | None = None
    agentic_index: Decimal | None = None
    benchmarks: tuple[tuple[str, Decimal], ...] = ()
    benchmark_clears: frozenset[str] = frozenset()


@dataclass(frozen=True)
class RawRow:
    model: str
    reasoning: str
    intelligence: Decimal | None
    time_seconds: Decimal | None
    cost: Decimal | None
    coding: Decimal | None = None
    reasoning_performance: Decimal | None = None
    agentic: Decimal | None = None
    math: Decimal | None = None
    median_response_time: Decimal | None = None
    coding_index: Decimal | None = None
    agentic_index: Decimal | None = None
    benchmark_values: dict[str, Decimal | None] = field(default_factory=dict)
    authoritative_benchmarks: frozenset[str] = frozenset()


@dataclass(frozen=True)
class PublicPageMetrics:
    time_seconds: Decimal | None
    fallback_cost: Decimal | None


@dataclass(frozen=True)
class ProviderMatchResult:
    selected: tuple[SelectedModel, ...]
    families: tuple[ModelFamily, ...]
    unmatched_by_provider: dict[str, tuple[str, ...]]
    discovered_by_provider: dict[str, int]
    excluded_by_provider: dict[str, int]
