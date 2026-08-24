// U09 — Benchmark groups settings page: the group list, a group detail
// (membership switches, rename, duplicate/delete) and a benchmark detail
// (model × reasoning results). Markup ported from the mockup
// (specs/desktop/mockup/demo.dc.html L400-491 and its `groupRows` / `grRows` /
// `benchRows` bindings around L1327-1470).
//
// Layout contract (U07): <main> carries NO horizontal padding, so every block
// here supplies its own 22px inline gutter — that is what lets the group-list
// row rules and their `.row` hover tint bleed edge to edge.
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Button, CoverageBar, Input, Tag, Toggle, cx, useToast } from '@which-model/ui'
import type { BenchRow } from '@which-model/core'
import { useBenchmarkDetail, useBenchmarks, useGroupDetail, useGroups } from '../../../lib/queries'
import { getHost } from '../../../lib/host'
import { DetailHeader } from '../../DetailHeader'
import { PAGE_META } from '../../pages'
import type { Detail, PageComponentProps } from '../../pages'
import styles from './GroupsPage.module.css'

function sanitizeSlug(s: string): string {
  return s.trim().toLowerCase().replace(/[^a-z0-9]+/g, '_').replace(/^_+/, '')
}

function errText(e: unknown, fallback: string): string {
  return (e as { message?: string }).message ?? fallback
}

// Literal copy from the mockup — kept verbatim (curly apostrophes and em
// dashes included) so the rendered page reads exactly like the design.
const LIST_NOTE =
  'Every group here is weightable in a profile. A model’s score for a group is the mean of its results on that group’s benchmarks, so changing the list changes the ranking.'
const GROUP_BLURB_BUILTIN =
  'A built-in group — its benchmark list is read-only. Duplicate it to make a version you can change.'
const GROUP_BLURB_CUSTOM =
  'Add or remove benchmarks. A model’s score for this group is the mean of its results on the benchmarks listed here.'
const GROUP_READONLY_NOTE =
  'A model’s score for this group is the mean of its results on the benchmarks switched on here — counted over every model and reasoning level that reports the benchmark. Duplicate the group to change what it measures.'
const BENCH_BLURB_FALLBACK =
  'Carried in the model data export. No description recorded for this benchmark yet.'
const BENCH_NO_GROUPS = 'not in any group — it does not affect any profile'

/** Trash glyph (mockup L455, L463) — 12px in the list, 13px in the detail. */
function TrashIcon({ size }: { size: number }) {
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
    >
      <path d="M2.4 4h9.2M5.6 4V2.7h2.8V4M3.7 4l.45 7.3h5.7L10.3 4M5.9 6.2v3.2M8.1 6.2v3.2" />
    </svg>
  )
}

/** Row-trailing chevron (mockup L455). Deliberately NOT an `.ib` box — it is
 *  a static affordance, not a control. */
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
    >
      <path d="M4.4 2.2 8.2 6l-3.8 3.8" />
    </svg>
  )
}

export function GroupsPage({ detail, openDetail, closeDetail }: PageComponentProps) {
  return detail?.kind === 'group' ? (
    <GroupDetailView
      slug={detail.id}
      onBack={closeDetail}
      openDetail={openDetail}
      closeDetail={closeDetail}
    />
  ) : detail?.kind === 'benchmark' ? (
    <BenchmarkDetailView
      name={detail.id}
      fromGroup={detail.fromGroup}
      onBack={closeDetail}
      openDetail={openDetail}
    />
  ) : (
    <GroupsListView openDetail={openDetail} />
  )
}

// ——— group list (mockup L431-456) ————————————————————————————————————————

