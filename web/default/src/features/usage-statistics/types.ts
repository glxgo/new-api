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

export type UsageStatisticsRange = '24h' | '7d' | '30d'

export interface UsageStatisticsSummary {
  request_count: number
  success_count: number
  error_count: number
  success_rate: number
  quota: number
  wallet_quota: number
  subscription_quota: number
  prompt_tokens: number
  cache_tokens: number
  effective_prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  cache_hit_rate: number
}

export interface UsageStatisticsPoint {
  timestamp: number
  request_count: number
  success_count: number
  error_count: number
  quota: number
  total_tokens: number
  cache_tokens: number
  effective_prompt_tokens: number
}

export interface UsageStatisticsModel {
  model_name: string
  request_count: number
  quota: number
  prompt_tokens: number
  cache_tokens: number
  completion_tokens: number
  total_tokens: number
}

export interface UsageStatisticsSubscription {
  subscription_id: number
  title: string
  request_count: number
  quota: number
}

export interface UsageStatisticsData {
  range: UsageStatisticsRange
  start_timestamp: number
  end_timestamp: number
  bucket_seconds: number
  summary: UsageStatisticsSummary
  series: UsageStatisticsPoint[]
  models: UsageStatisticsModel[]
  subscriptions: UsageStatisticsSubscription[]
}

export interface UsageStatisticsResponse {
  success: boolean
  message?: string
  data?: UsageStatisticsData
}
