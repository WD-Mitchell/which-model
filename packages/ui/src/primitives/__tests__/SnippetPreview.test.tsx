import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { SnippetPreview } from '../SnippetPreview'
import styles from '../SnippetPreview.module.css'

describe('SnippetPreview', () => {
  it('preserves newlines in the text', () => {
    render(<SnippetPreview text={'claude\n  --model gpt-5'} />)
    const pre = screen.getByText(/claude/) as HTMLElement
    expect(pre.tagName.toLowerCase()).toBe('pre')
    expect(pre.textContent).toBe('claude\n  --model gpt-5')
  })

  it('defaults to the tinted block variant', () => {
    render(<SnippetPreview text="abc" />)
    const el = screen.getByText('abc') as HTMLElement
    expect(el.classList.contains(styles.block)).toBe(true)
    expect(el.classList.contains(styles.command)).toBe(false)
    expect(el.classList.contains('input')).toBe(false)
  })

  it('composes mono + the nocturne input class for the command variant', () => {
    render(<SnippetPreview text="claude --model {model_id}" variant="command" />)
    const el = screen.getByText(/claude/) as HTMLElement
    expect(el.classList.contains('mono')).toBe(true)
    expect(el.classList.contains('input')).toBe(true)
    expect(el.classList.contains(styles.command)).toBe(true)
    expect(el.classList.contains(styles.block)).toBe(false)
  })

  it('fires onCopy on click when copyable', () => {
    const onCopy = vi.fn()
    render(<SnippetPreview text="abc" copyable onCopy={onCopy} />)
    fireEvent.click(screen.getByText('abc'))
    expect(onCopy).toHaveBeenCalledWith('abc')
  })

  it('does not fire onCopy when not copyable', () => {
    const onCopy = vi.fn()
    render(<SnippetPreview text="abc" onCopy={onCopy} />)
    fireEvent.click(screen.getByText('abc'))
    expect(onCopy).not.toHaveBeenCalled()
  })
})