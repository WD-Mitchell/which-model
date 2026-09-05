import type { RankedModel } from '@which-model/core'
import styles from './MissingBenchmarkNotice.module.css'

export function MissingBenchmarkNotice({ model }: { model: RankedModel }) {
  const missing = (['intelligence', 'cost', 'speed'] as const).filter((axis) => model[axis] == null)
  if (missing.length === 0) return null
  return (
    <span className={styles.notice}>
      Missing benchmark data: {missing.join(', ')}. Ranked using available scores.
    </span>
  )
}
