import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { SplitButton } from '../SplitButton'
import styles from '../SplitButton.module.css'

export const menu = [
  { key: 'claude', label: 'Claude Code', selected: true },
  { key: 'codex', label: 'Codex', selected: false },
]

function setup(overrides: Record<string, unknown> = {}) {
  const onMain = vi.fn()
  const onPick = vi.fn()
  const onOpenChange = vi.fn()
  const utils = render(
    <SplitButton
      label="Launch in Claude Code"
      onMain={onMain}
      menuItems={menu}
      onPick={onPick}
      open={false}
      onOpenChange={onOpenChange}
      {...(overrides as never)}
    />,
  )
  return { ...utils, onMain, onPick, onOpenChange }
}

describe('SplitButton', () => {
  it('fires onMain from the label segment', () => {
    const { getByText, onMain } = setup()
    fireEvent.click(getByText('Launch in Claude Code'))
    expect(onMain).toHaveBeenCalledTimes(1)
  })

  it('opens the menu via the chevron', () => {
    const { getByRole, onOpenChange } = setup()
    fireEvent.click(getByRole('button'))
    expect(onOpenChange).toHaveBeenCalledWith(true)
  })

  it('picks an item when open', () => {
    const { getByText, onPick } = setup({ open: true })
    fireEvent.click(getByText('Codex'))
    expect(onPick).toHaveBeenCalledWith('codex')
  })

  it('marks the selected item with the accent class', () => {
    const { getByText } = setup({ open: true })
    const selected = getByText('Claude Code')
    expect(selected.className).toContain(styles.itemSelected)
  })

  it('does not fire onMain or onOpenChange when disabled', () => {
    const { getByText, getByRole, onMain, onOpenChange } = setup({ disabled: true })
    fireEvent.click(getByText('Launch in Claude Code'))
    expect(onMain).not.toHaveBeenCalled()
    fireEvent.click(getByRole('button'))
    expect(onOpenChange).not.toHaveBeenCalled()
  })
})