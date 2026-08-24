import type { ReactNode } from 'react'
import { cx } from '../utils/cx'
import styles from './Table.module.css'

export interface TableColumn {
  key: string
  label: string
  width?: string // e.g. '124px'; absent → flex:1
  align?: 'left' | 'center' | 'right' // default 'left'
  sortable?: boolean
}

export interface TableSort {
  key: string
  dir: 'asc' | 'desc'
}

export interface TableProps {
  columns: TableColumn[]
  sort: TableSort | null
  onSort: (sort: TableSort) => void // first activation desc, then toggle
  rows: (sort: TableSort | null) => ReactNode // body render-prop; caller sorts
  className?: string
}

export function Table({ columns, sort, onSort, rows, className }: TableProps) {
  return (
    <div className={cx(className)}>
      <div className={styles.headerRow}>
        {columns.map((column) => {
          const active = sort !== null && sort.key === column.key
          const dir = active ? sort.dir : null
          const suffix = active ? (dir === 'desc' ? '  ↓' : '  ↑') : ''
          return (
            <span
              key={column.key}
              className={cx(
                styles.headerCell,
                styles[`align${column.align ?? 'left'}`],
                column.sortable && styles.sortable,
                active && styles.active,
              )}
              style={column.width ? { flex: 'none', width: column.width } : { flex: 1 }}
              onClick={
                column.sortable
                  ? () => onSort({ key: column.key, dir: dir === 'desc' ? 'asc' : 'desc' })
                  : undefined
              }
            >
              {column.label}
              {suffix}
            </span>
          )
        })}
      </div>
      {rows(sort)}
    </div>
  )
}