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
import { useEffect } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import { type ColumnDef } from '@tanstack/react-table'
import { useMediaQuery } from '@/hooks'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import { useIsAdmin } from '@/hooks/use-admin'
import { useTableUrlState } from '@/hooks/use-table-url-state'
import {
  DataTablePage,
  DataTableRow,
  useDataTable,
} from '@/components/data-table'
import { LOG_TYPE_ALL_VALUE, LOG_TYPE_ENUM } from '../constants'
import { useColumnsByCategory } from '../lib/columns'
import { usageLogsQueryOptions } from '../lib/queries'
import type { LogCategory } from '../types'
import { CommonLogsFilterBar } from './common-logs-filter-bar'
import { CommonLogsStats } from './common-logs-stats'
import { TaskLogsFilterBar } from './task-logs-filter-bar'
import { UsageLogsMobileList } from './usage-logs-mobile-card'

const route = getRouteApi('/_authenticated/usage-logs/$section')

const logTypeRowTint: Record<number, string> = {
  [LOG_TYPE_ENUM.ERROR]: 'bg-rose-50/40 dark:bg-rose-950/20',
  [LOG_TYPE_ENUM.REFUND]: 'bg-blue-50/30 dark:bg-blue-950/15',
}

function deserializeLogTypeFilter(value: unknown): unknown[] {
  const values = Array.isArray(value) ? value : value ? [value] : []
  return values.filter((item) => String(item) !== LOG_TYPE_ALL_VALUE)
}

interface UsageLogsTableProps {
  logCategory: LogCategory
}

export function UsageLogsTable({ logCategory }: UsageLogsTableProps) {
  const { t } = useTranslation()
  const isAdmin = useIsAdmin()
  const isMobile = useMediaQuery('(max-width: 640px)')
  const searchParams = route.useSearch()

  const {
    columnFilters,
    onColumnFiltersChange,
    pagination,
    onPaginationChange,
    ensurePageInRange,
  } = useTableUrlState({
    search: route.useSearch(),
    navigate: route.useNavigate(),
    pagination: { defaultPage: 1, defaultPageSize: isMobile ? 20 : 50 },
    globalFilter: { enabled: false },
    columnFilters: [
      {
        columnId: 'created_at',
        searchKey: 'type',
        type: 'array' as const,
        deserialize: deserializeLogTypeFilter,
      },
      { columnId: 'model_name', searchKey: 'model', type: 'string' as const },
      { columnId: 'token_name', searchKey: 'token', type: 'string' as const },
      { columnId: 'group', searchKey: 'group', type: 'string' as const },
      ...(isAdmin
        ? [
            {
              columnId: 'channel',
              searchKey: 'channel',
              type: 'string' as const,
            },
            {
              columnId: 'username',
              searchKey: 'username',
              type: 'string' as const,
            },
          ]
        : []),
    ],
  })

  const {
    data: queryResult,
    isLoading,
    isFetching,
  } = useQuery({
    ...usageLogsQueryOptions({
      logCategory,
      isAdmin,
      page: pagination.pageIndex + 1,
      pageSize: pagination.pageSize,
      searchParams,
      columnFilters,
    }),
    placeholderData: (previousData, previousQuery) => {
      if (previousQuery?.queryKey[1] === logCategory) {
        return previousData
      }
      return undefined
    },
  })

  useEffect(() => {
    if (queryResult?.errorMessage) {
      toast.error(
        queryResult.errorMessage === 'Failed to load logs'
          ? t('Failed to load logs')
          : queryResult.errorMessage
      )
    }
  }, [queryResult?.errorMessage, t])

  const data = queryResult?.data

  const logs = data?.items || []
  const columns = useColumnsByCategory(logCategory, isAdmin)
  const isLoadingData = isLoading || (isFetching && !data)

  const { table } = useDataTable({
    data: logs as Record<string, unknown>[],
    columns: columns as ColumnDef<Record<string, unknown>>[],
    columnFilters,
    pagination,
    enableRowSelection: false,
    onPaginationChange,
    onColumnFiltersChange,
    manualPagination: true,
    manualFiltering: true,
    totalCount: data?.total || 0,
    ensurePageInRange,
  })

  const isCommon = logCategory === 'common'

  return (
    <div className='flex h-full min-h-0 flex-col gap-2'>
      {isCommon && (
        <CommonLogsStats
          enabled={!isLoading && !isFetching}
          totalCount={data?.total || 0}
        />
      )}
      <div className='min-h-0 flex-1'>
        <DataTablePage
          table={table}
          columns={columns as ColumnDef<Record<string, unknown>>[]}
          isLoading={isLoadingData}
          isFetching={isFetching}
          emptyTitle={t('No Logs Found')}
          emptyDescription={t(
            'No usage logs available. Logs will appear here once API calls are made.'
          )}
          skeletonKeyPrefix='usage-log-skeleton'
          applyHeaderSize
          tableClassName={cn(
            'border-border/70 bg-background/75 rounded-xl shadow-xs [&_[data-slot=table]]:text-[13px] [&_[data-slot=table]_td]:text-[13px] [&_[data-slot=table]_td_*]:text-[13px] [&_[data-slot=table]_th]:text-[13px] [&_[data-slot=table]_th_*]:text-[13px]'
          )}
          mobile={
            <UsageLogsMobileList
              table={table}
              isLoading={isLoadingData}
              logCategory={logCategory}
            />
          }
          toolbar={
            isCommon ? (
              <CommonLogsFilterBar table={table} />
            ) : (
              <TaskLogsFilterBar table={table} logCategory={logCategory} />
            )
          }
          renderRow={(row) => {
            const logType = (row.original as Record<string, unknown>).type as
              | number
              | undefined
            const tintClass =
              isCommon && logType != null ? (logTypeRowTint[logType] ?? '') : ''

            return (
              <DataTableRow
                key={row.id}
                row={row}
                className={cn('transition-colors', tintClass)}
                getColumnClassName={() => (isCommon ? 'py-2' : 'py-3.5')}
              />
            )
          }}
        />
      </div>
    </div>
  )
}
