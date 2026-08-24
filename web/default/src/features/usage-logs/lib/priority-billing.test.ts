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
  getPriorityBillingAmounts,
  getPriorityBillingSummary,
} from './priority-billing.ts'

describe('getPriorityBillingSummary', () => {
  test('reports an applied 2.5x surcharge from persisted audit fields', () => {
    assert.deepEqual(
      getPriorityBillingSummary({
        service_tier: 'priority',
        priority_doubled: true,
      }),
      {
        isFast: true,
        multiplier: 2.5,
      }
    )
  })

  test('does not claim a multiplier when priority is expression-owned', () => {
    assert.deepEqual(
      getPriorityBillingSummary({
        service_tier: 'priority',
        priority_doubled: false,
      }),
      {
        isFast: true,
        multiplier: null,
      }
    )
  })

  test('returns null for non-priority requests', () => {
    assert.equal(
      getPriorityBillingSummary({
        service_tier: 'default',
        priority_doubled: false,
      }),
      null
    )
  })

  test('derives the persisted pre-FAST price from the final 2.5x quota', () => {
    assert.deepEqual(getPriorityBillingAmounts(3088, 2.5), {
      originalQuota: 1235.2,
      fastQuota: 3088,
    })
  })

  test('does not invent an original price without a persisted 2.5x marker', () => {
    assert.equal(getPriorityBillingAmounts(2468, null), null)
    assert.equal(getPriorityBillingAmounts(Number.NaN, 2.5), null)
    assert.equal(getPriorityBillingAmounts(-1, 2.5), null)
  })
})
