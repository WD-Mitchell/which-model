---
kind: feature-contracts
version: "1.0"
feature: U03-weight-controls
project: which-model-desktop
---

# U03-weight-controls — Contracts

## 1. Files

| Folder (`packages/ui/src/components/`) | Contents |
|---|---|
| `WeightRow/` | `WeightRow.tsx`, `WeightRow.module.css`, `WeightRow.test.tsx` |
| `BalanceSlider/` | `BalanceSlider.tsx`, `BalanceSlider.module.css`, `BalanceSlider.test.tsx` |
| `ComplexityScale/` | `ComplexityScale.tsx`, `ComplexityScale.module.css`, `ComplexityScale.test.tsx` |
| `WeightEditor/` | `WeightEditor.tsx`, `WeightEditor.module.css`, `WeightEditor.test.tsx` |

All exported from the `packages/ui` barrel. Imports allowed: react, U02's `usePointerFraction` and `cx`, sibling U03 components (WeightEditor → WeightRow). No `@which-model/core`, no data fetching (U00 SPEC §2.2).

## 2. Props (exported interfaces)

```ts
export type WeightVariant = 'step' | 'bar' | 'slider';

export interface WeightRowProps {
  variant: WeightVariant;
  label: string;              // benchmark key, mono
  value: number;              // 0..5 integer (D00 §6); clamped for render
  accent?: boolean;           // label in --color-accent-300 (popover cost row)
  readOnly?: boolean;         // no cursor/handlers/focus; visuals unchanged
  labelWidth?: 104 | 150;     // default 104 (popover); 150 in profile detail
  valueStyle?: 'compact' | 'verbose';
  // 'compact' (default): bare digit, 12px col, right-aligned, 11px mono.
  // 'verbose': 56px col, `${value} / 5` in accent-300, or `ignored` dim
  //            when value === 0 (zero also dims the label).
  onChange?: (v: number) => void; // fires only when mapped value differs
  onRemove?: () => void;      // present → 14px × button; absent → 14px spacer
}

export interface BalanceSliderProps {
  core: number;               // 10..90, step 5 (D00 §6)
  readOnly?: boolean;
  showRatio?: boolean;        // centre `${core} / ${100 - core}` caption
                              // in accent-300 (profile-detail pfRatio)
  onChange?: (v: number) => void;
  // mapper: v = max(10, min(90, round(f * 20) * 5)); fires only on change
}

export interface ComplexityScaleProps {
  stop: number;               // 0..4 (D00 §6: stops at 0/25/50/75/100%)
  labels?: [string, string];  // default ['simple action', 'planning']
  profileName?: string;       // centred accent-300 name 15px below labels
  readOnly?: boolean;
  onStop?: (i: number) => void; // mapper round(f * 4); fires only on change
}

export interface WeightEditorRow {
  key: string;                // benchmark key (label + callback identity)
  value: number;              // 0..5
  accent?: boolean;           // popover cost row
}

export interface WeightEditorProps {
  variant: 'popover' | 'profile-detail';
  sliderStyle: WeightVariant;           // passed through to every WeightRow
  coreRows: WeightEditorRow[];
  taskRows: WeightEditorRow[];
  sectionPcts: { core: string; task: string }; // e.g. '60%' / '40%'
  readOnly?: boolean;                   // profile detail, builtin profiles
  addable?: string[];                   // popup contents (popover only)
  addOpen?: boolean;                    // add-metric popup visibility
  onChangeWeight?: (key: string, v: number) => void;
  onRemoveWeight?: (key: string) => void; // task rows, popover only
  onAddMetric?: (key: string) => void;    // host adds at weight 3, closes
  onToggleAdd?: () => void;               // '+ Add metric' button
  onRevert?: () => void;                  // 'Revert' button
}
```

Drag on WeightRow uses `usePointerFraction` with mapper `v = Math.round(f * 5)`. Keyboard on all three sliders: `[role=slider]`, `aria-valuemin/max/now`, arrows ±1 step (weights 1, balance 5, complexity 1 stop), clamped, change-only-on-diff (SPEC §2.10).

## 3. Geometry (from the mockup; D00 §6 tokens not restated)

| Variant | Track / segments | Fill | Knob |
|---|---|---|---|
| `step` | 5 flex spans, gap 3px, height 6px, radius 2, unfilled bg 12% text tint | segment `i` (1-based) accent-500 when `i <= value` | — |
| `bar` | block 6px high, radius 3, 12% text tint, overflow hidden | accent-500, `width: (value/5*100)%` | — |
| `slider` | block 3px high, radius 2, 12% text tint | accent-500, `width: (value/5*100)%` | 12px, `top: -4.5px`, `left: calc(pct% - 6px)`, D00 knob ring |

Row frame: flex align-center, gap 10px (12px in profile detail), font 12.5px; control column `flex: 1`, `padding: 5px 0`, `cursor: pointer` (`default` + no handler when readOnly), `max-width: 300px` in profile detail. Label 104/150px mono 11.5px ellipsis. Remove button 14×14 `.ib`, 9×9 × icon, 40% text tint.

