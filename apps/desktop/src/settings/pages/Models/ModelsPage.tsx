// U15 — Models settings page: catalog-wide list with search, and a summary
// detail for one catalog identity (name, id, reasoning, intel/cost/speed).
//
// Layout follows U09/U14: <main> has no horizontal padding, so every block
// here carries its own 22px gutter and `.row` hover tints bleed edge to edge.
import { useMemo, useState } from 'react'
import { Button, EmptyState, Input, Tag, cx } from '@which-model/ui'
import { useCatalogModels } from '../../../lib/queries'
import { DetailHeader } from '../../DetailHeader'
import { PAGE_META } from '../../pages'
import type { Detail, PageComponentProps } from '../../pages'
import { ModelBenchmarksView } from '../Providers/ProvidersPage'
import { ModelCard } from './ModelCard'
import styles from './ModelsPage.module.css'

const LIST_KICKER = 'catalog models'
const FILTER_PLACEHOLDER = 'filter models'
const EMPTY_CATALOG = 'no models in the catalog'
const EMPTY_FILTER = 'no models match'
const LOAD_ERROR = "couldn't load models"

function formatScore(n: number | null | undefined): string {
  if (n == null) return '—'
  return Number.isInteger(n) ? String(n) : n.toFixed(1)
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
    >
      <path d="M4.4 2.2 8.2 6l-3.8 3.8" />
    </svg>
  )
}

export function ModelsPage({ detail, openDetail, closeDetail }: PageComponentProps) {
  if (detail?.kind === 'provider-model') {
    return (
      <ModelBenchmarksView
        provider={detail.provider}
        modelName={detail.modelName}
        reasoning={detail.reasoning}
        onBack={closeDetail}
      />
    )
  }
  if (detail?.kind === 'model') {
    return (
      <ModelCard
        name={detail.id}
        backLabel={detail.fromProvider ?? 'Models'}
        onBack={closeDetail}
        openDetail={openDetail}
      />
    )
  }
  return <ModelsListView openDetail={openDetail} />
}

function ModelsListView({ openDetail }: { openDetail(d: Detail): void }) {
  const { data, isError, isPending, refetch } = useCatalogModels()
  const [query, setQuery] = useState('')
  const list = data ?? []
  const needle = query.trim().toLowerCase()
  const visible = useMemo(() => {
    if (!needle) return list
    return list.filter(
      (m) =>
        m.model_name.toLowerCase().includes(needle) || m.model_id.toLowerCase().includes(needle),
    )
  }, [list, needle])

  return (
    <div className={styles.page}>
      <DetailHeader title={PAGE_META.Models[0]} blurb={PAGE_META.Models[1]} />
      <div className={styles.filter}>
        <Input
          value={query}
          onChange={setQuery}
          placeholder={FILTER_PLACEHOLDER}
          mono={false}
        />
      </div>
      {isPending ? null : (
      <>
      <span className={cx('mono', styles.kicker)}>{LIST_KICKER}</span>
      <div className={styles.colHeader}>
        <span className={cx('mono', styles.cName)}>model</span>
        <span className={cx('mono', styles.cReasoning)}>reasoning</span>
        <span className={cx('mono', styles.cScore)}>intel</span>
        <span className={cx('mono', styles.cScore)}>cost</span>
        <span className={cx('mono', styles.cScore)}>speed</span>
        <span className={cx('mono', styles.cProviders)}>providers</span>
        <span className={styles.cChevron} />
      </div>
      {isError ? (
        <div className={styles.empty}>
          <span>{LOAD_ERROR}</span>
          <Button variant="ghost" size="xs" onClick={() => void refetch()}>
            Retry
          </Button>
        </div>
      ) : visible.length === 0 ? (
        <div className={styles.empty}>
          <EmptyState text={needle ? EMPTY_FILTER : EMPTY_CATALOG} />
        </div>
      ) : (
        visible.map((m) => (
          <div
            key={m.model_name}
            className={cx('row', styles.modelRow)}
            onClick={() => openDetail({ kind: 'model', id: m.model_name })}
          >
            <span className={styles.nameCell}>
              <span className={styles.name}>{m.model_name}</span>
              {m.model_id ? <span className={cx('mono', styles.id)}>{m.model_id}</span> : null}
            </span>
            <span className={styles.reasoning}>
              {(m.reasoning ?? []).map((level) => (
                <Tag key={level} variant="neutral" size="chip">
                  {level}
                </Tag>
              ))}
            </span>
            <span className={cx('mono', styles.score)}>{formatScore(m.intelligence)}</span>
            <span className={cx('mono', styles.score)}>{formatScore(m.cost)}</span>
            <span className={cx('mono', styles.score)}>{formatScore(m.speed)}</span>
            <span className={cx('mono', styles.providers)}>{m.provider_count}</span>
            <span className={styles.chevron} aria-hidden="true">
              <ChevronIcon />
            </span>
          </div>
        ))
      )}
      </>
      )}
    </div>
  )
}

export default ModelsPage
