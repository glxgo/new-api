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
import { useMemo, useState, type ComponentType } from 'react'
import { useQuery } from '@tanstack/react-query'
import {
  Activity,
  ChevronLeft,
  ChevronRight,
  Coins,
  Gauge,
  Layers3,
  TimerReset,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatNumber, formatQuota } from '@/lib/format'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { getChannels } from '@/features/channels/api'
import { PanelWrapper } from '@/features/dashboard/components/ui/panel-wrapper'
import { useDashboardTraffic } from '@/features/dashboard/hooks/use-dashboard-traffic'
import type {
  DashboardChannelTraffic,
  DashboardFilters,
  DashboardTrafficDaily,
} from '@/features/dashboard/types'

const DAILY_PAGE_SIZE = 7
const EMPTY_DAILY: DashboardTrafficDaily[] = []
const EMPTY_CHANNELS: DashboardChannelTraffic[] = []

type TrafficLoadPanelProps = {
  filters?: DashboardFilters
  isAdmin: boolean
}

function metric(value: number, digits = 2) {
  return new Intl.NumberFormat(undefined, {
    maximumFractionDigits: digits,
  }).format(value || 0)
}

function formatDay(dayStart: number) {
  return new Intl.DateTimeFormat(undefined, {
    month: 'short',
    day: 'numeric',
  }).format(new Date(dayStart * 1000))
}

function MiniBars(props: { values: number[] }) {
  const max = Math.max(...props.values, 0)
  return (
    <div className='flex h-7 items-end gap-1' aria-hidden='true'>
      {props.values.map((value, index) => (
        <span
          key={index}
          className='bg-foreground/65 min-w-1 flex-1 rounded-t-[2px] transition-opacity group-hover:opacity-80'
          style={{
            height: `${max > 0 ? Math.max(12, (value / max) * 100) : 8}%`,
            opacity: max > 0 ? 1 : 0.15,
          }}
        />
      ))}
    </div>
  )
}

function LoadMetricCard(props: {
  title: string
  value: string
  detail: string
  values: number[]
  icon: ComponentType<{ className?: string }>
}) {
  const Icon = props.icon
  return (
    <div className='group bg-background/50 rounded-xl border p-3.5 transition-transform duration-200 hover:-translate-y-0.5 sm:p-4'>
      <div className='flex items-center justify-between gap-3'>
        <span className='text-muted-foreground text-xs font-medium'>
          {props.title}
        </span>
        <Icon className='text-muted-foreground size-3.5' aria-hidden='true' />
      </div>
      <div className='mt-2 font-mono text-xl font-semibold tracking-tight tabular-nums sm:text-2xl'>
        {props.value}
      </div>
      <div className='text-muted-foreground mt-1 text-[11px]'>
        {props.detail}
      </div>
      <div className='mt-3'>
        <MiniBars values={props.values} />
      </div>
    </div>
  )
}

function useDailyPagination(daily: DashboardTrafficDaily[]) {
  const [page, setPage] = useState(0)
  const orderedDaily = useMemo(
    () => [...daily].sort((a, b) => b.day_start - a.day_start),
    [daily]
  )
  const pageCount = Math.max(
    1,
    Math.ceil(orderedDaily.length / DAILY_PAGE_SIZE)
  )
  const currentPage = Math.min(page, pageCount - 1)

  return {
    page: currentPage,
    pageCount,
    setPage,
    rows: orderedDaily.slice(
      currentPage * DAILY_PAGE_SIZE,
      (currentPage + 1) * DAILY_PAGE_SIZE
    ),
  }
}

function DailyTablePager(props: {
  page: number
  pageCount: number
  onPageChange: (page: number) => void
}) {
  const { t } = useTranslation()
  if (props.pageCount <= 1) return null

  return (
    <div className='flex items-center justify-end gap-2'>
      <Button
        type='button'
        variant='outline'
        size='icon-sm'
        aria-label={t('Previous page')}
        disabled={props.page <= 0}
        onClick={() => props.onPageChange(props.page - 1)}
      >
        <ChevronLeft />
      </Button>
      <span className='text-muted-foreground min-w-14 text-center text-xs font-medium tabular-nums'>
        {props.page + 1} / {props.pageCount}
      </span>
      <Button
        type='button'
        variant='outline'
        size='icon-sm'
        aria-label={t('Next page')}
        disabled={props.page >= props.pageCount - 1}
        onClick={() => props.onPageChange(props.page + 1)}
      >
        <ChevronRight />
      </Button>
    </div>
  )
}

