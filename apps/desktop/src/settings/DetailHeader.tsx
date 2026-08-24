// U07 — DetailHeader: the settings content-column header block (mockup
// demo.dc.html L267-277). Back link above the title; title + blurb and the
// optional action button share one flex row aligned on their baselines.
// Supplies its own 22px inline padding — <main> has none (U07 layout contract).
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
        <button type="button" className={styles.back} title="Back" onClick={onBack}>
          <svg
            width="11"
            height="11"
            viewBox="0 0 12 12"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.7"
            strokeLinecap="round"
            strokeLinejoin="round"
          >
            <path d="M7.2 2.2 3.4 6l3.8 3.8" />
          </svg>
          {backLabel}
        </button>
      ) : null}
      <div className={styles.titleRow}>
        <div className={styles.titleBlock}>
          <h1 className={styles.title}>{title}</h1>
          <p className={styles.blurb}>{blurb}</p>
        </div>
        {action ? (
          <span className={styles.actionWrap}>
            {/* Global .btn/.btn-primary + the mockup's literal size overrides.
                These stay inline: .btn sets font-size/padding at the same
                specificity, and CSS-module order is not guaranteed to win. */}
            <button
              type="button"
              className="btn btn-primary"
              style={{ fontSize: '12px', padding: '5px 11px', whiteSpace: 'nowrap' }}
              onClick={action.onAction}
            >
              {action.label}
            </button>
          </span>
        ) : null}
      </div>
    </header>
  )
}
