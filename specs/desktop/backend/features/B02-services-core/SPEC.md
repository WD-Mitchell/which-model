---
kind: feature-spec
version: "1.0"
feature: B02-services-core
project: which-model-desktop
---

# B02-services-core — Services Construction, Locking, Boundary Mapping

## 1. Purpose

B02 implements the skeleton every other backend feature hangs methods on: the `Services` struct and `New()` eager loader (B00 CONTRACTS §2), the single-writer locking discipline (B00 SPEC §2.2), the sentinel-error → `ErrorDTO` boundary mapper (B00 CONTRACTS §3), the D00-owned files `dto.go` (DTO structs + route-key parser/formatter) and `events.go` (event consts), the weight-conversion helpers (B00 CONTRACTS §4), catalog cache invalidation, and the shared test helper (B00 CONTRACTS §5). No user-facing service methods live here — B03..B11 add those.

Inherits: `specs/global/*`, `specs/desktop/global/*` (D00), `specs/desktop/backend/*` (B00). Depends on B01 (`config.AtomicWriteFile`, GUI config sections).

## 2. Behaviour

1. **Construction inputs.** `New(paths config.Paths, cfg *config.Config, emit EmitFunc)` takes an already-loaded config (the host calls `config.Load`; `New` never re-reads `config.toml`), resolved `Paths`, and a non-nil emit function. A nil `emit` is replaced with a no-op so tests and `serve` can omit it.

2. **Eager load order.** `New` loads, in this order, failing fast on the first error: (a) scores CSV; (b) benchmarks config; (c) routes table. On success `Services` holds parsed `[]catalog.ScoreRow`, `*score.BenchmarkConfig`, and `routing.Table` in memory; features read these caches under RLock and never re-read the files outside `reloadCatalog`.

3. **Scores CSV.** Read from `<CacheDir>/catalog/available_model_scores.csv` and parsed with `score.ParseScoresCSV`. A missing file returns the typed sentinel error `errScoresMissing` whose message contains the absolute expected path and the remedy `run: which-model catalog refresh` (exact string in CONTRACTS §7); it maps to `io_error`. A present-but-invalid CSV returns the `score.Error` wrapped, also `io_error`. `New` NEVER degrades to an empty catalog (B00 Decisions; D00 §3).

4. **Benchmarks config.** Read from `catalog.benchmark_config_path` when that config key is set, else `<CacheDir>/catalog/benchmarks.toml`, parsed with `score.ParseBenchmarkConfig`. Missing or invalid → fail fast, `io_error`, message names the resolved path. (The GUI cannot walk cwd like the CLI does — Deviations below.)

5. **Routes table.** Loaded via `routing.LoadTable` from `<CacheDir>/catalog/routes.json`; when that file does not exist, the legacy CLI location `<CacheDir>/routes.json` is tried before giving up. A missing table (both paths) is NOT fatal: `Services` starts with an empty `routing.Table`, availability (B00 CONTRACTS §6.3) is empty, and the warning string in CONTRACTS §7 is recorded and returned by `Warnings()`. A present-but-corrupt table IS fatal (`io_error` naming the path) — silent data loss is worse than a missing cache.

6. **Locking.** One `sync.RWMutex` guards config document, catalog caches, and routes table. Read methods take `RLock` for the whole read. Every mutating method follows exactly: (a) validate inputs BEFORE locking (validation failures take no lock, write nothing, emit nothing); (b) `Lock`; (c) deep-copy the raw config document and mutate the copy; (d) `config.MarshalTOML` on the copy; (e) `config.AtomicWriteFile(paths.UserConfigFile, data)`; (f) swap the copy into memory; (g) `Unlock`; (h) emit exactly one event. Any failure at (c)–(e) unlocks and returns with in-memory state untouched and no event emitted. Emit happens after unlock so a re-entrant read from an event handler cannot deadlock.

7. **Error mapping.** `toErrorDTO(err)` converts any internal error at the boundary using `errors.Is`/`errors.As` against the six sentinels (CONTRACTS §5); a wrapped `ErrorDTO` passes through unchanged; everything unmatched → `io_error` with the error's message sanitised (no home-directory prefixes other than the config path). `ErrorDTO` implements `error`, so hosts return it directly.

