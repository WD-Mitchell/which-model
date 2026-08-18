import { fireEvent, render } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { Input } from '../Input'
import styles from '../Input.module.css'

describe('Input', () => {
  it('fires onChange with the string value on typing', () => {
    const onChange = vi.fn()
    const { container } = render(<Input value="" onChange={onChange} />)
    const input = container.querySelector('input')!
    fireEvent.change(input, { target: { value: 'text' } })
    expect(onChange).toHaveBeenCalledWith('text')
  })

  it('renders the placeholder', () => {
    const { container } = render(
      <Input value="" onChange={() => {}} placeholder="type to find" />,
    )
    expect(container.querySelector('input')!.getAttribute('placeholder')).toBe('type to find')
  })

  it('uses wminput and is mono by default', () => {
    const { container } = render(<Input value="" onChange={() => {}} />)
    const input = container.querySelector('input')!
    expect(input.className).toContain('wminput')
    expect(input.className).not.toContain(styles.body)
  })

  it('swaps to the body font class when mono is false', () => {
    const { container } = render(<Input value="" onChange={() => {}} mono={false} />)
    const input = container.querySelector('input')!
    expect(input.className).toContain(styles.body)
  })

  it('forwards focus and keydown', () => {
    const onFocus = vi.fn()
    const onKeyDown = vi.fn()
    const { container } = render(
      <Input value="" onChange={() => {}} onFocus={onFocus} onKeyDown={onKeyDown} />,
    )
    const input = container.querySelector('input')!
    fireEvent.focus(input)
    expect(onFocus).toHaveBeenCalledTimes(1)
    fireEvent.keyDown(input, { key: 'Enter' })
    expect(onKeyDown).toHaveBeenCalledTimes(1)
  })
})