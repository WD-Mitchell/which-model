package schema

// The four JSON Schema documents for the --json surfaces agents consume
// (specs/features/F28-agent-skills/CONTRACTS.md §2.1). Each is the
// docs/plan/annex-c-agent-integration.md §4.1/§4.2/§4.3 base document with
// exactly the CONTRACTS §2.1 deltas applied: schema_version const "2.0",
// usage_enabled required at the root with conditionally required
// usage_disabled_reason, and the degraded-mode optionality rules. 2-space
// indented JSON, each ending with exactly one "\n".

const usageSchemaJSON = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://github.com/WD-Mitchell/which-model/schema/usage-snapshot.json",
  "title": "which-model usage --json output",
  "type": "object",
  "required": ["schema_version", "usage_enabled", "snapshots"],
  "properties": {
    "schema_version": { "type": "string", "const": "2.0" },
    "usage_enabled": { "type": "boolean" },
    "usage_disabled_reason": {
      "type": "string",
      "enum": ["flag", "config", "compiled_out", "no_providers_enabled"]
    },
    "snapshots": {
      "type": "array",
      "items": { "$ref": "#/$defs/Snapshot" }
    }
  },
  "if": { "properties": { "usage_enabled": { "const": false } }, "required": ["usage_enabled"] },
  "then": { "required": ["usage_disabled_reason"] },
  "$defs": {
    "Unit": {
      "type": "string",
      "enum": ["percent", "tokens", "credits", "usd", "requests", "kwh", "none"]
    },
    "Window": {
      "type": "object",
      "required": ["id", "label", "unit", "usage_known"],
      "properties": {
        "id": { "type": "string" },
        "label": { "type": "string" },
        "unit": { "$ref": "#/$defs/Unit" },
        "used_percent": { "type": ["number", "null"] },
        "used": { "type": ["number", "null"] },
        "limit": { "type": ["number", "null"] },
        "remaining": { "type": ["number", "null"] },
        "unlimited": { "type": "boolean", "default": false },
        "window_minutes": { "type": ["integer", "null"] },
        "resets_at": { "type": ["string", "null"], "format": "date-time" },
        "reset_hint": { "type": "string" },
        "model_scope": { "type": "array", "items": { "type": "string" } },
        "synthetic": { "type": "boolean", "default": false },
        "usage_known": { "type": "boolean" }
      },
      "additionalProperties": false
    },
    "Failure": {
      "type": "object",
      "required": ["code", "message"],
      "properties": {
        "code": { "type": "string" },
        "message": { "type": "string" }
      },
      "additionalProperties": false
    },
    "Snapshot": {
      "type": "object",
      "required": ["provider", "windows", "fetched_at", "source", "confidence"],
      "properties": {
        "provider": { "type": "string" },
        "account": { "type": "string" },
        "plan": { "type": "string" },
        "windows": { "type": "array", "items": { "$ref": "#/$defs/Window" } },
        "fetched_at": { "type": "string", "format": "date-time" },
        "source": { "type": "string", "enum": ["oauth", "api", "cli", "web", "local", "cache"] },
        "confidence": { "type": "string", "enum": ["live", "cached", "estimated"] },
        "stale": { "type": "boolean", "default": false },
        "error": { "oneOf": [{ "$ref": "#/$defs/Failure" }, { "type": "null" }] }
      },
      "additionalProperties": false
    }
  }
}
`

const pickSchemaJSON = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://github.com/WD-Mitchell/which-model/schema/pick-result.json",
  "title": "which-model pick --json output",
  "type": "object",
  "required": ["schema_version", "usage_enabled", "profile", "strategy", "candidates", "excluded_candidates"],
  "properties": {
    "schema_version": { "type": "string", "const": "2.0" },
    "usage_enabled": { "type": "boolean" },
    "usage_disabled_reason": {
      "type": "string",
      "enum": ["flag", "config", "compiled_out", "no_providers_enabled"]
    },
    "profile": { "type": "string" },
    "strategy": {
      "type": "string",
      "enum": ["priority", "round-robin", "least-used", "most-used", "closest-to-reset"]
    },
    "candidates": {
      "type": "array",
      "items": { "$ref": "#/$defs/Candidate" }
    },
    "excluded_candidates": {
      "type": "array",
      "items": { "$ref": "#/$defs/ExcludedCandidate" }
    }
  },
  "if": { "properties": { "usage_enabled": { "const": false } }, "required": ["usage_enabled"] },
  "then": { "required": ["usage_disabled_reason"] },
  "$defs": {
    "Route": {
      "type": "object",
      "required": ["provider", "model_id", "model", "reasoning", "window_ids"],
      "properties": {
        "provider": { "type": "string" },
        "model_id": { "type": "string" },
        "model": { "type": "string" },
        "reasoning": {
          "type": "string",
          "enum": ["minimal", "low", "medium", "high", "xhigh", "max", "default"]
        },
        "window_ids": { "type": "array", "items": { "type": "string" } },
        "provenance": { "type": "string" }
      },
      "additionalProperties": false
    },
    "Candidate": {
      "type": "object",
      "required": ["candidate_id", "route", "model_score", "provider_weight", "final_score"],
      "properties": {
        "candidate_id": { "type": "string" },
        "route": { "$ref": "#/$defs/Route" },
        "model_score": { "type": "string" },
        "band": { "type": "string" },
        "band_weight": { "type": "string" },
        "provider_weight": { "type": "string" },
        "final_score": { "type": "string" },
        "warnings": { "type": "array", "items": { "type": "string" } }
      },
      "additionalProperties": false
    },
    "ExcludedCandidate": {
      "type": "object",
      "required": ["route", "reason_code", "reason"],
      "properties": {
        "route": { "$ref": "#/$defs/Route" },
        "reason_code": {
          "type": "string",
          "enum": ["band_gated", "no_score_row", "auth_required", "provider_error", "not_in_availability_list"]
        },
        "reason": { "type": "string" }
      },
      "additionalProperties": false
    }
  }
}
`

