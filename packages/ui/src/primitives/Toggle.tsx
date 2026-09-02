import type React from 'react'
import { cx } from '../utils/cx'

export interface ToggleProps {
  on: boolean
  disabled?: boolean // .sw.off + handler suppressed
  onToggle: (on: boolean) => void // called with !on
  'aria-label'?: string
}

export function Toggle({ on, disabled = false, onToggle, 'aria-label': ariaLabel }: ToggleProps) {
  function toggle() {
    if (!disabled) onToggle(!on)
  }

  function handleKey(e: React.KeyboardEvent<HTMLSpanElement>) {
    if (disabled) return
    if (e.key === ' ' || e.key === 'Enter') {
      e.preventDefault()
      onToggle(!on)
    }
  }

  return (
    <span
      role="switch"
      aria-checked={on}
      aria-label={ariaLabel}
      tabIndex={disabled ? -1 : 0}
      className={cx('sw', on && 'on', disabled && 'off')}
      onClick={toggle}
      onKeyDown={handleKey}
    >
      <i />
    </span>
  )
}