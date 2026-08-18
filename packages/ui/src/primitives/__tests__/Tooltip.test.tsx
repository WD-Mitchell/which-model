import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { Tooltip } from '../Tooltip'

describe('Tooltip', () => {
  it('is hidden by default', () => {
    render(<Tooltip content="tip text">anchor</Tooltip>)
    expect(screen.queryByText('tip text')).toBeNull()
    expect(screen.getByText('anchor')).not.toBeNull()
  })

  it('shows content on mouseenter and hides on mouseleave', () => {
    render(<Tooltip content="tip text">anchor</Tooltip>)
    const anchor = screen.getByText('anchor')
    fireEvent.mouseEnter(anchor)
    expect(screen.getByText('tip text')).not.toBeNull()
    fireEvent.mouseLeave(anchor)
    expect(screen.queryByText('tip text')).toBeNull()
  })
})