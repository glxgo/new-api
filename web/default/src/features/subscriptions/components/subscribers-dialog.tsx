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
import { Clock3, RefreshCw, Search, Users } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatQuota, formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'
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
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  sideDrawerContentClassName,
  sideDrawerHeaderClassName,
} from '@/components/drawer-layout'
import { getSubscriptionSubscribers } from '../api'
import type { AdminSubscriptionSubscriber } from '../types'
import { UserSubscriptionsDialog } from './dialogs/user-subscriptions-dialog'

type Filter = 'all' | 'active'

function isUsable(item: AdminSubscriptionSubscriber): boolean {
  const now = Date.now() / 1000
  const cycleTotal = Number(item.amount_total || 0)
  const cycleUsed = Number(item.amount_used || 0)
  const capTotal = Number(item.amount_cap || 0)
  const capUsed = Number(item.amount_cap_used || 0)
  const nextReset = Number(item.next_reset_time || 0)
  const cycleAvailable =
    cycleTotal <= 0 ||
    cycleUsed < cycleTotal ||
    item.reset_due === true ||
    (nextReset > 0 && nextReset <= now)
  const capAvailable = capTotal <= 0 || capUsed < capTotal
  return (
    item.status === 'active' &&
    Number(item.start_time || 0) <= now &&
    Number(item.end_time || 0) > now &&
    cycleAvailable &&
    capAvailable
  )
}

function QuotaCell({ item }: { item: AdminSubscriptionSubscriber }) {
  const { t } = useTranslation()
  const total = Number(item.amount_total || 0)
  const used = Math.max(0, Number(item.amount_used || 0))
  const remaining = total > 0 ? Math.max(0, total - used) : 0
  const capTotal = Number(item.amount_cap || 0)
  const capUsed = Math.max(0, Number(item.amount_cap_used || 0))
  const percent = total > 0 ? Math.min(100, (remaining / total) * 100) : 0
  const resetPeriod = item.quota_reset_period || 'never'
  const customSeconds = Number(item.quota_reset_custom_seconds || 0)
  const resetPeriodLabel =
    resetPeriod === 'daily'
      ? t('Daily')
      : resetPeriod === 'weekly'
        ? t('Weekly')
        : resetPeriod === 'monthly'
          ? t('Monthly')
          : resetPeriod === 'custom'
            ? customSeconds > 0
              ? `${t('Custom')} (${customSeconds}s)`
              : t('Custom')
            : t('No Reset')
  return (
    <div className='min-w-36 space-y-1.5'>
      <div className='flex items-baseline justify-between gap-2 text-xs'>
        <span className='font-semibold'>
          {total > 0 ? formatQuota(remaining) : t('Unlimited')}
        </span>
        <span className='text-muted-foreground'>
          {total > 0 ? `/ ${formatQuota(total)}` : ''}
        </span>
      </div>
      <div className='bg-muted h-1.5 overflow-hidden rounded-full'>
        <div
          className='h-full rounded-full bg-emerald-500'
          style={{ width: `${percent}%` }}
        />
      </div>
      <div className='text-muted-foreground flex items-center gap-1 text-[10px]'>
        <Clock3 className='size-3' />
        <span>
          {t('Reset Period')}: {resetPeriodLabel}
        </span>
      </div>
      <div className='text-muted-foreground text-[10px]'>
        {t('Next reset')}:{' '}
        {item.next_reset_time
          ? formatTimestampToDate(item.next_reset_time)
          : t('No Reset')}
      </div>
      {capTotal > 0 && (
        <div className='text-muted-foreground text-[10px]'>
          {t('Total Quota')}: {t('Used')} {formatQuota(capUsed)} /{' '}
          {formatQuota(capTotal)}
        </div>
      )}
    </div>
  )
}

