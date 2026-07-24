export interface TimedRequestCacheOptions<T> {
  ttlMs: number
  load: () => Promise<T>
  now?: () => number
  onCache?: (value: T) => void
}

export interface TimedRequestCache<T> {
  get: () => Promise<T>
  clear: () => void
}

export function createTimedRequestCache<T>(
  options: TimedRequestCacheOptions<T>
): TimedRequestCache<T> {
  const now = options.now ?? Date.now
  let cachedValue: T | undefined
  let hasCachedValue = false
  let expiresAt = 0
  let inFlight: Promise<T> | undefined
  let generation = 0

  function get(): Promise<T> {
    if (hasCachedValue && now() < expiresAt) {
      return Promise.resolve(cachedValue as T)
    }
    if (inFlight) return inFlight

    const requestGeneration = generation
    const request = options
      .load()
      .then((value) => {
        if (generation !== requestGeneration) {
          // The value completed after an explicit invalidation. Join (or
          // start) the current generation so existing callers cannot apply
          // stale status after a settings mutation.
          return get()
        }
        cachedValue = value
        hasCachedValue = true
        expiresAt = now() + options.ttlMs
        options.onCache?.(value)
        return value
      })
      .finally(() => {
        if (inFlight === request) inFlight = undefined
      })
    inFlight = request
    return request
  }

  return {
    get,
    clear() {
      generation += 1
      cachedValue = undefined
      hasCachedValue = false
      expiresAt = 0
      inFlight = undefined
    },
  }
}
