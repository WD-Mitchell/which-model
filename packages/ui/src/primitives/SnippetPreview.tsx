import { cx } from '../utils/cx'
import styles from './SnippetPreview.module.css'

export interface SnippetPreviewProps {
  text: string // newlines preserved
  copyable?: boolean
  onCopy?: (text: string) => void // fired on click when copyable
}

export function SnippetPreview({ text, copyable = false, onCopy }: SnippetPreviewProps) {
  return (
    <pre
      className={cx(styles.block, copyable && styles.copyable)}
      onClick={copyable ? () => onCopy?.(text) : undefined}
    >
      {text}
    </pre>
  )
}