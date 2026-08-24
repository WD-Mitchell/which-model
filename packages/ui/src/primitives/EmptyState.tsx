import styles from './EmptyState.module.css'

export interface EmptyStateProps {
  text: string
}

export function EmptyState({ text }: EmptyStateProps) {
  return <div className={styles.empty}>{text}</div>
}