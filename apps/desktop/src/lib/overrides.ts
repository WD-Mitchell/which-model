import { create } from 'zustand'
import type { ProfileDetail } from '@which-model/core'

// U06 §2.2 — ephemeral overrides store. A decomposed ProfileDetail that never
// persists: the labels identify the base profile, the weights/balance hold
// in-memory edits, and `isDirty` decides whether useRank sends overrides.
// U05 seeds it from the active profile (clear + seed on profile change); U06
// re-seeds on weights-view mount and mutates via the actions below.

export const CORE_KEYS = ['intelligence', 'cost', 'speed'] as const

export type WeightMap = Record<string, number>

interface OverridesState {
  baseSlug: string
  coreShare: number
  tier1: WeightMap
  tier2: WeightMap
  seed(profile: ProfileDetail): void
  setWeight(key: string, v: number): void
  addMetric(key: string): void
  removeMetric(key: string): void
  setCoreShare(v: number): void
  revert(profile: ProfileDetail): void
  clear(): void
  isDirty(profile: ProfileDetail): boolean
}

const EMPTY = { baseSlug: '', coreShare: 0, tier1: {}, tier2: {} }

function tierOf(key: string): 'tier1' | 'tier2' {
  return (CORE_KEYS as readonly string[]).includes(key) ? 'tier1' : 'tier2'
}

function mapEqual(a: WeightMap, b: WeightMap): boolean {
  const ak = Object.keys(a)
  const bk = Object.keys(b)
  if (ak.length !== bk.length) return false
  for (const k of ak) if (a[k] !== b[k]) return false
  return true
}

const clampWeight = (v: number): number => Math.max(0, Math.min(5, Math.round(v)))
const clampShare = (v: number): number =>
  Math.max(10, Math.min(90, Math.round(v / 5) * 5))

export const useOverridesStore = create<OverridesState>((set, get) => ({
  ...EMPTY,

  seed(profile) {
    set({
      baseSlug: profile.slug,
      coreShare: profile.core_share,
      tier1: { ...profile.tier1_weights },
      tier2: { ...profile.tier2_weights },
    })
  },

  setWeight(key, v) {
    const value = clampWeight(v)
    const tier = tierOf(key)
    set((s) => {
      const map = { ...s[tier] }
      if (value === 0) delete map[key]
      else map[key] = value
      return { [tier]: map } as Partial<OverridesState>
    })
  },

  addMetric(key) {
    const tier = tierOf(key)
    set((s) => ({ [tier]: { ...s[tier], [key]: 3 } }) as Partial<OverridesState>)
  },

  removeMetric(key) {
    const tier = tierOf(key)
    set((s) => {
      const map = { ...s[tier] }
      delete map[key]
      return { [tier]: map } as Partial<OverridesState>
    })
  },

  setCoreShare(v) {
    set({ coreShare: clampShare(v) })
  },

  revert(profile) {
    set({
      coreShare: profile.core_share,
      tier1: { ...profile.tier1_weights },
      tier2: { ...profile.tier2_weights },
    })
  },

  clear() {
    set({ ...EMPTY })
  },

  isDirty(profile) {
    const s = get()
    if (s.baseSlug !== profile.slug) return false
    return (
      s.coreShare !== profile.core_share ||
      !mapEqual(s.tier1, profile.tier1_weights) ||
      !mapEqual(s.tier2, profile.tier2_weights)
    )
  },
}))