function DailyLoadTable(props: { daily: DashboardTrafficDaily[] }) {
  const { t } = useTranslation()
  const pagination = useDailyPagination(props.daily)
  return (
    <div className='space-y-3'>
      <div className='overflow-x-auto rounded-xl border'>
        <Table className='min-w-[720px]'>
          <TableHeader>
            <TableRow>
              <TableHead>{t('Date')}</TableHead>
              <TableHead className='text-right'>{t('Average RPM')}</TableHead>
              <TableHead className='text-right'>{t('Peak RPM')}</TableHead>
              <TableHead className='text-right'>
                {t('Average concurrency')}
              </TableHead>
              <TableHead className='text-right'>
                {t('Peak concurrency')}
              </TableHead>
              <TableHead className='text-right'>
                {t('Successful requests')}
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {pagination.rows.map((day) => (
              <TableRow key={day.day_start}>
                <TableCell className='font-medium'>
                  {formatDay(day.day_start)}
                </TableCell>
                <TableCell className='text-right font-mono tabular-nums'>
                  {metric(day.avg_rpm)}
                </TableCell>
                <TableCell className='text-right font-mono tabular-nums'>
                  {formatNumber(day.peak_rpm)}
                </TableCell>
                <TableCell className='text-right font-mono tabular-nums'>
                  {metric(day.avg_concurrency)}
                </TableCell>
                <TableCell className='text-right font-mono tabular-nums'>
                  {formatNumber(day.peak_concurrency)}
                </TableCell>
                <TableCell className='text-right font-mono tabular-nums'>
                  {formatNumber(day.request_count)}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
      <DailyTablePager
        page={pagination.page}
        pageCount={pagination.pageCount}
        onPageChange={pagination.setPage}
      />
    </div>
  )
}

function DailyChannelSpend(props: {
  channels: DashboardChannelTraffic[]
  days: DashboardTrafficDaily[]
}) {
  const { t } = useTranslation()
  const pagination = useDailyPagination(props.days)
  // 过滤已删除渠道：后端 GetDashboardChannelNames 查不到已删 channel（表已删），name 为空
  const activeChannels = props.channels.filter((channel) =>
    channel.channel_name?.trim()
  )
  const dailyByChannel = new Map(
    activeChannels.map((channel) => [
      channel.channel_id,
      new Map(channel.daily.map((day) => [day.day_start, day.billed_quota])),
    ])
  )
  return (
    <div className='space-y-3'>
      <div className='overflow-x-auto rounded-xl border'>
        <Table className='min-w-max'>
          <TableHeader>
            <TableRow>
              <TableHead className='bg-card sticky left-0 z-10'>
                {t('Date')}
              </TableHead>
              {activeChannels.map((channel) => (
                <TableHead
                  key={channel.channel_id}
                  className='min-w-32 text-right'
                >
                  <span className='block max-w-40 truncate'>
                    {channel.channel_name}
                  </span>
                  <span className='text-muted-foreground font-normal'>
                    #{channel.channel_id}
                  </span>
                </TableHead>
              ))}
            </TableRow>
          </TableHeader>
          <TableBody>
            {pagination.rows.map((day) => (
              <TableRow key={day.day_start}>
                <TableCell className='bg-card sticky left-0 z-10 font-medium'>
                  {formatDay(day.day_start)}
                </TableCell>
                {activeChannels.map((channel) => (
                  <TableCell
                    key={channel.channel_id}
                    className='text-right font-mono tabular-nums'
                  >
                    {formatQuota(
                      dailyByChannel
                        .get(channel.channel_id)
                        ?.get(day.day_start) ?? 0
                    )}
                  </TableCell>
                ))}
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
      <DailyTablePager
        page={pagination.page}
        pageCount={pagination.pageCount}
        onPageChange={pagination.setPage}
      />
    </div>
  )
}

export function TrafficLoadPanel(props: TrafficLoadPanelProps) {
  const { t } = useTranslation()
  const query = useDashboardTraffic(props.filters, props.isAdmin)
  const data = query.data
  const summary = data?.summary
  const daily = data?.daily ?? EMPTY_DAILY

  // 管理员渠道列表：日志聚合的 channels 可能仍含渠道表里已删除/已禁用的渠道
  // （GetDashboardChannelNames 只按 id 查 name，不过滤 status）。这里用最新
  // 渠道状态再过一道，只保留 status=1 启用的渠道。
  const adminChannelsQuery = useQuery({
    queryKey: ['traffic-load-admin-channels'],
    queryFn: () => getChannels({ page_size: 1000 }),
    enabled: props.isAdmin,
    staleTime: 60_000,
  })
  const enabledChannelIds = useMemo(() => {
    const items = adminChannelsQuery.data?.data?.items ?? []
    // status: 1=enabled, 0=manual disabled, 2=auto disabled —— 只保留启用
    return new Set(items.filter((c) => c.status === 1).map((c) => c.id))
  }, [adminChannelsQuery.data])

  const rawChannels = data?.channels ?? EMPTY_CHANNELS
  const channels =
    props.isAdmin && !adminChannelsQuery.isSuccess
      ? EMPTY_CHANNELS
      : props.isAdmin
        ? rawChannels.filter((c) => enabledChannelIds.has(c.channel_id))
        : rawChannels

  if (query.isLoading) {
    return (
      <div className='bg-card rounded-2xl border p-4 sm:p-5'>
        <Skeleton className='h-5 w-40' />
        <Skeleton className='mt-2 h-4 w-72 max-w-full' />
        <div className='mt-4 grid grid-cols-2 gap-3 lg:grid-cols-4'>
          {Array.from({ length: 4 }).map((_, index) => (
            <Skeleton key={index} className='h-36 rounded-xl' />
          ))}
        </div>
      </div>
    )
  }

  if (query.isError) {
    return (
      <div className='border-destructive/30 bg-destructive/5 text-destructive rounded-xl border px-4 py-3 text-sm'>
        {t('Failed to load traffic statistics')}
      </div>
    )
  }

  return (
    <div className='space-y-3 sm:space-y-4'>
      <PanelWrapper
        title={t('Daily RPM and concurrency')}
        description={t(
          'Averages include only periods with successful requests; failed requests are excluded'
        )}
        empty={!summary || summary.request_count === 0}
        emptyMessage={t('No successful request data in this time range')}
        contentClassName='space-y-4'
      >
        {summary && (
          <>
            <div className='grid grid-cols-2 gap-3 lg:grid-cols-4'>
              <LoadMetricCard
                title={t('Average RPM')}
                value={metric(summary.avg_rpm)}
                detail={t('{{count}} active minutes', {
                  count: summary.active_minutes,
                })}
                values={daily.map((day) => day.avg_rpm)}
                icon={Gauge}
              />
              <LoadMetricCard
                title={t('Peak RPM')}
                value={formatNumber(summary.peak_rpm)}
                detail={t('Successful requests only')}
                values={daily.map((day) => day.peak_rpm)}
                icon={Activity}
              />
              <LoadMetricCard
                title={t('Average concurrency')}
                value={metric(summary.avg_concurrency)}
                detail={t('Idle periods excluded')}
                values={daily.map((day) => day.avg_concurrency)}
                icon={TimerReset}
              />
              <LoadMetricCard
                title={t('Peak concurrency')}
                value={formatNumber(summary.peak_concurrency)}
                detail={t('Maximum overlapping successful requests')}
                values={daily.map((day) => day.peak_concurrency)}
                icon={Layers3}
              />
            </div>
            <DailyLoadTable daily={daily} />
          </>
        )}
      </PanelWrapper>

      {props.isAdmin && channels.length > 0 && (
        <PanelWrapper
          title={
            <span className='inline-flex items-center gap-2'>
              <Coins className='text-muted-foreground size-4' />
              {t('Daily channel consumption')}
            </span>
          }
          description={t(
            'Billed consumption amount by completion date and final channel'
          )}
          contentClassName='space-y-4'
        >
          <DailyChannelSpend channels={channels} days={daily} />
        </PanelWrapper>
      )}
    </div>
  )
}
