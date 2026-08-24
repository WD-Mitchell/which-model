import type React from 'react'
import { cx } from '../utils/cx'
import styles from './Button.module.css'

export interface ButtonProps {
  variant: 'primary' | 'secondary' | 'ghost'
  /** 'md' = nocturne metrics; 'sm' = mockup footer scale (12px / 5px 11px,
   *  demo.dc.html 219–220, 274); 'xs' = the inline ghost scale the settings
   *  pages use (11px / 2px 7px, demo.dc.html 330, 462, 539–540, 760–761). */
  size?: 'md' | 'sm' | 'xs'
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
      // U02 SPEC §2.5 — the look is nocturne's: `.btn` + `.btn-primary` /
      // `.btn-secondary` / `.btn-ghost` (transparent ground, accent text and
      // border for primary), never hand-rolled here, so retuning
      // theme/nocturne.css propagates to every caller.
      className={cx('btn', `btn-${variant}`, size !== 'md' && styles[size], className)}
      disabled={disabled}
      onClick={onClick}
    >
      {children}
    </button>
  )
}
