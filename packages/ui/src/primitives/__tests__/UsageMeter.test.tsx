import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { UsageMeter } from '../UsageMeter'
import styles from '../UsageMeter.module.css'

function fill(container: HTMLElement): HTMLElement {
  const el = container.querySelector(`.${styles.track} > .${styles.fill}`)
  if (!el) throw new Error('no fill')
  return el as HTMLElement
}

describe('UsageMeter', () => {
  it('renders the percent text and a cool fill below 70', () => {
    const { container } = render(<UsageMeter label="cost" percent={62} />)
    expect(screen.getByText('62%')).not.toBeNull()
    const f = fill(container)
    expect(f.style.width).toBe('62%')
    expect(f.className).toContain(styles.fillCool)
  })

  it('uses the hot fill at 70 and above', () => {
    const { container } = render(<UsageMeter label="cost" percent={70} />)
    expect(fill(container).className).toContain(styles.fillHot)
  })

  it('renders a dash and grey 0-width fill for null percent', () => {
    const { container } = render(<UsageMeter label="cost" percent={null} />)
    expect(screen.getByText('—')).not.toBeNull()
    const f = fill(container)
    expect(f.style.width).toBe('0%')
    expect(f.className).toContain(styles.fillNull)
  })

  it('forces the hot fill when hot is set with a low percent', () => {
    const { container } = render(<UsageMeter label="cost" percent={12} hot />)
    expect(fill(container).className).toContain(styles.fillHot)
  })

  it('keeps the displayed percent even when clamping width', () => {
    const { container } = render(<UsageMeter label="cost" percent={130} />)
    expect(screen.getByText('130%')).not.toBeNull()
    expect(fill(container).style.width).toBe('100%')
  })
})