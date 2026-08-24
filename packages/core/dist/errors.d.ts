import type { ErrorDTO } from './types.js';
export type ErrorCode = 'validation_failed' | 'builtin_readonly' | 'not_found' | 'conflict' | 'io_error' | 'usage_unavailable' | 'launch_failed';
export declare class EngineError extends Error implements ErrorDTO {
    readonly code: ErrorCode;
    constructor(code: ErrorCode, message: string);
}
export declare function isEngineError(e: unknown): e is EngineError;
//# sourceMappingURL=errors.d.ts.map