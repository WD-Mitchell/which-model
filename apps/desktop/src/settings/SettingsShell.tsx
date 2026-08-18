// U07 — SettingsShell: the native settings-window chrome — a hidden-inset
// macOS titlebar (traffic lights, draggable region) over a web-drawn title,
// a three-group sidebar, and the content column where the active page mounts.
import type { ReactNode } from 'react'
import { NAV_GROUPS, type SettingsPageName } from './pages'
import styles from './SettingsShell.module.css'

export interface SettingsShellProps {
  page: SettingsPageName
  onPage(page: SettingsPageName): void
  configPath: string
  onClose(): void
  children: ReactNode
}

export function SettingsShell({ page, onPage, configPath, onClose, children }: SettingsShellProps) {
  return (
    <div className={styles.window}>
      <div
        className={styles.titlebar}
        style={{ ['--wails-draggable' as string]: 'drag' }}
      >
        <div className={styles.traffic}>
          <button
            type="button"
            className={styles.dotClose}
            style={{ ['--wails-draggable' as string]: 'no-drag' }}
            aria-label="Close settings"
            onClick={onClose}
          />
          <span className={styles.dot} />
          <span className={styles.dot} />
        </div>
        <span className={styles.title}>which-model — Settings</span>
      </div>
      <div className={styles.body}>
        <nav className={styles.sidebar}>
          {NAV_GROUPS.map(([label, items]) => (
            <div key={label} className={styles.navGroup}>
              <div className={styles.navLabel}>{label}</div>
              {items.map((name) => (
                <button
                  key={name}
                  type="button"
                  className={styles.navItem + (name === page ? ' ' + styles.navActive : '')}
                  onClick={() => onPage(name)}
                >
                  {name}
                </button>
              ))}
            </div>
          ))}
        </nav>
        <main className={styles.content}>
          {children}
          {configPath ? (
            <footer className={styles.footer}>
              <span className="mono">{configPath}</span>
            </footer>
          ) : null}
        </main>
      </div>
    </div>
  )
}