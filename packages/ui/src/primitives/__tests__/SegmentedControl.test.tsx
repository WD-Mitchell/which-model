import { fireEvent, render } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { SegmentedControl } from '../SegmentedControl'

const options = [
  { value: 'a', label: 'Alpha' },
  { value: 'b', label: 'Beta' },
]

describe('SegmentedControl', () => {
  it('renders one seg-opt per option with a hidden radio', () => {
    const { container } = render(
      <SegmentedControl options={options} value="a" onChange={() => {}} />,
    )
    const opts = container.querySelectorAll('.seg-opt')
    expect(opts.length).toBe(2)
    opts.forEach((opt) => {
      const input = opt.querySelector('input')
      expect(input).not.toBeNull()
      expect((input as HTMLInputElement).type).toBe('radio')
    })
  })

  it('checks the active option radio', () => {
    const { container } = render(
      <SegmentedControl options={options} value="b" onChange={() => {}} />,
    )
    const radios = Array.from(container.querySelectorAll<HTMLInputElement>('.seg-opt input'))
    expect(radios[0].checked).toBe(false)
    expect(radios[1].checked).toBe(true)
  })

  it('fires onChange when clicking an unselected option', () => {
    const onChange = vi.fn()
    const { container } = render(
      <SegmentedControl options={options} value="a" onChange={onChange} />,
    )
    const opts = container.querySelectorAll('.seg-opt')
    fireEvent.click(opts[1])
    expect(onChange).toHaveBeenCalledTimes(1)
    expect(onChange).toHaveBeenCalledWith('b')
  })

  it('fires nothing when clicking the selected option', () => {
    const onChange = vi.fn()
    const { container } = render(
      <SegmentedControl options={options} value="a" onChange={onChange} />,
    )
    const opts = container.querySelectorAll('.seg-opt')
    fireEvent.click(opts[0])
    expect(onChange).not.toHaveBeenCalled()
  })
})