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

export interface PlatformSiteUsage {
  request_count: number
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  pre_discount_quota: number
  exact_quota_log_count: number
  estimated_quota_log_count: number
}

export interface CPAQuotaWindow {
  id: string
  label: string
  used_percent: number | null
  remaining_percent: number | null
  reset_at: number | null
  window_seconds: number | null
}

export interface CPAAccountUsage {
  code: string
  masked_email: string
  plan_type: string
  available: boolean
  enabled: boolean
  windows: CPAQuotaWindow[]
}

export interface CPAModelUsage {
  model: string
  alias: string
  provider: string
  requests: number
  failed: number
  total_tokens: number
  input_tokens: number
  output_tokens: number
  reasoning_tokens: number
  cached_tokens: number
  cache_read_tokens: number
  cache_creation_tokens: number
  cost_usd: number
  cost_available: boolean
  avg_latency_ms: number
  avg_ttft_ms: number
  output_tokens_per_second: number
  slow_requests: number
  slow_ttft_requests: number
}

export interface CPAUsageSnapshot {
  configured: boolean
  status:
    | 'unconfigured'
    | 'syncing'
    | 'fresh'
    | 'partial'
    | 'stale'
    | 'unavailable'
  updated_at: number
  next_refresh_at: number
  accounts: CPAAccountUsage[]
  models: CPAModelUsage[]
}

export interface PlatformUsageData {
  timezone: string
  start_timestamp: number
  end_timestamp: number
  site: PlatformSiteUsage
  cpa: CPAUsageSnapshot
}

export interface PlatformUsageResponse {
  success: boolean
  message?: string
  data?: PlatformUsageData
}
