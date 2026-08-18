import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { Combobox } from '../Combobox'

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

  it('shows emptyText when open with no items', () => {
    setup({ open: true, items: [] })
    expect(screen.getByText('no profile by that name')).not.toBeNull()
  })

  it('opens on focus', () => {
    const { container, onOpenChange } = setup()
    const input = container.querySelector('input')!
    fireEvent.focus(input)
    expect(onOpenChange).toHaveBeenCalledWith(true)
  })

  it('forwards typing to onQuery', () => {
    const { container, onQuery } = setup()
    const input = container.querySelector('input')!
    fireEvent.change(input, { target: { value: 'cod' } })
    expect(onQuery).toHaveBeenCalledWith('cod')
  })
})