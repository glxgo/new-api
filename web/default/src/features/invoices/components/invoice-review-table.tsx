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
import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Check,
  ChevronDown,
  ChevronUp,
  FileCheck2,
  Loader2,
  X,
} from 'lucide-react'
import { toast } from 'sonner'
import dayjs from '@/lib/dayjs'
import { cn } from '@/lib/utils'
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
import {
  approveInvoiceApplication,
  getAdminInvoiceApplications,
  rejectInvoiceApplication,
} from '../api'
import type { InvoiceStatus } from '../types'

const PAGE_SIZE = 20

function money(amount: number, currency: string) {
  const symbol =
    currency === 'CNY' ? '¥' : currency === 'USD' ? '$' : `${currency} `
  return `${symbol}${Number(amount || 0).toFixed(2)}`
}

function statusMeta(status: InvoiceStatus) {
  if (status === 'approved')
    return {
      label: '已通过',
      className: 'text-emerald-600 border-emerald-500/20 bg-emerald-500/10',
    }
  if (status === 'rejected')
    return {
      label: '已拒绝',
      className: 'text-destructive border-destructive/20 bg-destructive/10',
    }
  return {
    label: '待审核',
    className: 'text-amber-600 border-amber-500/20 bg-amber-500/10',
  }
}

export function InvoiceReviewTable() {
  const queryClient = useQueryClient()
  const [page, setPage] = useState(1)
  const [status, setStatus] = useState('pending')
  const [expandedId, setExpandedId] = useState<number | null>(null)
  const [review, setReview] = useState<{
    id: number
    action: 'approve' | 'reject'
  } | null>(null)
  const [remark, setRemark] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const query = useQuery({
    queryKey: ['invoice-review', page, status],
    queryFn: () =>
      getAdminInvoiceApplications({ page, page_size: PAGE_SIZE, status }),
  })
  const applications = query.data?.data?.data ?? []
  const total = query.data?.data?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))
  const submitReview = async () => {
    if (!review) return
    setSubmitting(true)
    try {
      const result =
        review.action === 'approve'
          ? await approveInvoiceApplication(review.id, remark)
          : await rejectInvoiceApplication(review.id, remark)
      if (!result.success) {
        toast.error(result.message || '审核失败')
        return
      }
      toast.success(
        review.action === 'approve' ? '发票申请已通过' : '发票申请已拒绝'
      )
      setReview(null)
      setRemark('')
      await queryClient.invalidateQueries({ queryKey: ['invoice-review'] })
    } finally {
      setSubmitting(false)
    }
  }
  return (
    <div className='space-y-3'>
      <div className='flex flex-wrap items-center gap-2'>
        <Select
          value={status}
          onValueChange={(value) => {
            if (value) {
              setStatus(value)
              setPage(1)
            }
          }}
        >
          <SelectTrigger className='w-36'>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value='pending'>待审核</SelectItem>
            <SelectItem value='approved'>已通过</SelectItem>
            <SelectItem value='rejected'>已拒绝</SelectItem>
            <SelectItem value='all'>全部状态</SelectItem>
          </SelectContent>
        </Select>
        <span className='text-muted-foreground text-xs'>共 {total} 条申请</span>
      </div>
      <div className='bg-card overflow-hidden rounded-2xl border'>
        {query.isLoading ? (
          <div className='text-muted-foreground flex min-h-48 items-center justify-center text-sm'>
            加载中…
          </div>
        ) : applications.length === 0 ? (
          <div className='text-muted-foreground flex min-h-48 flex-col items-center justify-center gap-2 text-sm'>
            <FileCheck2 className='size-7 opacity-35' />
            暂无发票申请
          </div>
        ) : (
          <div className='divide-y'>
            {applications.map((application) => {
              const meta = statusMeta(application.status)
              const expanded = expandedId === application.id
              return (
                <article key={application.id} className='px-4 py-4 sm:px-5'>
                  <div className='flex flex-wrap items-start justify-between gap-3'>
                    <div className='min-w-0'>
                      <div className='flex flex-wrap items-center gap-2'>
                        <span className='font-mono text-sm font-semibold'>
                          申请 #{application.id}
                        </span>
                        <Badge variant='outline' className={cn(meta.className)}>
                          {meta.label}
                        </Badge>
                        <span className='text-muted-foreground text-xs'>
                          用户 ID {application.user_id}
                        </span>
                      </div>
                      <div className='text-muted-foreground mt-1 text-xs'>
                        {dayjs(application.created_at * 1000).format(
                          'YYYY-MM-DD HH:mm'
                        )}{' '}
                        · {application.subject_label} · {application.title}
                      </div>
                    </div>
                    <div className='flex items-center gap-2'>
                      <strong className='font-mono text-sm'>
                        {money(application.total_amount, application.currency)}
                      </strong>
                      <Button
                        variant='ghost'
                        size='icon-sm'
                        onClick={() =>
                          setExpandedId(expanded ? null : application.id)
                        }
                        aria-label='查看订单明细'
                      >
                        {expanded ? (
                          <ChevronUp className='size-4' />
                        ) : (
                          <ChevronDown className='size-4' />
                        )}
                      </Button>
                    </div>
                  </div>
                  {expanded && (
                    <div className='bg-muted/35 mt-3 rounded-xl border p-3 text-xs'>
                      <div className='grid gap-2 sm:grid-cols-2'>
                        <span>抬头：{application.title}</span>
                        <span>
                          纳税人识别号：{application.taxpayer_id || '—'}
                        </span>
                        <span>接收邮箱：{application.email}</span>
                        <span>订单数量：{application.orders.length}</span>
                      </div>
                      <div className='bg-background mt-3 divide-y rounded-lg border'>
                        {application.orders.map((order) => (
                          <div
                            key={order.id}
                            className='flex flex-wrap items-center justify-between gap-2 px-3 py-2'
                          >
                            <span className='font-mono'>{order.trade_no}</span>
                            <span className='text-muted-foreground'>
                              {dayjs(order.payment_time * 1000).format(
                                'YYYY-MM-DD HH:mm'
                              )}{' '}
                              · {order.payment_method || '在线支付'}
                            </span>
                            <strong>
                              {money(order.amount, order.currency)}
                            </strong>
                          </div>
                        ))}
                      </div>
                      {application.remark && (
                        <p className='text-muted-foreground mt-2'>
                          备注：{application.remark}
                        </p>
                      )}
                    </div>
                  )}
                  {application.status === 'pending' ? (
                    <div className='mt-3 flex justify-end gap-2'>
                      <Button
                        size='sm'
                        onClick={() =>
                          setReview({ id: application.id, action: 'approve' })
                        }
                      >
                        <Check className='size-3.5' />
                        通过
                      </Button>
                      <Button
                        size='sm'
                        variant='outline'
                        onClick={() =>
                          setReview({ id: application.id, action: 'reject' })
                        }
                      >
                        <X className='size-3.5' />
                        拒绝
                      </Button>
                    </div>
                  ) : (
                    <p className='text-muted-foreground mt-3 text-right text-xs'>
                      {application.handler_name || '管理员'} ·{' '}
                      {application.handled_at
                        ? dayjs(application.handled_at * 1000).format(
                            'YYYY-MM-DD HH:mm'
                          )
                        : '—'}
                    </p>
                  )}
                </article>
              )
            })}
          </div>
        )}
      </div>
      {totalPages > 1 && (
        <div className='flex items-center justify-center gap-3'>
          <Button
            variant='outline'
            size='sm'
            disabled={page <= 1}
            onClick={() => setPage((value) => Math.max(1, value - 1))}
          >
            上一页
          </Button>
          <span className='text-muted-foreground text-xs'>
            {page} / {totalPages}
          </span>
          <Button
            variant='outline'
            size='sm'
            disabled={page >= totalPages}
            onClick={() => setPage((value) => Math.min(totalPages, value + 1))}
          >
            下一页
          </Button>
        </div>
      )}
      <AlertDialog
        open={review !== null}
        onOpenChange={(open) => {
          if (!open && !submitting) {
            setReview(null)
            setRemark('')
          }
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {review?.action === 'approve' ? '通过发票申请' : '拒绝发票申请'}
            </AlertDialogTitle>
            <AlertDialogDescription>
              审核结果会通知用户，并发送到发票接收邮箱。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <Input
            value={remark}
            onChange={(event) => setRemark(event.target.value)}
            placeholder='审核备注（可选）'
          />
          <AlertDialogFooter>
            <AlertDialogCancel disabled={submitting}>取消</AlertDialogCancel>
            <AlertDialogAction
              onClick={(event) => {
                event.preventDefault()
                void submitReview()
              }}
              disabled={submitting}
            >
              {submitting && <Loader2 className='size-4 animate-spin' />}确认
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
