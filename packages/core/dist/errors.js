export class EngineError extends Error {
    code;
    constructor(code, message) {
        super(message);
        this.name = 'EngineError';
        this.code = code;
    }
}
export function isEngineError(e) {
    return e instanceof EngineError;
}
