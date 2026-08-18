// U09 — Benchmark groups settings page: list of groups with coverage, a group
// detail (member toggles, rename, duplicate/delete), and a benchmark detail
// (model×reasoning result table).
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Button, CoverageBar, Input, Table, Tag, useToast } from '@which-model/ui'
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

export function GroupsPage({ detail, openDetail, closeDetail }: PageComponentProps) {
  return detail?.kind === 'group' ? (
    <GroupDetailView slug={detail.id} onBack={closeDetail} openDetail={openDetail} />
  ) : detail?.kind === 'benchmark' ? (
    <BenchmarkDetailView name={detail.id} fromGroup={detail.fromGroup} onBack={closeDetail} />
  ) : (
    <GroupsListView openDetail={openDetail} />
  )
}

function GroupsListView({ openDetail }: { openDetail(d: Detail): void }) {
  const toast = useToast()
  const { data: groups } = useGroups()
  const { data: benchmarks } = useBenchmarks()
  const list = groups ?? []
  const all = benchmarks ?? []

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
        toast.show((e as { message?: string }).message ?? 'create failed')
        return
      }
    }
  }, [all, list.length, toast, openDetail])

  const total = all.length

  return (
    <div className={styles.page}>
      <DetailHeader
        title={PAGE_META['Benchmark groups'][0]}
        blurb={PAGE_META['Benchmark groups'][1]}
        action={{ label: PAGE_META['Benchmark groups'][2] as string, onAction: () => void handleNew() }}
      />
      <div className={styles.kicker}>benchmark groups</div>
      <div className={styles.colHeader}>
        <span className={styles.cName}>name</span>
        <span className={styles.cCount}>benchmarks</span>
        <span className={styles.cCover}>coverage</span>
        <span className={styles.cActions} />
      </div>
      {list.map((g) => (
        <div key={g.slug} className={styles.row} onClick={() => openDetail({ kind: 'group', id: g.slug })}>
          <span className={styles.name}><span className="mono">{g.slug}</span></span>
          <span className={styles.count} title={`${g.benchmark_count} of ${total} benchmarks · weighted by ${g.in_profiles} profiles`}>
            <span className="mono">{`${g.benchmark_count} of ${total} benchmarks`}</span>
            <span> · weighted by {g.in_profiles} profiles</span>
          </span>
          <span className={styles.cover}><CoverageBar covered={g.benchmark_count} total={total} /></span>
          <span className={styles.actions} onClick={(e) => e.stopPropagation()}>
            <Button variant="ghost" size="sm" onClick={() =>
              void getHost().catalog.duplicateGroup(g.slug)
                .then(() => toast.show(`duplicated ${g.slug}`))
                .catch((e) => toast.show((e as { message?: string }).message ?? 'duplicate failed'))
            }>Duplicate</Button>
            <button type="button" className={'ib ' + (g.builtin ? ' off' : '')}
              title={g.builtin ? 'Built-in group — cannot be deleted' : `Delete ${g.slug}`}
              disabled={g.builtin}
              onClick={() =>
                void getHost().catalog.deleteGroup(g.slug)
                  .then(() => toast.show(`deleted ${g.slug}`))
                  .catch((e) => toast.show((e as { message?: string }).message ?? 'delete failed'))
              }>×</button>
            <span className="ib">›</span>
          </span>
        </div>
      ))}
    </div>
  )
}

