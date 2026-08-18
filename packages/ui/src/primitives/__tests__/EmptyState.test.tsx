import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { EmptyState } from '../EmptyState'

describe('EmptyState', () => {
  it('renders the muted text', () => {
    render(<EmptyState text="nothing here yet" />)
    expect(screen.getByText('nothing here yet')).not.toBeNull()
  })
})