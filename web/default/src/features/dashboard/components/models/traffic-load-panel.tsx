import type { ComponentType } from 'react'
import {
  Activity,
  BarChart3,
  Coins,
  Gauge,
  Layers3,
  TimerReset,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatNumber, formatQuota } from '@/lib/format'
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
} from '@/features/dashboard/types'

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

function DailyLoadTable(props: { daily: DashboardTrafficDaily[] }) {
  const { t } = useTranslation()
  return (
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
          {props.daily.map((day) => (
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
  )
}

function AdminChannelTable(props: { channels: DashboardChannelTraffic[] }) {
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
          {props.channels.map((channel) => (
            <TableRow key={channel.channel_id}>
              <TableCell>
                <div className='font-medium'>{channel.channel_name}</div>
                <div className='text-muted-foreground text-[11px]'>
                  #{channel.channel_id}
                </div>
              </TableCell>
              <TableCell className='text-right font-mono tabular-nums'>
                {formatQuota(channel.summary.billed_quota)}
              </TableCell>
              <TableCell className='text-muted-foreground text-right font-mono tabular-nums'>
                {formatQuota(channel.summary.cost_quota)}
              </TableCell>
              <TableCell className='text-right font-mono tabular-nums'>
                {metric(channel.summary.avg_rpm)}
              </TableCell>
              <TableCell className='text-right font-mono tabular-nums'>
                {formatNumber(channel.summary.peak_rpm)}
              </TableCell>
              <TableCell className='text-right font-mono tabular-nums'>
                {metric(channel.summary.avg_concurrency)}
              </TableCell>
              <TableCell className='text-right font-mono tabular-nums'>
                {formatNumber(channel.summary.peak_concurrency)}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}

function DailyChannelSpend(props: {
  channels: DashboardChannelTraffic[]
  days: DashboardTrafficDaily[]
}) {
  const { t } = useTranslation()
  const dailyByChannel = new Map(
    props.channels.map((channel) => [
      channel.channel_id,
      new Map(channel.daily.map((day) => [day.day_start, day.billed_quota])),
    ])
  )
  return (
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
          {props.days.map((day) => (
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
  )
}

export function TrafficLoadPanel(props: TrafficLoadPanelProps) {
  const { t } = useTranslation()
  const query = useDashboardTraffic(props.filters, props.isAdmin)
  const data = query.data
  const summary = data?.summary
  const daily = data?.daily ?? []
  const channels = data?.channels ?? []

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
          <AdminChannelTable channels={channels} />
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
        >
          <DailyChannelSpend channels={channels} days={daily} />
        </PanelWrapper>
      )}
    </div>
  )
}
