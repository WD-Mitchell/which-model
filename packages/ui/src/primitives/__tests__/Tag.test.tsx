import { render } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { Tag } from '../Tag'

describe('Tag', () => {
  it('maps each variant to its nocturne class', () => {
    const a = render(<Tag variant="accent">a</Tag>)
    const b = render(<Tag variant="neutral">b</Tag>)
    const c = render(<Tag variant="outline">c</Tag>)
    expect(a.container.querySelector('span')!.className).toContain('tag-accent')
    expect(b.container.querySelector('span')!.className).toContain('tag-neutral')
    expect(c.container.querySelector('span')!.className).toContain('tag-outline')
  })

  it('starts with the tag base class', () => {
    const { container } = render(<Tag variant="neutral">x</Tag>)
    expect(container.querySelector('span')!.className).toContain('tag')
  })
})