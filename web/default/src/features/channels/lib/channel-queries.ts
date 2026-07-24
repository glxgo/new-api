import { queryOptions } from '@tanstack/react-query'
import { getChannels, getGroups, searchChannels } from '../api'
import { DEFAULT_PAGE_SIZE } from '../constants'
import type { SearchChannelsParams } from '../types'

export const channelsQueryKeys = {
  all: ['channels'] as const,
  lists: () => [...channelsQueryKeys.all, 'list'] as const,
  list: (params: SearchChannelsParams) =>
    [...channelsQueryKeys.lists(), params] as const,
  details: () => [...channelsQueryKeys.all, 'detail'] as const,
  detail: (id: number) => [...channelsQueryKeys.details(), id] as const,
}

function compactParams(params: SearchChannelsParams): SearchChannelsParams {
  return Object.fromEntries(
    Object.entries(params).filter(([, value]) => value !== undefined)
  ) as SearchChannelsParams
}

export function channelListQueryOptions(params: SearchChannelsParams) {
  const normalized = compactParams({
    ...params,
    keyword: params.keyword?.trim() || undefined,
    model: params.model?.trim() || undefined,
  })
  const shouldSearch = Boolean(normalized.keyword || normalized.model)

  return queryOptions({
    queryKey: [...channelsQueryKeys.list(normalized), shouldSearch] as const,
    queryFn: () =>
      shouldSearch ? searchChannels(normalized) : getChannels(normalized),
    staleTime: 15_000,
  })
}

export const channelGroupsQueryOptions = queryOptions({
  queryKey: ['groups'],
  queryFn: getGroups,
  staleTime: 30_000,
})

export function getDefaultChannelPageSize(): number {
  if (typeof window === 'undefined') return DEFAULT_PAGE_SIZE
  return window.matchMedia('(max-width: 640px)').matches
    ? 10
    : DEFAULT_PAGE_SIZE
}

export function readChannelListPreferences(): {
  tagMode: boolean
  idSort: boolean
} {
  if (typeof window === 'undefined') {
    return { tagMode: false, idSort: false }
  }
  return {
    tagMode: window.localStorage.getItem('enable-tag-mode') === 'true',
    idSort: window.localStorage.getItem('channels-id-sort') === 'true',
  }
}

export function firstActiveFilter(values?: string[]): string | undefined {
  return values?.find((value) => value !== 'all')
}

export function firstActiveNumberFilter(values?: string[]): number | undefined {
  const value = firstActiveFilter(values)
  if (value === undefined) return undefined
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : undefined
}
