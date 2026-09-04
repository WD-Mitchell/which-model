import { beforeEach, describe, expect, it } from 'vitest'
import { createMockEngineHost } from '@which-model/core/mock'
import { useOverridesStore } from './overrides'

describe('profile baseline reconciliation', () => {
 beforeEach(() => useOverridesStore.getState().clear())
 it('refreshes a clean draft without making it dirty during the refetch render', async () => {
  const profile = (await createMockEngineHost().profiles.get('planning'))
  const state = () => useOverridesStore.getState()
  state().seed(profile)
  const newer = { ...profile, core_share: 75 }
  expect(state().isDirty(newer)).toBe(false)
  state().reconcile(newer)
  expect(state().coreShare).toBe(75)
  expect(state().isDirty(newer)).toBe(false)
 })
 it('preserves dirty edits and reverts to the latest persisted profile', async () => {
  const profile = await createMockEngineHost().profiles.get('planning')
  const state = () => useOverridesStore.getState()
  state().seed(profile)
  state().setWeight('intelligence', 2)
  const newer = { ...profile, core_share: 75 }
  state().reconcile(newer)
  expect(state().tier1.intelligence).toBe(2)
  expect(state().coreShare).toBe(profile.core_share)
  expect(state().isDirty(newer)).toBe(true)
  state().revert(newer)
  expect(state().coreShare).toBe(75)
  expect(state().isDirty(newer)).toBe(false)
  state().reconcile({ ...newer, slug: 'another' })
  expect(state().baseSlug).toBe('another')
 })
})
