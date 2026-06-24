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
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { type ColumnDef } from '@tanstack/react-table'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import dayjs from '@/lib/dayjs'
import { formatQuota } from '@/lib/format'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { DataTablePage, useDataTable } from '@/components/data-table'
import { approveWithdraw, getAllWithdraws, rejectWithdraw } from '../api'
import {
  WITHDRAW_STATUS,
  WITHDRAW_TYPE,
  type Withdraw,
  type WithdrawStatus,
} from '../types'

const PAGE_SIZE = 20

type ReviewTarget = { id: number; action: 'approve' | 'reject' } | null

export function WithdrawReviewTable() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [page, setPage] = useState(0) // react-table is 0-based
  const [statusFilter, setStatusFilter] = useState<number>(-1) // -1 = all
  const [reviewTarget, setReviewTarget] = useState<ReviewTarget>(null)
  const [remark, setRemark] = useState('')

  const { data, isLoading, isFetching } = useQuery({
    queryKey: ['withdraws-review', page, statusFilter],
    queryFn: async () => {
      const res = await getAllWithdraws({
        page: page + 1,
        page_size: PAGE_SIZE,
        status: statusFilter,
      })
      if (!res.success) {
        toast.error(res.message || t('Failed to load data'))
        return { data: [] as Withdraw[], total: 0 }
      }
      return {
        data: res.data?.data ?? [],
        total: res.data?.total ?? 0,
      }
    },
    placeholderData: (prev) => prev,
  })

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['withdraws-review'] })
  }

  async function doReview() {
    if (!reviewTarget) return
    const fn =
      reviewTarget.action === 'approve' ? approveWithdraw : rejectWithdraw
    const res = await fn({ id: reviewTarget.id, remark })
    if (res.success) {
      toast.success(t('Withdrawal updated'))
      invalidate()
    } else {
      toast.error(res.message || t('Failed to update'))
    }
    setReviewTarget(null)
    setRemark('')
  }

  const columns = useMemo<ColumnDef<Withdraw>[]>(
    () => [
      { accessorKey: 'id', header: t('ID'), size: 60 },
      { accessorKey: 'user_id', header: t('User ID'), size: 80 },
      {
        accessorKey: 'type',
        header: t('Type'),
        size: 90,
        cell: ({ row }) => (
          <Badge
            variant={
              row.original.type === WITHDRAW_TYPE.PRINCIPAL
                ? 'secondary'
                : 'outline'
            }
          >
            {row.original.type === WITHDRAW_TYPE.PRINCIPAL
              ? t('Principal')
              : t('Dividend')}
          </Badge>
        ),
      },
      {
        accessorKey: 'amount',
        header: t('Amount'),
        size: 110,
        cell: ({ row }) => (
          <span className='font-mono'>{formatQuota(row.original.amount)}</span>
        ),
      },
      {
        id: 'actual',
        header: t('Actual Payout'),
        size: 150,
        cell: ({ row }) => (
          <span className='text-muted-foreground font-mono text-sm'>
            {formatQuota(row.original.actual_amount)}
            <span className='ml-1 text-xs'>
              ({t('fee')} {formatQuota(row.original.fee)})
            </span>
          </span>
        ),
      },
      {
        accessorKey: 'alipay_account',
        header: t('Payment Info'),
        size: 170,
        cell: ({ row }) =>
          row.original.type === WITHDRAW_TYPE.PRINCIPAL ? (
            <div className='text-xs'>
              <div className='font-medium'>{row.original.alipay_name}</div>
              <div className='text-muted-foreground'>
                {row.original.alipay_account}
              </div>
              {row.original.wechat_qrcode && (
                <a
                  href={row.original.wechat_qrcode}
                  target='_blank'
                  rel='noreferrer'
                  className='mt-2 inline-block'
                  title='点击查看大图'
                >
                  <img
                    src={row.original.wechat_qrcode}
                    alt='微信收款码'
                    className='h-20 w-20 rounded border object-contain transition hover:scale-105'
                  />
                </a>
              )}
            </div>
          ) : (
            <span className='text-muted-foreground text-xs'>-</span>
          ),
      },
      {
        accessorKey: 'status',
        header: t('Status'),
        size: 100,
        cell: ({ row }) => <StatusBadge status={row.original.status} t={t} />,
      },
      {
        accessorKey: 'created_at',
        header: t('Requested'),
        size: 150,
        cell: ({ row }) => (
          <span className='text-muted-foreground text-xs'>
            {dayjs(row.original.created_at * 1000).format('YYYY-MM-DD HH:mm')}
          </span>
        ),
      },
      {
        id: 'actions',
        header: t('Actions'),
        size: 200,
        cell: ({ row }) =>
          row.original.status === WITHDRAW_STATUS.PENDING ? (
            <div className='flex gap-1'>
              <Button
                size='sm'
                onClick={() =>
                  setReviewTarget({ id: row.original.id, action: 'approve' })
                }
              >
                {t('Approve')}
              </Button>
              <Button
                size='sm'
                variant='outline'
                onClick={() =>
                  setReviewTarget({ id: row.original.id, action: 'reject' })
                }
              >
                {t('Reject')}
              </Button>
            </div>
          ) : (
            <span className='text-muted-foreground text-xs'>
              {row.original.handler_name || '-'}
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
      <div className='flex items-center gap-2'>
        <Select
          value={String(statusFilter)}
          onValueChange={(v) => {
            setStatusFilter(Number(v))
            setPage(0)
          }}
        >
          <SelectTrigger className='w-[180px]'>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value='-1'>{t('All Status')}</SelectItem>
            <SelectItem value='0'>{t('Pending')}</SelectItem>
            <SelectItem value='1'>{t('Approved')}</SelectItem>
            <SelectItem value='2'>{t('Rejected')}</SelectItem>
          </SelectContent>
        </Select>
      </div>

      <DataTablePage
        table={table}
        columns={columns}
        isLoading={isLoading}
        isFetching={isFetching}
        emptyTitle={t('No withdrawal requests')}
        toolbarProps={null}
        paginationInFooter={false}
        skeletonKeyPrefix='withdraw-review'
      />

      <AlertDialog
        open={!!reviewTarget}
        onOpenChange={(v) => {
          if (!v) {
            setReviewTarget(null)
            setRemark('')
          }
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {reviewTarget?.action === 'approve'
                ? t('Approve Withdrawal')
                : t('Reject Withdrawal')}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {reviewTarget?.action === 'approve'
                ? t(
                    'Funds have been paid offline. The frozen balance will be cleared.'
                  )
                : t('The frozen balance will be refunded to the user.')}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <Input
            value={remark}
            onChange={(e) => setRemark(e.target.value)}
            placeholder={t('Optional review note')}
          />
          <AlertDialogFooter>
            <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
            <AlertDialogAction onClick={doReview}>
              {t('Confirm')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}

function StatusBadge({
  status,
  t,
}: {
  status: WithdrawStatus
  t: (k: string) => string
}) {
  if (status === WITHDRAW_STATUS.PENDING)
    return (
      <Badge variant='outline' className='border-amber-500 text-amber-600'>
        {t('Pending')}
      </Badge>
    )
  if (status === WITHDRAW_STATUS.APPROVED)
    return (
      <Badge className='bg-emerald-600 hover:bg-emerald-600'>
        {t('Approved')}
      </Badge>
    )
  return (
    <Badge variant='secondary' className='text-rose-600'>
      {t('Rejected')}
    </Badge>
  )
}
