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
import { getStandardPaymentMethods } from './payment-methods.ts'

describe('getStandardPaymentMethods', () => {
  test('keeps Epay, Stripe and Pancake but excludes legacy Waffo', () => {
    const methods = getStandardPaymentMethods([
      { name: 'Alipay', type: 'alipay', min_topup: 1 },
      { name: 'Stripe', type: 'stripe', min_topup: 1 },
      { name: 'Waffo', type: 'waffo', min_topup: 1 },
      { name: 'Pancake', type: 'waffo_pancake', min_topup: 1 },
    ])

    assert.deepEqual(
      methods.map((method) => method.type),
      ['alipay', 'stripe', 'waffo_pancake']
    )
  })
})
