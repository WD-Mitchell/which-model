"""Strict independent benchmark and provider TOML loaders."""

from __future__ import annotations

import tomllib
from pathlib import Path
from typing import Sequence

from model_types import BenchmarkConfiguration, ProviderConfiguration, UpdateError


REPOSITORY_ROOT = Path(__file__).resolve().parents[1]
DEFAULT_BENCHMARK_CONFIG_PATH = REPOSITORY_ROOT / "config/benchmarks.toml"
DEFAULT_PROVIDER_CONFIG_PATH = REPOSITORY_ROOT / "config/providers.toml"


def _load_toml(path: Path, *, label: str) -> dict[str, object]:
    try:
        content = path.read_bytes()
    except OSError as error:
        raise UpdateError(f"cannot read {label} config {path}: {error}") from error
    try:
        text = content.decode("utf-8")
    except UnicodeDecodeError as error:
        raise UpdateError(f"{label} config is not valid UTF-8: {path}") from error
    try:
        return tomllib.loads(text)
    except tomllib.TOMLDecodeError as error:
        raise UpdateError(f"{label} config is invalid TOML: {path}: {error}") from error


def _string_list(value: object, *, field_name: str) -> tuple[str, ...]:
    if not isinstance(value, list) or any(not isinstance(item, str) for item in value):
        raise UpdateError(f"{field_name} must be a list of strings")
    if any(not item or item != item.strip() for item in value):
        raise UpdateError(f"{field_name} contains blank or untrimmed entries")
    duplicates = [item for item in dict.fromkeys(value) if value.count(item) > 1]
    if duplicates:
        raise UpdateError(f"{field_name} contains duplicate entries: {', '.join(duplicates)}")
    return tuple(value)


def load_benchmark_config(
    path: Path = DEFAULT_BENCHMARK_CONFIG_PATH,
) -> BenchmarkConfiguration:
    document = _load_toml(path, label="benchmark")
    if set(document) != {"benchmark_selection", "benchmark_groups"}:
        raise UpdateError(
            "benchmark config must contain exactly benchmark_selection and benchmark_groups"
        )
    selection, groups = document["benchmark_selection"], document["benchmark_groups"]
    if not isinstance(selection, dict) or set(selection) != {"groups", "benchmarks"}:
        raise UpdateError("benchmark_selection must contain only groups and benchmarks")
    if not isinstance(groups, dict) or not groups:
        raise UpdateError("benchmark_groups must be a non-empty table")
    configured: dict[str, tuple[str, ...]] = {}
    for group_id, settings in groups.items():
        if not isinstance(group_id, str) or not group_id or group_id != group_id.strip():
            raise UpdateError(f"benchmark_groups has invalid group ID {group_id!r}")
        if not isinstance(settings, dict) or set(settings) != {"benchmarks"}:
            raise UpdateError(f"benchmark group {group_id} must contain only benchmarks")
        configured[group_id] = _string_list(
            settings["benchmarks"], field_name=f"benchmark group {group_id} benchmarks"
        )
    selected_groups = _string_list(
        selection["groups"], field_name="benchmark_selection groups"
    )
    direct = _string_list(
        selection["benchmarks"], field_name="benchmark_selection benchmarks"
    )
    unknown = [group for group in selected_groups if group not in configured]
    if unknown:
        raise UpdateError("benchmark_selection references unknown groups: " + ", ".join(unknown))
    selected = tuple(
        dict.fromkeys(
            [name for group in selected_groups for name in configured[group]] + list(direct)
        )
    )
    if not selected:
        raise UpdateError("benchmark_selection must expand to at least one benchmark")
    return BenchmarkConfiguration(configured, selected)


def load_provider_config(
    path: Path = DEFAULT_PROVIDER_CONFIG_PATH,
) -> ProviderConfiguration:
    document = _load_toml(path, label="provider")
    if set(document) != {"providers"}:
        raise UpdateError("provider config must contain only the providers table")
    providers = document["providers"]
    if not isinstance(providers, dict) or not providers:
        raise UpdateError("provider config 'providers' must be a non-empty table")
    result: dict[str, tuple[str, ...]] = {}
    for provider, settings in providers.items():
        if not isinstance(provider, str) or not provider or provider != provider.strip():
            raise UpdateError(f"provider config has invalid provider ID {provider!r}")
        if not isinstance(settings, dict):
            raise UpdateError(f"provider config for {provider} must be a table")
        unknown = sorted(set(settings) - {"excluded_models"})
        if unknown:
            raise UpdateError(
                f"provider config for {provider} has unknown keys: {', '.join(unknown)}"
            )
        try:
            excluded = _string_list(
                settings.get("excluded_models", []),
                field_name=f"provider config for {provider} excluded_models",
            )
        except UpdateError as error:
            raise UpdateError(
                f"provider config for {provider} has invalid excluded_models: {error}"
            ) from error
        result[provider] = excluded
    return ProviderConfiguration(result)


def resolve_provider_ids(
    config: ProviderConfiguration, providers: Sequence[str] | None = None
) -> tuple[str, ...]:
    requested = tuple(config.excluded_models_by_provider) if providers is None else tuple(providers)
    if not requested:
        raise UpdateError("at least one access provider must be selected")
    normalized = tuple(provider.strip() for provider in requested)
    if any(not provider for provider in normalized):
        raise UpdateError("access provider IDs must not be blank")
    unknown = sorted(set(normalized) - set(config.excluded_models_by_provider))
    if unknown:
        supported = ", ".join(config.excluded_models_by_provider)
        raise UpdateError(
            f"unknown access providers: {', '.join(unknown)}; supported: {supported}"
        )
    return tuple(dict.fromkeys(normalized))