function GroupsListView({ openDetail }: { openDetail(d: Detail): void }) {
  const toast = useToast()
  const { data: groups } = useGroups()
  const { data: benchmarks } = useBenchmarks()
  const list = groups ?? []
  const all = benchmarks ?? []
  const total = all.length

  const handleNew = useCallback(async () => {
    if (all.length === 0) return
    const base = `group_${list.length + 1}`
    let slug = base
    let n = 0
    // eslint-disable-next-line no-constant-condition
    while (true) {
      try {
        await getHost().catalog.saveGroup(slug, [all[0]])
        toast.show('new group created')
        openDetail({ kind: 'group', id: slug })
        return
      } catch (e) {
        if ((e as { code?: string }).code === 'conflict') {
          n += 1
          slug = `${base}_${n}`
          continue
        }
        toast.show(errText(e, 'create failed'))
        return
      }
    }
  }, [all, list.length, toast, openDetail])

  // Mockup `onDuplicate` (L450-455): the copy is opened straight away, since
  // duplicating is how you get an editable version of a built-in group.
  const handleDuplicate = useCallback(
    (slug: string) => {
      void getHost()
        .catalog.duplicateGroup(slug)
        .then((copy) => {
          toast.show(`editing ${copy.slug}`)
          openDetail({ kind: 'group', id: copy.slug })
        })
        .catch((e) => toast.show(errText(e, 'duplicate failed')))
    },
    [toast, openDetail],
  )

  const handleDelete = useCallback(
    (slug: string) => {
      void getHost()
        .catalog.deleteGroup(slug)
        .then(() => toast.show(`deleted ${slug}`))
        .catch((e) => toast.show(errText(e, 'delete failed')))
    },
    [toast],
  )

  return (
    <div className={styles.page}>
      <DetailHeader
        title={PAGE_META['Benchmark groups'][0]}
        blurb={PAGE_META['Benchmark groups'][1]}
        action={{
          label: PAGE_META['Benchmark groups'][2] as string,
          onAction: () => void handleNew(),
        }}
      />
      <span className={cx('mono', styles.kicker)}>benchmark groups</span>
      <div className={styles.colHeader}>
        <span className={cx('mono', styles.cName)}>group</span>
        <span className={cx('mono', styles.cCount)}>benchmark count</span>
        <span className={cx('mono', styles.cProfiles)}>profiles</span>
        <span className={styles.cActions} />
      </div>
      {list.map((g) => (
        <div
          key={g.slug}
          className={cx('row', styles.groupRow)}
          onClick={() => openDetail({ kind: 'group', id: g.slug })}
        >
          <span className={styles.slugCell}>
            <span className={cx('mono', styles.slug)}>{g.slug}</span>
            {!g.builtin ? <Tag variant="neutral">custom</Tag> : null}
          </span>
          <span
            className={cx('mono', styles.count)}
            title={`${g.benchmark_count} of ${total} benchmarks`}
          >
            {g.benchmark_count}
          </span>
          <span
            className={cx('mono', styles.profiles)}
            title={`weighted by ${g.in_profiles} profiles`}
          >
            {g.in_profiles}
          </span>
          <span className={styles.actions} onClick={(e) => e.stopPropagation()}>
            {/* Mockup L454 is 11px / 2px 6px — 1px tighter than the shared
                `xs` button scale, so the padding rides on a className the way
                the mockup rides on an inline style. */}
            <Button variant="ghost" className={styles.dupNarrow} onClick={() => handleDuplicate(g.slug)}>
              Duplicate
            </Button>
            <button
              type="button"
              className={cx('ib', g.builtin && 'off', styles.iconBtn)}
              title={g.builtin ? 'Built-in group — cannot be deleted' : `Delete ${g.slug}`}
              disabled={g.builtin}
              onClick={() => handleDelete(g.slug)}
            >
              <TrashIcon size={12} />
            </button>
            <span className={styles.chevron}>
              <ChevronIcon />
            </span>
          </span>
        </div>
      ))}
      <p className={styles.note}>{LIST_NOTE}</p>
    </div>
  )
}

// ——— group detail (mockup L457-491) ——————————————————————————————————————

