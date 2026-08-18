import type { ReactNode } from 'react'
import { cx } from '../../utils/cx'
import styles from './settings.module.css'

export interface SettingsDetailShellProps {
  /** Content for the master/list pane, when the parent uses a split view. */
  master?: ReactNode
  /** Content for the detail pane. Children are used when omitted. */
  detail?: ReactNode
  children?: ReactNode
  title?: ReactNode
  description?: ReactNode
  /** The back button is rendered when a callback is supplied. */
  onBack?: () => void
  backLabel?: string
  actions?: ReactNode
  className?: string
  bodyClassName?: string
}

export function SettingsDetailShell({
  master,
  detail,
  children,
  title,
  description,
  onBack,
  backLabel = 'Back',
  actions,
  className,
  bodyClassName,
}: SettingsDetailShellProps) {
  const detailContent = detail ?? children
  const hasHeader = title !== undefined || description !== undefined || onBack !== undefined || actions !== undefined

  return (
    <div className={cx(styles.detail, className)}>
      {hasHeader && (
        <header className={styles.detailHeader}>
          {onBack !== undefined && (
            <button type="button" className={styles.backButton} onClick={onBack} aria-label={backLabel}>
              <svg viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                <path d="M7.2 2.2 3.4 6l3.8 3.8" />
              </svg>
              {backLabel}
            </button>
          )}
          {(title !== undefined || description !== undefined) && (
            <div className={styles.detailHeaderCopy}>
              {title !== undefined && <h1 className={styles.detailTitle}>{title}</h1>}
              {description !== undefined && <p className={styles.detailDescription}>{description}</p>}
            </div>
          )}
          {actions !== undefined && <div className={styles.detailActions}>{actions}</div>}
        </header>
      )}
      <div className={cx(styles.detailBody, bodyClassName)}>
        {master !== undefined && <aside className={styles.master}>{master}</aside>}
        <div className={styles.detailPane}>{detailContent}</div>
      </div>
    </div>
  )
}
