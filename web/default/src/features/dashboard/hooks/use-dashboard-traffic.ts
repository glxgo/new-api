import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { computeTimeRange } from '@/lib/time'
import { getDashboardTraffic } from '@/features/dashboard/api'
import { getDefaultDays } from '@/features/dashboard/lib'
import type { DashboardFilters } from '@/features/dashboard/types'

export function useDashboardTraffic(
  filters: DashboardFilters | undefined,
  isAdmin: boolean
) {
  const params = useMemo(() => {
    const range = computeTimeRange(
      getDefaultDays(filters?.time_granularity),
      filters?.start_timestamp,
      filters?.end_timestamp
    )
    return {
      start_timestamp: range.start_timestamp,
      end_timestamp: range.end_timestamp,
      timezone_offset: new Date().getTimezoneOffset(),
    }
  }, [
    filters?.end_timestamp,
    filters?.start_timestamp,
    filters?.time_granularity,
  ])

  return useQuery({
    queryKey: [
      'dashboard',
      'traffic',
      isAdmin ? 'admin' : 'self',
      params.start_timestamp,
      params.end_timestamp,
      params.timezone_offset,
    ],
    queryFn: () => getDashboardTraffic(params, isAdmin),
    select: (response) => (response.success ? response.data : null),
    staleTime: 60_000,
  })
}
