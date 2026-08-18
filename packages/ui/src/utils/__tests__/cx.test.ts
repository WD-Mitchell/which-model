import { describe, expect, it } from 'vitest'
import { cx } from '../cx'

describe('cx', () => {
  it('joins truthy string arguments with a single space', () => {
    expect(cx('a', false, 'b', undefined)).toBe('a b')
  })

  it('skips null and empty-falsy values', () => {
    expect(cx('x', null, 'y', undefined, 'z')).toBe('x y z')
  })

  it('returns an empty string for no arguments', () => {
    expect(cx()).toBe('')
    expect(cx(false, null, undefined)).toBe('')
  })
})