function GroupDetailView({
  slug,
  onBack,
  openDetail,
  closeDetail,
}: {
  slug: string
  onBack(): void
  openDetail(d: Detail): void
  closeDetail(): void
}) {
  const toast = useToast()
  const { data: group } = useGroupDetail(slug)
  // Only for the "· weighted by N profiles" half of the summary line (mockup
  // `grSummary`); GroupDetail itself does not carry the profile count. Same
  // query key as the list view, so this is a cache hit on the way in.
  const { data: groups } = useGroups()
  const [query, setQuery] = useState('')
  // null = "showing the persisted slug"; a string is an uncommitted edit.
  const [nameDraft, setNameDraft] = useState<string | null>(null)
  const [local, setLocal] = useState<boolean[] | null>(null)
  const saveTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  // group.benchmarks IS the whole catalogue with an `on` flag per row
  // (internal/service/catalog.go groupDetailLocked), so membership is read and
  // written straight off it — no second, separately-ordered benchmark list.
  const schedule = useCallback(
    (rows: { name: string }[], nextFlags: boolean[]) => {
      setLocal(nextFlags)
      if (saveTimer.current) clearTimeout(saveTimer.current)
      saveTimer.current = setTimeout(() => {
        const members = rows.filter((_, i) => nextFlags[i] ?? false).map((b) => b.name)
        void getHost()
          .catalog.saveGroup(slug, members)
          .then(() => setLocal(null))
          .catch((e) => toast.show(errText(e, 'save failed')))
      }, 300)
    },
    [slug, toast],
  )

  useEffect(() => {
    return () => {
      if (saveTimer.current) clearTimeout(saveTimer.current)
    }
  }, [])

  if (!group) return <div className={cx(styles.page, styles.loading)}>loading…</div>

  const readOnly = group.builtin
  const onFlags = local ?? group.benchmarks.map((b) => b.on)
  const onCount = onFlags.filter(Boolean).length
  const inProfiles = (groups ?? []).find((g) => g.slug === slug)?.in_profiles
  const summary =
    `${onCount} of ${group.benchmarks.length} benchmarks` +
    (inProfiles === undefined ? '' : ` · weighted by ${inProfiles} profiles`)

  const q = query.trim().toLowerCase()
  const shown = group.benchmarks
    .map((b, i) => ({ ...b, on: onFlags[i] ?? false, idx: i }))
    .filter((r) => (q ? r.name.toLowerCase().includes(q) : true))
    .sort((a, b) => (b.on ? 1 : 0) - (a.on ? 1 : 0) || a.name.localeCompare(b.name))

  // Rename commits on blur/Enter (mockup `onGrRename`). The slug is this
  // view's identity, so the detail stack entry is swapped for the new one
  // rather than pushed onto — back still leaves for the group list.
  function commitRename() {
    if (nameDraft === null || !group) return
    const next = sanitizeSlug(nameDraft)
    setNameDraft(null)
    if (!next || next === group.slug) return
    const members = group.benchmarks.filter((_, i) => onFlags[i] ?? false).map((b) => b.name)
    void getHost()
      .catalog.saveGroup(slug, members, next)
      .then(() => {
        closeDetail()
        openDetail({ kind: 'group', id: next })
      })
      .catch((e) => toast.show(errText(e, 'rename failed')))
  }

  return (
    <div className={styles.page}>
      <DetailHeader
        title={group.slug}
        blurb={readOnly ? GROUP_BLURB_BUILTIN : GROUP_BLURB_CUSTOM}
        backLabel="Benchmark groups"
        onBack={onBack}
      />
      {/* Summary line + the group's own actions (mockup L458-465). Delete
          lives here as an `.ib` trash, not as the header action button. */}
      <div className={styles.grSummary}>
        {readOnly ? <Tag variant="neutral">built-in · read-only</Tag> : null}
        <span className={cx('mono', styles.grSummaryText)}>{summary}</span>
        <span className={styles.grSummaryActions}>
          <Button
            variant="ghost"
            size="xs"
            onClick={() =>
              void getHost()
                .catalog.duplicateGroup(slug)
                .then((copy) => {
                  toast.show(`editing ${copy.slug}`)
                  openDetail({ kind: 'group', id: copy.slug })
                })
                .catch((e) => toast.show(errText(e, 'duplicate failed')))
            }
          >
            {readOnly ? 'Duplicate & edit' : 'Duplicate'}
          </Button>
          <button
            type="button"
            className={cx('ib', readOnly && 'off', styles.iconBtnLg)}
            title={readOnly ? 'Built-in group — cannot be deleted' : 'Delete this group'}
            disabled={readOnly}
            onClick={() =>
              void getHost()
                .catalog.deleteGroup(slug)
                .then(() => {
                  toast.show(`deleted ${slug}`)
                  onBack()
                })
                .catch((e) => toast.show(errText(e, 'delete failed')))
            }
          >
            <TrashIcon size={13} />
          </button>
        </span>
      </div>
      {!readOnly ? (
        <div className={styles.nameRow}>
          <span className={styles.nameLabel}>name</span>
          <Input
            className={styles.nameInput}
            value={nameDraft ?? group.slug}
            onChange={setNameDraft}
            onBlur={commitRename}
            onKeyDown={(e) => {
              if (e.key === 'Enter') e.currentTarget.blur()
              if (e.key === 'Escape') setNameDraft(null)
            }}
          />
        </div>
      ) : null}
      <div className={styles.benchHead}>
        <span className={styles.benchHeadLabel}>benchmarks</span>
        <Input
          className={styles.filterInput}
          value={query}
          onChange={setQuery}
          placeholder="filter the catalogue"
        />
        <span className={cx('mono', styles.covKicker)}>models covered</span>
      </div>
      <div className={cx('scroll', styles.grList)}>
        {shown.map((r) => (
          <span key={r.name} className={cx(styles.grRow, r.on && styles.grRowOn)}>
            <Toggle
              on={r.on}
              disabled={readOnly}
              onToggle={() => {
                const next = [...onFlags]
                next[r.idx] = !onFlags[r.idx]
                schedule(group.benchmarks, next)
              }}
            />
            <button
              type="button"
              className={cx('mono', styles.grName, r.on && styles.grNameOn)}
              onClick={() => openDetail({ kind: 'benchmark', id: r.name, fromGroup: slug })}
            >
              {r.name}
            </button>
            <CoverageBar covered={r.covered} total={r.coverage_total} className={styles.grCov} />
            <span className={cx('mono', styles.grCovText, r.on && styles.grCovTextOn)}>
              {`${r.covered} / ${r.coverage_total}`}
            </span>
          </span>
        ))}
      </div>
      {readOnly ? <p className={styles.grNote}>{GROUP_READONLY_NOTE}</p> : null}
    </div>
  )
}

