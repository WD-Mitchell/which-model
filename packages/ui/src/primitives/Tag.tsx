import type React from 'react'
import { cx } from '../utils/cx'
import styles from './Tag.module.css'

export interface TagProps {
  variant: 'accent' | 'neutral' | 'outline'
  size?: 'badge' | 'chip' // badge = 8.5px/0 5px, chip = 9.5px/1px 7px
  onClick?: () => void
  className?: string
  children: React.ReactNode
}

export function Tag({ variant, size = 'badge', onClick, className, children }: TagProps) {
  return (
    <span
      className={cx('tag', `tag-${variant}`, size === 'badge' ? styles.badge : styles.chip, className)}
      onClick={onClick}
    >
      {children}
    </span>
  )
}