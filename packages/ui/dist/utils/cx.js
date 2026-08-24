/** Join truthy string arguments with a single space. `false | null | undefined` are skipped. */
export function cx(...args) {
    return args.filter(Boolean).join(' ');
}
