import { cx } from '../utils/cx'
import styles from './CoverageBar.module.css'

export interface CoverageBarProps {
  covered: number
  total: number // fill = round(covered/total*100)%; total<=0 → 0
  className?: string // width comes from the caller (56px in group rows)
}

export function CoverageBar({ covered, total, className }: CoverageBarProps) {
  const pct = total <= 0 ? 0 : Math.round((covered / total) * 100)
  return (
    <span className={cx(styles.track, className)}>
      <span className={styles.fill} style={{ width: pct + '%' }} />
    </span>
  )
}