import { fireEvent, render } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { Table } from '../Table'
import type { TableSort } from '../Table'

const columns = [
  { key: 'name', label: 'model', sortable: true },
  { key: 'score', label: 'score', width: '124px', sortable: true },
  { key: 'note', label: 'note' },
]

function headerCells(container: HTMLElement) {
  return Array.from(container.querySelectorAll('span')).filter((s) =>
    ['model', 'score', 'note'].some((l) => s.textContent?.includes(l) && s.textContent!.length < 30),
  )
}

function setup(sort: TableSort | null = null) {
  const onSort = vi.fn()
  const rows = vi.fn(() => <div>body</div>)
  const utils = render(<Table columns={columns} sort={sort} onSort={onSort} rows={rows} />)
  return { ...utils, onSort, rows }
}

describe('Table', () => {
  it('fires onSort desc on first sortable activation', () => {
    const { container, onSort } = setup(null)
    fireEvent.click(headerCells(container)[0])
    expect(onSort).toHaveBeenCalledWith({ key: 'name', dir: 'desc' })
  })

  it('appends ↓ on the active desc column and ↑ after toggle', () => {
    const first = setup({ key: 'name', dir: 'desc' })
    expect(headerCells(first.container)[0].textContent).toContain('↓')

    const second = setup({ key: 'name', dir: 'asc' })
    expect(headerCells(second.container)[0].textContent).toContain('↑')
  })

  it('toggles desc→asc on a second sortable click', () => {
    const { container, onSort, rerender } = setup({ key: 'name', dir: 'desc' })
    fireEvent.click(headerCells(container)[0])
    expect(onSort).toHaveBeenCalledWith({ key: 'name', dir: 'asc' })
    void rerender
  })

  it('fires nothing for a non-sortable header', () => {
    const { container, onSort } = setup(null)
    const note = headerCells(container).find((s) => s.textContent === 'note')
    expect(note).toBeDefined()
    fireEvent.click(note!)
    expect(onSort).not.toHaveBeenCalled()
  })

  it('calls the rows render prop with the sort prop', () => {
    const { rows } = setup({ key: 'score', dir: 'desc' })
    expect(rows).toHaveBeenCalledWith({ key: 'score', dir: 'desc' })
  })
})