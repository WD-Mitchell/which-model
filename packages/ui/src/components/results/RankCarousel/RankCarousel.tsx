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

export function RankCarousel({ items, index, onIndex }: RankCarouselProps) {
  const idx = clampIndex(index, items.length)
  const model = items[idx]

  const rankLine = model ? `rank ${idx + 1} of ${items.length}` : 'no route'
  const nameLine = model ? model.model_name : 'Enable a provider'
  const metaLine = model
    ? `${model.provider} · ${model.reasoning} · ${model.score.toFixed(2)}`
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