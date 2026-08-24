import { useEffect, useRef, useState } from 'react'
import type { RankedModel } from '@which-model/core'
import { cx } from '../../utils/cx'
import type { SplitButtonMenuItem } from '../../primitives/SplitButton'
import { RankCard } from './RankCard'
import type { SparkbarEntry } from './Sparkbar'
import styles from './ResultsCarousel.module.css'

export interface ResultsCarouselProps {
  items: RankedModel[]
  /** Per-model core metric bars. */
  metrics: (model: RankedModel) => SparkbarEntry[]
  /** Controlled focus index; clamped for display only. */
  index: number
  onIndex: (index: number) => void
  /** Rank line per card, e.g. (i, total) => `rank ${i + 1} of ${total}`. */
  rankLabel: (index: number, total: number) => string
  /** Favourite/pin state per model; absent ⇒ cards render no pin toggle. */
  pinned?: (model: RankedModel) => boolean
  onTogglePin?: (model: RankedModel) => void
  /** Harness choices for the launch split button; omitted ⇒ no launcher on cards. */
  harnessNames?: string[]
  selectedHarness?: string
  launchLabel?: (model: RankedModel) => string
  onLaunch?: (model: RankedModel) => void
  onHarnessChange?: (model: RankedModel, harness: string) => void
}

function clampIndex(i: number, length: number): number {
  if (length === 0) return 0
  return Math.max(0, Math.min(i, length - 1))
}

export function ResultsCarousel({
  items,
  metrics,
  index,
  onIndex,
  rankLabel,
  pinned,
  onTogglePin,
  harnessNames,
  selectedHarness = '',
  launchLabel,
  onLaunch,
  onHarnessChange,
}: ResultsCarouselProps) {
  const lastIndex = items.length - 1
  const shown = clampIndex(index, items.length)
  // Carousel card refs, one per item, for scrolling the focused card into view.
  const cardRefs = useRef<Array<HTMLDivElement | null>>([])
  // Launch menu open is ephemeral popover state (mockup harnessMenuOpen).
  const [openMenu, setOpenMenu] = useState<number | null>(null)

  useEffect(() => {
    const el = cardRefs.current[shown]
    if (el && typeof el.scrollIntoView === 'function') {
      el.scrollIntoView({ block: 'nearest', inline: 'nearest' })
    }
  }, [shown])

  const menuItems: SplitButtonMenuItem[] | undefined = harnessNames?.map((name) => ({
    key: name,
    label: name,
    selected: name === selectedHarness,
  }))

  const hasLauncher = Boolean(menuItems && onLaunch && harnessNames && harnessNames.length > 0)

  return (
    <div className={styles.root}>
      <div className={styles.viewport}>
        {items.map((model, i) => {
          const interactive = hasLauncher
            ? {
                launchLabel:
                  launchLabel?.(model) ?? `Launch in ${selectedHarness}`,
                harnesses: menuItems,
                launchMenuOpen: openMenu === i,
                onLaunchMenuOpenChange: (open: boolean) =>
                  setOpenMenu(open ? i : null),
                onLaunch: () => onLaunch?.(model),
                onHarnessChange: (key: string) => {
                  setOpenMenu(null)
                  onHarnessChange?.(model, key)
                },
              }
            : {}
          return (
            <div
              key={model.model_id}
              ref={(el) => {
                cardRefs.current[i] = el
              }}
              className={styles.cardSlot}
            >
              <RankCard
                model={model}
                rankLine={rankLabel(i, items.length)}
                metrics={metrics(model)}
                focused={i === shown}
                pinned={pinned ? pinned(model) : false}
                onTogglePin={
                  onTogglePin ? () => onTogglePin(model) : undefined
                }
                {...interactive}
              />
            </div>
          )
        })}
      </div>
      <div className={styles.controls}>
        <button
          type="button"
          className={styles.chevron}
          aria-label="previous rank"
          disabled={shown === 0}
          onClick={() => onIndex(clampIndex(shown - 1, items.length))}
        >
          <svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
            <path d="M7.2 2.2 3.4 6l3.8 3.8"></path>
          </svg>
        </button>
        <div className={styles.dots} role="tablist" aria-label="results">
          {items.map((model, i) => (
            <button
              key={model.model_id}
              type="button"
              className={cx(styles.dot, i === shown && styles.dotActive)}
              aria-label={`go to rank ${i + 1}`}
              aria-current={i === shown ? 'true' : undefined}
              onClick={() => onIndex(i)}
            />
          ))}
        </div>
        <button
          type="button"
          className={styles.chevron}
          aria-label="next rank"
          disabled={shown >= lastIndex}
          onClick={() => onIndex(clampIndex(shown + 1, items.length))}
        >
          <svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
            <path d="M4.8 2.2 8.6 6l-3.8 3.8"></path>
          </svg>
        </button>
      </div>
    </div>
  )
}