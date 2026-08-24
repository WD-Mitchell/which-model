import type { KeyboardEvent } from 'react'
import type { RankedModel } from '@which-model/core'
import { cx } from '../../utils/cx'
import { Tag } from '../../primitives/Tag'
import { Sparkbar } from './Sparkbar'
import type { SparkbarEntry } from './Sparkbar'
import styles from './ModelRow.module.css'

export interface ModelRowProps {
  model: RankedModel
  metrics: SparkbarEntry[]
  selected: boolean
  onSelect: () => void
  /** Launch action cell; omitted ⇒ the cell is left empty. */
  onLaunch?: () => void
}

export function ModelRow({ model, metrics, selected, onSelect, onLaunch }: ModelRowProps) {
  function onKeyDown(e: KeyboardEvent<HTMLTableRowElement>) {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault()
      onSelect()
    }
  }
  return (
    <tr
      className={cx(styles.row, selected && styles.selected)}
      aria-selected={selected}
      tabIndex={0}
      onClick={onSelect}
      onKeyDown={onKeyDown}
    >
      <td className={cx(styles.cell, styles.rank)}>{model.rank}</td>
      <td className={cx(styles.cell, styles.nameCell)}>
        <span className={styles.name}>{model.model_name}</span>
        <span className={styles.modelId}>{model.model_id}</span>
      </td>
      <td className={styles.cell}>{model.provider}</td>
      <td className={styles.cell}>
        <Tag variant="outline" size="badge">
          {model.reasoning}
        </Tag>
      </td>
      <td className={cx(styles.cell, styles.score)}>{model.score.toFixed(2)}</td>
      <td className={styles.cell}>
        <Sparkbar metrics={metrics} label={false} />
      </td>
      <td className={cx(styles.cell, styles.launch)}>
        {onLaunch && (
          <button
            type="button"
            className={styles.launchBtn}
            aria-label={`launch ${model.model_name}`}
            onClick={(e) => {
              e.stopPropagation()
              onLaunch()
            }}
          >
            Launch
          </button>
        )}
      </td>
    </tr>
  )
}