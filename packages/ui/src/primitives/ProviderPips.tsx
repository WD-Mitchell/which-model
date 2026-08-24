import { cx } from '../utils/cx'
import styles from './ProviderPips.module.css'

export interface ProviderPipsProps {
  states: boolean[]
}

export function ProviderPips({ states }: ProviderPipsProps) {
  return (
    <span className={styles.row}>
      {states.map((on, i) => (
        <span key={i} className={cx(styles.pip, on && styles.on)} />
      ))}
    </span>
  )
}