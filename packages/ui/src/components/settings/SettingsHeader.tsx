import type { ReactNode } from 'react'
import { cx } from '../../utils/cx'
import styles from './settings.module.css'

export interface SettingsHeaderAction {
  label: string
  onAction?: () => void
  onClick?: () => void
  disabled?: boolean
}

export type SettingsHeaderActionSlot = ReactNode | SettingsHeaderAction

export interface SettingsHeaderProps {
  title: ReactNode
  /** Supporting copy below the title. */
  description?: ReactNode
  /** `blurb` is an alias used by the settings page contracts. */
  blurb?: ReactNode
  /** One or more already-rendered action buttons. */
  actions?: ReactNode
  /** A convenience action object or a rendered action node. */
  action?: SettingsHeaderActionSlot
  /** Convenience props for a single action button. */
  actionLabel?: string
  onAction?: () => void
  className?: string
}

function isActionObject(action: SettingsHeaderActionSlot): action is SettingsHeaderAction {
  if (typeof action !== 'object' || action === null || !('label' in action)) return false
  return typeof action.label === 'string'
}

function renderAction(action: SettingsHeaderActionSlot) {
  if (!isActionObject(action)) return action
  return (
    <button
      type="button"
      className={styles.headerAction}
      disabled={action.disabled}
      onClick={action.onAction ?? action.onClick}
    >
      {action.label}
    </button>
  )
}

export function SettingsHeader({
  title,
  description,
  blurb,
  actions,
  action,
  actionLabel,
  onAction,
  className,
}: SettingsHeaderProps) {
  const supportingCopy = description ?? blurb
  const convenienceAction = actionLabel
    ? { label: actionLabel, onAction }
    : undefined
  const actionSlot = action ?? convenienceAction

  return (
    <header className={cx(styles.header, className)}>
      <div className={styles.headerCopy}>
        <h1 className={styles.headerTitle}>{title}</h1>
        {supportingCopy !== undefined && <p className={styles.headerDescription}>{supportingCopy}</p>}
      </div>
      {(actions !== undefined || actionSlot !== undefined) && (
        <div className={styles.headerActions}>
          {actions}
          {actionSlot !== undefined && renderAction(actionSlot)}
        </div>
      )}
    </header>
  )
}
