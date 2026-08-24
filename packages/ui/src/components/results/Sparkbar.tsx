import { Tooltip } from '../../primitives/Tooltip'
import styles from './Sparkbar.module.css'

/** A single metric axis: `value` is on the 0..5 weight scale (clamped for height). */
export interface SparkbarEntry {
  key: string
  value: number
}

export interface SparkbarProps {
  /** Metric bars rendered in the order given (e.g. intelligence, speed, cost). */
  metrics: SparkbarEntry[]
  /** Show the metric key as a small label beneath each bar (axis reading). */
  label?: boolean
}

/** round(4 + value/5*20) px, value clamped to 0..5 → 1→8, 3→16, 5→24. */
export function sparkbarHeight(value: number): number {
  const v = Math.max(0, Math.min(5, value))
  return Math.round(4 + (v / 5) * 20)
}

export function Sparkbar({ metrics, label = true }: SparkbarProps) {
  return (
    <div className={styles.strip} role="group" aria-label="core metrics">
      {metrics.map(({ key, value }) => {
        const tip = `${key}  ${value} / 5`
        return (
          <Tooltip key={key} content={tip}>
            <span className={styles.col} role="img" aria-label={tip}>
              <span className={styles.bar} style={{ height: sparkbarHeight(value) }} />
              {label && <span className={styles.tag}>{key}</span>}
            </span>
          </Tooltip>
        )
      })}
    </div>
  )
}