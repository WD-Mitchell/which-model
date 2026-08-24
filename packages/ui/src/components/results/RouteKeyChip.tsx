import styles from './RouteKeyChip.module.css'

export interface RouteKeyChipProps {
  /** Canonical route key: `provider/model_id@reasoning` (provider = text before the first `/`). */
  routeKey: string
}

export function RouteKeyChip({ routeKey }: RouteKeyChipProps) {
  const slash = routeKey.indexOf('/')
  const provider = slash === -1 ? routeKey : routeKey.slice(0, slash)
  const model = slash === -1 ? '' : routeKey.slice(slash)
  return (
    <span className={styles.chip} title={routeKey}>
      <span className={styles.provider}>{provider}</span>
      <span className={styles.model}>{model}</span>
    </span>
  )
}