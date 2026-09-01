---
kind: feature-tasks
feature: F20-strategies
version: "1.1"
task_count: 7
---

# F20 — Strategies: Tasks

Feature prerequisites: F10, F18, and F19. Canonical enums come from `specs/global/CONTRACTS.md` §4.

## Task F20-T1: Maintain shared strategy contracts

**Files:** `internal/pick/strategy/strategy.go`, `internal/pick/strategy/strategy_test.go`

1. Test route-key ordering, priority ordering, errors, and usage-disable messages.
2. Implement `State`, the complete shared `[strategy]` `Config` (`default`, `default_profile`, `tier1_share`, `tier2_share`), shared sorting helpers, and the errors in `CONTRACTS.md` §3-§5. Consumers must use this type rather than partial anonymous decoders.
3. Run `go test ./internal/pick/strategy/...`.

## Task F20-T2: Implement priority selection

**Files:** `internal/pick/strategy/priority.go`, `internal/pick/strategy/priority_test.go`

1. Test configured provider ordering, unlisted providers, score ties, route-key ties, input immutability, and empty input.
2. Implement `priority` exactly as `SPEC.md` §3.1.
3. Run `go test ./internal/pick/strategy/...`.

## Task F20-T3: Implement round-robin selection

**Files:** `internal/pick/strategy/round_robin.go`, `internal/pick/strategy/round_robin_state.go`, corresponding tests

1. Test cursor rotation, scope isolation, concurrent locking, permissions, corruption recovery, and dry-run behavior.
2. Implement `round-robin` exactly as `SPEC.md` §3.2.
3. Run `go test -race ./internal/pick/strategy/...`.

## Task F20-T4: Implement least-used selection

**Files:** `internal/pick/strategy/least_used.go`, `internal/pick/strategy/least_used_test.go`

1. Test minimum pressure, score and route ties, disabled usage, missing pressure, and empty input.
2. Implement `least-used` exactly as `SPEC.md` §3.3.
3. Run `go test ./internal/pick/strategy/...`.

## Task F20-T5: Implement most-used selection

**Files:** `internal/pick/strategy/most_used.go`, `internal/pick/strategy/most_used_test.go`

1. Test maximum pressure, score and route ties, disabled usage, missing pressure, and empty input.
2. Implement `most-used` exactly as `SPEC.md` §3.4.
3. Run `go test ./internal/pick/strategy/...`.

## Task F20-T6: Implement closest-to-reset selection

**Files:** `internal/pick/strategy/closest_to_reset.go`, `internal/pick/strategy/closest_to_reset_test.go`

1. Test earliest timestamp, score and route ties, disabled usage, missing reset data, and empty input.
2. Implement `closest-to-reset` exactly as `SPEC.md` §3.5.
3. Run `go test ./internal/pick/strategy/...`.

## Task F20-T7: Maintain the strategy registry

**Files:** `internal/pick/strategy/registry.go`, `internal/pick/strategy/registry_test.go`

1. Test all five implementations, the priority default, degraded availability, unknown names, and rejection of removed names.
2. `ParseStrategy("")` must return `StrategyPriority`.
3. `score`, `weighted-random`, and `cost-optimal` must return `ErrUnknownStrategy`.
4. Run `go test ./internal/pick/strategy/...` and `go test -tags nousage ./internal/pick/strategy/...`.
