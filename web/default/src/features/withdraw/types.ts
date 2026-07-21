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
// Withdraw (rebate/dividend system) type definitions.

export interface ApiResponse<T = unknown> {
  success?: boolean
  message?: string
  data?: T
}

// Withdraw type: 1 = principal, 2 = commission/dividend (agent/admin/root).
export const WITHDRAW_TYPE = {
  PRINCIPAL: 1,
  DIVIDEND: 2,
} as const
export type WithdrawType = (typeof WITHDRAW_TYPE)[keyof typeof WITHDRAW_TYPE]

// Withdraw status: 0 = pending review, 1 = approved, 2 = rejected.
export const WITHDRAW_STATUS = {
  PENDING: 0,
  APPROVED: 1,
  REJECTED: 2,
} as const
export type WithdrawStatus =
  (typeof WITHDRAW_STATUS)[keyof typeof WITHDRAW_STATUS]

export interface Withdraw {
  id: number
  user_id: number
  type: WithdrawType
  amount: number // requested amount (quota units)
  fee: number // fee deducted (quota units)
  actual_amount: number // actual payout = amount - fee
  status: WithdrawStatus
  handler_id: number
  handler_name: string
  // Payment info (required for principal withdrawal by regular users)
  alipay_name: string
  alipay_account: string
  wechat_qrcode: string // base64, backup payment QR
  remark: string // review note
  created_at: number
  handled_at: number
}

export interface RequestWithdrawPayload {
  type: WithdrawType
  amount: number
  alipay_name?: string
  alipay_account?: string
  wechat_qrcode?: string
}

export interface WithdrawActionPayload {
  id: number
  remark?: string
}

export interface WithdrawListResponse {
  data: Withdraw[]
  total: number
}
