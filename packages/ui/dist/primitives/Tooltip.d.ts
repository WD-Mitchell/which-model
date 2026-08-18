import type { ReactNode } from 'react';
export interface TooltipProps {
    content: ReactNode;
    children: ReactNode;
}
export declare function Tooltip({ content, children }: TooltipProps): import("react").JSX.Element;
