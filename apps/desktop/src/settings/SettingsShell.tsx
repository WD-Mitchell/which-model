// U07 — SettingsShell: the settings-window chrome (mockup demo.dc.html
// L243-277) — a titlebar that reserves the macOS hidden-inset traffic lights
// and centres the window title, a three-group sidebar with the config path
// pinned to its base, and the scrolling content column the active page mounts
// into. The window buttons are the REAL native ones (settings.go uses
// MacTitleBarHidden), so they carry standard sizing, hover symbols and a live
// zoom now that the window resizes: the shell draws no dots of its own.
import { useEffect, type ReactNode } from 'react'
import { NAV_GROUPS, type SettingsPageName } from './pages'
import styles from './SettingsShell.module.css'

export interface SettingsShellProps {
  page: SettingsPageName
  onPage(page: SettingsPageName): void
  configPath: string
  version: string
  onClose(): void
  children: ReactNode
}

export function SettingsShell({ page, onPage, configPath, version, onClose, children }: SettingsShellProps) {
  // Close = hide (S00 §4) belongs to the native red traffic light, which routes
  // through the WindowClosing hook in cmd/which-model-desktop/settings.go.
  // Escape is the web-side equivalent and keeps the onClose plumbing (U07
  // CONTRACTS §4) live now that no fake close button renders.
  useEffect(() => {
    function onKeyDown(e: KeyboardEvent) {
      if (e.key !== 'Escape' || e.defaultPrevented) return
      // A dialog owns Escape while it is open (page-level confirm/edit sheets).
      if (document.querySelector('[role="dialog"]')) return
      onClose()
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [onClose])

  return (
    <div className={styles.window}>
      {/* Whole titlebar is the drag region; the OS paints its traffic lights
          over the 78px reserve at its left edge. */}
      <div className={styles.titlebar} style={{ ['--wails-draggable' as string]: 'drag' }}>
        {/* No web-drawn window buttons: AppKit draws the real ones over this
            row (settings.go, MacTitleBarHidden). They carry the standard size,
            spacing, hover symbols and press states that CSS dots cannot, and
            the window is resizable now, so zoom is a live control. The titlebar
            reserves their width on the left instead. */}
        <span className={styles.title}>which-model — Settings</span>
        {/* Mirrors the traffic-light cluster so the title lands on the window's
            true centre line rather than the centre of what is left over. */}
        <span className={styles.titleSpacer} aria-hidden="true" />
      </div>
      <div className={styles.body}>
        <nav className={styles.sidebar} aria-label="Settings sections">
          {NAV_GROUPS.map(([label, items]) => (
            <div key={label} className={styles.navGroup}>
              <div className={'mono ' + styles.navLabel}>{label}</div>
              {items.map((name) => (
                <button
                  key={name}
                  type="button"
                  className={styles.navItem + (name === page ? ' ' + styles.navActive : '')}
                  aria-current={name === page ? 'page' : undefined}
                  onClick={() => onPage(name)}
                >
                  {/* Accent dot marks the active item — the mockup's active
                      state is a neutral raised chip, never accent text. */}
                  <span className={styles.navDot} aria-hidden="true" />
                  {name}
                </button>
              ))}
            </div>
          ))}
          {/* Version + config path share the sidebar's foot; when a version
              is known it REPLACES the config-path line (same .configPath
              style — the version is what "Check for updates" compares
              against). Unknown version → config path. */}
          {version ? (
            <div className={'mono ' + styles.configPath} title={version}>
              {version}
            </div>
          ) : (
            <div className={'mono ' + styles.configPath} title={configPath}>
              {configPath}
            </div>
          )}
        </nav>
        {/* No horizontal padding: every page child supplies its own 22px inline
            padding so row rules and hover tints bleed edge to edge. */}
        <main className={'scroll ' + styles.content}>{children}</main>
      </div>
    </div>
  )
}
