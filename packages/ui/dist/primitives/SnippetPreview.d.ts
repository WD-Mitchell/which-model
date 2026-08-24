export interface SnippetPreviewProps {
    text: string;
    /** 'block' (default) = U02 SPEC §2.21 / the mockup's shell-hook snippet
     *  (demo.dc.html 797: tinted 6% ground, 11px, line-height 1.7).
     *  'command' = the launch-command preview (demo.dc.html 530:
     *  `class="mono input"`, min-height 30px, 7px 10px, 11.5px, 72% text) —
     *  what a harness/agent command belongs in. */
    variant?: 'block' | 'command';
    copyable?: boolean;
    onCopy?: (text: string) => void;
}
export declare function SnippetPreview({ text, variant, copyable, onCopy, }: SnippetPreviewProps): import("react").JSX.Element;
