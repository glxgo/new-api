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
import type {
  GroupProbeSummary,
  ProbeSeriesPoint,
} from '@/features/performance-metrics/types'

export type ProbeTone = 'healthy' | 'degraded' | 'unhealthy' | 'unknown'

export function probeSeriesHealthyPercentage(point: ProbeSeriesPoint): number {
  if (point.total_channels < 1) return 0
  return Math.min(
    100,
    Math.max(0, (point.healthy_channels / point.total_channels) * 100)
  )
}

export function probeSeriesTone(point: ProbeSeriesPoint): ProbeTone {
  if (point.total_channels < 1 || point.checked_channels < 1) return 'unknown'
  if (point.healthy_channels < 1) return 'unhealthy'
  if (point.healthy_channels * 2 >= point.total_channels) return 'healthy'
  return 'degraded'
}

export function probeTone(probe?: GroupProbeSummary): ProbeTone {
  if (!probe || probe.total_channels < 1 || probe.checked_channels < 1)
    return 'unknown'
  if (
    probe.checked_channels === probe.total_channels &&
    !probe.healthy_channels
  )
    return 'unhealthy'
  if (probe.healthy_channels * 2 >= probe.total_channels) return 'healthy'
  return 'degraded'
}

export function probeLabel(tone: ProbeTone, probe?: GroupProbeSummary) {
  if (tone === 'healthy') {
    if (probe && probe.healthy_channels < probe.total_channels)
      return '部分正常'
    return '全部正常'
  }
  if (tone === 'degraded') {
    if (probe?.healthy_channels) return '部分正常'
    return '状态确认中'
  }
  if (tone === 'unhealthy') return '服务异常'
  return '等待探测'
}

export function formatProbeTime(timestamp?: number, now = Date.now()) {
  if (!timestamp) return '尚未探测'
  const seconds = Math.max(0, Math.floor(now / 1000 - timestamp))
  if (seconds < 60) return '刚刚'
  if (seconds < 3600) return `${Math.floor(seconds / 60)} 分钟前`
  if (seconds < 86400) return `${Math.floor(seconds / 3600)} 小时前`
  return `${Math.floor(seconds / 86400)} 天前`
}

export function probeErrorLabel(category?: string) {
  const labels: Record<string, string> = {
    timeout: '上游超时',
    authentication: '鉴权失败',
    rate_limit: '上游限流',
    upstream: '上游异常',
    response: '响应异常',
    configuration: '配置异常',
    unknown: '未知异常',
  }
  return category ? (labels[category] ?? '未知异常') : ''
}
