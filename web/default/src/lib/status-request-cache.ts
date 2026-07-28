/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
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