// ——— benchmark detail (mockup L400-430) ——————————————————————————————————

type BenchSortKey = 'model' | 'value' | 'score'

/** Sortable column set, mockup `benchSortCols` (L1333-1342). The 188px score
 *  column spans the 32px number plus the 12px gap plus the 144px bar. */
const BENCH_COLS: ReadonlyArray<{ key: BenchSortKey; label: string; cls: string }> = [
  { key: 'model', label: 'model (reasoning)', cls: styles.sortModel },
  { key: 'value', label: 'benchmark result', cls: styles.sortValue },
  { key: 'score', label: 'normalised score', cls: styles.sortScore },
]

function BenchmarkDetailView({
  name,
  fromGroup,
  onBack,
  openDetail,
}: {
  name: string
  fromGroup: string | null
  onBack(): void
  openDetail(d: Detail): void
}) {
  const { data: detail } = useBenchmarkDetail(name)
  // Cache hit for the group we came from — it carries this benchmark's
  // covered/total, which BenchmarkDetail itself does not (mockup
  // `benchCovText`). Disabled when there is no originating group.
  const { data: originGroup } = useGroupDetail(fromGroup ?? '')
  // Mockup's initial `bsort` is score/desc — the best-scoring routes first.
  const [sort, setSort] = useState<{ key: BenchSortKey; dir: 'asc' | 'desc' }>({
    key: 'score',
    dir: 'desc',
  })

  const sortedRows = useMemo(() => {
    if (!detail) return []
    const sign = sort.dir === 'desc' ? -1 : 1
    return [...detail.rows].sort((a: BenchRow, b: BenchRow) => {
      if (sort.key === 'model')
        return sign * `${a.model}${a.reasoning}`.localeCompare(`${b.model}${b.reasoning}`)
      if (sort.key === 'score') return sign * (a.norm - b.norm)
      return sign * (a.value - b.value)
    })
  }, [detail, sort])

  if (!detail) return <div className={cx(styles.page, styles.loading)}>loading…</div>

  const covEntry = originGroup?.benchmarks.find((b) => b.name === detail.name)
  const covText = covEntry
    ? `${covEntry.covered} of ${covEntry.coverage_total}`
    : `${detail.rows.length} tested`

  return (
    <div className={styles.page}>
      <DetailHeader
        title={detail.name}
        blurb={detail.note || BENCH_BLURB_FALLBACK}
        backLabel="Benchmark groups"
        onBack={onBack}
      />
      <div className={styles.benchGroups}>
        <span className={styles.inGroupsLabel}>in groups</span>
        {detail.groups.map((g) => (
          <Tag
            key={g}
            variant="accent"
            size="chip"
            className={styles.groupChip}
            // Returning to the group we came from pops the stack instead of
            // pushing a second copy of it onto the stack.
            onClick={() => (g === fromGroup ? onBack() : openDetail({ kind: 'group', id: g }))}
          >
            {g}
          </Tag>
        ))}
        {detail.groups.length === 0 ? (
          <span className={cx('mono', styles.noGroups)}>{BENCH_NO_GROUPS}</span>
        ) : null}
      </div>
      <div className={styles.testedHead}>
        <span className={styles.testedLabel}>tested models</span>
        <span className={cx('mono', styles.testedCount)}>{covText}</span>
      </div>
      <div className={styles.sortRow}>
        {BENCH_COLS.map((c) => {
          const active = sort.key === c.key
          return (
            <button
              key={c.key}
              type="button"
              className={cx('mono', styles.sortCol, c.cls, active && styles.sortColActive)}
              onClick={() =>
                setSort({ key: c.key, dir: active && sort.dir === 'desc' ? 'asc' : 'desc' })
              }
            >
              {c.label + (active ? (sort.dir === 'desc' ? '  ↓' : '  ↑') : '')}
            </button>
          )
        })}
      </div>
      <div className={cx('scroll', styles.benchList)}>
        {sortedRows.map((r) => {
          const score = Math.round(r.norm)
          return (
            <span key={`${r.model}@${r.reasoning}`} className={styles.benchRow}>
              <span className={styles.benchLabel}>{`${r.model}  (${r.reasoning})`}</span>
              <span className={cx('mono', styles.benchValue)}>{r.value.toFixed(1)}</span>
              <span className={cx('mono', styles.benchScore)}>{score}</span>
              <span className={styles.benchBar}>
                <b
                  className={styles.benchBarFill}
                  style={{ width: `${Math.max(0, Math.min(100, score))}%` }}
                />
              </span>
            </span>
          )
        })}
        {sortedRows.length === 0 ? (
          <span className={cx('mono', styles.benchEmpty)}>no tested models</span>
        ) : null}
      </div>
    </div>
  )
}
export default GroupsPage
