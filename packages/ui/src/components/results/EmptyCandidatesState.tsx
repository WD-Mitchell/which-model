import { Button } from '../../primitives/Button'
import styles from './EmptyCandidatesState.module.css'

const DEFAULT_TITLE = 'No models match your routes'
const DEFAULT_MESSAGE =
  'Switch on a provider or adjust your weights to see ranked candidates here.'

export interface EmptyCandidatesStateProps {
  title?: string
  message?: string
  /** When both `actionLabel` and `onAction` are provided, a call-to-action renders. */
  actionLabel?: string
  onAction?: () => void
}

export function EmptyCandidatesState({
  title = DEFAULT_TITLE,
  message = DEFAULT_MESSAGE,
  actionLabel,
  onAction,
}: EmptyCandidatesStateProps) {
  const showAction = Boolean(actionLabel && onAction)
  return (
    <div className={styles.empty} role="status">
      <span className={styles.kicker}>no routes</span>
      <span className={styles.title}>{title}</span>
      <span className={styles.message}>{message}</span>
      {showAction && (
        <Button variant="secondary" size="sm" onClick={onAction}>
          {actionLabel}
        </Button>
      )}
    </div>
  )
}