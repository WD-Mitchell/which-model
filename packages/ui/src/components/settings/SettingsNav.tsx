import type { ReactNode } from 'react'
import { cx } from '../../utils/cx'
import styles from './settings.module.css'

/** The settings pages exposed by the desktop settings window. */
export const SETTINGS_NAV_ITEMS = [
  'Profiles',
  'Groups & benchmarks',
  'Providers',
  'Harnesses',
  'General',
  'Usage',
  'Favourites',
  'Agent hooks',
] as const

export type SettingsNavItemName = (typeof SETTINGS_NAV_ITEMS)[number]

export interface SettingsNavItem {
  /** Stable value passed to the selection callback. */
  id?: string
  /** `key` and `value` are accepted as aliases for `id` when composing a custom nav. */
  key?: string
  value?: string
  /** Text (or a custom node) shown in the sidebar. */
  label?: ReactNode
  /** `name` is accepted as an alias for `label`. */
  name?: ReactNode
  disabled?: boolean
}

export type SettingsNavEntry = SettingsNavItem | string

export interface SettingsNavProps {
  /** Custom entries; omitted entries use the eight settings pages. */
  items?: readonly SettingsNavEntry[]
  /** Selected entry id. Defaults to the first entry. */
  activeItem?: string
  /** Alias for `activeItem`. */
  active?: string
  /** Alias for `activeItem`. */
  value?: string
  /** Called with the selected entry id. */
  onSelect?: (item: string) => void
  /** Alias for `onSelect`. */
  onItemSelect?: (item: string) => void
  /** Alias for `onSelect`. */
  onChange?: (item: string) => void
  className?: string
  'aria-label'?: string
}

function entryDetails(entry: SettingsNavEntry, index: number): {
  id: string
  label: ReactNode
  disabled: boolean
} {
  if (typeof entry === 'string') {
    return { id: entry, label: entry, disabled: false }
  }

  const label = entry.label ?? entry.name ?? entry.id ?? entry.key ?? entry.value ?? `Item ${index + 1}`
  const id = entry.id ?? entry.key ?? entry.value ?? (typeof label === 'string' ? label : `item-${index}`)
  return { id, label, disabled: entry.disabled ?? false }
}

export function SettingsNav({
  items = SETTINGS_NAV_ITEMS,
  activeItem,
  active,
  value,
  onSelect,
  onItemSelect,
  onChange,
  className,
  'aria-label': ariaLabel = 'Settings navigation',
}: SettingsNavProps) {
  const selected = activeItem ?? active ?? value ?? entryDetails(items[0] ?? '', 0).id
  const select = onSelect ?? onItemSelect ?? onChange

  return (
    <nav className={cx(styles.nav, className)} aria-label={ariaLabel}>
      {items.map((entry, index) => {
        const item = entryDetails(entry, index)
        const isActive = item.id === selected
        return (
          <button
            key={item.id}
            type="button"
            className={cx(styles.navItem, isActive && styles.navItemActive)}
            aria-current={isActive ? 'page' : undefined}
            aria-pressed={isActive}
            disabled={item.disabled}
            onClick={() => select?.(item.id)}
          >
            <span className={styles.navIndicator} aria-hidden="true" />
            <span className={styles.navLabel}>{item.label}</span>
          </button>
        )
      })}
    </nav>
  )
}
