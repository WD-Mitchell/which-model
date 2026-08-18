/** Join truthy string arguments with a single space. `false | null | undefined` are skipped. */
export function cx(...args: Array<string | false | null | undefined>): string {
  return args.filter(Boolean).join(' ')
}