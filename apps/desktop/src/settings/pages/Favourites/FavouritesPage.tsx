// U14 — Favourites settings page: pinned models with route chips, availability,
// add-model combobox, unpin.
//
// Layout ported verbatim from the mockup's `onFavourites` branch
// (specs/desktop/mockup/demo.dc.html L574-589): accent "pinned models" kicker,
// a mono column-header row (model / route), rows of a fixed 200px model column
// plus a flexible route column, a "pinned" chip, a ghost Unpin button pushed
// right with `margin-left:auto`, and the trailing note. Per the U07 content-
// column contract <main> has no horizontal padding, so every child below
// supplies its own 22px inline padding — that is what lets the row rules bleed
// edge to edge.
import { useCallback, useState } from 'react'
import { Button, Combobox, Tag, useToast } from '@which-model/ui'
import type { RankedModel } from '@which-model/core'
import { useFavourites, useRank } from '../../../lib/queries'
import { getHost } from '../../../lib/host'
import { DetailHeader } from '../../DetailHeader'
import { PAGE_META } from '../../pages'
import type { PageComponentProps } from '../../pages'
import styles from './FavouritesPage.module.css'

export function FavouritesPage(_props: PageComponentProps) {
  const toast = useToast()
  const { data: favourites } = useFavourites()
  const [searchOpen, setSearchOpen] = useState(false)
  const [query, setQuery] = useState('')
  const list = favourites ?? []

  // Candidate models use the configured engine-valid hold count.
  const { data: rank } = useRank('balanced_implementation', 'none', 0)
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
      {/* The header block (back link, 19px title, blurb, right-aligned primary
          action) is DetailHeader's job — "Add model" toggles the combobox. */}
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

      {/* demo.dc.html L575 — section kicker. */}
      <span className={`mono ${styles.kicker}`}>pinned models</span>

      {/* demo.dc.html L576-579 — column headers; only the model and route
          columns are labelled, the chip and Unpin cells are unlabelled. */}
      <div className={styles.headRow}>
        <span className={`mono ${styles.headModel}`}>model</span>
        <span className={`mono ${styles.headRoute}`}>route</span>
      </div>

      {list.length === 0 ? (
        <div className={styles.empty}>No pinned models yet — pin one to offer it first when in range.</div>
      ) : (
        list.map((f) => (
          // demo.dc.html L581-587 — a favourite row is presentational (no
          // detail view behind it), so it carries no `.row` hover tint.
          <div key={f.route_key} className={styles.row}>
            <span className={styles.name}>{f.model_name}</span>
            {/* route_label is already "provider · reasoning", exactly the
                string the mockup composes for this column. */}
            <span className={`mono ${styles.route}`}>{f.route_label}</span>
            {/* The mockup only ever renders the accent "pinned" chip; the real
                engine also reports whether the pin still ranks in range, so an
                out-of-range favourite swaps to the neutral variant. */}
            <Tag variant={f.in_range ? 'accent' : 'neutral'} size="chip" className={styles.chip}>
              {f.in_range ? 'pinned' : 'out of range'}
            </Tag>
            <span className={styles.actions}>
              <Button
                variant="ghost"
                className={styles.unpin}
                onClick={() => void handleUnpin(f.route_key)}
              >
                Unpin
              </Button>
            </span>
          </div>
        ))
      )}

      {/* demo.dc.html L588 — trailing note, verbatim copy. */}
      <div className={styles.note}>
        A favourite is only offered when the profile&apos;s weights still rank it in range — pinning never
        forces a model that does not fit the task.
      </div>
    </div>
  )
}

export default FavouritesPage
