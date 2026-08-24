import type { ReactNode } from 'react'
import { cx } from '../../utils/cx'
import styles from './settings.module.css'

export interface SettingsRowProps {
  label: ReactNode
  description?: ReactNode
  /** The control rendered at the trailing edge of the row. */
  control?: ReactNode
  /** Children are an alias for the control slot. */
  children?: ReactNode
  className?: string
}

export function SettingsRow({ label, description, control, children, className }: SettingsRowProps) {
  return (
    <div className={cx(styles.row, className)}>
      <div className={styles.rowInfo}>
        <span className={styles.rowLabel}>{label}</span>
        {description !== undefined && <p className={styles.rowDescription}>{description}</p>}
      </div>
      {(control !== undefined || children !== undefined) && (
        <div className={styles.rowControl}>{control ?? children}</div>
      )}
    </div>
  )
}
