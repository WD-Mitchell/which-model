// U13 — Usage detection settings page: detection mode, backend, per-provider
// snapshot meters, refresh.
import { useEffect, useState } from 'react'
import { Button, SegmentedControl, Tag, UsageMeter, useToast } from '@which-model/ui'
import type { UsageDTO } from '@which-model/core'
import { useUsage } from '../../../lib/queries'
import { getHost } from '../../../lib/host'
import { DetailHeader } from '../../DetailHeader'
import { PAGE_META } from '../../pages'
import type { PageComponentProps } from '../../pages'
import styles from './UsagePage.module.css'

export function UsagePage(_props: PageComponentProps) {
  const toast = useToast()
  const { data: usage } = useUsage(false)
  const [mode, setMode] = useState<{ mode: string; backend: string } | null>(null)
  const [force, setForce] = useState(false)

  useEffect(() => {
    void getHost().usage.mode().then(setMode).catch(() => {})
  }, [])

  const list: UsageDTO[] = usage ?? []

  const setBackend = async (backend: 'off' | 'native' | 'codexbar') => {
    try {
      await getHost().usage.setBackend(backend)
      setMode((m) => (m ? { ...m, backend } : m))
    } catch (e) {
      toast.show((e as { message?: string }).message ?? 'update failed')
    }
  }

  const setModeValue = async (m: 'auto' | 'on' | 'off') => {
    try {
      await getHost().usage.setMode(m)
      setMode((prev) => (prev ? { ...prev, mode: m } : prev))
    } catch (e) {
      toast.show((e as { message?: string }).message ?? 'update failed')
    }
  }

  return (
    <div className={styles.page}>
      <DetailHeader
        title={PAGE_META['Usage detection'][0]}
        blurb={PAGE_META['Usage detection'][1]}
        action={{ label: 'Refetch now', onAction: () => { setForce(true); toast.show('refreshing') } }}
      />

      <SectionHeader>detection mode</SectionHeader>
      <div className={styles.controls}>
        <SegmentedControl
          options={[
            { value: 'auto', label: 'auto' },
            { value: 'on', label: 'on' },
            { value: 'off', label: 'off' },
          ]}
          value={mode?.mode ?? 'auto'}
          onChange={(v) => void setModeValue(v as 'auto' | 'on' | 'off')}
        />
      </div>

      <SectionHeader>backend</SectionHeader>
      <div className={styles.controls}>
        <SegmentedControl
          options={[
            { value: 'off', label: 'off' },
            { value: 'native', label: 'native' },
            { value: 'codexbar', label: 'codexbar' },
          ]}
          value={mode?.backend ?? 'native'}
          onChange={(v) => void setBackend(v as 'off' | 'native' | 'codexbar')}
        />
      </div>

      <SectionHeader>usage snapshots</SectionHeader>
      {list.length === 0 ? (
        <div className={styles.empty}>{'no usage data — enable detection or press Refetch now'}</div>
      ) : (
        list.map((u) => (
          <div key={u.provider} className={styles.provider}>
            <div className={styles.providerHead}>
              <span className="mono">{u.provider}</span>
              <Tag variant={u.confidence === 'live' ? 'accent' : 'outline'}>{u.confidence}</Tag>
              {u.stale ? <Tag variant="neutral">stale</Tag> : null}
            </div>
            {u.windows.map((w) => (
              <div key={w.id} className={styles.window}>
                <UsageMeter
                  label={w.label}
                  percent={w.used_percent}
                  hot={w.used_percent !== null && w.used_percent >= 70}
                />
                <span className={styles.reset}>{w.reset_hint}</span>
              </div>
            ))}
            <div className={styles.meta}>
              <span className="mono">{u.credits}</span>
              <span className="mono">{u.resets}</span>
            </div>
          </div>
        ))
      )}
      {force ? null : null}
    </div>
  )
}

function SectionHeader({ children }: { children: string }) {
  return <div className={styles.sectionHeader}>{children}</div>
}
export default UsagePage
