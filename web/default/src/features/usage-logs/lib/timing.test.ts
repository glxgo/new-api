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
import { getThroughputColor, resolveFirstTokenMs } from './timing.ts'

describe('getThroughputColor', () => {
  test('uses red below 10 tokens per second', () => {
    assert.equal(getThroughputColor(0), 'danger')
    assert.equal(getThroughputColor(9.99), 'danger')
  })

  test('uses yellow from 10 up to 20 tokens per second', () => {
    assert.equal(getThroughputColor(10), 'warning')
    assert.equal(getThroughputColor(19.99), 'warning')
  })

  test('uses green from 20 tokens per second', () => {
    assert.equal(getThroughputColor(20), 'success')
    assert.equal(getThroughputColor(100), 'success')
  })
})

describe('resolveFirstTokenMs', () => {
  test('prefers the upstream-to-server timing for new logs', () => {
    assert.equal(resolveFirstTokenMs({ upstream_frt: 1250, frt: 4250 }), 1250)
  })

  test('falls back to the historical request-ingress timing', () => {
    assert.equal(resolveFirstTokenMs({ frt: 4250 }), 4250)
  })

  test('rejects missing or invalid timing values', () => {
    assert.equal(resolveFirstTokenMs(undefined), null)
    assert.equal(resolveFirstTokenMs({ upstream_frt: 0, frt: 4250 }), null)
  })
})
