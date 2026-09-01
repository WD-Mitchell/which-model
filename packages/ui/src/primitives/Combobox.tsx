import type React from 'react'
import { useEffect, useRef } from 'react'
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
  onOpenChange: (open: boolean) => void // click/typing → true; Escape → false
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
  const rootRef = useRef<HTMLDivElement>(null)

  // Click-away closes. The list is not focus-managed (the input keeps focus),
  // so a blur handler would fire on every scroll and selection instead.
  useEffect(() => {
    if (!open) return
    function onPointerDown(e: PointerEvent) {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) onOpenChange(false)
    }
    window.addEventListener('pointerdown', onPointerDown)
    return () => window.removeEventListener('pointerdown', onPointerDown)
  }, [open, onOpenChange])

  function handleKeyDown(e: React.KeyboardEvent<HTMLInputElement>) {
    if (e.key === 'Enter') {
      if (items.length > 0) onPick(items[0].key)
    } else if (e.key === 'Escape') {
      // The Combobox owns Escape while its list is open: close the list and
      // stop the event from reaching window-level dismiss listeners (the
      // settings shell closes the whole window otherwise — Menu.tsx does the
      // same). preventDefault so React marks it consumed for native menus.
      e.stopPropagation()
      e.preventDefault()
      onOpenChange(false)
    }
  }

  return (
    // Deliberate interaction opens the list: a pointer-down on the field, or
    // typing (the consumer's onQuery opens it). NOT bare focus — the popover
    // window auto-focuses this input every time it is shown, and focus-opens
    // meant the full profile list popped up before the user touched anything.
    <div ref={rootRef} className={styles.root} onPointerDown={() => onOpenChange(true)}>
      <Input
        value={query}
        onChange={onQuery}
        placeholder={placeholder}
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