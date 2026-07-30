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
export interface LuckyCampaign {
  id: number
  name: string
  issuance_paused: boolean
  draw_paused: boolean
}

export interface LuckyRechargeProgress {
  eligible_cents: number
  highest_awarded_stage: number
  next_threshold_cents: number
}

export interface LuckySubscriptionProgress {
  subscribed: boolean
  eligible: boolean
  next_card_at: number
}

export interface LuckyWheelStatus {
  campaign: LuckyCampaign
  rule_set_id: number
  available_cards: number
  recharge_progress: LuckyRechargeProgress
  subscription_progress: LuckySubscriptionProgress
  server_time: number
}

export interface LuckyCard {
  id: number
  rule_set_id: number
  pool_type: 'subscription' | 'recharge'
  source_type: string
  source_ref: string
  source_subscription_id: number
  status: string
  issued_at: number
  expires_at: number
}

export interface LuckyDraw {
  id: number
  card_id: number
  prize_type: string
  display_usd_micros: number
  actual_usd_micros: number
  awarded_quota: number
  reward_subscription_id: number
  gift_quota_awarded: number
  awarded_at: number
}

export interface LuckyPrize {
  code: string
  display_usd_micros: number
  weight: number
}

export interface LuckyRuleSet {
  id: number
  version: number
  subscription_pool: string
  recharge_pool: string
  recharge_bonus_usd_micros: number
  activity_group: string
}

export interface PageResult<T> {
  items: T[]
  total: number
  page: number
  page_size: number
}
