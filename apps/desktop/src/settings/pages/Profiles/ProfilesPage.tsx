// U08 — Profiles settings page: list of all profiles with weight sparkbars,
// pick counts, CRUD actions, and a detail view for editing weights (custom)
// or read-only viewing (builtin).
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  BalanceSlider,
  Button,
  Sparkbar,
  Tag,
  useToast,
  WeightEditor,
} from '@which-model/ui'
import type { ProfileDetail, ProfileSummary } from '@which-model/core'
import { useProfile, useProfiles, useSettings } from '../../../lib/queries'
import { getHost } from '../../../lib/host'
import { DetailHeader } from '../../DetailHeader'
import { PAGE_META } from '../../pages'
import type { PageComponentProps } from '../../pages'
import styles from './ProfilesPage.module.css'

function relativeTime(iso: string): string {
  if (!iso) return '—'
  const t = new Date(iso).getTime()
  if (Number.isNaN(t)) return '—'
  const mins = Math.round((Date.now() - t) / 60000)
  if (mins < 1) return 'just now'
  if (mins < 60) return `${mins}m ago`
  const hrs = Math.round(mins / 60)
  if (hrs < 24) return `${hrs}h ago`
  const days = Math.round(hrs / 24)
  if (days < 30) return `${days}d ago`
  const months = Math.round(days / 30)
  return `${months}mo ago`
}

function weightedKeys(p: { tier1_weights: Record<string, number>; tier2_weights: Record<string, number> }): number {
  const all = { ...p.tier1_weights, ...p.tier2_weights }
  return Object.values(all).filter((v) => v > 0).length
}

export function ProfilesPage({ detail, openDetail, closeDetail }: PageComponentProps) {
  const toast = useToast()
  const { data: profiles } = useProfiles()

  const profilesList = profiles ?? []

  const handleNew = useCallback(async () => {
    let n = profilesList.length + 1
    // eslint-disable-next-line no-constant-condition
    while (true) {
      const slug = `profile_${n}`
      const payload: ProfileDetail = {
        slug,
        name: `profile ${n}`,
        builtin: false,
        core_share: 60,
        tier1_weights: { intelligence: 3, cost: 3, speed: 3 },
        tier2_weights: {},
        picks: 0,
        last_used: '',
      }
      try {
        await getHost().profiles.save(payload)
        toast.show('new profile created')
        openDetail({ kind: 'profile', id: slug })
        return
      } catch (e) {
        if ((e as { code?: string }).code === 'conflict') {
          n += 1
          continue
        }
        toast.show((e as { message?: string }).message ?? 'create failed')
        return
      }
    }
  }, [profilesList.length, toast, openDetail])

  const detailPage =
    detail?.kind === 'profile' ? (
      <ProfileDetailView slug={detail.id} onBack={closeDetail} openDetail={openDetail} />
    ) : null

  if (detailPage) {
    return <>{detailPage}</>
  }

  return (
    <div className={styles.page}>
      <DetailHeader
        title={PAGE_META.Profiles[0]}
        blurb={PAGE_META.Profiles[1]}
        action={{ label: PAGE_META.Profiles[2] as string, onAction: () => void handleNew() }}
      />
      <div className={styles.listHeader}>
        <span className={styles.kicker}>profiles</span>
      </div>
      <div className={styles.colHeader}>
        <span className={styles.colName}>name</span>
        <span className={styles.colWeights}>weights</span>
        <span className={styles.colNum}>picks</span>
        <span className={styles.colNum}>used</span>
        <span className={styles.colActions} />
      </div>
      {profilesList.length === 0 ? (
        <div className={styles.empty}>{'No profiles.'}</div>
      ) : (
        profilesList.map((p) => (
          <div key={p.slug} className={styles.row} onClick={() => openDetail({ kind: 'profile', id: p.slug })}>
            <span className={styles.name} title={p.name}>
              <span className="mono">{p.slug}</span>
            </span>
            <span className={styles.weights}>
              <Sparkbar
                metrics={weightedMetrics(p)}
              />
            </span>
            <span className={styles.num}>{p.picks.toLocaleString()}</span>
            <span className={styles.num}>{relativeTime(p.last_used)}</span>
            <span className={styles.actions} onClick={(e) => e.stopPropagation()}>
              <Button
                variant="ghost"
                size="sm"
                onClick={() =>
                  void getHost()
                    .profiles.duplicate(p.slug)
                    .then(() => toast.show(`duplicated ${p.slug}`))
                    .catch((e) => toast.show((e as { message?: string }).message ?? 'duplicate failed'))
                }
              >
                Duplicate
              </Button>
              <button
                type="button"
                className={'ib ' + (p.builtin ? ' off' : '')}
                title={p.builtin ? 'Built-in profile — cannot be deleted' : `Delete ${p.slug}`}
                disabled={p.builtin}
                onClick={() =>
                  void getHost()
                    .profiles.delete(p.slug)
                    .then(() => toast.show(`deleted ${p.slug}`))
                    .catch((e) => toast.show((e as { message?: string }).message ?? 'delete failed'))
                }
              >
                ×
              </button>
              <span className="ib">›</span>
            </span>
          </div>
        ))
      )}
      <div className={styles.footnote}>
        {'Picks count every launch made with the profile — from the popover and from the `wm` CLI alike.'}
      </div>
    </div>
  )
}

