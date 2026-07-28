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
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import { createTimedRequestCache } from './status-request-cache.ts'

describe('createTimedRequestCache', () => {
  test('coalesces concurrent loads and reuses the value until the TTL expires', async () => {
    let now = 1_000
    let loadCount = 0
    let resolveFirstLoad: ((value: string) => void) | undefined

    const cache = createTimedRequestCache({
      ttlMs: 30_000,
      now: () => now,
      load: () => {
        loadCount += 1
        if (loadCount === 1) {
          return new Promise<string>((resolve) => {
            resolveFirstLoad = resolve
          })
        }
        return Promise.resolve(`status-${loadCount}`)
      },
    })

    const first = cache.get()
    const concurrent = cache.get()

    assert.equal(loadCount, 1)
    resolveFirstLoad?.('status-1')
    assert.equal(await first, 'status-1')
    assert.equal(await concurrent, 'status-1')

    now += 29_999
    assert.equal(await cache.get(), 'status-1')
    assert.equal(loadCount, 1)

    now += 2
    assert.equal(await cache.get(), 'status-2')
    assert.equal(loadCount, 2)
  })

  test('does not retain a failed in-flight request', async () => {
    let loadCount = 0
    const cache = createTimedRequestCache({
      ttlMs: 30_000,
      load: async () => {
        loadCount += 1
        if (loadCount === 1) throw new Error('temporary failure')
        return 'recovered'
      },
    })

    await assert.rejects(cache.get(), /temporary failure/)
    assert.equal(await cache.get(), 'recovered')
    assert.equal(loadCount, 2)
  })

  test('clear forces the next read to reload', async () => {
    let loadCount = 0
    const cache = createTimedRequestCache({
      ttlMs: 30_000,
      load: async () => {
        loadCount += 1
        return loadCount
      },
    })

    assert.equal(await cache.get(), 1)
    cache.clear()
    assert.equal(await cache.get(), 2)
  })

  test('clear does not allow an older in-flight value to replace fresh data', async () => {
    let loadCount = 0
    let resolveOld: ((value: string) => void) | undefined
    const cachedValues: string[] = []
    const cache = createTimedRequestCache({
      ttlMs: 30_000,
      onCache: (value) => cachedValues.push(value),
      load: () => {
        loadCount += 1
        if (loadCount === 1) {
          return new Promise<string>((resolve) => {
            resolveOld = resolve
          })
        }
        return Promise.resolve('fresh')
      },
    })

    const oldRequest = cache.get()
    cache.clear()
    assert.equal(await cache.get(), 'fresh')
    resolveOld?.('old')
    assert.equal(await oldRequest, 'fresh')
    assert.equal(await cache.get(), 'fresh')
    assert.equal(loadCount, 2)
    assert.deepEqual(cachedValues, ['fresh'])
  })
})
