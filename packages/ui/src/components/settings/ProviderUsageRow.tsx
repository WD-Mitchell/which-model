import type { ReactNode } from 'react'
import type { ProviderInfo } from '@which-model/core'
import { cx } from '../../utils/cx'
import { Toggle } from '../../primitives/Toggle'
import { UsageMeter } from '../../primitives/UsageMeter'
import styles from './ProviderUsageRow.module.css'

export interface ProviderUsageRowProps {
  provider: ProviderInfo
  /** The switch's state. Its MEANING is the caller's: global enablement on the
   *  Providers page, per-harness permission on a harness detail. */
  on: boolean
  onToggle(next: boolean): void
  /** When false the meters, credits and resets read blank and the subtitle
   *  shows `offLabel` — a provider that cannot report has no usage, and
   *  showing a stale bar would misrepresent remaining quota. */
  live: boolean
  /** Subtitle when `live` is false. Harness detail says "off globally"
   *  (the provider is on, but disabled app-wide); the Providers page says
   *  "not enabled". */
  offLabel?: string
  /** Extra content after the credits column (e.g. a route count, a chevron). */
  trailing?: ReactNode
  /** Content between the switch and the id (e.g. a priority number). Kept
   *  inside the card so it does not consume the flexible meter column. */
  leading?: ReactNode
}

/**
 * One provider as a usage card: switch, id over its auth line, the three usage
 * meters, then credits over the reset hint.
 *
 * Ported from the harness detail view (mockup demo.dc.html 542-566, bindings
 * 1096-1118) and shared, because the same row is the Providers page's list —
 * it carries the live quota picture that a plain id-and-count row cannot.
 */
export function ProviderUsageRow({
  provider,
  on,
  onToggle,
  live,
  offLabel = 'not enabled',
  trailing,
  leading,
}: ProviderUsageRowProps) {
  // Only the windows this provider actually reports get a meter.
  //
  // The mockup drew all three because its fixture reported all three; real
  // providers do not (claude: session+weekly, copilot: monthly), and rendering
  // permanent "—" columns spent the width the live meters need. At three
  // meters each cell is ~62px while the label "SESSION" alone needs 65, so the
  // labels truncated to "SESSI…"; at one or two they fit outright.
  const windows: Array<[string, number | null]> = live
    ? (
        [
          ['session', provider.session],
          ['weekly', provider.weekly],
          ['monthly', provider.monthly],
        ] as Array<[string, number | null]>
      ).filter(([, v]) => v !== null)
    : []
  return (
    <div className={cx(styles.row, on && styles.rowOn)}>
      <span onClick={(e) => e.stopPropagation()}>
        <Toggle on={on} onToggle={onToggle} />
      </span>
      {leading ? <span className={styles.leading}>{leading}</span> : null}
      <span className={styles.idCell}>
        <span className={cx('mono', styles.id, on && styles.idOn)} title={provider.id}>{provider.id}</span>
        <span className={cx('mono', styles.auth, !live && styles.authOff)}>
          {live ? provider.auth : offLabel}
        </span>
      </span>
      <span className={styles.meters}>
        {windows.length > 0 ? (
          windows.map(([id, value]) => <UsageMeter key={id} label={id} percent={value} />)
        ) : (
          <span className={cx('mono', styles.noUsage)}>
            {live ? 'no usage data' : ''}
          </span>
        )}
      </span>
      <span className={styles.creditsCell}>
        <span className={cx('mono', styles.credits, !live && styles.creditsOff)}>
          {live ? provider.credits : ''}
        </span>
        <span className={cx('mono', styles.resets)}>{live ? provider.resets : ''}</span>
      </span>
      {trailing}
    </div>
  )
}
