import type React from 'react';
export interface ToggleProps {
    on: boolean;
    disabled?: boolean;
    onToggle: (on: boolean) => void;
    'aria-label'?: string;
}
export declare function Toggle({ on, disabled, onToggle, 'aria-label': ariaLabel }: ToggleProps): React.JSX.Element;
