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
import {
  RECOVERABLE_STORAGE_KEYS,
  clearRecoverableAppCache,
} from './recovery-cache.ts'

describe('clearRecoverableAppCache', () => {
  test('removes only recoverable app caches and preserves user state', () => {
    const removed: string[] = []

    clearRecoverableAppCache({
      removeItem: (key) => removed.push(key),
    })

    assert.equal(removed.includes('user'), false)
    assert.equal(removed.includes('uid'), false)
    assert.equal(removed.includes('notification-storage'), false)
    assert.deepEqual(removed, [...RECOVERABLE_STORAGE_KEYS])
  })

  test('continues clearing when one storage key fails', () => {
    const attempted: string[] = []

    assert.doesNotThrow(() =>
      clearRecoverableAppCache({
        removeItem: (key) => {
          attempted.push(key)
          if (key === RECOVERABLE_STORAGE_KEYS[0]) {
            throw new Error('storage unavailable')
          }
        },
      })
    )

    assert.deepEqual(attempted, [...RECOVERABLE_STORAGE_KEYS])
  })
})
