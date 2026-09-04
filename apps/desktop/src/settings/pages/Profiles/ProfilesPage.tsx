// U08 — Profiles settings page. Two views behind one registry entry:
//   • the list  — mockup demo.dc.html L278-324: slug, a core/task weights
//     sparkline, pick count, last-used, and the Duplicate / delete / chevron
//     trailing cell.
//   • the detail — mockup demo.dc.html L325-399: read-only tag + summary +
//     verbs, the core and task benchmark editors with their share-of-score
//     labels, and the core/task balance rail.
// Both supply their own 22px gutter: <main> has none (U07 layout contract).
import { useCallback, useEffect, useRef, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { createAutosave, hasActiveAutosave, whenAutosaveIdle } from '../../../lib/autosave'
import {
  BalanceSlider,
  Button,
  sparkbarHeight,
  Tag,
  Tooltip,
  useToast,
  WeightRow,
} from '@which-model/ui'
import type { ProfileDetail, ProfileSummary } from '@which-model/core'
import { useProfile, useProfiles, useSettings } from '../../../lib/queries'
import { getHost } from '../../../lib/host'
import { DetailHeader } from '../../DetailHeader'
import { PAGE_META } from '../../pages'
import type { PageComponentProps } from '../../pages'
import styles from './ProfilesPage.module.css'

/** Core benchmarks are a fixed triple (engine tier 1); everything else is a
 *  benchmark group carried in tier 2. */
const CORE_KEYS = ['intelligence', 'cost', 'speed'] as const

// Mockup icon set (demo.dc.html L317-318). Never a literal "×" or "›".
function TrashIcon({ size }: { size: 12 | 13 }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 14 14"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.25"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="M2.4 4h9.2M5.6 4V2.7h2.8V4M3.7 4l.45 7.3h5.7L10.3 4M5.9 6.2v3.2M8.1 6.2v3.2" />
    </svg>
  )
}

function ChevronIcon() {
  return (
    <svg
      width="10"
      height="10"
      viewBox="0 0 12 12"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.6"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="M4.4 2.2 8.2 6l-3.8 3.8" />
    </svg>
  )
}

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

function weightedKeys(p: {
  tier1_weights: Record<string, number>
  tier2_weights: Record<string, number>
}): number {
  const all = { ...p.tier1_weights, ...p.tier2_weights }
  return Object.values(all).filter((v) => v > 0).length
}

/** Only weighted benchmarks get a bar — an ignored one draws nothing at all,
 *  so the sparkline reads as "what this profile cares about" (mockup binding
 *  `profileRows`, demo.dc.html L1476-1493). */
function sparkEntries(p: ProfileSummary): {
  core: { key: string; value: number }[]
  task: { key: string; value: number }[]
} {
  const core = CORE_KEYS.filter((k) => (p.tier1_weights[k] ?? 0) > 0).map((k) => ({
    key: k as string,
    value: p.tier1_weights[k],
  }))
  const task = Object.keys(p.tier2_weights)
    .filter((k) => p.tier2_weights[k] > 0)
    .map((k) => ({ key: k, value: p.tier2_weights[k] }))
  return { core, task }
}

/** One 5px bar in the list sparkline. Core bars sit two accent steps brighter
 *  than task bars so the split reads without the hairline. */
function SparkBar({ metric, tier }: { metric: { key: string; value: number }; tier: 'core' | 'task' }) {
  return (
    <Tooltip content={`${metric.key}  ${metric.value} / 5`}>
      <span className={styles.barCol}>
        <span
          className={styles.bar}
          style={{
            height: `${sparkbarHeight(metric.value)}px`,
            background: tier === 'core' ? 'var(--color-accent-400)' : 'var(--color-accent-700)',
          }}
        />
      </span>
    </Tooltip>
  )
}

