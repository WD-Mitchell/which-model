// Outstanding persistence is shared across mounts of the same entity.
const active = new Map<string, Promise<void>>()

export function hasActiveAutosave(key: string): boolean { return active.has(key) }
export function whenAutosaveIdle(key: string): Promise<void> {
  return (active.get(key) ?? Promise.resolve()).catch(() => {})
}

function serialize(key: string | undefined, action: () => Promise<void>): Promise<void> {
  if (!key) return Promise.resolve().then(action)
  const result = whenAutosaveIdle(key).then(action)
  const tracked = result.finally(() => { if (active.get(key) === tracked) active.delete(key) })
  active.set(key, tracked)
  return tracked
}

/** One retained snapshot and one writer per editor. A flush survives unmount. */
export function createAutosave<T>(
  write: (value: T) => Promise<void>,
  options: {
    key?: string
    delay: number
    onSuccess?: (value: T, generation: number) => void | Promise<void>
    onError?: (error: unknown, generation: number) => void | Promise<void>
  },
) {
  let generation = 0
  let pending: { value: T; generation: number } | undefined
  let timer: ReturnType<typeof setTimeout> | undefined
  let running: Promise<void> | undefined

  function cancelTimer() { clearTimeout(timer); timer = undefined }

  function flush(): Promise<void> {
    cancelTimer()
    if (running) return running
    if (!pending) return options.key ? whenAutosaveIdle(options.key) : Promise.resolve()
    // Defer the drain until running is assigned, including for synchronous throws.
    running = serialize(options.key, async () => {
      let failure: unknown
      let failed = false
      while (pending) {
        const next = pending
        pending = undefined
        try {
          await write(next.value)
          await options.onSuccess?.(next.value, next.generation)
        } catch (error) {
          failed = true
          failure = error
          await options.onError?.(error, next.generation)
        }
      }
      if (failed) throw failure
    }).finally(() => { running = undefined })
    return running
  }

  return {
    schedule(value: T) {
      pending = { value, generation: ++generation }
      cancelTimer()
      timer = setTimeout(() => { void flush().catch(() => {}) }, options.delay)
      return generation
    },
    flush,
    cancelPending() { cancelTimer(); pending = undefined; generation++ },
    isCurrent(value: number) { return generation === value },
    hasPending() { return pending !== undefined || running !== undefined },
  }
}
