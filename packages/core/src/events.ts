import type { GUISettings } from './types.js'

// Closed event enum (D00 CONTRACTS §3).
export const ENGINE_EVENTS = [
  'config:changed',
  'catalog:changed',
  'usage:updated',
  'settings:changed',
  'pick:recorded',
] as const

export type EngineEvent = (typeof ENGINE_EVENTS)[number]

export type EngineEventPayloads = {
  'config:changed': { section: string }
  'catalog:changed': Record<string, never>
  'usage:updated': Record<string, never>
  'settings:changed': GUISettings
  'pick:recorded': { profile_slug: string; route_key: string }
}
