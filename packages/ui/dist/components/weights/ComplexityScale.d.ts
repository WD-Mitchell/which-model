export interface ComplexityScaleProps {
    stop: number;
    labels?: [string, string];
    profileName?: string;
    readOnly?: boolean;
    onStop?: (i: number) => void;
}
export declare function ComplexityScale({ stop, labels, profileName, readOnly, onStop, }: ComplexityScaleProps): import("react").JSX.Element;
