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
import { useState, type ReactNode } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import { Activity, Gauge, Layers, ReceiptText, Waypoints } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatLogQuota } from '@/lib/format'
import { useIsAdmin } from '@/hooks/use-admin'
import { useTokenCountFormatter } from '@/hooks/use-token-count-formatter'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { getLogStats, getUserLogStats, getAllLogs, getUserLogs } from '../api'
import { buildApiParams } from '../lib/utils'
import { useUsageLogsContext } from './usage-logs-provider'

const route = getRouteApi('/_authenticated/usage-logs/$section')

function StatCard(props: {
  label: string
  value: ReactNode
  helper?: string
  inlineHelper?: boolean
  icon: React.ComponentType<{ className?: string }>
}) {
  const Icon = props.icon

  return (
    <div className='group border-border/70 bg-background/80 hover:border-foreground/20 flex min-h-16 items-center gap-2.5 rounded-lg border px-3 py-2 transition-[transform,box-shadow,border-color] duration-200 hover:-translate-y-0.5 hover:shadow-[0_8px_24px_rgba(0,0,0,0.08)] dark:hover:shadow-[0_8px_24px_rgba(0,0,0,0.4)]'>
      <span className='border-border/70 bg-muted/25 group-hover:bg-muted/50 flex size-8 shrink-0 items-center justify-center rounded-md border transition-[transform,background-color] duration-200 group-hover:scale-105'>
        <Icon className='text-foreground/80 size-3.5' />
      </span>
      <div className='min-w-0'>
        <div className='flex min-w-0 items-baseline gap-2'>
          <p className='text-muted-foreground shrink-0 text-xs'>
            {props.label}
          </p>
          {props.inlineHelper && props.helper && (
            <span className='text-muted-foreground truncate text-[10px]'>
              {props.helper}
            </span>
          )}
        </div>
        <p className='truncate font-mono text-lg leading-tight font-semibold tracking-tight tabular-nums'>
          {props.value}
        </p>
        {!props.inlineHelper && props.helper && (
          <p className='text-muted-foreground truncate text-[10px]'>
            {props.helper}
          </p>
        )}
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
  const formatTokenCount = useTokenCountFormatter()
  const searchParams = route.useSearch()
  const { sensitiveVisible } = useUsageLogsContext()

  // Pagination does not change aggregate filters. Reuse the same cache while
  // browsing pages instead of issuing another SUM on every click.
  const statsParams = buildApiParams({
    page: 1,
    pageSize: 1,
    searchParams,
    columnFilters: [],
    isAdmin,
  })
  const countScope = JSON.stringify([isAdmin, statsParams])
  const [requestedCountScope, setRequestedCountScope] = useState('')
  const count = useQuery({
    queryKey: ['usage-logs-count', isAdmin, statsParams],
    enabled: props.enabled && requestedCountScope === countScope,
    queryFn: async ({ signal }) => {
      const response = isAdmin
        ? await getAllLogs(statsParams, signal)
        : await getUserLogs(statsParams, signal)
      if (!response.success || !response.data)
        throw new Error(response.message || 'Failed to load logs')
      return response.data.total
    },
    staleTime: 30_000,
    retry: false,
    refetchOnWindowFocus: false,
  })
  let countValue: ReactNode = count.data ?? props.totalCount
  if (props.totalCount < 0 && count.data === undefined) {
    countValue = (
      <Button
        variant='ghost'
        size='sm'
        disabled={count.isFetching}
        onClick={() => {
          setRequestedCountScope(countScope)
          if (requestedCountScope === countScope) void count.refetch()
        }}
      >
        {count.isFetching ? t('Loading...') : t('Calculate total')}
      </Button>
    )
  }
  const {
    data: stats,
    isPending,
    isError,
  } = useQuery({
    queryKey: ['usage-logs-stats', isAdmin, statsParams],
    enabled: props.enabled,
    queryFn: async ({ signal }) => {
      const params = statsParams

      const result = isAdmin
        ? await getLogStats(params, signal)
        : await getUserLogStats(params, signal)

      if (!result.success || !result.data)
        throw new Error(result.message || 'Failed to load logs')
      return result.data
    },
    staleTime: 30_000,
    retry: false,
    refetchOnWindowFocus: false,
  })

  if (isPending) {
    return (
      <div className='grid grid-cols-1 gap-2.5 sm:grid-cols-2 lg:grid-cols-5'>
        {Array.from({ length: 5 }).map((_, index) => (
          <Skeleton key={index} className='h-16 rounded-lg' />
        ))}
      </div>
    )
  }

  function renderQuota(): ReactNode {
    if (isError) return t('Unavailable')
    if (!sensitiveVisible) return '••••'
    return (
      <span className='inline-flex flex-wrap items-baseline gap-2'>
        {(stats?.pre_discount_quota ?? 0) > (stats?.quota ?? 0) && (
          <span className='text-muted-foreground text-sm font-normal line-through'>
            {formatLogQuota(stats?.pre_discount_quota ?? 0)}
          </span>
        )}
        <span>{formatLogQuota(stats?.quota ?? 0)}</span>
      </span>
    )
  }

  return (
    <div className='grid grid-cols-1 gap-2.5 sm:grid-cols-2 lg:grid-cols-5'>
      <StatCard
        label={t('Request Count')}
        value={countValue}
        icon={ReceiptText}
      />
      <StatCard
        label={t('Total Usage')}
        value={renderQuota()}
        icon={Waypoints}
      />
      <StatCard
        label={t('Total Tokens')}
        value={stats?.tokens == null ? '—' : formatTokenCount(stats.tokens)}
        icon={Layers}
      />
      <StatCard
        label={t('RPM')}
        value={isError ? '—' : (stats?.rpm ?? 0)}
        helper={t('Requests per minute')}
        inlineHelper
        icon={Gauge}
      />
      <StatCard
        label={t('TPM')}
        value={isError ? '—' : formatTokenCount(stats?.tpm ?? 0)}
        helper={t('Tokens per minute')}
        inlineHelper
        icon={Activity}
      />
    </div>
  )
}
