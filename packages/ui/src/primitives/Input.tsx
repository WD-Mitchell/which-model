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
  onFocus?: () => void
  onKeyDown?: (e: React.KeyboardEvent<HTMLInputElement>) => void
}

export function Input({
  value,
  onChange,
  placeholder,
  mono = true,
  disabled = false,
  className,
  onFocus,
  onKeyDown,
}: InputProps) {
  return (
    <span className={cx('input', styles.wrap, className)}>
      <input
        className={cx('wminput', !mono && styles.body)}
        value={value}
        placeholder={placeholder}
        disabled={disabled}
        onChange={(e) => onChange(e.target.value)}
        onFocus={onFocus}
        onKeyDown={onKeyDown}
      />
    </span>
  )
}