import { useState } from 'react'
import type { ReactNode } from 'react'
import styles from './Tooltip.module.css'

export interface TooltipProps {
  content: ReactNode // e.g. "software_engineering  4 / 5"
  children: ReactNode // hover anchor
}

export function Tooltip({ content, children }: TooltipProps) {
  const [shown, setShown] = useState(false)
  return (
    <span className={styles.wrap} onMouseEnter={() => setShown(true)} onMouseLeave={() => setShown(false)}>
      {children}
      {shown && <span className={styles.surface}>{content}</span>}
    </span>
  )
}