import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { Combobox } from '../Combobox'
import styles from '../Combobox.module.css'

const items = [
  { key: 'p1', label: 'main', sub: '60/40' },
  { key: 'p2', label: 'coding', sub: '30/70' },
]

function setup(overrides: Record<string, unknown> = {}) {
  const onQuery = vi.fn()
  const onOpenChange = vi.fn()
  const onPick = vi.fn()
  const utils = render(
    <Combobox
      items={items}
      query=""
      onQuery={onQuery}
      open={false}
      onOpenChange={onOpenChange}
      onPick={onPick}
      emptyText="no profile by that name"
      placeholder="type to find a profile"
      {...(overrides as never)}
    />,
  )
  return { ...utils, onQuery, onOpenChange, onPick }
}

describe('Combobox', () => {
  it('renders label and sub per row when open', () => {
    setup({ open: true })
    expect(screen.getByText('main')).not.toBeNull()
    expect(screen.getByText('60/40')).not.toBeNull()
    expect(screen.getByText('coding')).not.toBeNull()
    expect(screen.getByText('30/70')).not.toBeNull()
  })

  it('marks only the selected row (demo.dc.html 1053-1054 accent tint)', () => {
    const { container } = setup({ open: true, selectedKey: 'p2' })
    const rows = Array.from(container.querySelectorAll<HTMLElement>(`.${styles.row}`))
    expect(rows.length).toBe(2)
    expect(rows[0].classList.contains(styles.rowSelected)).toBe(false)
    expect(rows[1].classList.contains(styles.rowSelected)).toBe(true)
  })

  it('picks a row on click', () => {
    const { getByText, onPick } = setup({ open: true })
    fireEvent.click(getByText('main'))
    expect(onPick).toHaveBeenCalledWith('p1')
  })

  it('picks the first item on Enter', () => {
    const { container, onPick } = setup({ open: true })
    const input = container.querySelector('input')!
    fireEvent.keyDown(input, { key: 'Enter' })
    expect(onPick).toHaveBeenCalledWith('p1')
  })

  it('does nothing on Enter with no items', () => {
    const { container, onPick } = setup({ open: true, items: [] })
    const input = container.querySelector('input')!
    fireEvent.keyDown(input, { key: 'Enter' })
    expect(onPick).not.toHaveBeenCalled()
  })

  it('closes on Escape', () => {
    const { container, onOpenChange } = setup()
    const input = container.querySelector('input')!
    fireEvent.keyDown(input, { key: 'Escape' })
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })

  // Issue #31: the Combobox owns Escape — the event must not reach the
  // window-level dismiss listener (the settings shell closes the whole
  // window), matching Menu.tsx's stopPropagation convention.
  it('stops Escape from reaching window-level dismiss listeners', () => {
    const { container, onOpenChange } = setup()
    const input = container.querySelector('input')!
    const windowListener = vi.fn()
    window.addEventListener('keydown', windowListener)
    try {
      fireEvent.keyDown(input, { key: 'Escape' })
      expect(onOpenChange).toHaveBeenCalledWith(false)
      expect(windowListener).not.toHaveBeenCalled()
    } finally {
      window.removeEventListener('keydown', windowListener)
    }
  })

  it('shows emptyText when open with no items', () => {
    setup({ open: true, items: [] })
    expect(screen.getByText('no profile by that name')).not.toBeNull()
  })

  it('opens on pointer-down, not on bare focus', () => {
    const { container, onOpenChange } = setup({ open: false })
    const input = container.querySelector('input')!
    // Bare focus must NOT open: the popover auto-focuses this input on every
    // show, which used to pop the full list before any user interaction.
    fireEvent.focus(input)
    expect(onOpenChange).not.toHaveBeenCalled()
    fireEvent.pointerDown(input)
    expect(onOpenChange).toHaveBeenCalledWith(true)
  })

  it('forwards typing to onQuery', () => {
    const { container, onQuery } = setup()
    const input = container.querySelector('input')!
    fireEvent.change(input, { target: { value: 'cod' } })
    expect(onQuery).toHaveBeenCalledWith('cod')
  })
})
describe('Combobox click-away', () => {
  it('closes when a pointer-down lands outside the control', () => {
    const { onOpenChange } = setup({ open: true })
    fireEvent.pointerDown(document.body)
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })

  it('stays open for a pointer-down inside it', () => {
    const { container, onOpenChange } = setup({ open: true })
    const input = container.querySelector('input')!
    fireEvent.pointerDown(input)
    expect(onOpenChange).not.toHaveBeenCalledWith(false)
  })
})
