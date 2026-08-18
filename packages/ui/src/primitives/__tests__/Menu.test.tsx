import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { Menu } from '../Menu'

const items = [
  { key: 'custom', label: 'Custom weights…' },
  { key: 'settings', label: 'Settings…' },
  { key: 'sep', separator: true },
  { key: 'quit', label: 'Quit which-model', dim: true },
]

describe('Menu', () => {
  it('renders items and separator', () => {
    render(<Menu items={items} onPick={() => {}} onClose={() => {}} />)
    expect(screen.getByText('Custom weights…')).not.toBeNull()
    expect(screen.getByText('Quit which-model')).not.toBeNull()
    expect(screen.getAllByRole('menuitem').length).toBe(3)
    expect(screen.getAllByRole('separator').length).toBe(1)
  })

  it('fires onPick on item click', () => {
    const onPick = vi.fn()
    render(<Menu items={items} onPick={onPick} onClose={() => {}} />)
    fireEvent.click(screen.getByText('Settings…'))
    expect(onPick).toHaveBeenCalledWith('settings')
  })

  it('closes on Escape', () => {
    const onClose = vi.fn()
    render(<Menu items={items} onPick={() => {}} onClose={onClose} />)
    fireEvent.keyDown(window, { key: 'Escape' })
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('closes on pointerdown outside the surface', () => {
    const onClose = vi.fn()
    render(<Menu items={items} onPick={() => {}} onClose={onClose} />)
    fireEvent.pointerDown(document.body)
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('does not close on pointerdown inside the surface', () => {
    const onClose = vi.fn()
    render(<Menu items={items} onPick={() => {}} onClose={onClose} />)
    fireEvent.pointerDown(screen.getByText('Custom weights…'))
    expect(onClose).not.toHaveBeenCalled()
  })
})