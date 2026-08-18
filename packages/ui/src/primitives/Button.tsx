import type React from 'react'
import { cx } from '../utils/cx'
import styles from './Button.module.css'

export interface ButtonProps {
  variant: 'primary' | 'secondary' | 'ghost'
  size?: 'md' | 'sm' // default 'md' (nocturne metrics); 'sm' = mockup compact
  disabled?: boolean
  onClick?: () => void
  className?: string // merged last via cx
  children: React.ReactNode
}

export function Button({
  variant,
  size = 'md',
  disabled = false,
  onClick,
  className,
  children,
}: ButtonProps) {
  return (
    <button
      type="button"
      className={cx('btn', `btn-${variant}`, size === 'sm' && styles.sm, className)}
      disabled={disabled}
      onClick={onClick}
    >
      {children}
    </button>
  )
}