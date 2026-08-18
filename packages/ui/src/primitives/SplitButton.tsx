import type React from 'react'
import { cx } from '../utils/cx'
import styles from './SplitButton.module.css'

export interface SplitButtonMenuItem {
  key: string
  label: string
  selected: boolean
}

export interface SplitButtonProps {
  label: string // e.g. "Launch in Claude Code"
  onMain: () => void
  menuItems: SplitButtonMenuItem[]
  onPick: (key: string) => void
  open: boolean // controlled
  onOpenChange: (open: boolean) => void
}

export function SplitButton({
  label,
  onMain,
  menuItems,
  onPick,
  open,
  onOpenChange,
}: SplitButtonProps) {
  return (
    <span className={styles.root}>
      <span className={cx(styles.pill, open && styles.openBg)}>
        <span className={styles.label} onClick={onMain}>
          {label}
        </span>
        <span
          className={styles.chevron}
          onClick={() => onOpenChange(!open)}
          aria-expanded={open}
          role="button"
        >
          <svg
            width="9"
            height="9"
            viewBox="0 0 12 12"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.6"
            strokeLinecap="round"
          >
            <path d="M2.5 4.5 6 8l3.5-3.5"></path>
          </svg>
        </span>
      </span>
      {open && (
        <span className={styles.surface}>
          {menuItems.map((item) => (
            <span
              key={item.key}
              className={cx(styles.item, item.selected && styles.itemSelected)}
              onClick={() => onPick(item.key)}
            >
              {item.label}
            </span>
          ))}
        </span>
      )}
    </span>
  )
}