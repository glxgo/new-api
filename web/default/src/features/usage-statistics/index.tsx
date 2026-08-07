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
import { Link } from '@tanstack/react-router'
import {
  ArrowRight,
  BadgeCheck,
  ChartNoAxesColumnIncreasing,
  Coins,
  CreditCard,
  Layers3,
  ReceiptText,
  Sparkles,
  Zap,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import {
  formatCompactNumber,
  formatNumber,
  formatQuota,
  formatTimestampToDate,
} from '@/lib/format'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { SectionPageLayout } from '@/components/layout'
import { StatCard } from '@/features/dashboard/components/ui/stat-card'
import { getUsageStatistics } from './api'
import type {
  UsageStatisticsData,
  UsageStatisticsRange,
  UsageStatisticsSubscription,
} from './types'

const RANGE_OPTIONS: { value: UsageStatisticsRange; label: string }[] = [
  { value: '24h', label: '24 hours' },
  { value: '7d', label: '7 days' },
  { value: '30d', label: '30 days' },
]

function percent(value: number | undefined) {
  return `${Math.max(0, Number(value) || 0).toFixed(1)}%`
}

function metricSeries(
  data: UsageStatisticsData | undefined,
  key:
    | 'quota'
    | 'request_count'
    | 'total_tokens'
    | 'cache_hit_rate'
    | 'success_rate'
) {
  const series = data?.series ?? []
  if (key === 'cache_hit_rate') {
    return series.map((point) =>
      point.effective_prompt_tokens > 0
        ? (point.cache_tokens * 100) / point.effective_prompt_tokens
        : 0
    )
  }
  if (key === 'success_rate') {
    return series.map((point) =>
      point.request_count > 0
        ? (point.success_count * 100) / point.request_count
        : 0
    )
  }
  return series.map((point) => point[key])
}

function MetricShell({ children }: { children: React.ReactNode }) {
  return (
    <div className='bg-card hover:border-primary/25 rounded-xl border p-4 transition-[border-color,transform,box-shadow] duration-200 hover:-translate-y-0.5 hover:shadow-[0_12px_30px_rgba(17,24,39,0.06)]'>
      {children}
    </div>
  )
}

function ModelsPanel({
  data,
  loading,
}: {
  data: UsageStatisticsData | undefined
  loading: boolean
}) {
  const { t } = useTranslation()
  const models = data?.models ?? []
  const maxRequests = Math.max(1, ...models.map((item) => item.request_count))

  return (
    <section className='bg-card min-h-[22rem] rounded-xl border p-4 sm:p-5'>
      <div className='mb-5 flex items-start justify-between gap-4'>
        <div>
          <h3 className='font-serif text-lg font-semibold tracking-[-0.02em]'>
            {t('Model Usage Distribution')}
          </h3>
          <p className='text-muted-foreground mt-1 text-xs'>
            {t('Ranked by successful requests in the selected period')}
          </p>
        </div>
        <ChartNoAxesColumnIncreasing className='text-primary/70 mt-0.5 size-4' />
      </div>

      {loading ? (
        <div className='space-y-3'>
          {Array.from({ length: 7 }).map((_, index) => (
            <Skeleton key={index} className='h-8 w-full rounded-lg' />
          ))}
        </div>
      ) : models.length === 0 ? (
        <div className='text-muted-foreground flex min-h-56 flex-col items-center justify-center gap-2 text-sm'>
          <Layers3 className='size-7 opacity-35' />
          <span>{t('No usage in this period')}</span>
        </div>
      ) : (
        <div className='space-y-2.5'>
          {models.map((model) => {
            const width = Math.max(2, (model.request_count / maxRequests) * 100)
            return (
              <div
                key={model.model_name}
                className='grid grid-cols-[minmax(7rem,0.34fr)_minmax(10rem,1fr)_auto] items-center gap-3'
              >
                <span
                  className='truncate text-sm font-medium'
                  title={model.model_name}
                >
                  {model.model_name}
                </span>
                <div className='bg-muted/55 relative h-7 overflow-hidden rounded-md'>
                  <div
                    className='absolute inset-y-0 left-0 rounded-md bg-[linear-gradient(90deg,color-mix(in_oklch,var(--primary)_17%,transparent),color-mix(in_oklch,var(--primary)_7%,transparent))] transition-[width] duration-500'
                    style={{ width: `${width}%` }}
                  />
                  <span className='relative flex h-full items-center px-2.5 font-mono text-xs font-semibold tabular-nums'>
                    {formatCompactNumber(model.request_count)}
                  </span>
                </div>
                <span className='text-muted-foreground min-w-16 text-right font-mono text-xs tabular-nums'>
                  {formatCompactNumber(model.total_tokens)} T
                </span>
              </div>
            )
          })}
        </div>
      )}
    </section>
  )
}

function SubscriptionUsageRow({
  usage,
  totalQuota,
}: {
  usage: UsageStatisticsSubscription
  totalQuota: number
}) {
  const { t } = useTranslation()
  const quota = Math.max(0, Number(usage.quota) || 0)
  const usagePercent =
    totalQuota > 0 ? Math.min(100, (quota / totalQuota) * 100) : 0
  const title =
    usage.title?.trim() ||
    (usage.subscription_id > 0
      ? `${t('Subscription instance')} #${usage.subscription_id}`
      : t('Historical subscription usage'))

  return (
    <div className='space-y-2.5'>
      <div className='flex items-start justify-between gap-3'>
        <div className='min-w-0'>
          <div className='truncate text-sm font-semibold' title={title}>
            {title}
          </div>
          <div className='text-muted-foreground mt-0.5 text-[11px]'>
            {formatNumber(usage.request_count)} {t('successful requests')}
          </div>
        </div>
        <div className='shrink-0 text-right font-mono text-xs tabular-nums'>
          {formatQuota(quota)}
        </div>
      </div>
      <div className='bg-muted h-1.5 overflow-hidden rounded-full'>
        <div
          className='bg-primary h-full rounded-full transition-[width] duration-500'
          style={{ width: `${usagePercent}%` }}
        />
      </div>
    </div>
  )
}

function SubscriptionsPanel({
  data,
  loading,
}: {
  data: UsageStatisticsData | undefined
  loading: boolean
}) {
  const { t } = useTranslation()
  const subscriptions = data?.subscriptions ?? []
  const totalQuota = Math.max(0, Number(data?.summary.subscription_quota) || 0)

  return (
    <section className='bg-card min-h-[22rem] rounded-xl border p-4 sm:p-5'>
      <div className='mb-5 flex items-start justify-between gap-4'>
        <div>
          <h3 className='font-serif text-lg font-semibold tracking-[-0.02em]'>
            {t('Subscription Usage')}
          </h3>
          <p className='text-muted-foreground mt-1 text-xs'>
            {t(
              'Actual subscription deductions from successful requests in the selected period'
            )}
          </p>
        </div>
        <CreditCard className='text-primary/70 mt-0.5 size-4' />
      </div>

      {loading ? (
        <div className='space-y-5'>
          {Array.from({ length: 4 }).map((_, index) => (
            <div key={index} className='space-y-2'>
              <Skeleton className='h-4 w-2/3' />
              <Skeleton className='h-2 w-full' />
            </div>
          ))}
        </div>
      ) : subscriptions.length === 0 ? (
        <div className='flex min-h-56 flex-col items-center justify-center gap-3 text-center'>
          <span className='bg-muted flex size-11 items-center justify-center rounded-full'>
            <CreditCard className='text-muted-foreground size-5' />
          </span>
          <div>
            <p className='text-sm font-medium'>
              {t('No subscription quota used in this period')}
            </p>
            <p className='text-muted-foreground mt-1 text-xs'>
              {t('Only actual subscription deductions are shown here')}
            </p>
          </div>
        </div>
      ) : (
        <div className='max-h-[27rem] space-y-5 overflow-y-auto pr-1'>
          {subscriptions.map((usage) => (
            <SubscriptionUsageRow
              key={usage.subscription_id}
              usage={usage}
              totalQuota={totalQuota}
            />
          ))}
        </div>
      )}
    </section>
  )
}

export function UsageStatistics() {
  const { t } = useTranslation()
  const [range, setRange] = useState<UsageStatisticsRange>('7d')
  const query = useQuery({
    queryKey: ['usage-statistics', range],
    queryFn: async () => {
      const response = await getUsageStatistics(range)
      if (!response.success || !response.data) {
        throw new Error(response.message || 'Failed to load usage statistics')
      }
      return response.data
    },
    staleTime: 30_000,
    refetchOnWindowFocus: false,
    retry: false,
  })
  const data = query.data
  const summary = data?.summary

  const cards = useMemo(
    () => [
      {
        key: 'usage',
        title: t('Billed Usage'),
        value: formatQuota(summary?.quota ?? 0),
        description: t('Usage generated by successful requests'),
        icon: Coins,
        tone: 'teal' as const,
        sparkline: metricSeries(data, 'quota'),
      },
      {
        key: 'deduction',
        title: t('Quota Deduction'),
        value: formatQuota(summary?.quota ?? 0),
        description: t(
          'Wallet, subscription, and virtual membership deductions'
        ),
        icon: ReceiptText,
        tone: 'rose' as const,
        details: [
          {
            label: t('Wallet'),
            value: formatQuota(summary?.wallet_quota ?? 0),
          },
          {
            label: t('Subscription'),
            value: formatQuota(summary?.subscription_quota ?? 0),
          },
          {
            label: t('Virtual Membership'),
            value: formatQuota(summary?.virtual_membership_quota ?? 0),
          },
        ],
      },
      {
        key: 'requests',
        title: t('Request Count'),
        value: formatCompactNumber(summary?.request_count ?? 0),
        description: t('Successful and failed relay requests'),
        icon: Sparkles,
        tone: 'gray' as const,
        sparkline: metricSeries(data, 'request_count'),
      },
      {
        key: 'tokens',
        title: t('Total Tokens'),
        value: formatCompactNumber(summary?.total_tokens ?? 0),
        description: t('Input and output tokens'),
        icon: Layers3,
        tone: 'teal' as const,
        details: [
          {
            label: t('Input Tokens'),
            value: formatCompactNumber(summary?.prompt_tokens ?? 0),
          },
          {
            label: t('Output Tokens'),
            value: formatCompactNumber(summary?.completion_tokens ?? 0),
          },
        ],
      },
      {
        key: 'cache',
        title: t('Cache Hit Rate'),
        value: percent(summary?.cache_hit_rate),
        description: t('Cached tokens as a share of effective input'),
        icon: Zap,
        tone: 'teal' as const,
        sparkline: metricSeries(data, 'cache_hit_rate'),
      },
      {
        key: 'success',
        title: t('Success Rate'),
        value: percent(summary?.success_rate),
        description: t('Successful requests as a share of all requests'),
        icon: BadgeCheck,
        tone: 'gray' as const,
        sparkline: metricSeries(data, 'success_rate'),
      },
    ],
    [data, summary, t]
  )

  return (
    <SectionPageLayout fixedContent variant='editorial'>
      <SectionPageLayout.Title>{t('Usage Statistics')}</SectionPageLayout.Title>
      <SectionPageLayout.Description>
        {t(
          'Understand costs, traffic quality, cache efficiency, and model usage'
        )}
      </SectionPageLayout.Description>
      <SectionPageLayout.Content>
        <div className='h-full overflow-y-auto pb-1'>
          <div className='mb-3 flex items-center justify-between gap-3'>
            <div className='bg-muted/60 flex rounded-lg border p-0.5'>
              {RANGE_OPTIONS.map((option) => (
                <button
                  key={option.value}
                  type='button'
                  onClick={() => setRange(option.value)}
                  className={cn(
                    'rounded-md px-3 py-1.5 text-xs font-medium transition-colors',
                    range === option.value
                      ? 'bg-foreground text-background shadow-sm'
                      : 'text-muted-foreground hover:text-foreground'
                  )}
                >
                  {t(option.label)}
                </button>
              ))}
            </div>
            <Button
              variant='ghost'
              size='sm'
              render={
                <Link
                  to='/usage-logs/$section'
                  params={{ section: 'common' }}
                />
              }
            >
              {t('View Logs')}
              <ArrowRight data-icon='inline-end' />
            </Button>
          </div>

          {query.isError && (
            <div className='border-destructive/25 bg-destructive/5 text-destructive mb-4 flex items-center justify-between gap-3 rounded-lg border px-3 py-2 text-sm'>
              <span>
                {t('Usage statistics could not be loaded. Please try again.')}
              </span>
              <Button
                type='button'
                variant='ghost'
                size='sm'
                onClick={() => void query.refetch()}
                disabled={query.isFetching}
                className='text-destructive hover:bg-destructive/10 hover:text-destructive shrink-0'
              >
                {t('Retry')}
              </Button>
            </div>
          )}

          <div className='grid gap-3 @3xl/content:grid-cols-2 @5xl/content:grid-cols-3 @7xl/content:grid-cols-6'>
            {cards.map((card) => (
              <MetricShell key={card.key}>
                <StatCard
                  title={card.title}
                  value={card.value}
                  description={card.description}
                  icon={card.icon}
                  tone={card.tone}
                  sparkline={card.sparkline}
                  sparklineVariant='line'
                  details={card.details}
                  loading={query.isPending}
                  error={query.isError}
                />
              </MetricShell>
            ))}
          </div>

          <div className='mt-3 grid gap-3 @5xl/content:grid-cols-[minmax(0,1.35fr)_minmax(22rem,0.85fr)]'>
            <ModelsPanel data={data} loading={query.isPending} />
            <SubscriptionsPanel data={data} loading={query.isPending} />
          </div>

          {data && (
            <p className='text-muted-foreground mt-3 text-right text-[11px] tabular-nums'>
              {t('Updated through')} {formatTimestampToDate(data.end_timestamp)}
              {' · '}
              {formatNumber(data.summary.success_count)}{' '}
              {t('successful requests')}
            </p>
          )}
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
