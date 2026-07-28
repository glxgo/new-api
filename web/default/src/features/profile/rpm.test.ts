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
import { resolveCurrentRpm, resolveRpmLimit } from './rpm.ts'

describe('resolveRpmLimit', () => {
  test('uses the backend limit when the new field is available', () => {
    assert.equal(resolveRpmLimit({ concurrency_limit: 8, rpm_limit: 20 }), 20)
  })

  test('uses an independent compatibility default for an older backend', () => {
    assert.equal(resolveRpmLimit({ concurrency_limit: 1000 }), 12)
    assert.equal(resolveRpmLimit({ concurrency_limit: 9 }), 12)
  })
})

describe('resolveCurrentRpm', () => {
  test('does not present a missing runtime counter as zero', () => {
    assert.equal(resolveCurrentRpm({ concurrency_limit: 1000 }), null)
    assert.equal(resolveCurrentRpm({ current_rpm: 0 }), 0)
  })
})
