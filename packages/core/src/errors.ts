import type { ErrorDTO } from './types.js'

// Closed error-code enum (D00 CONTRACTS §4).
export type ErrorCode =
  | 'validation_failed'
  | 'builtin_readonly'
  | 'not_found'
  | 'conflict'
  | 'io_error'
  | 'usage_unavailable'
  | 'launch_failed'

export class EngineError extends Error implements ErrorDTO {
  readonly code: ErrorCode

  constructor(code: ErrorCode, message: string) {
    super(message)
    this.name = 'EngineError'
    this.code = code
  }
}

export function isEngineError(e: unknown): e is EngineError {
  return e instanceof EngineError
}
