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
export type PerformanceSeriesPoint = {
  ts: number
  request_count?: number
  success_count?: number
  avg_ttft_ms: number
  avg_latency_ms: number
  success_rate: number
  avg_tps: number
  cache_rate: number
}

export type PerformanceGroup = {
  group: string
  avg_ttft_ms: number
  avg_latency_ms: number
  success_rate: number
  avg_tps: number
  cache_rate: number
  series: PerformanceSeriesPoint[]
}

export type PerformanceMetricsData = {
  success: boolean
  message?: string
  data: {
    model_name: string
    series_schema?: string
    groups: PerformanceGroup[]
  }
}

export type PerfModelSummary = {
  model_name: string
  avg_latency_ms: number
  success_rate: number
  avg_tps: number
  cache_rate: number
  request_count?: number
}

export type PerfSummaryAllData = {
  success: boolean
  message?: string
  data: {
    models: PerfModelSummary[]
  }
}

export type GroupCacheSummary = {
  group: string
  avg_ttft_ms?: number
  avg_latency_ms?: number
  success_rate?: number
  avg_tps?: number
  cache_rate: number
  request_count: number
  success_count?: number
  cache_tokens: number
  prompt_tokens: number
  series?: PerformanceSeriesPoint[]
  probe?: GroupProbeSummary
}

export type ProbeSeriesPoint = {
  ts: number
  probe_count: number
  success_count: number
  success_rate: number
  avg_latency_ms: number
  avg_ttft_ms: number
  total_channels: number
  checked_channels: number
  healthy_channels: number
}

export type GroupProbeSummary = {
  status: 'healthy' | 'degraded' | 'unhealthy' | 'unknown'
  total_channels: number
  checked_channels: number
  healthy_channels: number
  degraded_channels: number
  unhealthy_channels: number
  success_rate: number
  avg_latency_ms: number
  avg_ttft_ms: number
  last_probe_ts: number
  last_success_ts: number
  last_error_category?: string
  last_error_code?: string
  series: ProbeSeriesPoint[]
}

export type GroupSummaryAllData = {
  success: boolean
  message?: string
  data: {
    groups: GroupCacheSummary[]
    available_groups?: string[]
    group_ratios?: Record<string, number>
  }
}
