import { cx } from '../utils/cx'
import styles from './UsageMeter.module.css'

export interface UsageMeterProps {
  label: string // rendered uppercase 9px mono
  percent: number | null // null → "—" text, 0-width grey fill
  hot?: boolean // force accent-300 fill; else auto at percent >= 70
}

export function UsageMeter({ label, percent, hot = false }: UsageMeterProps) {
  const pct = percent === null ? 0 : Math.max(0, Math.min(100, percent))
  const fillClass =
    percent === null
      ? styles.fillNull
      : hot || percent >= 70
        ? styles.fillHot
        : styles.fillCool
  return (
    <div className={styles.meter}>
      <span className={styles.labelLine}>
        <span className={styles.label}>{label}</span>
        <span className={cx(styles.value, percent === null && styles.valueNull)}>
          {percent === null ? '—' : percent + '%'}
        </span>
      </span>
      <span className={styles.track}>
        <span className={cx(styles.fill, fillClass)} style={{ width: pct + '%' }} />
      </span>
    </div>
  )
}