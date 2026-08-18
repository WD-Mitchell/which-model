// U07 — DetailHeader: shared settings detail-view header (title, blurb,
// optional back chevron + action button).
import styles from './DetailHeader.module.css'

export interface DetailHeaderProps {
  title: string
  blurb: string
  backLabel?: string
  onBack?(): void
  action?: { label: string; onAction(): void }
}

export function DetailHeader({ title, blurb, backLabel, onBack, action }: DetailHeaderProps) {
  return (
    <header className={styles.header}>
      {backLabel ? (
        <button type="button" className={styles.back} onClick={onBack}>
          <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5">
            <path d="M9.8 3.2 5 8l4.8 4.8" />
          </svg>
          {backLabel}
        </button>
      ) : null}
      <h1 className={styles.title}>{title}</h1>
      <p className={styles.blurb}>{blurb}</p>
      {action ? (
        <button type="button" className={styles.action} onClick={action.onAction}>
          {action.label}
        </button>
      ) : null}
    </header>
  )
}