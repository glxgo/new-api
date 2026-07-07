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
import { memo, useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Activity, AlertCircle, ChevronRight, Zap } from 'lucide-react'
import { cn } from '@/lib/utils'
import type { PricingModel } from '@/features/pricing/types'
import { getPerfMetrics } from '@/features/performance-metrics/api'
import type { PerformanceGroup } from '@/features/performance-metrics/types'
import { UptimeSparkline } from '@/features/pricing/components/model-details-uptime-sparkline'
import type { UptimeDayPoint } from '@/features/pricing/lib/mock-stats'

type Summary = {
  success_rate: number
  avg_latency_ms: number
  avg_tps: number
  cache_rate?: number
  request_count?: number
} | undefined

type Props = {
  model: PricingModel
  summary: Summary
  onClick: () => void
}

// 聚合多分组的逐桶 success_rate → 时序点（卡片展示整体分时绿柱）
function aggregateGroupSeries(groups: PerformanceGroup[]): UptimeDayPoint[] {
  const byTs = new Map<number, { sum: number; count: number; incidents: number }>()
  for (const g of groups) {
    for (const p of g.series) {
      const cur = byTs.get(p.ts) ?? { sum: 0, count: 0, incidents: 0 }
      cur.sum += p.success_rate
      cur.count += 1
      if (p.success_rate < 100) cur.incidents += 1
      byTs.set(p.ts, cur)
    }
  }
  return Array.from(byTs.entries())
    .sort(([a], [b]) => a - b)
    .map(([ts, v]) => ({
      date: new Date(ts * 1000).toISOString(),
      uptime_pct: v.count > 0 ? Math.round((v.sum / v.count) * 100) / 100 : 0,
      incidents: v.incidents,
      outage_minutes: 0,
    }))
}

function statusText(rate: number): string {
  if (rate >= 95) return 'text-emerald-600 dark:text-emerald-400'
  if (rate >= 90) return 'text-amber-600 dark:text-amber-400'
  return 'text-rose-600 dark:text-rose-400'
}

export const ModelStatusCard = memo(function ModelStatusCard({
  model,
  summary,
  onClick,
}: Props) {
  const { t } = useTranslation()
  const rate = summary?.success_rate
  const hasData = typeof rate === 'number' && rate >= 0
  const groupCount = model.enable_groups?.length ?? 0

  // 拉该模型 24h 逐分组时序（queryKey 与抽屉一致，缓存复用，打开抽屉不重复请求）
  const metricsQuery = useQuery({
    queryKey: ['perf-metrics', model.model_name],
    queryFn: () => getPerfMetrics(model.model_name, 24),
    staleTime: 5 * 60 * 1000,
  })
  const series = useMemo(
    () => aggregateGroupSeries(metricsQuery.data?.data?.groups ?? []),
    [metricsQuery.data]
  )

  return (
    <div
      role='button'
      tabIndex={0}
      onClick={onClick}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault()
          onClick()
        }
      }}
      className='hover:border-primary/50 bg-card group relative flex cursor-pointer flex-col gap-3 rounded-xl border p-4 text-left transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-ring'
    >
      <div className='flex items-start justify-between gap-3'>
        <div className='min-w-0 flex-1'>
          <h3 className='truncate font-medium'>{model.model_name}</h3>
          <p className='text-muted-foreground mt-0.5 text-xs'>
            {groupCount} {t('groups')}
          </p>
        </div>
        {hasData ? (
          <span
            className={cn(
              'shrink-0 text-lg font-bold tabular-nums',
              statusText(rate as number)
            )}
          >
            {(rate as number).toFixed(1)}%
          </span>
        ) : (
          <span className='text-muted-foreground shrink-0 text-xs'>—</span>
        )}
      </div>

      {/* 真实分时绿柱（24h 逐桶，聚合所有分组），替代之前的纯色条 */}
      <UptimeSparkline
        series={series}
        size='sm'
        showOverall={false}
        className='w-full'
        emptyLabel={
          metricsQuery.isLoading ? t('Loading...') : t('No data')
        }
      />

      <div className='text-muted-foreground flex items-center justify-between text-xs'>
        <span className='inline-flex items-center gap-3'>
          <span className='inline-flex items-center gap-1'>
            {hasData && (rate as number) < 95 ? (
              <AlertCircle className='size-3 text-amber-500' />
            ) : (
              <Activity className='size-3 text-emerald-500' />
            )}
            {summary?.avg_latency_ms ? `${(summary.avg_latency_ms / 1000).toFixed(2)}s` : t('No data')}
          </span>
          {typeof summary?.cache_rate === 'number' && summary.cache_rate > 0 && (
            <span className='inline-flex items-center gap-1'>
              <Zap className='size-3 text-sky-500' />
              {summary.cache_rate.toFixed(0)}% {t('cache')}
            </span>
          )}
        </span>
        <span className='group-hover:text-primary inline-flex items-center gap-0.5'>
          {t('Details')}
          <ChevronRight className='size-3 transition-transform group-hover:translate-x-0.5' />
        </span>
      </div>
    </div>
  )
})
