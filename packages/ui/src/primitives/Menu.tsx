import { useEffect, useRef } from 'react'
import { cx } from '../utils/cx'
import styles from './Menu.module.css'

export interface MenuItem {
  key: string
  label?: string // required unless separator
  separator?: boolean
  dim?: boolean // 55%-text
  mono?: boolean // mono 11.5px rows
  selected?: boolean // accent bg + accent-200 text
}

export interface MenuProps {
  items: MenuItem[]
  onPick: (key: string) => void
  onClose: () => void // Escape or outside pointerdown
  className?: string // caller positions the surface
}

export function Menu({ items, onPick, onClose, className }: MenuProps) {
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    function onPointerDown(e: PointerEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) onClose()
    }
    function onKeyDown(e: KeyboardEvent) {
      if (e.key === 'Escape') {
        e.stopPropagation()
        onClose()
      }
    }
    window.addEventListener('pointerdown', onPointerDown)
    window.addEventListener('keydown', onKeyDown)
    return () => {
      window.removeEventListener('pointerdown', onPointerDown)
      window.removeEventListener('keydown', onKeyDown)
    }
  }, [onClose])

  return (
    <div ref={ref} role="menu" className={cx(styles.surface, className)}>
      {items.map((item) =>
        item.separator ? (
          <div key={item.key} role="separator" className={styles.separator} />
        ) : (
          <div
            key={item.key}
            role="menuitem"
            className={cx(
              styles.item,
              item.dim && styles.dim,
              item.mono && styles.mono,
              item.selected && styles.selected,
            )}
            onClick={() => onPick(item.key)}
          >
            {item.label}
          </div>
        ),
      )}
    </div>
  )
}