export function SubscribersDialog({
  open,
  onOpenChange,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const [query, setQuery] = useState('')
  const [filter, setFilter] = useState<Filter>('all')
  const [selectedUserId, setSelectedUserId] = useState<number | null>(null)
  const { data, isLoading, isFetching, refetch } = useQuery({
    queryKey: ['subscription-subscriber-instances'],
    queryFn: getSubscriptionSubscribers,
    enabled: open,
  })
  const subscribers = useMemo(
    () => (data?.data ?? []).filter(isUsable),
    [data?.data]
  )
  const filtered = useMemo(() => {
    const keyword = query.trim().toLocaleLowerCase()
    return subscribers.filter((item) => {
      if (filter === 'active' && item.status !== 'active') return false
      if (!keyword) return true
      return [
        item.username,
        item.display_name,
        item.email,
        item.plan_title,
        String(item.user_id),
        String(item.id),
      ].some((value) => value?.toLocaleLowerCase().includes(keyword))
    })
  }, [filter, query, subscribers])
  const activeCount = subscribers.filter(
    (item) => item.status === 'active'
  ).length
  const usersCount = new Set(subscribers.map((item) => item.user_id)).size

  return (
    <>
      <Sheet open={open} onOpenChange={onOpenChange}>
        <SheetContent
          className={sideDrawerContentClassName(
            'w-[96vw] sm:max-w-[min(1200px,96vw)]'
          )}
        >
          <SheetHeader className={sideDrawerHeaderClassName()}>
            <SheetTitle className='flex items-center gap-2'>
              <Users className='size-4' />
              {t('Subscription Subscribers')}
            </SheetTitle>
            <SheetDescription>
              {t(
                'View every purchased plan, owner, remaining quota, and reset time'
              )}
            </SheetDescription>
          </SheetHeader>
          <div className='min-h-0 flex-1 overflow-y-auto p-4 sm:p-6'>
            <div className='grid gap-2 sm:grid-cols-3'>
              <div className='bg-card rounded-xl border p-3'>
                <p className='text-muted-foreground text-[10px]'>
                  {t('Subscription Instances')}
                </p>
                <p className='text-lg font-semibold'>{subscribers.length}</p>
              </div>
              <div className='bg-card rounded-xl border p-3'>
                <p className='text-muted-foreground text-[10px]'>
                  {t('Active')}
                </p>
                <p className='text-lg font-semibold text-emerald-600'>
                  {activeCount}
                </p>
              </div>
              <div className='bg-card rounded-xl border p-3'>
                <p className='text-muted-foreground text-[10px]'>
                  {t('Purchasing Users')}
                </p>
                <p className='text-lg font-semibold'>{usersCount}</p>
              </div>
            </div>
            <div className='my-4 flex flex-col gap-2 sm:flex-row'>
              <div className='relative min-w-0 flex-1'>
                <Search className='text-muted-foreground absolute top-1/2 left-3 size-4 -translate-y-1/2' />
                <Input
                  value={query}
                  onChange={(event) => setQuery(event.target.value)}
                  className='pl-9'
                  placeholder={t('Search username, email, user ID or plan')}
                />
              </div>
              <Select
                value={filter}
                onValueChange={(value) => setFilter(value as Filter)}
              >
                <SelectTrigger className='w-full sm:w-32'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='all'>{t('All')}</SelectItem>
                  <SelectItem value='active'>{t('Active')}</SelectItem>
                </SelectContent>
              </Select>
              <Button
                variant='outline'
                onClick={() => void refetch()}
                disabled={isFetching}
              >
                <RefreshCw
                  className={cn('size-4', isFetching && 'animate-spin')}
                />
                {t('Refresh')}
              </Button>
            </div>
            <div className='overflow-hidden rounded-xl border'>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('User')}</TableHead>
                    <TableHead>{t('Plan')}</TableHead>
                    <TableHead>{t('Remaining / Reset')}</TableHead>
                    <TableHead>{t('Validity')}</TableHead>
                    <TableHead className='text-right'>{t('Status')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {isLoading ? (
                    Array.from({ length: 5 }).map((_, index) => (
                      <TableRow key={index}>
                        <TableCell colSpan={5}>
                          <Skeleton className='h-12 w-full' />
                        </TableCell>
                      </TableRow>
                    ))
                  ) : filtered.length === 0 ? (
                    <TableRow>
                      <TableCell
                        colSpan={5}
                        className='text-muted-foreground py-10 text-center'
                      >
                        {t('No subscribers')}
                      </TableCell>
                    </TableRow>
                  ) : (
                    filtered.map((item) => {
                      return (
                        <TableRow
                          key={item.id}
                          className='cursor-pointer'
                          onClick={() => setSelectedUserId(item.user_id)}
                        >
                          <TableCell>
                            <div className='font-medium'>
                              {item.display_name || item.username}
                            </div>
                            <div className='text-muted-foreground text-xs'>
                              @{item.username} · ID {item.user_id}
                            </div>
                            {item.email && (
                              <div className='text-muted-foreground text-xs'>
                                {item.email}
                              </div>
                            )}
                          </TableCell>
                          <TableCell>
                            <div className='font-medium'>
                              {item.plan_title || `#${item.plan_id}`}
                            </div>
                            <div className='text-muted-foreground text-xs'>
                              实例 #{item.id}
                              {item.source === 'admin' ? ' · 管理员添加' : ''}
                            </div>
                          </TableCell>
                          <TableCell>
                            <QuotaCell item={item} />
                          </TableCell>
                          <TableCell>
                            <div className='text-xs'>
                              至 {formatTimestampToDate(item.end_time)}
                            </div>
                            <div className='text-muted-foreground text-[10px]'>
                              开始 {formatTimestampToDate(item.start_time)}
                            </div>
                          </TableCell>
                          <TableCell className='text-right'>
                            <Badge
                              className='border-emerald-500/20 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300'
                              variant='secondary'
                            >
                              {t('Active')}
                            </Badge>
                          </TableCell>
                        </TableRow>
                      )
                    })
                  )}
                </TableBody>
              </Table>
            </div>
            <p className='text-muted-foreground mt-3 text-xs'>
              {t('Showing {{shown}} of {{total}} subscription instances', {
                shown: filtered.length,
                total: subscribers.length,
              })}
            </p>
          </div>
        </SheetContent>
      </Sheet>
      {selectedUserId !== null && (
        <UserSubscriptionsDialog
          user={{ id: selectedUserId }}
          open
          onOpenChange={(next) => {
            if (!next) setSelectedUserId(null)
          }}
        />
      )}
    </>
  )
}
