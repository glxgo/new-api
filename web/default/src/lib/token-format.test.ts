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
import { formatTokenCount } from './token-format.ts'

const units = {
  tenThousand: '万',
  hundredMillion: '亿',
}

describe('formatTokenCount', () => {
  test('keeps values below ten thousand as grouped integers', () => {
    assert.equal(formatTokenCount(9_999, units, 'en-US'), '9,999')
  })

  test('uses the ten-thousand unit with exactly two decimals', () => {
    assert.equal(formatTokenCount(10_000, units, 'en-US'), '1.00万')
    assert.equal(formatTokenCount(1_000_000, units, 'en-US'), '100.00万')
    assert.equal(formatTokenCount(12_345_678, units, 'en-US'), '1234.57万')
  })

  test('uses the hundred-million unit with exactly two decimals', () => {
    assert.equal(formatTokenCount(100_000_000, units, 'en-US'), '1.00亿')
    assert.equal(formatTokenCount(987_654_321, units, 'en-US'), '9.88亿')
  })

  test('rejects missing and invalid values', () => {
    assert.equal(formatTokenCount(undefined, units, 'en-US'), '-')
    assert.equal(formatTokenCount(Number.NaN, units, 'en-US'), '-')
  })
})