8. **Route keys.** `ParseRouteKey`/`FormatRouteKey` implement the D00 CONTRACTS §1 grammar exactly. Parsing splits on the FIRST `/` and the LAST `@`; each component is then validated against its character class and the reasoning enum. Every failure is `errValidation` wrapped with the exact messages in CONTRACTS §6. `FormatRouteKey` is pure concatenation and performs no validation (callers hold already-valid parts).

9. **Weight conversion.** `dtoWeights` rounds each decimal weight half-up to an int and DROPS keys that round to 0. `engineWeights` drops keys with `v <= 0` (0 = "ignored", D00 §2 ProfileSummary) and rejects `v > 5` with `errValidation`; accepted ints convert via `decimal.NewFromInt`. `engineProfile` builds a `catalog.Profile` from a `ProfileDetail`: `Tier1Share = CoreShare/100`, `Tier2Share = 1 − Tier1Share` (decimal division, no rounding), weights via `engineWeights`. `round2(d)` = `d.Round(2).InexactFloat64()` — the ONLY place decimal becomes float64 (D00 §2.7).

10. **Cache invalidation.** `reloadCatalog()` re-runs eager-load steps (a)–(c) of clause 2 under the write lock and swaps all three caches atomically together; on any error the previous caches remain and the error is returned. B05 calls it after re-deriving scores; it emits nothing (the caller owns the `catalog:changed` emit).

11. **Events.** `events.go` declares the five D00 CONTRACTS §3 event names as untyped string consts. No other event name may be emitted anywhere in `internal/service`.

12. **Test helper.** `newTestServices(t, opts...)` builds a `t.TempDir()` tree (config/cache/state), writes the default fixtures (B00 CONTRACTS §5), applies options, resolves `Paths` pointing into the tree, loads config, and returns `New`'s result plus an `emitRecorder` whose `Events()` snapshot is safe to call concurrently. Fixture details in CONTRACTS §8.

## 3. Error behaviour

- `New` returns errors already convertible by `toErrorDTO`; the host surfaces them as a blocking startup dialog, not a toast.
- Validation happens before locking; a validation failure is observable as: no config file mtime change, no event recorded (asserted via `emitRecorder` per B00 CONTRACTS §6.5).
- `reloadCatalog` failure leaves the previous catalog serving reads — stale beats broken mid-session; the error propagates to the mutation that triggered it.

## 4. Decisions

| Decision | Value | Rationale |
|---|---|---|
| Missing routes table | Non-fatal: empty availability + `Warnings()` entry | Ranking degrades visibly; scores CSV absence is the true "no catalog" case |
| Routes path | `<CacheDir>/catalog/routes.json`, fallback `<CacheDir>/routes.json` | Desktop groups catalog artefacts under `catalog/`; fallback keeps CLI-produced tables usable |
| Benchmarks path | config key `catalog.benchmark_config_path`, else `<CacheDir>/catalog/benchmarks.toml` | GUI has no meaningful cwd for the CLI's walk-up |
| Emit after unlock | Step (h) outside the critical section | Event handlers may call read methods; emitting under lock risks deadlock |
| Validate before lock | Step (a) precedes `Lock` | Cheap rejection, no writer starvation, "zero events on failure" trivially holds |
| Rounding primitive | `decimal.Round(2).InexactFloat64()` in `round2` only | Single boundary crossing point (D00 §2.7) |

## Deviations

- **B00 SPEC §2.8 / CLI benchmarks resolution**: the CLI resolves `benchmarks.toml` by walking cwd upward (`pkg/whichmodel/catalog_config.go`); B02 clause 4 replaces that with a fixed cache path + config override, because a menu-bar app's cwd is meaningless.

## 5. Out of scope

Service methods (B03–B11); `AtomicWriteFile`/GUI config sections (B01); history aggregation (B11); Wails registration and event transport (S02/S04).
