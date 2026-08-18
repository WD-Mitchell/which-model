import { jsx as _jsx } from "react/jsx-runtime";
import { cx } from '../utils/cx';
import styles from './SnippetPreview.module.css';
export function SnippetPreview({ text, copyable = false, onCopy }) {
    return (_jsx("pre", { className: cx(styles.block, copyable && styles.copyable), onClick: copyable ? () => onCopy?.(text) : undefined, children: text }));
}
