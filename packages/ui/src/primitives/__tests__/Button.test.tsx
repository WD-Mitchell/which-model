import { fireEvent, render } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { Button } from '../Button'
import styles from '../Button.module.css'

function btn(container: HTMLElement): HTMLButtonElement {
  const el = container.querySelector('button')
  if (!el) throw new Error('no button')
  return el
}

describe('Button', () => {
  it('applies the variant class', () => {
    const { container } = render(<Button variant="primary">Go</Button>)
    expect(btn(container).className).toContain('btn-primary')
  })

  it('applies each variant class', () => {
    const a = render(<Button variant="primary">1</Button>)
    const b = render(<Button variant="secondary">2</Button>)
    const c = render(<Button variant="ghost">3</Button>)
    expect(btn(a.container).className).toContain('btn-primary')
    expect(btn(b.container).className).toContain('btn-secondary')
    expect(btn(c.container).className).toContain('btn-ghost')
  })

  it('applies only the requested size class', () => {
    const md = render(<Button variant="ghost">md</Button>)
    const sm = render(
      <Button variant="ghost" size="sm">
        sm
      </Button>,
    )
    const xs = render(
      <Button variant="ghost" size="xs">
        xs
      </Button>,
    )
    expect(btn(md.container).classList.contains(styles.sm)).toBe(false)
    expect(btn(md.container).classList.contains(styles.xs)).toBe(false)
    expect(btn(sm.container).classList.contains(styles.sm)).toBe(true)
    expect(btn(sm.container).classList.contains(styles.xs)).toBe(false)
    expect(btn(xs.container).classList.contains(styles.xs)).toBe(true)
    expect(btn(xs.container).classList.contains(styles.sm)).toBe(false)
  })

  it('keeps the nocturne classes alongside a size class', () => {
    const { container } = render(
      <Button variant="primary" size="xs" className="extra">
        go
      </Button>,
    )
    const list = btn(container).classList
    expect(list.contains('btn')).toBe(true)
    expect(list.contains('btn-primary')).toBe(true)
    expect(list.contains(styles.xs)).toBe(true)
    expect(list.contains('extra')).toBe(true)
  })

  it('does not use native disabled by default', () => {
    const { container } = render(<Button variant="secondary">ok</Button>)
    expect(btn(container).disabled).toBe(false)
  })

  it('fires onClick', () => {
    const onClick = vi.fn()
    const { container } = render(<Button variant="secondary" onClick={onClick}>ok</Button>)
    fireEvent.click(btn(container))
    expect(onClick).toHaveBeenCalledTimes(1)
  })

  it('does not fire onClick when disabled', () => {
    const onClick = vi.fn()
    const { container } = render(
      <Button variant="primary" disabled onClick={onClick}>
        nope
      </Button>,
    )
    expect(btn(container).disabled).toBe(true)
    fireEvent.click(btn(container))
    expect(onClick).not.toHaveBeenCalled()
  })
})