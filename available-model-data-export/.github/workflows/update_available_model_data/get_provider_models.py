#!/usr/bin/env python3
"""Collect provider model availability from models.dev/api.json."""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path
from typing import Callable, Mapping, Sequence

from model_config import (
    DEFAULT_PROVIDER_CONFIG_PATH,
    load_provider_config,
    resolve_provider_ids,
)
from http_client import HttpClient
from model_types import (
    ProviderModel,
    REASONING_LEVELS,
    UpdateError,
    clean_model_name,
)


MODELS_DEV_PROVIDER_URL = "https://models.dev/api.json"


def _reasoning_levels(
    provider: str, model_id: str, model: Mapping[str, object]
) -> tuple[str, ...]:
    if not isinstance(model.get("reasoning"), bool):
        raise UpdateError(f"models.dev {provider} model {model_id!r} has invalid reasoning")
    options = model.get("reasoning_options")
    if options is None:
        return ("high",)
    if not isinstance(options, list):
        raise UpdateError(
            f"models.dev {provider} model {model_id!r} has invalid reasoning_options"
        )
    efforts: list[str] | None = None
    for index, option in enumerate(options):
        if not isinstance(option, dict) or not isinstance(option.get("type"), str):
            raise UpdateError(
                f"models.dev {provider} model {model_id!r} reasoning option {index} is invalid"
            )
        if option["type"] != "effort":
            continue
        if efforts is not None:
            raise UpdateError(
                f"models.dev {provider} model {model_id!r} has duplicate effort options"
            )
        values = option.get("values")
        if not isinstance(values, list) or not values or any(
            not isinstance(value, str) for value in values
        ):
            raise UpdateError(
                f"models.dev {provider} model {model_id!r} has invalid effort values"
            )
        if len(values) != len(set(values)):
            raise UpdateError(
                f"models.dev {provider} model {model_id!r} has invalid effort values"
            )
        efforts = ["high" if value in {"none", "default"} else value for value in values]
        if any(
            value not in REASONING_LEVELS for value in efforts
        ):
            raise UpdateError(
                f"models.dev {provider} model {model_id!r} has invalid effort values"
            )
    return tuple(dict.fromkeys(efforts or ("high",)))


def parse_provider_models(
    text: str,
    providers: Sequence[str],
    exclusions_by_provider: Mapping[str, Sequence[str]],
    *,
    reporter: Callable[[str], None] | None = None,
) -> dict[str, list[ProviderModel]]:
    try:
        payload = json.loads(text)
    except json.JSONDecodeError as error:
        raise UpdateError("models.dev provider catalogue returned invalid JSON") from error
    if not isinstance(payload, dict) or not payload:
        raise UpdateError("models.dev provider catalogue must be a non-empty mapping")
    result: dict[str, list[ProviderModel]] = {}
    for provider in providers:
        record = payload.get(provider)
        if not isinstance(record, dict):
            raise UpdateError(f"models.dev has no valid provider {provider!r}")
        if record.get("id") != provider:
            raise UpdateError(f"models.dev provider {provider!r} has a mismatched id")
        models = record.get("models")
        if not isinstance(models, dict) or not models:
            raise UpdateError(f"models.dev provider {provider!r} has no valid models mapping")
        exclusions = set(exclusions_by_provider.get(provider, ()))
        stale = sorted(exclusions - set(models))
        if reporter and stale:
            reporter(
                f"provider {provider} exclusions absent from current models.dev catalogue: "
                + ", ".join(stale)
            )
        selected: list[ProviderModel] = []
        for model_id in sorted(models):
            model = models[model_id]
            if (
                not isinstance(model_id, str)
                or not model_id
                or model_id != model_id.strip()
                or not isinstance(model, dict)
                or model.get("id") != model_id
            ):
                raise UpdateError(f"models.dev {provider} model {model_id!r} has an invalid record")
            name, status, base_model = model.get("name"), model.get("status"), model.get("base_model")
            if not isinstance(name, str) or not name.strip():
                raise UpdateError(f"models.dev {provider} model {model_id!r} has an invalid name")
            cleaned_name = clean_model_name(name)
            if not cleaned_name:
                raise UpdateError(
                    f"models.dev {provider} model {model_id!r} has an invalid name"
                )
            if status is not None and not isinstance(status, str):
                raise UpdateError(f"models.dev {provider} model {model_id!r} has an invalid status")
            if base_model is not None and (not isinstance(base_model, str) or not base_model.strip()):
                raise UpdateError(f"models.dev {provider} model {model_id!r} has invalid base_model")
            levels = _reasoning_levels(provider, model_id, model)
            if status == "deprecated" or model_id in exclusions:
                continue
            selected.append(
                ProviderModel(
                    provider,
                    model_id,
                    cleaned_name,
                    levels,
                    base_model if isinstance(base_model, str) else None,
                )
            )
        result[provider] = selected
    return result


def fetch_provider_models(
    client: HttpClient,
    providers: Sequence[str],
    exclusions_by_provider: Mapping[str, Sequence[str]],
    *,
    reporter: Callable[[str], None] | None = None,
) -> dict[str, list[ProviderModel]]:
    text = client.get_text(
        MODELS_DEV_PROVIDER_URL, purpose="models.dev provider model catalogue"
    )
    return parse_provider_models(
        text, providers, exclusions_by_provider, reporter=reporter
    )


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--provider-config", type=Path, default=DEFAULT_PROVIDER_CONFIG_PATH)
    parser.add_argument("--provider", action="append")
    args = parser.parse_args(argv)
    try:
        config = load_provider_config(args.provider_config)
        providers = resolve_provider_ids(config, args.provider)
        models = fetch_provider_models(
            HttpClient(), providers, config.excluded_models_by_provider, reporter=lambda m: print(m, file=sys.stderr)
        )
    except UpdateError as error:
        print(f"error: {error}", file=sys.stderr)
        return 1
    json.dump(
        {
            provider: [
                {"id": model.model_id, "name": model.display_name, "reasoning": model.reasoning_levels}
                for model in provider_models
            ]
            for provider, provider_models in models.items()
        },
        sys.stdout,
        ensure_ascii=False,
        indent=2,
    )
    print()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
