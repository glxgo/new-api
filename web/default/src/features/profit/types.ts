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
// Recharge commission audit type definitions (root only). Historical route
// names are retained for compatibility.

export interface ApiResponse<T = unknown> {
  success?: boolean
  message?: string
  data?: T
}

export interface ProfitSummary {
  start: number
  end: number
  paid_recharge_cents: number
  paid_order_count: number
  affiliate_rebate: number
  admin_dividend: number
  root_dividend: number
  total_commission: number
  legacy_commission_paid: number
  pending_reconciliation_count: number
}

// Dividend record type: 1=direct rebate, 2=indirect rebate, 3=admin, 4=root.
export const DIVIDEND_TYPE = {
  DIRECT: 1,
  INDIRECT: 2,
  ADMIN: 3,
  ROOT: 4,
} as const
export type DividendRecordType =
  (typeof DIVIDEND_TYPE)[keyof typeof DIVIDEND_TYPE]

// Commission records aggregate recipients for the same paid source. Version 0
// rows are immutable legacy settlements; version 1 rows use paid recharge.
export interface DividendRecord {
  source_user_id: number
  source_username: string
  batch_id: string
  source_ref: string
  policy_version: number
  source_recharge_cents: number
  amount: number
  record_count: number
  created_at: number
}

export interface DividendRecordsResponse {
  data: DividendRecord[]
  total: number
}
