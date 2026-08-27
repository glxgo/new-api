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
import { availabilityBarClass, availabilityBarHeight } from './visuals.ts'

describe('availabilityBarHeight', () => {
  test('holds rates at or below 85 percent at the floor', () => {
    assert.equal(availabilityBarHeight(0), 18)
    assert.equal(availabilityBarHeight(85), 18)
  })

  test('scales uniformly from 85 percent to full height', () => {
    assert.equal(availabilityBarHeight(92.5), 59)
    assert.equal(availabilityBarHeight(100), 100)
  })

  test('clamps invalid and out-of-range values', () => {
    assert.equal(availabilityBarHeight(-10), 18)
    assert.equal(availabilityBarHeight(110), 100)
    assert.equal(availabilityBarHeight(Number.NaN), 18)
  })
})

describe('availabilityBarClass', () => {
  test('uses green at or above 90 percent', () => {
    assert.match(availabilityBarClass(100), /emerald/)
    assert.match(availabilityBarClass(90), /emerald/)
  })

  test('uses yellow from 80 percent up to but excluding 90 percent', () => {
    assert.match(availabilityBarClass(89.9), /amber/)
    assert.match(availabilityBarClass(80), /amber/)
  })

  test('uses red below 80 percent', () => {
    assert.match(availabilityBarClass(79.9), /rose/)
    assert.match(availabilityBarClass(0), /rose/)
  })
})
