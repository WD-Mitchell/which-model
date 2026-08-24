import { render } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { Tag } from '../Tag'
import styles from '../Tag.module.css'

function tag(container: HTMLElement): HTMLSpanElement {
  const el = container.querySelector('span')
  if (!el) throw new Error('no tag')
  return el
}

describe('Tag', () => {
  it('maps each variant to its nocturne class', () => {
    const a = render(<Tag variant="accent">a</Tag>)
    const b = render(<Tag variant="neutral">b</Tag>)
    const c = render(<Tag variant="outline">c</Tag>)
    const d = render(<Tag variant="accent-2">d</Tag>)
    expect(tag(a.container).className).toContain('tag-accent')
    expect(tag(b.container).className).toContain('tag-neutral')
    expect(tag(c.container).className).toContain('tag-outline')
    expect(tag(d.container).className).toContain('tag-accent-2')
  })

  it('starts with the tag base class', () => {
    const { container } = render(<Tag variant="neutral">x</Tag>)
    expect(tag(container).className).toContain('tag')
  })

  it('is mono by default and drops mono when asked', () => {
    const on = render(<Tag variant="neutral">x</Tag>)
    const off = render(
      <Tag variant="neutral" mono={false}>
        x
      </Tag>,
    )
    expect(on.container.querySelector('span.mono')).not.toBeNull()
    expect(off.container.querySelector('span.mono')).toBeNull()
  })

  it('applies the badge scale by default and the chip scale on request', () => {
    const badge = render(<Tag variant="neutral">x</Tag>)
    const chip = render(
      <Tag variant="accent" size="chip">
        y
      </Tag>,
    )
    expect(tag(badge.container).classList.contains(styles.badge)).toBe(true)
    expect(tag(chip.container).classList.contains(styles.chip)).toBe(true)
    expect(tag(chip.container).classList.contains(styles.badge)).toBe(false)
  })

  it('leaves nocturne standalone metrics alone at size md', () => {
    const { container } = render(
      <Tag variant="accent" size="md">
        z
      </Tag>,
    )
    const list = tag(container).classList
    expect(list.contains('tag-accent')).toBe(true)
    expect(list.contains(styles.badge)).toBe(false)
    expect(list.contains(styles.chip)).toBe(false)
  })
})
