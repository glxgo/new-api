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
import { api } from '@/lib/api'
import type {
  ApiResponse,
  DividendRecordsResponse,
  ProfitSummary,
} from './types'

// Root-only: profit dashboard summary. start/end are unix seconds; defaults to current month.
export async function getProfitSummary(
  start?: number,
  end?: number
): Promise<ApiResponse<ProfitSummary>> {
  const params = new URLSearchParams()
  if (start !== undefined) params.set('start', String(start))
  if (end !== undefined) params.set('end', String(end))
  const query = params.toString()
  const res = await api.get(`/api/profit/summary${query ? `?${query}` : ''}`)
  return res.data
}

// Root-only: dividend audit records with optional filters.
export async function getDividendRecords(
  params: {
    page?: number
    page_size?: number
    user_id?: number
    source_user_id?: number
    type?: number
  } = {}
): Promise<ApiResponse<DividendRecordsResponse>> {
  const { page = 1, page_size = 20, ...rest } = params
  const qs = new URLSearchParams({
    page: String(page),
    page_size: String(page_size),
  })
  Object.entries(rest).forEach(([k, v]) => {
    if (v != null) qs.set(k, String(v))
  })
  const res = await api.get(`/api/profit/dividend_records?${qs.toString()}`)
  return res.data
}