function weightedMetrics(p: ProfileSummary): { key: string; value: number }[] {
  const order = ['intelligence', 'cost', 'speed']
  const tier1 = order
    .filter((k) => (p.tier1_weights[k] ?? 0) > 0)
    .map((k) => ({ key: k, value: p.tier1_weights[k] }))
  const tier2 = Object.keys(p.tier2_weights)
    .filter((k) => p.tier2_weights[k] > 0)
    .map((k) => ({ key: k, value: p.tier2_weights[k] }))
  return [...tier1, ...tier2]
}

function ProfileDetailView({
  slug,
  onBack,
  openDetail,
}: {
  slug: string
  onBack(): void
  openDetail(d: { kind: 'profile'; id: string }): void
}) {
  const toast = useToast()
  const { data: profile } = useProfile(slug)
  const { data: settings } = useSettings()
  const [local, setLocal] = useState<ProfileDetail | null>(null)
  const saveTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  const sliderStyle = settings?.weight_control ?? 'slider'
  const current = local ?? profile

  // Reset local draft when the loaded profile changes.
  useEffect(() => {
    if (profile) setLocal(null)
  }, [profile])

  const scheduleSave = useCallback(
    (next: ProfileDetail) => {
      setLocal(next)
      if (saveTimer.current) clearTimeout(saveTimer.current)
      saveTimer.current = setTimeout(() => {
        void getHost()
          .profiles.save(next)
          .catch((e) => {
            toast.show((e as { message?: string }).message ?? 'save failed')
            setLocal(null) // re-sync with persisted truth
          })
      }, 300)
    },
    [toast],
  )

  useEffect(() => {
    return () => {
      if (saveTimer.current) clearTimeout(saveTimer.current)
    }
  }, [])

  if (!current) {
    return <div className={styles.page}>{'Loading…'}</div>
  }

  const readOnly = current.builtin
  const weighted = weightedKeys(current)
  const total = 3 + Object.keys(current.tier2_weights).length

  const handleWeight = (key: string, v: number) => {
    const t1 = current.tier1_weights
    const t2 = current.tier2_weights
    const isTier1 = ['intelligence', 'cost', 'speed'].includes(key)
    const w = { ...(isTier1 ? t1 : t2) }
    if (v <= 0) delete w[key]
    else w[key] = v
    scheduleSave({
      ...current,
      tier1_weights: isTier1 ? w : t1,
      tier2_weights: isTier1 ? t2 : w,
    })
  }

  const handleBalance = (v: number) => scheduleSave({ ...current, core_share: v })

  const coreRows = ['intelligence', 'cost', 'speed'].map((k) => ({
    key: k,
    value: current.tier1_weights[k] ?? 0,
    accent: k === 'cost',
  }))
  const taskRows = Object.keys(current.tier2_weights)
    .sort()
    .map((k) => ({ key: k, value: current.tier2_weights[k] ?? 0 }))

  return (
    <div className={styles.page}>
      <DetailHeader
        title={current.slug}
        blurb={
          readOnly
            ? 'A built-in profile — its weights are read-only. Duplicate it to make a version you can change.'
            : 'Drag a weight to change how much this profile cares about each benchmark. Zero means the benchmark is ignored.'
        }
        backLabel="Profiles"
        onBack={onBack}
        action={
          readOnly
            ? {
                label: 'Duplicate & edit',
                onAction: () =>
                  void getHost()
                    .profiles.duplicate(slug)
                    .then((d) => {
                      toast.show(`editing ${d.slug}`)
                      openDetail({ kind: 'profile', id: d.slug })
                    })
                    .catch(() => {}),
              }
            : {
                label: 'Duplicate',
                onAction: () =>
                  void getHost()
                    .profiles.duplicate(slug)
                    .then((d) => {
                      toast.show(`editing ${d.slug}`)
                      openDetail({ kind: 'profile', id: d.slug })
                    })
                    .catch(() => {}),
              }
        }
      />
      <div className={styles.strip}>
        {readOnly ? <Tag variant="neutral">{'built-in · read-only'}</Tag> : null}
        <span className={styles.summary}>
          <span className="mono">{`${weighted} of ${total} benchmarks weighted`}</span>
          <span> · </span>
          <span className="mono">{`${current.picks.toLocaleString()} picks`}</span>
        </span>
      </div>
      <div style={{ maxWidth: '434px' }}>
        <label className={styles.balRow}>
          <span className="mono">core</span>
          <span className={styles.ratio}>{`${current.core_share} / ${100 - current.core_share}`}</span>
          <span className="mono">task</span>
        </label>
        <BalanceSlider core={current.core_share} readOnly={readOnly} showRatio onChange={handleBalance} />
      </div>
      <div className={styles.editor}>
        <WeightEditor
          variant="profile-detail"
          sliderStyle={sliderStyle}
          coreRows={coreRows}
          taskRows={taskRows}
          sectionPcts={{ core: `${current.core_share}% of the score`, task: `${100 - current.core_share}% of the score` }}
          readOnly={readOnly}
          onChangeWeight={readOnly ? undefined : handleWeight}
        />
      </div>
    </div>
  )
}
export default ProfilesPage
