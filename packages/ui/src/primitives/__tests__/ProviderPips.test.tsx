import { render } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { ProviderPips } from '../ProviderPips'
import styles from '../ProviderPips.module.css'

describe('ProviderPips', () => {
  it('renders a dot per state', () => {
    const { container } = render(<ProviderPips states={[true, false, true]} />)
    expect(container.querySelectorAll(`.${styles.pip}`).length).toBe(3)
  })

  it('marks true dots accent and false dots neutral', () => {
    const { container } = render(<ProviderPips states={[true, false]} />)
    const dots = Array.from(container.querySelectorAll<HTMLElement>(`.${styles.pip}`))
    expect(dots[0].className).toContain(styles.on)
    expect(dots[1].className).not.toContain(styles.on)
  })
})