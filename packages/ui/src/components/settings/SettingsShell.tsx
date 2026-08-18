import type { ReactNode } from 'react'
import { cx } from '../../utils/cx'
import { SETTINGS_NAV_ITEMS, SettingsNav, type SettingsNavEntry, type SettingsNavItemName } from './SettingsNav'
import styles from './settings.module.css'

export type SettingsSectionName = SettingsNavItemName

export interface SettingsShellProps {
  /** Selected sidebar page. Defaults to `Profiles`. */
  activeSection?: string
  /** Aliases for `activeSection`, useful when embedding the shell in a router. */
  activePage?: string
  selectedSection?: string
  /** Called when the sidebar selection changes. */
  onSectionChange?: (section: string) => void
  /** Aliases for `onSectionChange`. */
  onPageChange?: (section: string) => void
  onNavigate?: (section: string) => void
  /** Override the built-in eight-item navigation. */
  navItems?: readonly SettingsNavEntry[]
  /** Optional content rendered alongside the sidebar. */
  children?: ReactNode
  /** Alias for `children`. */
  content?: ReactNode
  className?: string
  sidebarClassName?: string
  contentClassName?: string
}

export function SettingsShell({
  activeSection,
  activePage,
  selectedSection,
  onSectionChange,
  onPageChange,
  onNavigate,
  navItems = SETTINGS_NAV_ITEMS,
  children,
  content,
  className,
  sidebarClassName,
  contentClassName,
}: SettingsShellProps) {
  const selected = activeSection ?? activePage ?? selectedSection
  const onChange = onSectionChange ?? onPageChange ?? onNavigate

  return (
    <div className={cx(styles.shell, className)} data-testid="settings-shell">
      <aside className={cx(styles.sidebar, sidebarClassName)}>
        <SettingsNav items={navItems} activeItem={selected} onSelect={onChange} />
      </aside>
      <main className={cx(styles.content, contentClassName)}>{content ?? children}</main>
    </div>
  )
}
