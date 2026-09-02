// U12 — General settings page: the `system` and `results display` sections,
// ported from the mockup's <sc-if onGeneral> block (demo.dc.html L591-696).
//
// Layout note: unlike the full-bleed list pages, General draws every row inside
// ONE 22px-inset column (mockup L592 `padding:0 22px 12px`), so the row rules
// stop short of the content edge and there is no hover tint (the mockup rows
// carry no class="row"). <main> supplies no padding of its own (U07 contract),
// and the config-path footer now lives in the sidebar, not on this page.
import { useCallback, useEffect, useState } from 'react'
import { Input, SegmentedControl, Toggle, useToast } from '@which-model/ui'
import type { GUISettings } from '@which-model/core'
import { useSettings } from '../../../lib/queries'
import { getHost } from '../../../lib/host'
import { DetailHeader } from '../../DetailHeader'
import { PAGE_META } from '../../pages'
import type { PageComponentProps } from '../../pages'
import styles from './GeneralPage.module.css'

/** Shortcut options (mockup L1531). The glyph strings are the mockup's literal
 *  labels; the persisted values stay the engine's ASCII ids. */
const SHORTCUT_OPTS: ReadonlyArray<{ value: GUISettings['shortcut']; label: string }> = [
  { value: 'alt+space', label: '⌥ Space' },
  { value: 'ctrl+space', label: '⌃ Space' },
  { value: 'cmd+shift+m', label: '⇧⌘ M' },
]

/** Ranks-held options (mockup L1530 `holdOpts`). */
const HOLD_OPTS = [3, 5, 10] as const

/** Update-frequency options (mockup L618-621) — capitalised labels, lowercase
 *  values, matching GUISettings['auto_update_frequency']. */
const FREQ_OPTS: ReadonlyArray<{ value: GUISettings['auto_update_frequency']; label: string }> = [
  { value: 'hourly', label: 'Hourly' },
  { value: 'daily', label: 'Daily' },
  { value: 'weekly', label: 'Weekly' },
  { value: 'monthly', label: 'Monthly' },
]

/** System switches. read/patch keep the mapping to GUISettings explicit —
 *  no computed-key casts. */
const TOGGLES: ReadonlyArray<{
  name: string
  read(s: GUISettings): boolean
  patch(on: boolean): Partial<GUISettings>
}> = [
  {
    name: 'Show menu bar icon',
    read: (s) => s.show_menu_bar_icon,
    patch: (on) => ({ show_menu_bar_icon: on }),
  },
  {
    name: 'Launch at startup',
    read: (s) => s.launch_at_login,
    patch: (on) => ({ launch_at_login: on }),
  },
  {
    name: 'Store sign-ins in system keychain',
    read: (s) => s.use_keychain,
    patch: (on) => ({ use_keychain: on }),
  },
  {
    name: 'Copy launch command instead',
    read: (s) => s.copy_command_instead,
    patch: (on) => ({ copy_command_instead: on }),
  },
  {
    name: 'Close popover after launching',
    read: (s) => s.close_popover_after_launch,
    patch: (on) => ({ close_popover_after_launch: on }),
  },
  {
    name: 'Install updates automatically',
    read: (s) => s.auto_update,
    patch: (on) => ({ auto_update: on }),
  },
]

