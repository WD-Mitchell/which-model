import type { RankedModel } from '@which-model/core'
import { cx } from '../../../utils/cx'
import styles from './RankList.module.css'

export interface RankListProps {
  items: RankedModel[]
  /** Selected row (array index); controlled. */
  index: number
  /** Fired on any row click, including the selected row. */
  onPick: (i: number) => void
}

export function RankList({ items, index, onPick }: RankListProps) {
  return (
    <div className={styles.band}>
      {items.map((model, i) => {
        const selected = i === index
        const provider = model.route_key.split('/')[0]
        return (
          <button
            type="button"
            key={model.route_key}
            aria-current={selected ? 'true' : undefined}
            className={cx(styles.row, selected && styles.rowSelected)}
            onClick={() => onPick(i)}
          >
            <span className={styles.rank}>{model.rank}</span>
            <span className={cx(styles.name, selected && styles.nameSelected)}>
              {model.model_name}
            </span>
            <span className={cx(styles.score, selected && styles.scoreSelected)}>
              {model.score.toFixed(2)}
            </span>
            <span className={styles.route}>{provider}</span>
          </button>
        )
      })}
    </div>
  )
}