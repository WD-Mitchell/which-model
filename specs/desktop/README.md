---
kind: roadmap
version: "1.0"
project: which-model-desktop
---

# which-model Desktop — SDD Roadmap

Spec-as-source development for the desktop app (Wails v3 + React). One hierarchical tree covers backend, UI, and shell. Child specs inherit ALL parent SPEC/CONTRACTS clauses; the deepest spec wins on conflict (with a mandatory Deviations note).

## Reading order (mandatory before implementing any feature)

1. `specs/global/SPEC.md` + `CONTRACTS.md` — the CLI tree's rules still bind Go code
2. [`global/SPEC.md`](./global/SPEC.md) — D00: layering, monorepo, naming, boundary rules
3. [`global/CONTRACTS.md`](./global/CONTRACTS.md) — canonical DTOs, events, error codes, EngineHost, visual tokens
4. Your area parent: [`backend/`](./backend/SPEC.md) | [`ui/`](./ui/SPEC.md) | [`shell/`](./shell/SPEC.md)
5. Your feature's `SPEC.md` + `CONTRACTS.md`
6. [`DEPENDENCY-GRAPH.md`](./DEPENDENCY-GRAPH.md) — work units, waves, gates
7. The mockup [`mockup/demo.dc.html`](./mockup/demo.dc.html) — normative visuals/behaviour (UI features)

## Feature index

| Feature | Title | Area | Depends on |
|---|---|---|---|
| B01 | config-schema | backend | — |
| B02 | services-core | backend | B01 |
| B03 | profiles | backend | B02, B11 |
| B04 | pick | backend | B02, B03, B06, B11 |
| B05 | catalog-groups | backend | B02 |
| B06 | providers-routes | backend | B02 |
| B07 | harnesses | backend | B02, B04 (RecordPick) |
| B08 | usage | backend | B02 |
| B09 | favourites | backend | B02, B06 |
| B10 | settings | backend | B02 |
| B11 | history | backend | — |
| U01 | core-types-host | ui | D00 |
| U02 | theme-primitives | ui | U01 |
| U03 | weight-controls | ui | U02 |
| U04 | results | ui | U02 |
| U05 | popover-landing | ui | U03, U04 |
| U06 | popover-weights | ui | U03, U04 |
| U07 | settings-shell | ui | U02 |
| U08 | page-profiles | ui | U03, U04, U07 |
| U09 | page-groups-benchmarks | ui | U02, U07 |
| U10 | page-providers | ui | U02, U07 |
| U11 | page-harnesses | ui | U02, U07 |
| U12 | page-general | ui | U02, U07 |
| U13 | page-usage | ui | U02, U07 |
| U14 | page-favourites-agent | ui | U02, U07 |
| S01 | scaffold | shell | D00 |
| S02 | tray-popover | shell | S01, B02 |
| S03 | settings-window | shell | S01 |
| S04 | bindings-host | shell | S02, S03, all B* |
| S05 | integrations | shell | S04, B07, B10 |

## File ownership

Each feature's CONTRACTS §"Package and files" table is the lock table: a work unit may only create/edit files its feature owns. Cross-feature changes require editing the owning contract first.
