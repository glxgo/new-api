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
import type { LuckyCard, LuckyPrize, LuckyRuleSet } from './types.ts'

export const PRIZE_NAMES: Record<string, string> = {
  quota_5: '$5 套餐额度',
  quota_10: '$10 套餐额度',
  quota_20: '$20 套餐额度',
  quota_30: '$30 套餐额度',
  quota_50: '$50 套餐额度',
  quota_100: '$100 套餐额度',
  gift_5: '$5 钱包赠金',
  gift_10: '$10 钱包赠金',
  gift_20: '$20 钱包赠金',
  subscription_double: '套餐双倍卡',
  subscription_full_reset: '套餐全额重置卡',
  crazy_5h: '5 小时狂蹬卡',
}

export interface WheelSegment extends LuckyPrize {
  label: string
}

export function selectLuckyRules(
  rules: LuckyRuleSet[],
  currentRuleId?: number
) {
  const publicRule = rules.find((rule) => rule.id === currentRuleId) || rules[0]
  const drawRule = publicRule
  return { drawRule, publicRule }
}

function availableCards(cards: LuckyCard[], poolType?: LuckyCard['pool_type']) {
  return cards
    .filter(
      (card) =>
        card.status === 'available' &&
        (poolType === undefined || card.pool_type === poolType)
    )
    .sort((a, b) => a.expires_at - b.expires_at || a.id - b.id)
}

function formatUsd(micros: number) {
  return (micros / 1_000_000).toLocaleString('en-US', {
    maximumFractionDigits: 2,
  })
}

export function chooseAvailableCardId(cards: LuckyCard[], currentId: string) {
  const available = availableCards(cards)
  if (available.some((card) => String(card.id) === currentId)) {
    return currentId
  }
  return available[0] ? String(available[0].id) : ''
}

export function chooseAvailableCardIdForPool(
  cards: LuckyCard[],
  currentId: string,
  poolType: LuckyCard['pool_type']
) {
  const available = availableCards(cards, poolType)
  if (available.some((card) => String(card.id) === currentId)) {
    return currentId
  }
  return available[0] ? String(available[0].id) : ''
}

export function chooseNextAvailableCardId(
  cards: LuckyCard[],
  currentId: string
) {
  const available = availableCards(cards)
  const currentIndex = available.findIndex(
    (card) => String(card.id) === currentId
  )
  if (available.length <= 1) return ''
  if (currentIndex < 0) return available[0] ? String(available[0].id) : ''
  return String(available[(currentIndex + 1) % available.length]?.id || '')
}

export function chooseNextAvailableCardIdForPool(
  cards: LuckyCard[],
  currentId: string,
  poolType: LuckyCard['pool_type']
) {
  const available = availableCards(cards, poolType)
  const currentIndex = available.findIndex(
    (card) => String(card.id) === currentId
  )
  if (available.length <= 1) return ''
  if (currentIndex < 0) return available[0] ? String(available[0].id) : ''
  return String(available[(currentIndex + 1) % available.length]?.id || '')
}

export function buildWheelSegments(
  pool: LuckyPrize[],
  _poolType: LuckyCard['pool_type']
): WheelSegment[] {
  return pool.map((prize) => {
    if (prize.code.startsWith('quota_')) {
      return {
        ...prize,
        label: `$${formatUsd(prize.display_usd_micros)} 套餐额度`,
      }
    }
    if (prize.code.startsWith('gift_')) {
      return {
        ...prize,
        label: `$${formatUsd(prize.display_usd_micros)} 钱包赠金`,
      }
    }
    return {
      ...prize,
      label: PRIZE_NAMES[prize.code] || prize.code,
    }
  })
}

export function buildWheelBackground(segmentCount: number) {
  const count = Math.max(segmentCount, 1)
  const slice = 360 / count
  const stops = Array.from({ length: count }, (_, index) => {
    const start = index * slice
    const end = (index + 1) * slice
    const color = index % 2 === 0 ? '#fff8ea' : '#f8c27c'
    return `${color} ${start}deg ${end}deg`
  })
  return `conic-gradient(from -${slice / 2}deg,${stops.join(',')})`
}

export function getTargetRotation(
  currentRotation: number,
  prizeIndex: number,
  segmentCount: number,
  minimumTurns = 6
) {
  const count = Math.max(segmentCount, 1)
  const normalizedCurrent = ((currentRotation % 360) + 360) % 360
  const target = (((360 - (prizeIndex * 360) / count) % 360) + 360) % 360
  const finalDelta = (target - normalizedCurrent + 360) % 360
  return currentRotation + minimumTurns * 360 + finalDelta
}

export function getReadableLabelRotation(angle: number) {
  const normalized = ((angle % 360) + 360) % 360
  return normalized > 90 && normalized < 270 ? 180 : 0
}

export function getWheelLabelRotation(
  sliceAngle: number,
  wheelRotation: number
) {
  return getReadableLabelRotation(sliceAngle + wheelRotation)
}

export function formatPrizeProbability(weight: number) {
  const probability = weight / 10_000
  if (probability < 1) return probability.toFixed(3)
  if (Number.isInteger(probability)) return probability.toFixed(0)
  return probability.toFixed(3).replace(/0+$/, '').replace(/\.$/, '')
}
