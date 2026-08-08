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
import type { ApiResponse } from '@/features/wallet/types'

export interface TopUpCoupon {
  id: number
  code: string
  title: string
  description: string
  discount: number
  user_limit: number
  enabled: boolean
  created_at: number
  updated_at: number
}

export async function getTopUpCoupons(): Promise<ApiResponse<TopUpCoupon[]>> {
  const response = await api.get('/api/topup-coupon/admin')
  return response.data
}

export async function saveTopUpCoupon(
  coupon: Partial<TopUpCoupon>
): Promise<ApiResponse<TopUpCoupon>> {
  const { id, ...payload } = coupon
  const response = id
    ? await api.put(`/api/topup-coupon/admin/${id}`, payload)
    : await api.post('/api/topup-coupon/admin', payload)
  return response.data
}

export async function deleteTopUpCoupon(id: number): Promise<ApiResponse> {
  const response = await api.delete(`/api/topup-coupon/admin/${id}`)
  return response.data
}
