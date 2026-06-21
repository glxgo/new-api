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
// Profit dashboard + dividend audit type definitions (root only).

export interface ApiResponse<T = unknown> {
  success?: boolean
  message?: string
  data?: T
}

// Profit summary over a time range (all amounts in quota units).
// Note: total_consume/total_cost are real-time (all logs incl. unsettled);
// settled_gross/rebate/dividend/net_profit are T+1-settled figures (lagged).
export interface ProfitSummary {
  start: number
  end: number
  total_consume: number // all-site consumption
  total_cost: number // all-site cost
  settled_gross: number // settled gross profit
  affiliate_rebate: number // settled referral rebate (direct + indirect)
  admin_dividend: number // settled admin dividend
  root_dividend: number // settled root dividend
  net_profit: number // = settled_gross - rebate - admin - root
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

// Dividend record aggregated by source_user + batch(day): same consuming user in the
// same batch (one T+1 day) merged into one row. type filter is applied before aggregation.
export interface DividendRecord {
  source_user_id: number // user whose consumption generated the profit
  batch_id: string // e.g. "2026-06-16"
  gross_profit: number // sum of gross profit (quota)
  amount: number // sum of dividend amount (quota)
  record_count: number // how many dividend records merged into this row
  created_at: number
}

export interface DividendRecordsResponse {
  data: DividendRecord[]
  total: number
}
