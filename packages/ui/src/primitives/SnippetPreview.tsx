import { cx } from '../utils/cx'
import styles from './SnippetPreview.module.css'

export interface SnippetPreviewProps {
  text: string // newlines preserved
  /** 'block' (default) = U02 SPEC §2.21 / the mockup's shell-hook snippet
   *  (demo.dc.html 797: tinted 6% ground, 11px, line-height 1.7).
   *  'command' = the launch-command preview (demo.dc.html 530:
   *  `class="mono input"`, min-height 30px, 7px 10px, 11.5px, 72% text) —
   *  what a harness/agent command belongs in. */
  variant?: 'block' | 'command'
  copyable?: boolean
  onCopy?: (text: string) => void // fired on click when copyable
}

export function SnippetPreview({
  text,
  variant = 'block',
  copyable = false,
  onCopy,
}: SnippetPreviewProps) {
  // <pre> in both variants: the mockup's command preview is a <div> with
  // white-space:pre-wrap, which renders identically, and keeping one element
  // keeps the newline-preservation contract (§2.21) variant-independent.
  return (
    <pre
      className={cx(
        variant === 'command' ? 'mono input' : undefined,
        variant === 'command' ? styles.command : styles.block,
        copyable && styles.copyable,
      )}
      onClick={copyable ? () => onCopy?.(text) : undefined}
    >
      {text}
    </pre>
  )
}
