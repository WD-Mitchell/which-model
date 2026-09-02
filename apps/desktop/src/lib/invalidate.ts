import { useEffect } from 'react'
import { useQueryClient, type QueryKey } from '@tanstack/react-query'
import type { EngineEvent } from '@which-model/core'
import { getHost } from './host'

// U05 §2.12 / U00 CONTRACTS §5 — subscribe once per app root to every host
// event and invalidate exactly the mapped query keys. `['rank', …]` is
// invalidated by prefix so every overrides-hash variant refetches.
const INVALIDATION: Record<EngineEvent, QueryKey[]> = {
  'config:changed': [
    ['profiles'],
    ['providers'],
    ['provider'],
    ['harnesses'],
    ['favourites'],
    ['rank'],
    ['catalog-line'],
    ['catalog-models'],
    ['catalog-model'],
  ],
  'catalog:changed': [['groups'], ['group'], ['benchmarks'], ['benchmark'], ['model-score'], ['catalog-models'], ['catalog-model'], ['rank']],
  'usage:updated': [['usage'], ['providers']],
  'settings:changed': [['settings'], ['rank']],
  'pick:recorded': [['profiles'], ['catalog-line']],
}

const EVENTS = Object.keys(INVALIDATION) as EngineEvent[]

export function useEngineEvents(): void {
  const qc = useQueryClient()
  const host = getHost()

  useEffect(() => {
    const disposers = EVENTS.map((event) =>
      host.on(event, () => {
        for (const qkey of INVALIDATION[event]) {
          void qc.invalidateQueries({ queryKey: qkey })
        }
      }),
    )
    return () => {
      disposers.forEach((dispose) => dispose())
    }
  }, [qc, host])
}