const explainSchemaJSON = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://github.com/WD-Mitchell/which-model/schema/explain-result.json",
  "title": "which-model explain --json output",
  "type": "object",
  "required": ["schema_version", "usage_enabled", "candidate", "evidence"],
  "properties": {
    "schema_version": { "type": "string", "const": "2.0" },
    "usage_enabled": { "type": "boolean" },
    "usage_disabled_reason": {
      "type": "string",
      "enum": ["flag", "config", "compiled_out", "no_providers_enabled"]
    },
    "candidate": { "type": "string", "description": "candidate_id echoed back" },
    "evidence": { "$ref": "#/$defs/Evidence" }
  },
  "if": { "properties": { "usage_enabled": { "const": false } }, "required": ["usage_enabled"] },
  "then": { "required": ["usage_disabled_reason"] },
  "$defs": {
    "Evidence": {
      "type": "object",
      "required": ["profile", "score_inputs", "route_provenance", "excluded_candidates"],
      "properties": {
        "profile": { "type": "string" },
        "score_inputs": {
          "type": "object",
          "description": "tier1 + category composite values that produced model_score",
          "additionalProperties": { "type": "number" }
        },
        "band": {
          "type": "object",
          "required": ["name", "used_percent", "weight"],
          "properties": {
            "name": { "type": "string" },
            "used_percent": { "type": "number" },
            "weight": { "type": "number" }
          },
          "additionalProperties": false
        },
        "snapshot_age_seconds": { "type": "number" },
        "confidence": { "type": "string", "enum": ["live", "cached", "estimated"] },
        "route_provenance": {
          "type": "string",
          "description": "how the provider<->model route was resolved, e.g. 'static-config' | 'live-availability-probe'"
        },
        "excluded_candidates": {
          "type": "array",
          "items": { "$ref": "https://github.com/WD-Mitchell/which-model/schema/pick-result.json#/$defs/ExcludedCandidate" }
        },
        "last_verified": {
          "type": "string",
          "format": "date-time",
          "description": "when the provider adapter last confirmed this route/model was live-accepted by the target harness"
        }
      },
      "additionalProperties": false
    }
  }
}
`

const routesSchemaJSON = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://github.com/WD-Mitchell/which-model/schema/routes-list.json",
  "title": "which-model routes list --json output",
  "type": "object",
  "required": ["schema_version", "routes"],
  "properties": {
    "schema_version": { "type": "string", "const": "2.0" },
    "routes": { "type": "array", "items": { "$ref": "#/$defs/Route" } }
  },
  "$defs": {
    "Route": {
      "type": "object",
      "required": ["provider", "model_id", "model", "reasoning", "window_ids"],
      "properties": {
        "provider": { "type": "string" },
        "model_id": { "type": "string" },
        "model": { "type": "string" },
        "reasoning": {
          "type": "string",
          "enum": ["minimal", "low", "medium", "high", "xhigh", "max", "default"]
        },
        "window_ids": { "type": "array", "items": { "type": "string" } },
        "provenance": {
          "type": "string",
          "enum": ["provider_live", "models_dev", "user_declared"]
        }
      },
      "additionalProperties": false
    }
  }
}
`
