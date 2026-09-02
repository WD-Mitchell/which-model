import type { RankedModel } from '@which-model/core'
import { cx } from '../../../utils/cx'
import styles from './RankCarousel.module.css'

export interface RankCarouselProps {
  items: RankedModel[]
  /** Controlled focus index; clamped for display only. */
  index: number
  /** Fired by enabled chevrons only. */
  onIndex: (i: number) => void
}

function clampIndex(i: number, length: number): number {
  if (length <= 0) return 0
  return Math.max(0, Math.min(i, length - 1))
}

function formatScore(n: number | null | undefined): string {
  if (n == null) return '—'
  return Number.isInteger(n) ? String(n) : n.toFixed(1)
}
export function RankCarousel({ items, index, onIndex }: RankCarouselProps) {
  const idx = clampIndex(index, items.length)
  const model = items[idx]

  const rankLine = model ? `rank ${idx + 1} of ${items.length}` : 'no route'
  // Reasoning belongs to the model's identity, so it is bracketed after the
  // name in the same type rather than repeated in the meta line.
  const nameLine = model
    ? model.reasoning
      ? `${model.model_name} (${model.reasoning})`
      : model.model_name
    : 'Enable a provider'
  const metaLine = model
    ? `${model.provider} · ${model.score.toFixed(2)}`
    : 'every provider is switched off'

  // Prev disabled iff at the first item (empty list ⇒ only slot); next
  // disabled iff at the last item.
  const prevDisabled = idx <= 0
  const nextDisabled = idx >= items.length - 1

  return (
    <div className={styles.band}>
      <div className={styles.row}>
        <button
          type="button"
          aria-label="previous rank"
          disabled={prevDisabled}
          className={cx(styles.chev, prevDisabled && styles.chevDisabled)}
          onClick={() => onIndex(idx - 1)}
        >
          <svg
            width="12"
            height="12"
            viewBox="0 0 12 12"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.7"
            strokeLinecap="round"
            strokeLinejoin="round"
          >
            <path d="M7.2 2.2 3.4 6l3.8 3.8"></path>
          </svg>
        </button>

        <span className={styles.center}>
          <span className={styles.rank}>{rankLine}</span>
          <span className={styles.name}>{nameLine}</span>
          <span className={styles.meta}>{metaLine}</span>
          {model && (model.intelligence != null || model.cost != null || model.speed != null) ? (
            <span className={styles.ratings}>
              <span className={styles.ratingItem}>
                <span className={styles.ratingLabel}>intel</span>{' '}
                <span className={styles.ratingValue}>{formatScore(model.intelligence)}</span>
              </span>
              <span className={styles.ratingDot}>·</span>
              <span className={styles.ratingItem}>
                <span className={styles.ratingLabel}>cost</span>{' '}
                <span className={styles.ratingValue}>{formatScore(model.cost)}</span>
              </span>
              <span className={styles.ratingDot}>·</span>
              <span className={styles.ratingItem}>
                <span className={styles.ratingLabel}>speed</span>{' '}
                <span className={styles.ratingValue}>{formatScore(model.speed)}</span>
              </span>
            </span>
          ) : null}
        </span>
        <button
          type="button"
          aria-label="next rank"
          disabled={nextDisabled}
          className={cx(styles.chev, nextDisabled && styles.chevDisabled)}
          onClick={() => onIndex(idx + 1)}
        >
          <svg
            width="12"
            height="12"
            viewBox="0 0 12 12"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.7"
            strokeLinecap="round"
            strokeLinejoin="round"
          >
            <path d="M4.8 2.2 8.6 6l-3.8 3.8"></path>
          </svg>
        </button>
      </div>
    </div>
  )
}