BalanceSlider: caption row mono 10px uppercase tracking .06em 55% tint; slider row height 14px, gap 3px; bars 5px high radius 3 (accent-500 `flex: core` | accent-800 `flex: 100-core`); knob 14px, D00 ring.

ComplexityScale: track 5px high radius 3, gradient accent-800 → accent-500; ticks 1×13px at `top: -4px`, `left: i*25%`, 20% text tint; knob 15px at `top: -5px`, `left: calc(stop*25% - 7px)`; labels mono 9px 45% tint, margin-top 9px; profile name heading-font 15px accent-300, margin-top 15px, centred.

WeightEditor: section stack gap 12px; headers 10px uppercase tracking .11em 42% tint with mono 10.5px 62% pct right-aligned; add/revert buttons `btn-ghost` 11.5px padding 2px 6px; popup absolute `left: 0; bottom: 26px`, 180px wide, padding 5px, radius 8, surface bg, `--shadow-md`, column gap 1px, max-height 150px auto-scroll; items mono 11.5px, padding 6px 9px, radius 5, 80% text tint, pointer cursor.

## 4. Copy (exact strings)

| Where | String |
|---|---|
| Core section header | `core benchmarks (higher = better, cheaper, faster)` (paren part not uppercased/tracked) |
| Task section header | `task benchmarks` |
| Add button | `+ Add metric` |
| Revert button | `Revert` |
| Scale labels (defaults) | `simple action`, `planning` |
| Verbose value | `${value} / 5`; zero renders `ignored` |
| Balance captions | `core`, `task`; ratio `${core} / ${100 - core}` |

## 5. Test fixtures (vitest + testing-library, synthetic PointerEvents)

Drag helper: pointerdown on the control (rect mocked to width 100, left 0), pointermove at clientX, pointerup. `f = clamp(clientX/100, 0, 1)`.

| # | Component / setup | Sequence | Expected callbacks |
|---|---|---|---|
| 1 | WeightRow `value=2`, each variant | down+move x=90 | `onChange(5)` once (round(0.9×5)) |
| 2 | WeightRow `value=3` | moves x=55, 62, 64 | `onChange` NOT called (all map to 3 — no-op repeats suppressed) |
| 3 | WeightRow `value=0` | moves x=10, 30, 50 | `onChange(1)`, then `(2)`, then `(3)` — one per distinct mapped value; note repeats against the *prop* value: if the host does not re-render, x=30 then x=32 fires `(2)` twice ⇒ tests re-render with each new value |
| 4 | WeightRow `readOnly` | down+move x=90; keyboard arrows | no `onChange`; control not focusable; `cursor: default` |
| 5 | WeightRow keyboard `value=5` | focus, ArrowRight; then ArrowLeft | no call (clamped 5); then `onChange(4)` |
| 6 | BalanceSlider `core=60` | move x=0; x=100; x=51 | `onChange(10)` (clamp), `(90)` (clamp), `(50)` (round(10.2)×5 snap) |
| 7 | BalanceSlider `core=50` | move x=50, x=52 | no call (both map to 50) |
| 8 | ComplexityScale `stop=1` | move x=70; x=100 | `onStop(3)` (round(0.7×4)), `onStop(4)` |
| 9 | ComplexityScale `stop=2` | move x=45..55 sweep | no call until mapped stop ≠ 2 |
| 10 | WeightEditor popover, 3 core + 2 task rows | render | core rows have no remove button + 14px spacer; task rows each have ×; headers show `sectionPcts` |
| 11 | WeightEditor popover | click task × ; drag core row x=90 | `onRemoveWeight(key)`; `onChangeWeight(key, 5)` |
| 12 | WeightEditor `addOpen`, `addable=['mmlu','gpqa']` | click `mmlu`; click `+ Add metric`; click `Revert` | `onAddMetric('mmlu')`; `onToggleAdd()`; `onRevert()` |
| 13 | WeightEditor profile-detail, `readOnly` | render + drag attempts | 150px labels, verbose values (`3 / 5` / `ignored`), no × and no spacer, no add/revert row, zero callbacks |

Also assert (per U00 SPEC §2.5): representative render for each variant (step fill count, bar/slider width and knob position styles computed from `value`), `aria-valuenow` correctness, and that pointerup removes listeners (a move after up fires nothing).

## 6. External symbols referenced

| Symbol | Source |
|---|---|
| `usePointerFraction` | U00 CONTRACTS §3 (owned by U02) |
| `cx` | U00 CONTRACTS §2 (owned by U02) |
| Tokens: weight 0–5, balance 10..90/5, stops 0..4, 12px knob ring, mono stack | D00 CONTRACTS §6 |

## 7. Notes for consumers

- U06 binds WeightEditor + BalanceSlider to the popover overrides store; `onAddMetric` must set the key to 3 and close the popup; `onRevert` restores the profile baseline (mockup semantics — host-owned, not U03's).
- U05 uses ComplexityScale with `profileName`; U10 (profile detail) uses `variant: 'profile-detail'`, `labelWidth` 150 via the variant, `showRatio` BalanceSlider, and `readOnly` for builtin profiles.
- Components never reorder rows: pass `coreRows`/`taskRows` in display order.
