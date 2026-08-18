import type { ReactNode } from 'react';
export interface ToastHandle {
    show: (message: string) => void;
}
export declare const TOAST_DURATION_MS = 2600;
export declare function ToastProvider(props: {
    children: ReactNode;
}): import("react").JSX.Element;
export declare function useToast(): ToastHandle;
