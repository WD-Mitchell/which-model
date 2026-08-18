import { act, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { TOAST_DURATION_MS, ToastProvider, useToast } from '../Toast'
import styles from '../Toast.module.css'

function Harness() {
  const toast = useToast()
  return <button onClick={() => toast.show('hello')}>show</button>
}

describe('Toast', () => {
  afterEach(() => {
    vi.useRealTimers()
  })

  it('renders a single toast on show', () => {
    vi.useFakeTimers()
    const { container } = render(
      <ToastProvider>
        <Harness />
      </ToastProvider>,
    )
    fireEvent.click(screen.getByText('show'))
    const nodes = container.querySelectorAll(`.${styles.toast}`)
    expect(nodes.length).toBe(1)
    expect(nodes[0].textContent).toBe('hello')
  })

  it('replaces the visible toast (still one node)', () => {
    vi.useFakeTimers()
    const { container } = render(
      <ToastProvider>
        <Harness />
      </ToastProvider>,
    )
    const show = screen.getByText('show')
    fireEvent.click(show)
    fireEvent.click(show)
    const nodes = container.querySelectorAll(`.${styles.toast}`)
    expect(nodes.length).toBe(1)
    expect(nodes[0].textContent).toBe('hello')
  })

  it('auto-dismisses after the 2.6s timer', () => {
    vi.useFakeTimers()
    const { container } = render(
      <ToastProvider>
        <Harness />
      </ToastProvider>,
    )
    fireEvent.click(screen.getByText('show'))
    expect(container.querySelector(`.${styles.toast}`)).not.toBeNull()

    act(() => {
      vi.advanceTimersByTime(TOAST_DURATION_MS)
    })
    expect(container.querySelector(`.${styles.toast}`)).toBeNull()
  })

  it('throws when used outside the provider', () => {
    expect(() => {
      render(
        <div>
          <Harness />
        </div>,
      )
    }).toThrow('useToast requires ToastProvider')
  })
})