import { Button, EmptyState, Tag, cx } from '@which-model/ui'
import type { CatalogModelProvider } from '@which-model/core'
import { useCatalogModel } from '../../../lib/queries'
import { DetailHeader } from '../../DetailHeader'
import type { Detail } from '../../pages'
import styles from './ModelsPage.module.css'

const NO_ID = 'no provider id yet'
const DETAIL_KICKER = 'catalog scores'
const PROVIDERS_KICKER = 'enabled providers'
const MISSING_DETAIL = 'not yet in catalog'
const LOAD_ERROR = "couldn't load this model"
const NOT_YET_IN_CATALOG = 'not yet in catalog'
const EMPTY_PROVIDERS = 'no enabled providers offer this model'
const NO_PRICE = 'no listed price'
function isNotFoundError(err: unknown): boolean {
  if (!err || typeof err !== 'object') return false
  if ('code' in err && typeof err.code === 'string') {
    return err.code === 'not_found'
  }
  if ('message' in err && typeof err.message === 'string') {
    return err.message.toLowerCase().includes('not found')
  }
  return false
}
function formatScore(n: number | null | undefined): string {
  if (n == null) return '—'
  return Number.isInteger(n) ? String(n) : n.toFixed(1)
}

function formatUSD(n: number): string {
  return `$${n}`
}

function formatProviderCost(row: CatalogModelProvider): string {
  if (row.input_cost_usd_per_m == null && row.output_cost_usd_per_m == null) return NO_PRICE
  const inn = row.input_cost_usd_per_m == null ? '—' : formatUSD(row.input_cost_usd_per_m)
  const out = row.output_cost_usd_per_m == null ? '—' : formatUSD(row.output_cost_usd_per_m)
  return `${inn} in / ${out} out per 1M`
}

export function ModelCard({
  name,
  backLabel,
  onBack,
  openDetail,
}: {
  name: string
  backLabel: string
  onBack(): void
  openDetail(d: Detail): void
}) {
  const { data: model, isPending, isError, error, refetch } = useCatalogModel(name)

  if (isPending) {
    return (
      <div className={styles.page}>
        <DetailHeader title={name} blurb="" backLabel={backLabel} onBack={onBack} />
        <div className={styles.loading}>loading…</div>
      </div>
    )
  }

  if (isError || !model) {
    const isNotFound = isNotFoundError(error)

    return (
      <div className={styles.page}>
        <DetailHeader title={name} blurb="" backLabel={backLabel} onBack={onBack} />
        <div className={styles.empty}>
          {isNotFound || (!isError && !model) ? (
            <EmptyState text={NOT_YET_IN_CATALOG} />
          ) : (
            <>
              <span>{LOAD_ERROR}</span>
              <Button variant="ghost" size="xs" onClick={() => void refetch()}>
                Retry
              </Button>
            </>
          )}
        </div>
      </div>
    )
  }
  return (
    <div className={styles.page}>
      <DetailHeader
        title={model.model_name}
        blurb={model.model_id || NO_ID}
        backLabel={backLabel}
        onBack={onBack}
      />
      <span className={cx('mono', styles.kicker)}>{DETAIL_KICKER}</span>
      <div className={styles.detailReasoning}>
        {(model.reasoning ?? []).map((level) => (
          <Tag key={level} variant="neutral" size="chip">
            {level}
          </Tag>
        ))}
      </div>
      {!model.in_catalog ? (
        <div className={styles.empty}>
          <EmptyState text="No benchmark data yet" />
        </div>
      ) : (
        <div className={styles.detailScores}>
          <span className={styles.detailScore}>
            <span className={cx('mono', styles.detailLabel)}>intel</span>
            <span className={cx('mono', styles.detailValue)}>{formatScore(model.intelligence)}</span>
          </span>
          <span className={styles.detailScore}>
            <span className={cx('mono', styles.detailLabel)}>cost</span>
            <span className={cx('mono', styles.detailValue)}>{formatScore(model.cost)}</span>
          </span>
          <span className={styles.detailScore}>
            <span className={cx('mono', styles.detailLabel)}>speed</span>
            <span className={cx('mono', styles.detailValue)}>{formatScore(model.speed)}</span>
          </span>
        </div>
      )}
      <span className={cx('mono', styles.kicker)}>{PROVIDERS_KICKER}</span>
      {(model.providers ?? []).length === 0 ? (
        <div className={styles.empty}>
          <EmptyState text={EMPTY_PROVIDERS} />
        </div>
      ) : (
        (model.providers ?? []).map((row) => (
          <ProviderRow
            key={`${row.provider}/${row.model_id}`}
            modelName={model.model_name}
            row={row}
            openDetail={openDetail}
          />
        ))
      )}
    </div>
  )
}

function ProviderRow({
  modelName,
  row,
  openDetail,
}: {
  modelName: string
  row: CatalogModelProvider
  openDetail(d: Detail): void
}) {
  return (
    <div className={styles.providerRow}>
      <span className={styles.providerCell}>
        <span className={styles.providerId}>{row.provider}</span>
        <span className={cx('mono', styles.id)}>{row.model_id}</span>
      </span>
      <span className={styles.reasoning}>
        {(row.reasoning ?? []).map((level) => (
          <Tag
            key={level}
            variant="neutral"
            size="chip"
            onClick={() =>
              openDetail({
                kind: 'provider-model',
                provider: row.provider,
                modelName,
                reasoning: level,
              })
            }
          >
            {level}
          </Tag>
        ))}
      </span>
      <span className={cx('mono', styles.providerCost)}>{formatProviderCost(row)}</span>
    </div>
  )
}

