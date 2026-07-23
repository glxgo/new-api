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
import { useEffect, useState } from 'react'
import { useAuthStore } from '@/stores/auth-store'
import { formatNumber, formatQuota } from '@/lib/format'
import { computeTimeRange } from '@/lib/time'
import { Skeleton } from '@/components/ui/skeleton'
import { getUserQuotaDates } from '@/features/dashboard/api'
import { useModelStatCardsConfig } from '@/features/dashboard/hooks/use-dashboard-config'
import { useDashboardTraffic } from '@/features/dashboard/hooks/use-dashboard-traffic'
import {
  buildQueryParams,
  calculateDashboardStats,
  getDefaultDays,
} from '@/features/dashboard/lib'
import type {
  QuotaDataItem,
  DashboardFilters,
} from '@/features/dashboard/types'

// 能量条渐变（每卡一色），模仿 suning dashboard 多彩进度条配色
const CARD_ACCENTS = [
  'from-sky-500 to-blue-500',
  'from-violet-500 to-purple-500',
  'from-emerald-500 to-teal-500',
  'from-amber-500 to-orange-500',
  'from-rose-500 to-pink-500',
] as const

interface LogStatCardsProps {
  filters?: DashboardFilters
  onDataUpdate?: (data: QuotaDataItem[], loading: boolean) => void
}

export function LogStatCards(props: LogStatCardsProps) {
  const statCardsConfig = useModelStatCardsConfig()
  const user = useAuthStore((state) => state.auth.user)
  const isAdmin = !!(user?.role && user.role >= 10)
  const [stats, setStats] = useState<{
    totalQuota: number
    totalCount: number
    totalTokens: number
  } | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(false)

  const [timeRangeMinutes, setTimeRangeMinutes] = useState(0)

  const { filters, onDataUpdate } = props
  const trafficQuery = useDashboardTraffic(filters, isAdmin)

  useEffect(() => {
    const abortController = new AbortController()
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setLoading(true)

    setError(false)
    onDataUpdate?.([], true)

    const timeRange = computeTimeRange(
      getDefaultDays(filters?.time_granularity),
      filters?.start_timestamp,
      filters?.end_timestamp
    )
    const timeDiff = (timeRange.end_timestamp - timeRange.start_timestamp) / 60
    setTimeRangeMinutes(timeDiff)

    getUserQuotaDates(buildQueryParams(timeRange, filters), isAdmin)
      .then((res) => {
        if (abortController.signal.aborted) return
        const data = res?.data || []
        setStats(calculateDashboardStats(data))
        onDataUpdate?.(data, false)
      })
      .catch(() => {
        if (abortController.signal.aborted) return
        setStats(null)
        setError(true)
        onDataUpdate?.([], false)
      })
      .finally(() => {
        if (!abortController.signal.aborted) {
          setLoading(false)
        }
      })

    return () => {
      abortController.abort()
    }
  }, [filters, isAdmin, onDataUpdate])

  const adaptedStats = {
    rpm: stats?.totalCount ?? 0,
    quota: stats?.totalQuota ?? 0,
    tpm: stats?.totalTokens ?? 0,
    avgRpm: trafficQuery.data?.summary.avg_rpm ?? 0,
  }

  const items = statCardsConfig.map((config) => ({
    key: config.key,
    title: config.title,
    value:
      config.key === 'quota'
        ? formatQuota(config.getValue(adaptedStats, timeRangeMinutes))
        : formatNumber(config.getValue(adaptedStats, timeRangeMinutes)),
    desc: config.description,
    icon: config.icon,
  }))

  return (
    <div className='grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5'>
      {items.map((it, idx) => {
        const Icon = it.icon
        const accent = CARD_ACCENTS[idx % CARD_ACCENTS.length]
        return (
          <div
            key={it.title}
            className='group border-border bg-card rounded-xl border p-4 shadow-sm transition-all duration-300 hover:scale-[1.03] hover:shadow-md'
          >
            <div className='flex items-center gap-2'>
              <Icon className='text-muted-foreground/70 size-3.5 shrink-0' />
              <div className='text-muted-foreground truncate text-xs font-medium tracking-wider uppercase'>
                {it.title}
              </div>
            </div>

            {loading || (it.key === 'avgRpm' && trafficQuery.isLoading) ? (
              <Skeleton className='mt-2 h-7 w-24' />
            ) : error || (it.key === 'avgRpm' && trafficQuery.isError) ? (
              <div className='text-muted-foreground mt-2 font-mono text-lg font-bold tracking-tight tabular-nums sm:text-2xl'>
                --
              </div>
            ) : (
              <div className='text-foreground mt-2 font-mono text-lg font-bold tracking-tight tabular-nums sm:text-2xl'>
                {it.value}
              </div>
            )}

            {/* 能量条：默认 50%，hover 充到 80%（模仿 suning dashboard）*/}
            <div className='bg-muted mt-3 h-1.5 w-full overflow-hidden rounded-full'>
              <div
                className={`h-full w-full origin-left scale-x-50 rounded-full bg-gradient-to-r ${accent} transition-transform duration-500 ease-out group-hover:scale-x-[0.8]`}
              />
            </div>

            <div className='text-muted-foreground/60 mt-1.5 hidden text-xs md:block'>
              {it.desc}
            </div>
          </div>
        )
      })}
    </div>
  )
}
