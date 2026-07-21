import type { GroupProbeSummary } from '@/features/performance-metrics/types'

export type ProbeTone = 'healthy' | 'degraded' | 'unhealthy' | 'unknown'

export function probeTone(probe?: GroupProbeSummary): ProbeTone {
  if (!probe || probe.checked_channels === 0) return 'unknown'
  return probe.status
}

export function probeLabel(tone: ProbeTone) {
  if (tone === 'healthy') return '全部正常'
  if (tone === 'degraded') return '部分波动'
  if (tone === 'unhealthy') return '探测异常'
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