export function ProfilesPage({ detail, openDetail, closeDetail }: PageComponentProps) {
  const toast = useToast()
  const { data: profiles } = useProfiles()

  const profilesList = profiles ?? []
  const creatingRef = useRef(false)
  const [creating, setCreating] = useState(false)

  const handleNew = useCallback(async () => {
    if (creatingRef.current) return
    creatingRef.current = true
    setCreating(true)
    try {
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
        await getHost().profiles.create(payload)
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
    } finally { creatingRef.current = false; setCreating(false) }
  }, [profilesList.length, toast, openDetail])

  const detailPage =
    detail?.kind === 'profile' ? (
      <ProfileDetailView key={detail.id} slug={detail.id} onBack={closeDetail} openDetail={openDetail} />
    ) : null

  if (detailPage) {
    return <>{detailPage}</>
  }

  return (
    <div className={styles.page}>
      <DetailHeader
        title={PAGE_META.Profiles[0]}
        blurb={PAGE_META.Profiles[1]}
        action={{ label: PAGE_META.Profiles[2] as string, onAction: () => void handleNew(), disabled: creating }}
      />
      <span className={'mono ' + styles.kicker}>profiles</span>
      <div className={styles.colHeader}>
        <span className={'mono ' + styles.colName}>name</span>
        <span className={'mono ' + styles.colWeights}>weights</span>
        <span className={'mono ' + styles.colPicks}>picks</span>
        <span className={'mono ' + styles.colUsed}>used</span>
        <span className={styles.colActions} aria-hidden="true" />
      </div>
      {profilesList.length === 0 ? (
        <div className={styles.placeholder}>{'No profiles.'}</div>
      ) : (
        profilesList.map((p) => {
          const spark = sparkEntries(p)
          return (
            <div
              key={p.slug}
              className={'row ' + styles.row}
              onClick={() => openDetail({ kind: 'profile', id: p.slug })}
            >
              {/* Display name, with the slug as the tooltip: the slug is the
                  id (and what the toasts below echo), but every other surface
                  names a profile the way the service spells it. */}
              <span className={'mono ' + styles.slug} title={p.slug}>
                {p.name}
              </span>
              <span className={styles.spark}>
                <span className={styles.sparkGroup}>
                  {spark.core.map((m) => (
                    <SparkBar key={m.key} metric={m} tier="core" />
                  ))}
                </span>
                {/* The hairline stands whether or not either side has bars —
                    it is the axis, not a separator between two lists. */}
                <span className={styles.sparkRule} aria-hidden="true" />
                <span className={styles.sparkGroup}>
                  {spark.task.map((m) => (
                    <SparkBar key={m.key} metric={m} tier="task" />
                  ))}
                </span>
              </span>
              <span className={'mono ' + styles.picks}>{p.picks.toLocaleString()}</span>
              <span className={'mono ' + styles.used}>{relativeTime(p.last_used)}</span>
              <span className={styles.actions} onClick={(e) => e.stopPropagation()}>
                {/* 11px / 2px 6px — the list's ghost scale, 1px tighter than
                    Button's `xs` (see Button.module.css). Inline, as in the
                    mockup, so it beats `.btn-ghost { padding-inline }`. */}
                <button
                  type="button"
                  className="btn btn-ghost"
                  style={{ fontSize: '11px', padding: '2px 6px' }}
                  onClick={() =>
                    void getHost()
                      .profiles.duplicate(p.slug)
                      .then(() => toast.show(`duplicated ${p.slug}`))
                      .catch((e) => toast.show((e as { message?: string }).message ?? 'duplicate failed'))
                  }
                >
                  Duplicate
                </button>
                <button
                  type="button"
                  className={'ib' + (p.builtin ? ' off' : '') + ' ' + styles.iconBtn}
                  title={p.builtin ? 'Built-in profile — cannot be deleted' : `Delete ${p.slug}`}
                  aria-label={p.builtin ? 'Built-in profile — cannot be deleted' : `Delete ${p.slug}`}
                  disabled={p.builtin}
                  onClick={() =>
                    void getHost()
                      .profiles.delete(p.slug)
                      .then(() => toast.show(`deleted ${p.slug}`))
                      .catch((e) => toast.show((e as { message?: string }).message ?? 'delete failed'))
                  }
                >
                  <TrashIcon size={12} />
                </button>
                <span className={styles.chevron} aria-hidden="true">
                  <ChevronIcon />
                </span>
              </span>
            </div>
          )
        })
      )}
      <p className={styles.note}>
        {'Picks count every launch made with the profile — from the popover and from the '}
        <span className="mono">wm</span>
        {' CLI alike.'}
      </p>
    </div>
  )
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
  const qc = useQueryClient()
  const persistenceKey = 'profile:' + slug
  const [opening] = useState(() => hasActiveAutosave(persistenceKey))
  const busyRef = useRef(opening)
  const [busy, setBusy] = useState(opening)
  const [queue] = useState(() => createAutosave<ProfileDetail>(
    async (next) => {
      await getHost().profiles.save(next)
      await qc.invalidateQueries({ queryKey: ['profile', slug] })
    },
    {
      key: persistenceKey,
      delay: 300,
      onSuccess: (_next, generation) => { if (queue.isCurrent(generation)) setLocal(null) },
      onError: async (error, generation) => {
        toast.show((error as { message?: string }).message ?? 'save failed')
        await qc.invalidateQueries({ queryKey: ['profile', slug] })
        if (queue.isCurrent(generation)) setLocal(null)
      },
    },
  ))
  const sliderStyle = settings?.weight_control ?? 'slider'
  const current = local ?? profile

  const scheduleSave = useCallback((next: ProfileDetail) => {
    if (busyRef.current || next.builtin) return
    setLocal(next)
    queue.schedule(next)
  }, [queue])

  useEffect(() => () => { void queue.flush().catch(() => {}) }, [queue])

  // Returning to an editor waits for a previous mount's final write and reads
  // that saved snapshot before accepting another full-profile/list edit.
  useEffect(() => {
    if (!opening) return
    let mounted = true
    void whenAutosaveIdle(persistenceKey)
      .then(() => qc.invalidateQueries({ queryKey: ['profile', slug] }))
      .finally(() => { if (mounted) { busyRef.current = false; setBusy(false) } })
    return () => { mounted = false }
  }, [opening, persistenceKey, qc, slug])


  const remove = async () => {
    if (busyRef.current) return
    busyRef.current = true
    setBusy(true)
    try {
      await queue.flush()
      await getHost().profiles.delete(slug)
      queue.cancelPending()
      toast.show(`deleted ${slug}`)
      onBack()
    } catch (error) {
      toast.show((error as { message?: string }).message ?? 'delete failed')
    } finally { busyRef.current = false; setBusy(false) }
  }

  if (!current) {
    return <div className={styles.placeholder}>{'Loading…'}</div>
  }

  const readOnly = current.builtin
  const weighted = weightedKeys(current)
  const total = 3 + Object.keys(current.tier2_weights).length

  const handleWeight = (key: string, v: number) => {
    const t1 = current.tier1_weights
    const t2 = current.tier2_weights
    const isTier1 = (CORE_KEYS as readonly string[]).includes(key)
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

  // Mockup `pfRow` (demo.dc.html L1128-1146) colours a row by its value only —
  // no benchmark is singled out with the accent here.
  const coreRows = CORE_KEYS.map((k) => ({ key: k as string, value: current.tier1_weights[k] ?? 0 }))
  const taskRows = Object.keys(current.tier2_weights).sort()
    .map((k) => ({ key: k, value: current.tier2_weights[k] ?? 0 }))

  const duplicate = async () => {
    if (busyRef.current) return
    busyRef.current = true
    setBusy(true)
    try {
      await queue.flush()
      const copy = await getHost().profiles.duplicate(slug)
      toast.show(`editing ${copy.slug}`)
      openDetail({ kind: 'profile', id: copy.slug })
    } catch (error) { toast.show((error as { message?: string }).message ?? 'duplicate failed') }
    finally { busyRef.current = false; setBusy(false) }
  }

  return (
    <div className={styles.page}>
      {/* No page action here: PAGE_META's "New profile" belongs to the list.
          The detail's verbs live in the strip below (mockup L326-333). */}
      <DetailHeader
        title={current.name}
        blurb={
          readOnly
            ? 'A built-in profile — its weights are read-only. Duplicate it to make a version you can change.'
            : 'Drag a weight to change how much this profile cares about each benchmark. Zero means the benchmark is ignored.'
        }
        backLabel="Profiles"
        onBack={onBack}
      />
      <div className={styles.detailStrip}>
        {readOnly ? <Tag variant="neutral">{'built-in · read-only'}</Tag> : null}
        <span className={'mono ' + styles.summary}>
          {`${weighted} of ${total} benchmarks weighted · ${current.picks.toLocaleString()} picks`}
        </span>
        <span className={styles.detailActions}>
          <Button variant="ghost" size="xs" disabled={busy} onClick={duplicate}>
            {readOnly ? 'Duplicate & edit' : 'Duplicate'}
          </Button>
          <button
            type="button"
            className={'ib' + (readOnly ? ' off' : '') + ' ' + styles.iconBtnLg}
            title={readOnly ? 'Built-in profile — cannot be deleted' : 'Delete this profile'}
            aria-label={readOnly ? 'Built-in profile — cannot be deleted' : 'Delete this profile'}
            disabled={readOnly || busy}
            onClick={() => void remove()}
          >
            <TrashIcon size={13} />
          </button>
        </span>
      </div>

      <div className={styles.sectionHead}>
        <span className={styles.sectionLabel}>core benchmarks</span>
        <span className={'mono ' + styles.sectionPct}>{`${current.core_share}% of the score`}</span>
      </div>
      <div className={styles.rows}>
        {coreRows.map((r) => (
          <WeightRow
            key={r.key}
            variant={sliderStyle}
            label={r.key}
            value={r.value}
            labelWidth={150}
            valueStyle="verbose"
            readOnly={readOnly || busy}
            // Core axes are required keys (engine rule 3), so they cannot be
            // dropped to "ignored" the way a task benchmark below can.
            min={1}
            onChange={readOnly || busy ? undefined : (v) => handleWeight(r.key, v)}
          />
        ))}
      </div>

      <div className={styles.sectionHead + ' ' + styles.sectionHeadTask}>
        <span className={styles.sectionLabel}>task benchmarks</span>
        <span className={'mono ' + styles.sectionPct}>
          {`${100 - current.core_share}% of the score`}
        </span>
      </div>
      {taskRows.length === 0 ? (
        <div className={'mono ' + styles.rowsEmpty}>
          {'no task benchmarks weighted — only the core benchmarks affect this profile'}
        </div>
      ) : (
        <div className={styles.rows}>
          {taskRows.map((r) => (
            <WeightRow
              key={r.key}
              variant={sliderStyle}
              label={r.key}
              value={r.value}
              labelWidth={150}
              valueStyle="verbose"
              readOnly={readOnly || busy}
              onChange={readOnly || busy ? undefined : (v) => handleWeight(r.key, v)}
            />
          ))}
        </div>
      )}

      {/* Balance rail closes the page — it weighs the two sections above it,
          so it can only be read after them (mockup L390-396). */}
      <div className={styles.balance}>
        <BalanceSlider core={current.core_share} readOnly={readOnly || busy} showRatio onChange={handleBalance} />
      </div>
    </div>
  )
}
export default ProfilesPage
