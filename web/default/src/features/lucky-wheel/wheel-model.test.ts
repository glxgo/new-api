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
import type { LuckyCard, LuckyPrize } from './types.ts'
import {
  buildWheelSegments,
  chooseAvailableCardId,
  chooseNextAvailableCardId,
  formatPrizeProbability,
  getReadableLabelRotation,
  getTargetRotation,
  getWheelLabelRotation,
} from './wheel-model.ts'

function card(id: number, status = 'available', expiresAt = id): LuckyCard {
  return {
    id,
    rule_set_id: 1,
    pool_type: 'subscription',
    source_type: 'subscription_purchase',
    source_ref: '',
    source_subscription_id: 1,
    status,
    issued_at: 1,
    expires_at: expiresAt,
  }
}

describe('lucky wheel model', () => {
  test('selects the earliest-expiring available card when the old card was consumed', () => {
    const cards = [
      card(10, 'consumed'),
      card(11, 'available', 30),
      card(12, 'available', 40),
    ]

    assert.equal(chooseAvailableCardId(cards, '10'), '11')
    assert.equal(chooseAvailableCardId(cards, '12'), '12')
    assert.equal(chooseAvailableCardId([], '12'), '')
  })

  test('advances to the adjacent card after a manually selected card is consumed', () => {
    const cards = [
      card(16, 'available', 30),
      card(17, 'available', 30),
      card(66, 'available', 60),
      card(67, 'available', 60),
    ]

    assert.equal(chooseNextAvailableCardId(cards, '66'), '67')
    assert.equal(chooseNextAvailableCardId(cards, '67'), '16')
    assert.equal(chooseNextAvailableCardId([card(66)], '66'), '')
  })

  test('builds the wheel from the selected pool and shows recharge quota with its real award', () => {
    const pool: LuckyPrize[] = [
      { code: 'quota_5', display_usd_micros: 5_000_000, weight: 500_000 },
      { code: 'gift_5', display_usd_micros: 5_000_000, weight: 500_000 },
    ]

    assert.deepEqual(
      buildWheelSegments(pool, 'recharge', 40_000_000).map(
        (item) => item.label
      ),
      ['$45 套餐额度', '$5 钱包赠金']
    )
    assert.equal(
      buildWheelSegments(pool, 'subscription', 40_000_000)[0]?.label,
      '$5 套餐额度'
    )
  })

  test('lands on the requested prize after consecutive spins without accumulating angle error', () => {
    const first = getTargetRotation(0, 2, 9)
    const second = getTargetRotation(first, 4, 9)

    assert.equal(((first % 360) + 360) % 360, 280)
    assert.equal(((second % 360) + 360) % 360, 200)
    assert.ok(second - first >= 6 * 360)
  })

  test('keeps labels aligned with their slice while flipping the lower half for readability', () => {
    assert.equal(getReadableLabelRotation(0), 0)
    assert.equal(getReadableLabelRotation(80), 0)
    assert.equal(getReadableLabelRotation(100), 180)
    assert.equal(getReadableLabelRotation(260), 180)
    assert.equal(getReadableLabelRotation(280), 0)
  })

  test('keeps the winning label upright after the wheel rotates it to the pointer', () => {
    const segmentCount = 11
    const prizeIndex = 7
    const sliceAngle = (360 / segmentCount) * prizeIndex
    const landedRotation = getTargetRotation(0, prizeIndex, segmentCount)

    assert.equal(getWheelLabelRotation(sliceAngle, landedRotation), 0)
    assert.equal(getWheelLabelRotation(sliceAngle + 180, landedRotation), 180)
  })

  test('formats disclosed probabilities without meaningless trailing zeros', () => {
    assert.equal(formatPrizeProbability(360_000), '36')
    assert.equal(formatPrizeProbability(47_000), '4.7')
    assert.equal(formatPrizeProbability(250), '0.025')
    assert.equal(formatPrizeProbability(1_500), '0.150')
  })
})
