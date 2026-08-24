import type { ReactNode } from 'react'
import { cx } from '../../utils/cx'
import styles from './settings.module.css'

export interface SettingsSectionProps {
  /** Section heading. `label` is the preferred name in settings forms. */
  label?: ReactNode
  title?: ReactNode
  description?: ReactNode
  children?: ReactNode
  /** Optional content aligned with the section heading. */
  actions?: ReactNode
  className?: string
}

export function SettingsSection({
  label,
  title,
  description,
  children,
  actions,
  className,
}: SettingsSectionProps) {
  const heading = label ?? title
  const headingId = typeof heading === 'string' ? `settings-section-${heading.toLowerCase().replace(/[^a-z0-9]+/g, '-')}` : undefined

  return (
    <section className={cx(styles.section, className)} aria-labelledby={headingId}>
      {(heading !== undefined || description !== undefined || actions !== undefined) && (
        <header className={styles.sectionHeader}>
          <div>
            {heading !== undefined && (
              <h2 className={styles.sectionLabel} id={headingId}>
                {heading}
              </h2>
            )}
            {description !== undefined && <p className={styles.sectionDescription}>{description}</p>}
          </div>
          {actions !== undefined && <div className={styles.headerActions}>{actions}</div>}
        </header>
      )}
      <div className={styles.sectionBody}>{children}</div>
    </section>
  )
}
