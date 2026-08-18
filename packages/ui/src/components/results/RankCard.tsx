import type { RankedModel } from '@which-model/core'
import { cx } from '../../utils/cx'
import { SplitButton } from '../../primitives/SplitButton'
import type { SplitButtonMenuItem } from '../../primitives/SplitButton'
import { Sparkbar } from './Sparkbar'
import type { SparkbarEntry } from './Sparkbar'
import { RouteKeyChip } from './RouteKeyChip'
import styles from './RankCard.module.css'

export interface RankCardProps {
  model: RankedModel
  /** Rank line, e.g. `rank 1 of 3`. */
  rankLine: string
  /** Core metric bars (intelligence/speed/cost). */
  metrics: SparkbarEntry[]
  /** Highlight as the focused/selected card of a carousel. */
  focused?: boolean
  /** Favourite/pin state; supplying `onTogglePin` renders the toggle. */
  pinned?: boolean
  onTogglePin?: () => void
  /** Launch split-button group; all four are required together to render. */
  launchLabel?: string
  harnesses?: SplitButtonMenuItem[]
  launchMenuOpen?: boolean
  onLaunchMenuOpenChange?: (open: boolean) => void
  onLaunch?: () => void
  onHarnessChange?: (key: string) => void
}

function PinGlyph() {
  return (
    <svg width="11" height="11" viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M9 4.5 7.5 3M9 4.5l-1.7 1.7 1 .9-.9 2.2L4 5.4l2.2-.9.9 1L8.8 3.8 7.5 2.3 9 4.5Z"></path>
    </svg>
  )
}

export function RankCard({
  model,
  rankLine,
  metrics,
  focused = false,
  pinned = false,
  onTogglePin,
  launchLabel,
  harnesses,
  launchMenuOpen = false,
  onLaunchMenuOpenChange,
  onLaunch,
  onHarnessChange,
}: RankCardProps) {
  const meta = `${model.provider} · ${model.reasoning} · ${model.score.toFixed(2)}`
  const hasLauncher = Boolean(
    launchLabel && harnesses && onLaunch && onLaunchMenuOpenChange,
  )
  return (
    <article className={cx(styles.card, focused && styles.focused)}>
      <div className={styles.heading} aria-live="polite">
        <span className={styles.rank}>{rankLine}</span>
        <span className={styles.name}>{model.model_name}</span>
        <span className={styles.meta}>{meta}</span>
      </div>
      <div className={styles.detailRow}>
        <RouteKeyChip routeKey={model.route_key} />
        <span className={styles.score}>{model.score.toFixed(2)}</span>
        {onTogglePin && (
          <button
            type="button"
            className={cx(styles.pin, pinned && styles.pinActive)}
            onClick={onTogglePin}
            aria-pressed={pinned}
            aria-label={pinned ? 'unpin model' : 'pin model'}
          >
            <PinGlyph />
          </button>
        )}
      </div>
      <Sparkbar metrics={metrics} />
      {hasLauncher && (
        <div className={styles.footer}>
          <SplitButton
            label={launchLabel!}
            onMain={onLaunch!}
            menuItems={harnesses!}
            onPick={onHarnessChange!}
            open={launchMenuOpen}
            onOpenChange={onLaunchMenuOpenChange!}
          />
        </div>
      )}
    </article>
  )
}