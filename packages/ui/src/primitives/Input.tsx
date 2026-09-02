import type React from 'react'
import { cx } from '../utils/cx'
import styles from './Input.module.css'

export interface InputProps {
  value: string
  onChange: (value: string) => void // string, not the event
  placeholder?: string
  mono?: boolean // default true (.wminput); false → font-body
  disabled?: boolean
  className?: string // on the .input wrapper
  type?: 'text' | 'password' | 'search'
  'aria-label'?: string
  onFocus?: () => void
  /** Commit-on-blur editors (the mockup's inline rename fields, demo.dc.html
   *  462) need the counterpart to onFocus. */
  onBlur?: () => void
  onKeyDown?: (e: React.KeyboardEvent<HTMLInputElement>) => void
}

export function Input({
  value,
  onChange,
  placeholder,
  mono = true,
  disabled = false,
  className,
  type = 'text',
  'aria-label': ariaLabel,
  onFocus,
  onBlur,
  onKeyDown,
}: InputProps) {
  return (
    <span className={cx('input', styles.wrap, className)}>
      <input
        className={cx('wminput', !mono && styles.body)}
        type={type}
        value={value}
        placeholder={placeholder}
        disabled={disabled}
        aria-label={ariaLabel}
        onChange={(e) => onChange(e.target.value)}
        onFocus={onFocus}
        onBlur={onBlur}
        onKeyDown={onKeyDown}
      />
    </span>
  )
}