function GroupDetailView({
  slug,
  onBack,
  openDetail,
}: {
  slug: string
  onBack(): void
  openDetail(d: Detail): void
}) {
  const toast = useToast()
  const { data: group } = useGroupDetail(slug)
  const { data: allBench } = useBenchmarks()
  const [query, setQuery] = useState('')
  const [rename, setRename] = useState('')
  const [local, setLocal] = useState<boolean[] | null>(null)
  const saveTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const all = allBench ?? []

  const schedule = useCallback(
    (nextFlags: boolean[]) => {
      setLocal(nextFlags)
      if (saveTimer.current) clearTimeout(saveTimer.current)
      saveTimer.current = setTimeout(() => {
        const members = all.filter((_, i) => nextFlags[i] ?? false)
        const san = sanitizeSlug(rename)
        void getHost()
          .catalog.saveGroup(slug, members, san || undefined)
          .then(() => setLocal(null))
          .catch((e) => toast.show((e as { message?: string }).message ?? 'save failed'))
      }, 300)
    },
    [all, slug, rename, toast],
  )

  useEffect(() => {
    return () => {
      if (saveTimer.current) clearTimeout(saveTimer.current)
    }
  }, [])

  if (!group) return <div className={styles.page}>Loading…</div>

  const readOnly = group.builtin
  const onFlags = local ?? group.benchmarks.map((b) => b.on)
  const q = query.trim().toLowerCase()
  const shown = group.benchmarks
    .map((b, i) => ({ name: b.name, on: onFlags[i] ?? false, covered: b.covered, coverage_total: b.coverage_total, idx: i }))
    .filter((r) => (q ? r.name.toLowerCase().includes(q) : true))
    .sort((a, b) => (b.on ? 1 : 0) - (a.on ? 1 : 0) || a.name.localeCompare(b.name))

  return (
    <div className={styles.page}>
      <DetailHeader
        title={group.slug}
        blurb={`${group.benchmarks.filter((b) => b.on).length} of ${group.benchmarks.length} benchmarks`}
        backLabel="Benchmark groups"
        onBack={onBack}
        action={
          readOnly
            ? undefined
            : {
                label: 'Delete',
                onAction: () =>
                  void getHost()
                    .catalog.deleteGroup(slug)
                    .then(() => {
                      toast.show(`deleted ${slug}`)
                      onBack()
                    })
                    .catch((e) => toast.show((e as { message?: string }).message ?? 'delete failed')),
              }
        }
      />
      <div className={styles.controls}>
        <div style={{ width: '220px' }}>
          <Input value={query} onChange={setQuery} placeholder="filter benchmarks" />
        </div>
        {!readOnly ? (
          <div style={{ width: '200px' }}>
            <Input value={rename} onChange={setRename} placeholder="rename (slug)" mono />
          </div>
        ) : null}
      </div>
      {readOnly ? <Tag variant="neutral">built-in · read-only</Tag> : null}
      <div className={styles.list}>
        {shown.map((r) => (
          <div key={r.name} className={styles.brow} onClick={() => openDetail({ kind: 'benchmark', id: r.name, fromGroup: slug })}>
            <span className={styles.bname}><span className="mono">{r.name}</span></span>
            <span className={styles.bcover}><CoverageBar covered={r.covered} total={r.coverage_total} /></span>
            <span className={styles.bon}>
              <button type="button"
                className={'ib ' + (r.on ? '' : 'off')}
                disabled={readOnly}
                title={readOnly ? 'read-only' : 'toggle membership'}
                onClick={(e) => {
                  e.stopPropagation()
                  if (readOnly) return
                  const next = [...onFlags]
                  next[r.idx] = !onFlags[r.idx]
                  schedule(next)
                }}
              >{r.on ? '✓' : ''}</button>
            </span>
          </div>
        ))}
      </div>
    </div>
  )
}

function BenchmarkDetailView({
  name,
  fromGroup,
  onBack,
}: {
  name: string
  fromGroup: string | null
  onBack(): void
}) {
  const { data: detail } = useBenchmarkDetail(name)
  const [sort, setSort] = useState<{ key: string; dir: 'asc' | 'desc' } | null>(null)

  const sortedRows = useMemo(() => {
    if (!detail) return []
    const rows = [...detail.rows]
    if (sort) {
      rows.sort((a, b) => {
        const av = a[sort.key as keyof BenchRow]
        const bv = b[sort.key as keyof BenchRow]
        const cmp = typeof av === 'number' && typeof bv === 'number' ? av - bv : String(av).localeCompare(String(bv))
        return sort.dir === 'asc' ? cmp : -cmp
      })
    }
    return rows
  }, [detail, sort])

  if (!detail) return <div className={styles.page}>Loading…</div>

  return (
    <div className={styles.page}>
      <DetailHeader title={detail.name} blurb={detail.note || 'Benchmark detail'} backLabel="Benchmark groups" onBack={onBack} />
      <div className={styles.benchGroups}>
        {detail.groups.map((g) => <Tag key={g} variant="accent">{g}</Tag>)}
      </div>
      <div className={styles.tableWrap}>
        <Table
          columns={[
            { key: 'model', label: 'model', sortable: true },
            { key: 'reasoning', label: 'reasoning', sortable: true },
            { key: 'value', label: 'value', sortable: true, align: 'right' },
            { key: 'norm', label: 'norm', sortable: true, align: 'right' },
          ]}
          sort={sort}
          onSort={setSort}
          rows={() => (
            <>
              {sortedRows.map((r) => (
                <tr key={`${r.model}@${r.reasoning}`}>
                  <td className="mono">{r.model}</td>
                  <td className="mono">{r.reasoning}</td>
                  <td style={{ textAlign: 'right' }}>{r.value.toFixed(2)}</td>
                  <td style={{ textAlign: 'right' }}>{Math.round(r.norm)}</td>
                </tr>
              ))}
              {sortedRows.length === 0 ? (
                <tr><td colSpan={4} style={{ color: 'color-mix(in srgb, var(--color-text) 42%, transparent)' }}>no tested models</td></tr>
              ) : null}
            </>
          )}
        />
      </div>
    </div>
  )
}
export default GroupsPage
