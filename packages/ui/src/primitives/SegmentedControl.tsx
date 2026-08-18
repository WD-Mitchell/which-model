import { useId } from 'react'
import { cx } from '../utils/cx'
import styles from './SegmentedControl.module.css'

export interface SegmentedOption {
  value: string
  label: string
}

export interface SegmentedControlProps {
  options: SegmentedOption[]
  value: string
  onChange: (value: string) => void // fires only for a different value
  className?: string
}

export function SegmentedControl({ options, value, onChange, className }: SegmentedControlProps) {
  const name = useId()
  return (
    <div className={cx('seg', className)} role="radiogroup">
      {options.map((option) => (
        <label key={option.value} className={cx('seg-opt', styles.opt)}>
          <input
            type="radio"
            name={name}
            value={option.value}
            checked={value === option.value}
            onChange={() => {
              if (value !== option.value) onChange(option.value)
            }}
          />
          {option.label}
        </label>
      ))}
    </div>
  )
}