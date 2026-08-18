export interface SnippetPreviewProps {
    text: string;
    copyable?: boolean;
    onCopy?: (text: string) => void;
}
export declare function SnippetPreview({ text, copyable, onCopy }: SnippetPreviewProps): import("react").JSX.Element;
