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
// Affiliate program (邀新计划) type definitions.

export interface ApiResponse<T = unknown> {
  success?: boolean
  message?: string
  data?: T
}

// 邀新概览(邀请码/链接 + 直邀/间邀人数 + 累计返利 + 当前返利率)。
export interface AffiliateSummary {
  aff_code: string
  aff_link: string
  direct_count: number
  indirect_count: number
  total_rebate: number // quota 单位
  direct_rate: number // 小数, 0.05 = 5%
  indirect_rate: number
  rebate_withdrawable: boolean
}

// 下级结算汇总（不包含邮箱和当前钱包余额等账户隐私字段）。
export interface DownlineUser {
  id: number
  username: string
  created_at: number
  recharge_cents: number // 真实支付金额，分
  rebate: number // 该下级为我产生的累计返利(quota)
}

// 返利明细(=拉新返利 dividend_record, type 1=直接 2=间接)。
export interface RebateRecord {
  id: number
  batch_id: string
  user_id: number
  source_user_id: number
  log_id: number
  type: 1 | 2
  amount: number
  source_recharge_cents: number
  source_ref: string
  policy_version: number
  created_at: number
}

export interface ListResponse<T> {
  data: T[]
  total: number
}
