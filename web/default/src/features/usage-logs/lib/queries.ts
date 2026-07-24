import { queryOptions } from '@tanstack/react-query'
import { DEFAULT_LOGS_DATA } from '../constants'
import type { FetchLogsConfig, GetLogsResponse } from '../types'
import { fetchLogsByCategory } from './utils'

type UsageLogsData =
  | NonNullable<GetLogsResponse['data']>
  | typeof DEFAULT_LOGS_DATA

export interface UsageLogsQueryResult {
  data: UsageLogsData
  errorMessage?: string
}

export function usageLogsQueryOptions(config: FetchLogsConfig) {
  const requestConfig: FetchLogsConfig = {
    ...config,
    // URL search is the canonical filter state. Omitting the mirrored table
    // state makes route-prefetched and component queries share one cache key.
    columnFilters: [],
  }

  return queryOptions({
    queryKey: ['logs', requestConfig.logCategory, requestConfig],
    queryFn: async (): Promise<UsageLogsQueryResult> => {
      const result = await fetchLogsByCategory(requestConfig)
      if (!result?.success) {
        return {
          data: DEFAULT_LOGS_DATA,
          errorMessage: result?.message || 'Failed to load logs',
        }
      }
      return { data: result.data || DEFAULT_LOGS_DATA }
    },
    staleTime: 10_000,
  })
}

export function getDefaultUsageLogsPageSize(): number {
  if (typeof window === 'undefined') return 50
  return window.matchMedia('(max-width: 640px)').matches ? 20 : 50
}
