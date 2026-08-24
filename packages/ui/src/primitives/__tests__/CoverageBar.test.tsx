import { render } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { CoverageBar } from '../CoverageBar'
import styles from '../CoverageBar.module.css'

function fill(container: HTMLElement): HTMLElement {
  const el = container.querySelector(`.${styles.fill}`)
  if (!el) throw new Error('no fill')
  return el as HTMLElement
}

describe('CoverageBar', () => {
  it('sizes the fill to the rounded percentage', () => {
    const { container } = render(<CoverageBar covered={3} total={4} />)
    expect((fill(container).style.width)).toBe('75%')
  })

  it('renders a 0-width fill when total is 0', () => {
    const { container } = render(<CoverageBar covered={3} total={0} />)
    expect((fill(container).style.width)).toBe('0%')
  })

  it('renders a 0-width fill when total is negative', () => {
    const { container } = render(<CoverageBar covered={3} total={-2} />)
    expect((fill(container).style.width)).toBe('0%')
  })
})