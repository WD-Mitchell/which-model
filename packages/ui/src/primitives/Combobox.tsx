import type React from 'react'
import { cx } from '../utils/cx'
import { Input } from './Input'
import styles from './Combobox.module.css'

export interface ComboboxItem {
  key: string
  label: string
  sub: string
}

export interface ComboboxProps {
  items: ComboboxItem[]
  query: string
  onQuery: (query: string) => void
  open: boolean
  onOpenChange: (open: boolean) => void // focus → true; Escape → false
  onPick: (key: string) => void // click a row, or Enter → items[0]
  emptyText: string // shown when open && items.length === 0
  placeholder?: string
  selectedKey?: string // row highlighted accent
}

export function Combobox({
  items,
  query,
  onQuery,
  open,
  onOpenChange,
  onPick,
  emptyText,
  placeholder,
  selectedKey,
}: ComboboxProps) {
  function handleKeyDown(e: React.KeyboardEvent<HTMLInputElement>) {
    if (e.key === 'Enter') {
      if (items.length > 0) onPick(items[0].key)
    } else if (e.key === 'Escape') {
      onOpenChange(false)
    }
  }

  return (
    <div className={styles.root}>
      <Input
        value={query}
        onChange={onQuery}
        placeholder={placeholder}
        onFocus={() => onOpenChange(true)}
        onKeyDown={handleKeyDown}
      />
      {open && (
        <div className={styles.surface}>
          {items.length === 0 ? (
            <div className={styles.empty}>{emptyText}</div>
          ) : (
            items.map((item) => (
              <div
                key={item.key}
                className={cx(styles.row, item.key === selectedKey && styles.rowSelected)}
                onClick={() => onPick(item.key)}
              >
                <span className={styles.rowLabel}>{item.label}</span>
                <span className={styles.rowSub}>{item.sub}</span>
              </div>
            ))
          )}
        </div>
      )}
    </div>
  )
}