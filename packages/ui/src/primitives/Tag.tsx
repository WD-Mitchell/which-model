import type React from 'react'
import { cx } from '../utils/cx'
import styles from './Tag.module.css'

export interface TagProps {
  /** Maps 1:1 onto nocturne's `.tag-accent` / `.tag-accent-2` /
   *  `.tag-neutral` / `.tag-outline`. */
  variant: 'accent' | 'accent-2' | 'neutral' | 'outline'
  /** 'badge' = 8.5px / 0 5px (inline badges), 'chip' = 9.5px / 1px 7px,
   *  'md' = nocturne's own standalone scale (11px / 3px 10px). */
  size?: 'badge' | 'chip' | 'md'
  /** The mockup writes every tag `class="mono tag tag-*"` (demo.dc.html 327,
   *  404, 443, 459, 504, 584, 776), so mono is the default. */
  mono?: boolean
  onClick?: () => void
  className?: string
  children: React.ReactNode
}

export function Tag({
  variant,
  size = 'badge',
  mono = true,
  onClick,
  className,
  children,
}: TagProps) {
  return (
    <span
      className={cx(
        mono && 'mono',
        'tag',
        `tag-${variant}`,
        size !== 'md' && styles[size],
        className,
      )}
      onClick={onClick}
    >
      {children}
    </span>
  )
}
