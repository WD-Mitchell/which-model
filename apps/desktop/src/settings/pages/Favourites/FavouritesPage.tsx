// U14 — Favourites settings page: pinned models with route chips, availability,
// add-model combobox, unpin.
import { useCallback, useState } from 'react'
import { Combobox, Tag, useToast } from '@which-model/ui'
import type { RankedModel } from '@which-model/core'
import { useFavourites, useRank, useSettings } from '../../../lib/queries'
import { getHost } from '../../../lib/host'
import { DetailHeader } from '../../DetailHeader'
import { PAGE_META } from '../../pages'
import type { PageComponentProps } from '../../pages'
import styles from './FavouritesPage.module.css'

export function FavouritesPage(_props: PageComponentProps) {
  const toast = useToast()
  const { data: favourites } = useFavourites()
  const { data: settings } = useSettings()
  const holds = settings?.holds ?? 5
  const [searchOpen, setSearchOpen] = useState(false)
  const [query, setQuery] = useState('')
  const list = favourites ?? []

  // Candidate models come from a default-profile rank (broad availability).
  const { data: rank } = useRank('balanced_implementation', 'none', 50)
  const candidates: RankedModel[] = rank?.candidates ?? []

  const matching = candidates
    .filter((c) => {
      const q = query.trim().toLowerCase()
      if (!q) return true
      return c.model_name.toLowerCase().includes(q) || c.model_id.toLowerCase().includes(q)
    })
    .slice(0, 6)

  const handleAdd = useCallback(
    async (routeKey: string) => {
      setSearchOpen(false)
      setQuery('')
      try {
        await getHost().favourites.pin(routeKey)
        toast.show(`pinned ${routeKey}`)
      } catch (e) {
        toast.show((e as { message?: string }).message ?? 'pin failed')
      }
    },
    [toast],
  )

  const handleUnpin = useCallback(
    async (routeKey: string) => {
      try {
        await getHost().favourites.unpin(routeKey)
        toast.show(`unpinned ${routeKey}`)
      } catch (e) {
        toast.show((e as { message?: string }).message ?? 'unpin failed')
      }
    },
    [toast],
  )

  return (
    <div className={styles.page}>
      <DetailHeader
        title={PAGE_META.Favourites[0]}
        blurb={PAGE_META.Favourites[1]}
        action={{ label: PAGE_META.Favourites[2] as string, onAction: () => setSearchOpen((v) => !v) }}
      />
      {searchOpen ? (
        <div className={styles.addSearch}>
          <Combobox
            placeholder="type to find a model"
            items={matching.map((c) => ({
              key: c.route_key,
              label: c.model_name,
              sub: c.route_key,
            }))}
            query={query}
            onQuery={setQuery}
            open={searchOpen}
            onOpenChange={setSearchOpen}
            onPick={(k) => void handleAdd(k)}
            emptyText="no model by that name"
          />
        </div>
      ) : null}
      <div className={styles.kicker}>favourites</div>
      {list.length === 0 ? (
        <div className={styles.empty}>{'No pinned models yet — pin one to offer it first when in range.'}</div>
      ) : (
        list.map((f) => (
          <div key={f.route_key} className={styles.row}>
            <span className={styles.name}>
              <span className="mono">{f.model_name}</span>
            </span>
            <span className={styles.route}>
              <CodeLabel text={f.route_key} />
            </span>
            <span className={styles.tag}>
              <Tag variant={f.in_range ? 'accent' : 'outline'}>{f.in_range ? 'in range' : 'out of range'}</Tag>
            </span>
            <span className={styles.actions}>
              <button type="button" className="ib" title={`Unpin ${f.route_key}`}
                onClick={() => void handleUnpin(f.route_key)}>×</button>
            </span>
          </div>
        ))
      )}
    </div>
  )
}

function CodeLabel({ text }: { text: string }) {
  return <code className="mono">{text}</code>
}
export default FavouritesPage
