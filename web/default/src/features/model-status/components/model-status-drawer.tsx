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
import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Dialog } from '@/components/dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { GroupBadge } from '@/components/group-badge'
import { getPerfMetrics } from '@/features/performance-metrics/api'
import type { PerformanceGroup } from '@/features/performance-metrics/types'
import type { PricingModel } from '@/features/pricing/types'
import { UptimeSparkline } from '@/features/pricing/components/model-details-uptime-sparkline'
import type { UptimeDayPoint } from '@/features/pricing/lib/mock-stats'

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  model: PricingModel
}

// 把单个分组的逐桶 success_rate 映射成 UptimeSparkline 需要的时序点
function toGroupUptimeSeries(group: PerformanceGroup): UptimeDayPoint[] {
  return group.series.map((p) => ({
    date: new Date(p.ts * 1000).toISOString(),
    uptime_pct: Math.round(p.success_rate * 100) / 100,
    incidents: p.success_rate < 100 ? 1 : 0,
    outage_minutes: 0,
  }))
}

export function ModelStatusDrawer({ open, onOpenChange, model }: Props) {
  const { t } = useTranslation()
  const metricsQuery = useQuery({
    queryKey: ['perf-metrics', model.model_name],
    queryFn: () => getPerfMetrics(model.model_name, 24),
    enabled: open,
    staleTime: 60 * 1000,
  })

  const groups = metricsQuery.data?.data?.groups ?? []

  const sortedGroups = useMemo(
    () => [...groups].sort((a, b) => b.success_rate - a.success_rate),
    [groups]
  )

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={model.model_name}
      description={t('Health status by group in the last 24 hours')}
      contentClassName='sm:max-w-lg'
      contentHeight='auto'
      bodyClassName='space-y-3'
    >
      {metricsQuery.isLoading ? (
        <div className='space-y-3'>
          {Array.from({ length: 3 }).map((_, i) => (
            <Skeleton key={i} className='h-24 w-full rounded-lg' />
          ))}
        </div>
      ) : sortedGroups.length === 0 ? (
        <div className='text-muted-foreground py-10 text-center text-sm'>
          {t('No performance data yet for this model.')}
        </div>
      ) : (
        sortedGroups.map((group) => (
          <div key={group.group} className='rounded-lg border p-3'>
            <div className='mb-2 flex items-center justify-between gap-2'>
              <GroupBadge group={group.group} size='sm' />
              <span className='text-base font-bold tabular-nums'>
                {group.success_rate.toFixed(1)}%
              </span>
            </div>
            <UptimeSparkline
              series={toGroupUptimeSeries(group)}
              size='md'
              showOverall={false}
              maxPoints={24}
              fill
              className='w-full'
              emptyLabel={t('No data')}
            />
            <div className='text-muted-foreground mt-3 grid grid-cols-2 gap-2 text-xs sm:grid-cols-4'>
              <div>
                <div className='text-[10px] tracking-wide uppercase'>
                  {t('Latency')}
                </div>
                <div className='font-medium text-foreground'>
                  {group.avg_latency_ms > 0
                    ? `${(group.avg_latency_ms / 1000).toFixed(2)}s`
                    : '—'}
                </div>
              </div>
              <div>
                <div className='text-[10px] tracking-wide uppercase'>
                  {t('TTFT')}
                </div>
                <div className='font-medium text-foreground'>
                  {group.avg_ttft_ms > 0 ? `${(group.avg_ttft_ms / 1000).toFixed(2)}s` : '—'}
                </div>
              </div>
              <div>
                <div className='text-[10px] tracking-wide uppercase'>
                  {t('Throughput')}
                </div>
                <div className='font-medium text-foreground'>
                  {group.avg_tps > 0 ? `${group.avg_tps.toFixed(2)} tok/s` : '—'}
                </div>
              </div>
              <div>
                <div className='text-[10px] tracking-wide uppercase'>
                  {t('Cache')}
                </div>
                <div className='font-medium text-foreground'>
                  {group.cache_rate > 0 ? `${group.cache_rate.toFixed(0)}%` : '—'}
                </div>
              </div>
            </div>
          </div>
        ))
      )}
    </Dialog>
  )
}
