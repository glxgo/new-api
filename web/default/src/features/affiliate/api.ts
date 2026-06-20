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
  AffiliateSummary,
  ApiResponse,
  DownlineUser,
  ListResponse,
  RebateRecord,
} from './types'

export async function getAffiliateSummary(): Promise<ApiResponse<AffiliateSummary>> {
  const res = await api.get('/api/user/affiliate/summary')
  return res.data
}

export async function getAffiliateDownline(
  layer: 1 | 2,
  page = 1,
  page_size = 10
): Promise<ApiResponse<ListResponse<DownlineUser>>> {
  const res = await api.get(
    `/api/user/affiliate/downline?layer=${layer}&page=${page}&page_size=${page_size}`
  )
  return res.data
}

export async function getAffiliateRebates(
  page = 1,
  page_size = 10
): Promise<ApiResponse<ListResponse<RebateRecord>>> {
  const res = await api.get(
    `/api/user/affiliate/rebates?page=${page}&page_size=${page_size}`
  )
  return res.data
}
