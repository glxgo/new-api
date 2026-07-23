import { useMemo, useState, type ComponentType } from 'react'
import {
  Activity,
  BarChart3,
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
import { PanelWrapper } from '@/features/dashboard/components/ui/panel-wrapper'
import { useDashboardTraffic } from '@/features/dashboard/hooks/use-dashboard-traffic'
import type {
  DashboardChannelTraffic,
  DashboardFilters,
  DashboardTrafficDaily,
  DashboardTrafficSummary,
} from '@/features/dashboard/types'

const DAILY_PAGE_SIZE = 7
const EMPTY_DAILY: DashboardTrafficDaily[] = []
const EMPTY_CHANNELS: DashboardChannelTraffic[] = []
const EMPTY_TRAFFIC_SUMMARY: DashboardTrafficSummary = {
  request_count: 0,
  active_minutes: 0,
  avg_rpm: 0,
  peak_rpm: 0,
  avg_concurrency: 0,
  peak_concurrency: 0,
  billed_quota: 0,
  cost_quota: 0,
}

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

function AdminChannelTable(props: {
  channels: DashboardChannelTraffic[]
  dayStart: number | null
}) {
  const { t } = useTranslation()
  return (
    <div className='overflow-x-auto rounded-xl border'>
      <Table className='min-w-[940px]'>
        <TableHeader>
          <TableRow>
            <TableHead>{t('Channel')}</TableHead>
            <TableHead className='text-right'>{t('Consumption')}</TableHead>
            <TableHead className='text-right'>{t('Upstream cost')}</TableHead>
            <TableHead className='text-right'>{t('Average RPM')}</TableHead>
            <TableHead className='text-right'>{t('Peak RPM')}</TableHead>
            <TableHead className='text-right'>
              {t('Average concurrency')}
            </TableHead>
            <TableHead className='text-right'>
              {t('Peak concurrency')}
            </TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {props.channels.map((channel) => {
            const summary =
              props.dayStart === null
                ? channel.summary
                : (channel.daily.find(
                    (day) => day.day_start === props.dayStart
                  ) ?? EMPTY_TRAFFIC_SUMMARY)
            return (
              <TableRow key={channel.channel_id}>
                <TableCell>
                  <div className='font-medium'>{channel.channel_name}</div>
                  <div className='text-muted-foreground text-[11px]'>
                    #{channel.channel_id}
                  </div>
                </TableCell>
                <TableCell className='text-right font-mono tabular-nums'>
                  {formatQuota(summary.billed_quota)}
                </TableCell>
                <TableCell className='text-muted-foreground text-right font-mono tabular-nums'>
                  {formatQuota(summary.cost_quota)}
                </TableCell>
                <TableCell className='text-right font-mono tabular-nums'>
                  {metric(summary.avg_rpm)}
                </TableCell>
                <TableCell className='text-right font-mono tabular-nums'>
                  {formatNumber(summary.peak_rpm)}
                </TableCell>
                <TableCell className='text-right font-mono tabular-nums'>
                  {metric(summary.avg_concurrency)}
                </TableCell>
                <TableCell className='text-right font-mono tabular-nums'>
                  {formatNumber(summary.peak_concurrency)}
                </TableCell>
              </TableRow>
            )
          })}
        </TableBody>
      </Table>
    </div>
  )
}

function ChannelPeriodButtons(props: {
  days: DashboardTrafficDaily[]
  selectedDay: number | null
  onSelect: (dayStart: number | null) => void
}) {
  const { t } = useTranslation()
  const orderedDays = useMemo(
    () => [...props.days].sort((a, b) => b.day_start - a.day_start),
    [props.days]
  )

  return (
    <div className='flex gap-2 overflow-x-auto pb-1'>
      <Button
        type='button'
        variant={props.selectedDay === null ? 'default' : 'outline'}
        size='sm'
        aria-pressed={props.selectedDay === null}
        onClick={() => props.onSelect(null)}
      >
        {t('Overview')}
      </Button>
      {orderedDays.map((day) => (
        <Button
          key={day.day_start}
          type='button'
          variant={props.selectedDay === day.day_start ? 'default' : 'outline'}
          size='sm'
          aria-pressed={props.selectedDay === day.day_start}
          onClick={() => props.onSelect(day.day_start)}
        >
          {formatDay(day.day_start)}
        </Button>
      ))}
    </div>
  )
}

function DailyChannelSpend(props: {
  channels: DashboardChannelTraffic[]
  days: DashboardTrafficDaily[]
}) {
  const { t } = useTranslation()
  const pagination = useDailyPagination(props.days)
  const dailyByChannel = new Map(
    props.channels.map((channel) => [
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
              {props.channels.map((channel) => (
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
                {props.channels.map((channel) => (
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
  const channels = data?.channels ?? EMPTY_CHANNELS
  const [selectedChannelDay, setSelectedChannelDay] = useState<number | null>(
    null
  )
  const visibleChannelDay =
    selectedChannelDay !== null &&
    daily.some((day) => day.day_start === selectedChannelDay)
      ? selectedChannelDay
      : null

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
              <BarChart3 className='text-muted-foreground size-4' />
              {t('Channel load')}
            </span>
          }
          description={t(
            'Successful traffic statistics by final selected channel'
          )}
          contentClassName='space-y-4'
        >
          <ChannelPeriodButtons
            days={daily}
            selectedDay={visibleChannelDay}
            onSelect={setSelectedChannelDay}
          />
          <AdminChannelTable channels={channels} dayStart={visibleChannelDay} />
        </PanelWrapper>
      )}

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
