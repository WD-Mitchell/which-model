import type React from 'react';
export interface ToggleProps {
    on: boolean;
    disabled?: boolean;
    onToggle: (on: boolean) => void;
}
export declare function Toggle({ on, disabled, onToggle }: ToggleProps): React.JSX.Element;
