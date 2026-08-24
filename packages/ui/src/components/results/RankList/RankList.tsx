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
        // Reasoning reads as part of the model's identity, so it sits in
        // brackets right after the name in the same type — not off in the route
        // column. It is load-bearing: the same model routinely holds several
        // adjacent ranks at different efforts, and the name alone renders them
        // as duplicate rows.
        const provider = model.route_key.split('/')[0]
        const name = model.reasoning ? `${model.model_name} (${model.reasoning})` : model.model_name
        return (
          <button
            type="button"
            key={model.route_key}
            aria-current={selected ? 'true' : undefined}
            className={cx(styles.row, selected && styles.rowSelected)}
            onClick={() => onPick(i)}
          >
            <span className={styles.rank}>{model.rank}</span>
            <span className={cx(styles.name, selected && styles.nameSelected)}>{name}</span>
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