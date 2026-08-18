// U12 — General settings page: app + results display + shortcut sections.
import { useCallback, useEffect, useState } from 'react'
import { SegmentedControl, Toggle, useToast } from '@which-model/ui'
import type { GUISettings } from '@which-model/core'
import { useSettings } from '../../../lib/queries'
import { getHost } from '../../../lib/host'
import { DetailHeader } from '../../DetailHeader'
import { PAGE_META } from '../../pages'
import type { PageComponentProps } from '../../pages'
import styles from './GeneralPage.module.css'

export function GeneralPage(_props: PageComponentProps) {
  const toast = useToast()
  const { data: settings } = useSettings()
  const [draft, setDraft] = useState<GUISettings | null>(null)
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
    return <div className={styles.page}><DetailHeader title={PAGE_META.General[0]} blurb={PAGE_META.General[1]} />Loading…</div>
  }

  const set = (patch: Partial<GUISettings>) => persist({ ...current, ...patch })

  return (
    <div className={styles.page}>
      <DetailHeader title={PAGE_META.General[0]} blurb={PAGE_META.General[1]} />

      <SectionHeader>app</SectionHeader>
      <Row label="Launch at login" hint={undefined}>
        <Toggle on={current.launch_at_login} onToggle={(on) => set({ launch_at_login: on })} />
      </Row>
      <Row label="Show menu bar icon" hint={undefined}>
        <Toggle on={current.show_menu_bar_icon} onToggle={(on) => set({ show_menu_bar_icon: on })} />
      </Row>
      <Row label="Close popover after launch" hint={undefined}>
        <Toggle on={current.close_popover_after_launch} onToggle={(on) => set({ close_popover_after_launch: on })} />
      </Row>
      <Row label="Copy command instead of launching" hint={undefined}>
        <Toggle on={current.copy_command_instead} onToggle={(on) => set({ copy_command_instead: on })} />
      </Row>
      <Row label="Auto-update" hint={undefined}>
        <Toggle on={current.auto_update} onToggle={(on) => set({ auto_update: on })} />
      </Row>
      <Row label="Check for updates" hint="frequency">
        <SegmentedControl
          options={[
            { value: 'hourly', label: 'hourly' },
            { value: 'daily', label: 'daily' },
            { value: 'weekly', label: 'weekly' },
            { value: 'monthly', label: 'monthly' },
          ]}
          value={current.auto_update_frequency}
          onChange={(v) => set({ auto_update_frequency: v as GUISettings['auto_update_frequency'] })}
        />
      </Row>

      <SectionHeader>results display</SectionHeader>
      <Row label="Layout" hint={undefined}>
        <SegmentedControl
          options={[
            { value: 'carousel', label: 'carousel' },
            { value: 'list', label: 'list' },
          ]}
          value={current.layout}
          onChange={(v) => set({ layout: v as GUISettings['layout'] })}
        />
      </Row>
      <Row label="Weight control" hint={undefined}>
        <SegmentedControl
          options={[
            { value: 'slider', label: 'slider' },
            { value: 'bar', label: 'bar' },
            { value: 'step', label: 'step' },
          ]}
          value={current.weight_control}
          onChange={(v) => set({ weight_control: v as GUISettings['weight_control'] })}
        />
      </Row>
      <Row label="Ranks shown" hint={undefined}>
        <SegmentedControl
          options={[
            { value: '3', label: '3' },
            { value: '5', label: '5' },
            { value: '10', label: '10' },
          ]}
          value={String(current.holds)}
          onChange={(v) => set({ holds: Number(v) })}
        />
      </Row>

      <SectionHeader>shortcut</SectionHeader>
      <Row label="Toggle popover" hint={undefined}>
        <SegmentedControl
          options={[
            { value: 'alt+space', label: 'alt+space' },
            { value: 'ctrl+space', label: 'ctrl+space' },
            { value: 'cmd+shift+m', label: 'cmd+shift+m' },
          ]}
          value={current.shortcut}
          onChange={(v) => set({ shortcut: v as GUISettings['shortcut'] })}
        />
      </Row>

      <div className={styles.configPath}>
        <span className="mono">{current.config_path}</span>
      </div>
    </div>
  )
}

function SectionHeader({ children }: { children: string }) {
  return <div className={styles.sectionHeader}>{children}</div>
}

function Row({ label, hint, children }: { label: string; hint?: string; children: React.ReactNode }) {
  return (
    <div className={styles.row}>
      <span className={styles.label}>
        {label}
        {hint ? <span className={styles.hint}>{hint}</span> : null}
      </span>
      <span className={styles.control}>{children}</span>
    </div>
  )
}
export default GeneralPage
