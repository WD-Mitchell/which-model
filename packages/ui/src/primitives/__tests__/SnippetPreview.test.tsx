import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { SnippetPreview } from '../SnippetPreview'

describe('SnippetPreview', () => {
  it('preserves newlines in the text', () => {
    render(<SnippetPreview text={'claude\n  --model gpt-5'} />)
    const pre = screen.getByText(/claude/) as HTMLElement
    expect(pre.tagName.toLowerCase()).toBe('pre')
    expect(pre.textContent).toBe('claude\n  --model gpt-5')
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