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
import { getUserAvailableBalance } from './user-balance.ts'

describe('getUserAvailableBalance', () => {
  test('includes principal and gift quota in the displayed balance', () => {
    assert.equal(
      getUserAvailableBalance({ quota: 498054, gift_quota: 1946 }),
      500000
    )
  })

  test('treats missing or invalid quota fields as zero', () => {
    assert.equal(getUserAvailableBalance({}), 0)
    assert.equal(
      getUserAvailableBalance({ quota: Number.NaN, gift_quota: 250000 }),
      250000
    )
  })
})
