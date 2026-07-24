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
import { useQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import { Activity, Gauge, ReceiptText, Waypoints } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatLogQuota } from '@/lib/format'
import { useIsAdmin } from '@/hooks/use-admin'
import { Skeleton } from '@/components/ui/skeleton'
import { getLogStats, getUserLogStats } from '../api'
import { DEFAULT_LOG_STATS } from '../constants'
import { buildApiParams } from '../lib/utils'
import { useUsageLogsContext } from './usage-logs-provider'

const route = getRouteApi('/_authenticated/usage-logs/$section')

function StatCard(props: {
  label: string
  value: string | number
  helper: string
  icon: React.ComponentType<{ className?: string }>
}) {
  const Icon = props.icon

  return (
    <div className='group border-border/70 bg-background/80 hover:border-foreground/20 flex min-h-24 origin-center items-center gap-3 rounded-xl border px-4 py-3 shadow-xs transition-[transform,box-shadow,border-color] duration-300 ease-out will-change-transform hover:-translate-y-1 hover:scale-[1.025] hover:shadow-lg motion-reduce:transform-none motion-reduce:transition-none'>
      <span className='border-border/70 bg-muted/25 group-hover:bg-muted/50 flex size-9 shrink-0 items-center justify-center rounded-lg border transition-[transform,background-color] duration-300 group-hover:scale-105 group-hover:-rotate-2 motion-reduce:transform-none'>
        <Icon className='text-foreground/80 size-4' />
      </span>
      <div className='min-w-0'>
        <p className='text-muted-foreground text-xs'>{props.label}</p>
        <p className='mt-0.5 truncate font-mono text-xl font-semibold tracking-tight tabular-nums'>
          {props.value}
        </p>
        <p className='text-muted-foreground mt-0.5 truncate text-[11px]'>
          {props.helper}
        </p>
      </div>
    </div>
  )
}

interface CommonLogsStatsProps {
  enabled: boolean
  totalCount: number
}

export function CommonLogsStats(props: CommonLogsStatsProps) {
  const { t } = useTranslation()
  const isAdmin = useIsAdmin()
  const searchParams = route.useSearch()
  const { sensitiveVisible } = useUsageLogsContext()

  const { data: stats, isPending } = useQuery({
    queryKey: ['usage-logs-stats', isAdmin, searchParams],
    enabled: props.enabled,
    queryFn: async () => {
      const params = buildApiParams({
        page: 1,
        pageSize: 1,
        searchParams,
        columnFilters: [],
        isAdmin,
      })

      const result = isAdmin
        ? await getLogStats(params)
        : await getUserLogStats(params)

      return result.success
        ? result.data || DEFAULT_LOG_STATS
        : DEFAULT_LOG_STATS
    },
    staleTime: 10_000,
    refetchOnWindowFocus: false,
  })

  if (!props.enabled || isPending) {
    return (
      <div className='grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4'>
        {Array.from({ length: 4 }).map((_, index) => (
          <Skeleton key={index} className='h-24 rounded-xl' />
        ))}
      </div>
    )
  }

  return (
    <div className='grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4'>
      <StatCard
        label={t('Request Count')}
        value={props.totalCount}
        helper={t('Total Count')}
        icon={ReceiptText}
      />
      <StatCard
        label={t('Total Usage')}
        value={sensitiveVisible ? formatLogQuota(stats?.quota || 0) : '••••'}
        helper={t('Usage')}
        icon={Waypoints}
      />
      <StatCard
        label={t('RPM')}
        value={stats?.rpm || 0}
        helper={t('Requests per minute')}
        icon={Gauge}
      />
      <StatCard
        label={t('TPM')}
        value={stats?.tpm || 0}
        helper={t('Tokens per minute')}
        icon={Activity}
      />
    </div>
  )
}
