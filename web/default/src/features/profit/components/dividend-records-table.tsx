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
import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { type ColumnDef } from '@tanstack/react-table'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatQuota } from '@/lib/format'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { DataTablePage, useDataTable } from '@/components/data-table'
import { getDividendRecords } from '../api'
import { type DividendRecord } from '../types'

const PAGE_SIZE = 20

function formatRechargeCents(cents: number) {
  return new Intl.NumberFormat('zh-CN', {
    style: 'currency',
    currency: 'CNY',
    minimumFractionDigits: 2,
  }).format((cents || 0) / 100)
}

export function DividendRecordsTable() {
  const { t } = useTranslation()
  const [page, setPage] = useState(0)
  const [typeFilter, setTypeFilter] = useState<number>(-1) // -1 = all

  const { data, isLoading, isFetching } = useQuery({
    queryKey: ['dividend-records', page, typeFilter, t],
    queryFn: async () => {
      const res = await getDividendRecords({
        page: page + 1,
        page_size: PAGE_SIZE,
        type: typeFilter >= 0 ? typeFilter : undefined,
      })
      if (!res.success) {
        toast.error(res.message || t('Failed to load data'))
        return { data: [] as DividendRecord[], total: 0 }
      }
      return {
        data: res.data?.data ?? [],
        total: res.data?.total ?? 0,
      }
    },
    placeholderData: (prev) => prev,
  })

  const columns = useMemo<ColumnDef<DividendRecord>[]>(
    () => [
      { accessorKey: 'batch_id', header: t('Date'), size: 120 },
      {
        accessorKey: 'source_user_id',
        header: t('Source User'),
        size: 150,
        cell: ({ row }) => (
          <div className='min-w-0'>
            <div className='truncate text-sm font-medium'>
              {row.original.source_username ||
                `#${row.original.source_user_id}`}
            </div>
            <div className='text-muted-foreground text-xs'>
              #{row.original.source_user_id}
            </div>
          </div>
        ),
      },
      {
        accessorKey: 'source_recharge_cents',
        header: t('Recharge'),
        size: 120,
        cell: ({ row }) => (
          <span className='font-mono text-sm'>
            {formatRechargeCents(row.original.source_recharge_cents)}
          </span>
        ),
      },
      {
        accessorKey: 'policy_version',
        header: t('结算策略'),
        size: 120,
        cell: ({ row }) => (
          <span className='text-muted-foreground text-sm'>
            {row.original.policy_version === 1
              ? t('充值固定比例')
              : t('历史已结算')}
          </span>
        ),
      },
      {
        accessorKey: 'amount',
        header: t('分润金额'),
        size: 120,
        cell: ({ row }) => (
          <span className='font-mono font-semibold'>
            {formatQuota(row.original.amount)}
          </span>
        ),
      },
      {
        accessorKey: 'record_count',
        header: t('记录数'),
        size: 90,
        cell: ({ row }) => (
          <span className='text-muted-foreground text-xs'>
            {row.original.record_count}
          </span>
        ),
      },
    ],
    [t]
  )

  const { table } = useDataTable({
    data: data?.data ?? [],
    columns,
    manualPagination: true,
    manualFiltering: true,
    totalCount: data?.total ?? 0,
    pagination: { pageIndex: page, pageSize: PAGE_SIZE },
    onPaginationChange: (updater) => {
      const next =
        typeof updater === 'function'
          ? updater({ pageIndex: page, pageSize: PAGE_SIZE })
          : updater
      setPage(next.pageIndex)
    },
  })

  return (
    <div className='space-y-3'>
      <Select
        value={String(typeFilter)}
        onValueChange={(v) => {
          setTypeFilter(Number(v))
          setPage(0)
        }}
      >
        <SelectTrigger className='w-[180px]'>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value='-1'>{t('All Types')}</SelectItem>
          <SelectItem value='1'>{t('Direct Rebate')}</SelectItem>
          <SelectItem value='2'>{t('Indirect Rebate')}</SelectItem>
          <SelectItem value='3'>{t('Admin Dividend')}</SelectItem>
          <SelectItem value='4'>{t('Root Dividend')}</SelectItem>
        </SelectContent>
      </Select>

      <DataTablePage
        table={table}
        columns={columns}
        isLoading={isLoading}
        isFetching={isFetching}
        emptyTitle={t('No dividend records')}
        toolbarProps={null}
        paginationInFooter={false}
        skeletonKeyPrefix='dividend-records'
      />
    </div>
  )
}
