import { jsx as _jsx } from "react/jsx-runtime";
import { cx } from '../utils/cx';
import styles from './SnippetPreview.module.css';
export function SnippetPreview({ text, variant = 'block', copyable = false, onCopy, }) {
    // <pre> in both variants: the mockup's command preview is a <div> with
    // white-space:pre-wrap, which renders identically, and keeping one element
    // keeps the newline-preservation contract (§2.21) variant-independent.
    return (_jsx("pre", { className: cx(variant === 'command' ? 'mono input' : undefined, variant === 'command' ? styles.command : styles.block, copyable && styles.copyable), onClick: copyable ? () => onCopy?.(text) : undefined, children: text }));
}
