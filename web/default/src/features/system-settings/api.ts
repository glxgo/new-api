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
  ConfirmPaymentComplianceResponse,
  DeleteLogsResponse,
  FetchUpstreamRatiosRequest,
  SystemOptionsResponse,
  UpdateOptionRequest,
  UpdateOptionResponse,
  UpstreamChannelsResponse,
  UpstreamRatiosResponse,
} from './types'

export async function getSystemOptions() {
  const res = await api.get<SystemOptionsResponse>('/api/option/')
  return res.data
}

export async function updateSystemOption(request: UpdateOptionRequest) {
  const res = await api.put<UpdateOptionResponse>('/api/option/', request)
  return res.data
}

export async function confirmPaymentCompliance() {
  const res = await api.post<ConfirmPaymentComplianceResponse>(
    '/api/option/payment_compliance',
    { confirmed: true }
  )
  return res.data
}

export async function deleteLogsBefore(targetTimestamp: number) {
  const res = await api.delete<DeleteLogsResponse>('/api/log/', {
    params: { target_timestamp: targetTimestamp },
  })
  return res.data
}

export async function resetModelRatios() {
  const res = await api.post<UpdateOptionResponse>(
    '/api/option/rest_model_ratio'
  )
  return res.data
}

export async function getUpstreamChannels() {
  const res = await api.get<UpstreamChannelsResponse>(
    '/api/ratio_sync/channels'
  )
  return res.data
}

export async function fetchUpstreamRatios(request: FetchUpstreamRatiosRequest) {
  const res = await api.post<UpstreamRatiosResponse>(
    '/api/ratio_sync/fetch',
    request
  )
  return res.data
}

// 分组定价预览(2026-06-22): 后端按「官方价 × GroupRatio / GroupCostRatio」算出每个模型
// 在某分组下的实际售价/成本/毛利。成本字段仅 Root 返回。详见 controller/group_pricing_preview.go。
export async function getGroupPricingPreview(
  group: string,
  includeDisabled: boolean
) {
  const res = await api.get('/api/option/pricing/group-preview', {
    params: { group, include_disabled: includeDisabled },
  })
  return res.data as {
    success: boolean
    message?: string
    data: {
      group: string
      is_auto: boolean
      sale_ratio: number
      cost_ratio?: number
      cost_ratio_source?: string
      message?: string
      items: Array<{
        model: string
        billing_mode: string
        has_price: boolean
        base_input_price_per_m?: number
        base_output_price_per_m?: number
        base_cache_price_per_m?: number
        final_input_price_per_m?: number
        final_output_price_per_m?: number
        final_cache_price_per_m?: number
        base_request_price?: number
        final_request_price?: number
        final_input_cost_per_m?: number
        final_output_cost_per_m?: number
        final_cache_cost_per_m?: number
        final_request_cost?: number
        gross_margin?: number
        enabled_channel_count: number
        total_channel_count: number
        statuses: string[]
      }>
    }
  }
}
