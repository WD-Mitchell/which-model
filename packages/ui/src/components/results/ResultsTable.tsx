import type { RankedModel } from '@which-model/core'
import { ModelRow } from './ModelRow'
import type { SparkbarEntry } from './Sparkbar'
import styles from './ResultsTable.module.css'

export interface ResultsTableProps {
  /** Rank-ascending candidates as delivered by the backend. */
  items: RankedModel[]
  /** Per-model core metric bars. */
  metrics: (model: RankedModel) => SparkbarEntry[]
  /** Index of the selected row. */
  selectedIndex: number
  onSelect: (index: number) => void
  /** Launch action column; omitted ⇒ every row's launch cell is empty. */
  onLaunch?: (model: RankedModel) => void
}

export function ResultsTable({
  items,
  metrics,
  selectedIndex,
  onSelect,
  onLaunch,
}: ResultsTableProps) {
  return (
    <table className={styles.table}>
      <thead>
        <tr>
          <th scope="col" className={styles.th}>#</th>
          <th scope="col" className={styles.th}>Model</th>
          <th scope="col" className={styles.th}>Provider</th>
          <th scope="col" className={styles.th}>Reasoning</th>
          <th scope="col" className={styles.thRight}>Score</th>
          <th scope="col" className={styles.th}>Metrics</th>
          <th scope="col" className={styles.thLaunch} />
        </tr>
      </thead>
      <tbody>
        {items.map((model, i) => (
          <ModelRow
            key={model.model_id}
            model={model}
            metrics={metrics(model)}
            selected={i === selectedIndex}
            onSelect={() => onSelect(i)}
            onLaunch={onLaunch ? () => onLaunch(model) : undefined}
          />
        ))}
      </tbody>
    </table>
  )
}