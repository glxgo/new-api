import { api } from '@/lib/api'

export type ChannelMetric = {
  id: number
  name: string
  type: number
  status: number
  request_count: number
  success_count: number
  success_rate: number | null
  cache_rate: number | null
  avg_ttft_ms: number | null
  ttft_count: number
  legacy_resolution: boolean
}

export type ChannelMetricsParams = {
  period: 'hour' | 'day'
  date: string
  keyword: string
  p: number
  page_size: number
}

export type ChannelMetricsPage = {
  items: ChannelMetric[]
  total: number
  page: number
  page_size: number
  start_ts: number
  end_ts: number
  timezone: string
  enabled: boolean
  flush_interval_minutes: number
}

export async function getChannelMetrics(
  params: ChannelMetricsParams,
  signal?: AbortSignal
) {
  const { data } = await api.get<{
    success: boolean
    message?: string
    data: ChannelMetricsPage
  }>('/api/channel/metrics', { params, signal, disableDuplicate: true })
  if (!data.success)
    throw new Error(data.message || 'Failed to load channel metrics')
  return data.data
}

export function shanghaiDate(now = new Date()) {
  return new Intl.DateTimeFormat('en-CA', {
    timeZone: 'Asia/Shanghai',
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).format(now)
}
