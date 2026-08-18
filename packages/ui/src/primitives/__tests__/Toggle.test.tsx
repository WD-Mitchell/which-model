import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { Toggle } from '../Toggle'

function sw() {
  const el = screen.getByRole('switch') as HTMLElement
  return el
}

describe('Toggle', () => {
  it('applies the on class when on', () => {
    render(<Toggle on onToggle={() => {}} />)
    expect(sw().className).toContain('on')
    expect(screen.getByRole('switch').getAttribute('aria-checked')).toBe('true')
  })

  it('omits the on class when off', () => {
    render(<Toggle on={false} onToggle={() => {}} />)
    expect(sw().className).not.toContain('on')
    expect(screen.getByRole('switch').getAttribute('aria-checked')).toBe('false')
  })

  it('fires onToggle with the inverted value on click', () => {
    const onToggle = vi.fn()
    render(<Toggle on={false} onToggle={onToggle} />)
    fireEvent.click(sw())
    expect(onToggle).toHaveBeenCalledWith(true)
  })

  it('applies off class and suppresses the handler when disabled', () => {
    const onToggle = vi.fn()
    render(<Toggle on onToggle={onToggle} disabled />)
    expect(sw().className).toContain('off')
    fireEvent.click(sw())
    expect(onToggle).not.toHaveBeenCalled()
  })

  it('toggles on Space', () => {
    const onToggle = vi.fn()
    render(<Toggle on onToggle={onToggle} />)
    fireEvent.keyDown(sw(), { key: ' ' })
    expect(onToggle).toHaveBeenCalledWith(false)
  })

  it('toggles on Enter', () => {
    const onToggle = vi.fn()
    render(<Toggle on={false} onToggle={onToggle} />)
    fireEvent.keyDown(sw(), { key: 'Enter' })
    expect(onToggle).toHaveBeenCalledWith(true)
  })
})