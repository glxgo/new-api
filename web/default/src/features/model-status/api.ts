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

export type ChannelProbeAdminState = {
  channel_id: number
  channel_name: string
  channel_status: number
  groups: string[]
  status: 'checking' | 'healthy' | 'degraded' | 'unhealthy'
  model_name: string
  last_probe_ts: number
  last_success_ts: number
  last_failure_ts: number
  last_latency_ms: number
  last_ttft_ms: number
  has_ttft: boolean
  last_http_status: number
  last_error_code: string
  last_error_category: string
  last_error_message: string
  consecutive_failures: number
  consecutive_successes: number
}

export async function getChannelProbeStatus() {
  const response = await api.get<{
    success: boolean
    data: ChannelProbeAdminState[]
  }>('/api/channel/probe-status')
  return response.data
}

export async function probeChannelNow(channelId: number) {
  const response = await api.post(`/api/channel/probe/${channelId}`)
  return response.data
}