export function GeneralPage(_props: PageComponentProps) {
  const toast = useToast()
  const { data: settings } = useSettings()
  const [draft, setDraft] = useState<GUISettings | null>(null)
  const [repoDraft, setRepoDraft] = useState<string | null>(null)
  const [aaKeyDraft, setAaKeyDraft] = useState('')
  const current = draft ?? settings

  useEffect(() => {
    if (settings && !draft) setDraft(settings)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [settings])

  const persist = useCallback(
    async (next: GUISettings) => {
      setDraft(next)
      try {
        await getHost().settings.set(next)
      } catch (e) {
        toast.show((e as { message?: string }).message ?? 'save failed')
        setDraft(settings ?? null)
      }
    },
    [settings, toast],
  )

  if (!current) {
    return (
      <div className={styles.page}>
        <DetailHeader title={PAGE_META.General[0]} blurb={PAGE_META.General[1]} />
        <p className={styles.loading}>Loading…</p>
      </div>
    )
  }

  const set = (patch: Partial<GUISettings>) => persist({ ...current, ...patch })

  return (
    <div className={styles.page}>
      <DetailHeader title={PAGE_META.General[0]} blurb={PAGE_META.General[1]} />

      <div className={styles.body}>
        <span className={`mono ${styles.kicker}`}>system</span>

        {/* Global shortcut (mockup L595-604): label + note left, seg right. */}
        <div className={styles.row}>
          <span className={styles.labelBlock}>
            <span className={styles.label}>Open the popover</span>
            <span className={styles.note}>Global shortcut, works from any app.</span>
          </span>
          <SegmentedControl
            options={SHORTCUT_OPTS.map((o) => ({ value: o.value, label: o.label }))}
            value={current.shortcut}
            onChange={(v) => set({ shortcut: v as GUISettings['shortcut'] })}
          />
        </div>

        {/* Two-column grid: the system switches followed by the
            update-frequency select. */}
        <div className={styles.grid}>
          {TOGGLES.map((t) => {
            const on = t.read(current)
            return (
              <div className={styles.row} key={t.name}>
                <span className={styles.toggleLabel} data-on={on}>
                  {t.name}
                </span>
                <Toggle on={on} onToggle={(next) => set(t.patch(next))} />
              </div>
            )
          })}

          {/* Dimmed to .38 while auto-update is off (mockup `freqOpacity`,
              L615/L1533) — still interactive, exactly as the mockup. */}
          <div
            className={`${styles.row} ${styles.freqRow}`}
            data-dim={!current.auto_update}
          >
            <span className={styles.labelPlain}>Check for updates</span>
            <select
              className="wmsel"
              aria-label="Check for updates"
              value={current.auto_update_frequency}
              onChange={(e) =>
                set({
                  auto_update_frequency: e.target
                    .value as GUISettings['auto_update_frequency'],
                })
              }
            >
              {FREQ_OPTS.map((o) => (
                <option key={o.value} value={o.value}>
                  {o.label}
                </option>
              ))}
            </select>
          </div>
        </div>

        {/* Which popover tab opens by default (gui.default_tab). */}
        <div className={styles.row}>
          <span className={styles.labelBlock}>
            <span className={styles.label}>Default popover tab</span>
            <span className={styles.note}>Which tab the popover opens on.</span>
          </span>
          <SegmentedControl
            options={[
              // The stored values keep their original names (gui.default_tab);
              // only the popover's tab LABELS were renamed to Quick/Advanced.
              { value: 'profiles', label: 'quick' },
              { value: 'sliders', label: 'advanced' },
            ]}
            value={current.default_tab ?? 'profiles'}
            onChange={(v) => set({ default_tab: v as 'profiles' | 'sliders' })}
          />
        </div>

        <span className={`mono ${styles.kicker} ${styles.kickerNext}`}>catalogue</span>

        <div className={styles.row}>
          <span className={styles.labelBlock}>
            <span className={styles.label}>Data source repo</span>
            <span className={styles.note}>
              Benchmarks are pulled from this GitHub repository. Default is the main which-model
              repo.
            </span>
          </span>
          <Input
            className={styles.catalogInput}
            value={repoDraft ?? current.catalog_repo}
            placeholder="WD-Mitchell/which-model"
            onChange={setRepoDraft}
            onBlur={() => {
              const next = (repoDraft ?? current.catalog_repo).trim() || 'WD-Mitchell/which-model'
              setRepoDraft(null)
              if (next !== current.catalog_repo) set({ catalog_repo: next })
            }}
          />
        </div>

        <div className={styles.row}>
          <span className={styles.labelBlock}>
            <span className={styles.label}>Collect locally</span>
            <span className={styles.note}>
              Optional. Use your own Artificial Analysis API key instead of the repo.
            </span>
          </span>
          <Toggle
            on={current.use_local_aa}
            onToggle={(on) => set({ use_local_aa: on })}
            aria-label="Collect locally"
          />
        </div>

        {current.use_local_aa ? (
          <div className={styles.row}>
            <span className={styles.labelBlock}>
              <span className={styles.label}>Artificial Analysis API key</span>
              <span className={styles.note}>
                {current.aa_api_key_set
                  ? 'A key is saved. Paste a new one to replace it, or “-” to remove it.'
                  : 'Required for local collect. Not stored in config.toml.'}
              </span>
            </span>
            <Input
              className={styles.catalogInput}
              type="password"
              value={aaKeyDraft}
              placeholder={current.aa_api_key_set ? 'saved' : 'ARTIFICIAL_ANALYSIS_API'}
              onChange={setAaKeyDraft}
              onBlur={() => {
                const next = aaKeyDraft.trim()
                if (!next) return
                setAaKeyDraft('')
                void persist({ ...current, aa_api_key: next })
              }}
            />
          </div>
        ) : null}

        <span className={`mono ${styles.kicker} ${styles.kickerNext}`}>results display</span>

        {/* Ranking layout (mockup L627-652): two miniature previews of the
            popover, not a text control. */}
        <div className={`${styles.row} ${styles.rowTight}`}>
          <span className={styles.labelBlock}>
            <span className={styles.label}>Ranking layout</span>
            <span className={styles.note}>How the popover presents the held ranks.</span>
          </span>
          <div className={styles.swatches} role="radiogroup" aria-label="Ranking layout">
            <Swatch
              label="carousel"
              on={current.layout === 'carousel'}
              onPick={() => set({ layout: 'carousel' })}
            >
              <span className={`${styles.box} ${styles.dispBox} ${styles.carouselBox}`}>
                <span className={styles.carArrow} />
                <span className={styles.carStack}>
                  <span className={styles.carBarA} />
                  <span className={styles.carBarB} />
                </span>
                <span className={styles.carArrow} />
              </span>
            </Swatch>
            <Swatch label="list" on={current.layout === 'list'} onPick={() => set({ layout: 'list' })}>
              <span className={`${styles.box} ${styles.dispBox} ${styles.listBox}`}>
                <span className={styles.listBarA} />
                <span className={styles.listBarB} />
                <span className={styles.listBarC} />
              </span>
            </Swatch>
          </div>
        </div>

        {/* Weight control (mockup L654-683): step / bar / slider previews, in
            that order. */}
        <div className={`${styles.row} ${styles.rowTight}`}>
          <span className={styles.labelBlock}>
            <span className={styles.label}>Weight control</span>
            <span className={styles.note}>Used for every profile weight and the scale.</span>
          </span>
          <div className={styles.swatches} role="radiogroup" aria-label="Weight control">
            <Swatch
              label="step"
              on={current.weight_control === 'step'}
              onPick={() => set({ weight_control: 'step' })}
            >
              <span className={`${styles.box} ${styles.wcBox} ${styles.stepBox}`}>
                <span className={`${styles.stepChip} ${styles.stepChipOn}`} />
                <span className={`${styles.stepChip} ${styles.stepChipOn}`} />
                <span className={styles.stepChip} />
                <span className={styles.stepChip} />
              </span>
            </Swatch>
            <Swatch
              label="bar"
              on={current.weight_control === 'bar'}
              onPick={() => set({ weight_control: 'bar' })}
            >
              <span className={`${styles.box} ${styles.wcBox} ${styles.barBox}`}>
                <span className={styles.barTrack}>
                  <span className={styles.barFill} />
                </span>
              </span>
            </Swatch>
            <Swatch
              label="slider"
              on={current.weight_control === 'slider'}
              onPick={() => set({ weight_control: 'slider' })}
            >
              <span className={`${styles.box} ${styles.wcBox} ${styles.sliderBox}`}>
                <span className={styles.sliderTrack} />
                <span className={styles.sliderKnob} />
              </span>
            </Swatch>
          </div>
        </div>

        {/* Ranks held per pick (mockup L685-693). */}
        <div className={`${styles.row} ${styles.rowTight}`}>
          <span className={styles.labelPlain}>Ranks held per pick</span>
          <SegmentedControl
            options={HOLD_OPTS.map((v) => ({ value: String(v), label: String(v) }))}
            value={String(current.holds)}
            onChange={(v) => set({ holds: Number(v) })}
          />
        </div>
      </div>
    </div>
  )
}

/** One preview tile + its mono caption (mockup `swatch()`, L1045-1049): the
 *  selected tile gets the 1.5px accent ring, full opacity and an accent-200
 *  caption; the rest sit at .55 behind a 1px hairline. Rendered as a real
 *  radio so the group stays keyboard-reachable — the mockup's plain <span>
 *  would not be. */
function Swatch({
  label,
  on,
  onPick,
  children,
}: {
  label: string
  on: boolean
  onPick(): void
  children: React.ReactNode
}) {
  return (
    <button
      type="button"
      role="radio"
      aria-checked={on}
      className={styles.swatch}
      data-on={on}
      onClick={onPick}
    >
      {children}
      <span className={`mono ${styles.swatchLabel}`}>{label}</span>
    </button>
  )
}
export default GeneralPage
