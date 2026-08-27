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
import { useQuery } from '@tanstack/react-query'
import {
  Activity,
  ArrowDownToLine,
  ArrowUpFromLine,
  Boxes,
  ChevronLeft,
  ChevronRight,
  CircleGauge,
  Clock3,
  Coins,
  DatabaseZap,
  Layers3,
  ShieldCheck,
  Sparkles,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatCompactNumber, formatQuota } from '@/lib/format'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { SectionPageLayout } from '@/components/layout'
import { getPlatformUsage } from './api'
import type {
  CPAAccountUsage,
  CPAModelUsage,
  CPAQuotaWindow,
  CPAUsageSnapshot,
} from './types'
import { getPlatformAccountPage, metricCardSurfaceClass } from './visuals'

function formatSyncTime(timestamp: number, fallback = '—') {
  if (!timestamp) return fallback
  return new Intl.DateTimeFormat(undefined, {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(new Date(timestamp * 1000))
}

function formatReset(timestamp: number | null) {
  if (!timestamp) return '—'
  return new Intl.DateTimeFormat(undefined, {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(new Date(timestamp * 1000))
}

function clampPercent(value: number | null) {
  if (value == null || !Number.isFinite(value)) return null
  return Math.min(100, Math.max(0, value))
}

function modelCacheTokens(model: CPAModelUsage) {
  return (
    Math.max(
      model.cached_tokens -
        model.cache_read_tokens -
        model.cache_creation_tokens,
      0
    ) + model.cache_read_tokens
  )
}

function modelCacheStats(model: CPAModelUsage) {
  const cached = modelCacheTokens(model)
  const cacheCreated = Math.max(0, model.cache_creation_tokens)
  const input = Math.max(0, model.input_tokens)
  const total = Math.max(0, model.total_tokens)
  const detailsAlreadyInInput = !(
    total > 0 &&
    cached > 0 &&
    total - input - Math.max(0, model.output_tokens) >= cached
  )
  const denominator = detailsAlreadyInInput
    ? Math.max(input, cached + cacheCreated)
    : input + cached + cacheCreated
  return { cached, denominator }
}

function modelCacheRate(model: CPAModelUsage) {
  const { cached, denominator } = modelCacheStats(model)
  return denominator > 0 ? (cached * 100) / denominator : 0
}

function aggregateCacheRate(models: CPAModelUsage[]) {
  const totals = models.reduce(
    (sum, model) => {
      const stats = modelCacheStats(model)
      sum.cached += stats.cached
      sum.denominator += stats.denominator
      return sum
    },
    { cached: 0, denominator: 0 }
  )
  return totals.denominator > 0 ? (totals.cached * 100) / totals.denominator : 0
}

function formatLatency(milliseconds: number) {
  if (!Number.isFinite(milliseconds) || milliseconds <= 0) return '—'
  return milliseconds >= 1000
    ? `${(milliseconds / 1000).toFixed(1)}s`
    : `${Math.round(milliseconds)}ms`
}

function formatModelCost(model: CPAModelUsage) {
  if (!model.cost_available && !(model.cost_usd > 0)) return '—'
  return new Intl.NumberFormat('en-US', {
    style: 'currency',
    currency: 'USD',
    minimumFractionDigits: model.cost_usd < 1 ? 4 : 2,
    maximumFractionDigits: model.cost_usd < 1 ? 4 : 2,
  }).format(model.cost_usd)
}

function SyncBadge({ cpa }: { cpa: CPAUsageSnapshot | undefined }) {
  const { t } = useTranslation()
  const status = cpa?.status ?? 'syncing'
  const label = {
    fresh: t('Fresh snapshot'),
    partial: t('Partial snapshot'),
    stale: t('Stale snapshot'),
    unavailable: t('Temporarily unavailable'),
    unconfigured: t('Not configured'),
    syncing: t('Syncing'),
  }[status]
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-[11px] font-semibold',
        status === 'fresh' &&
          'border-emerald-500/25 bg-emerald-500/8 text-emerald-700 dark:text-emerald-300',
        status === 'partial' &&
          'border-amber-500/25 bg-amber-500/8 text-amber-700 dark:text-amber-300',
        status === 'stale' &&
          'border-orange-500/25 bg-orange-500/8 text-orange-700 dark:text-orange-300',
        (status === 'unavailable' || status === 'unconfigured') &&
          'border-destructive/25 bg-destructive/5 text-destructive',
        status === 'syncing' && 'bg-muted text-muted-foreground'
      )}
    >
      <span className='relative flex size-1.5'>
        {status === 'syncing' && (
          <span className='absolute inline-flex size-full animate-ping rounded-full bg-current opacity-50' />
        )}
        <span className='relative inline-flex size-1.5 rounded-full bg-current' />
      </span>
      {label}
    </span>
  )
}

function QuotaRow({ window }: { window: CPAQuotaWindow }) {
  const { t } = useTranslation()
  const remaining = clampPercent(window.remaining_percent)
  return (
    <div className='space-y-1.5'>
      <div className='flex items-center justify-between gap-3 text-xs'>
        <span className='min-w-0 truncate font-medium' title={window.label}>
          {window.label}
        </span>
        <div className='flex shrink-0 items-center gap-2 font-mono tabular-nums'>
          <strong>
            {remaining == null ? '—' : `${remaining.toFixed(0)}%`}
          </strong>
          <span className='text-muted-foreground text-[10px]'>
            {t('resets')} {formatReset(window.reset_at)}
          </span>
        </div>
      </div>
      <div className='bg-muted h-1.5 overflow-hidden rounded-full'>
        <div
          className={cn(
            'h-full rounded-full transition-[width] duration-700',
            remaining == null
              ? 'bg-muted-foreground/20'
              : remaining <= 15
                ? 'bg-rose-500'
                : remaining <= 40
                  ? 'bg-amber-500'
                  : 'bg-emerald-500'
          )}
          style={{ width: `${remaining ?? 0}%` }}
        />
      </div>
    </div>
  )
}

function AccountCard({ account }: { account: CPAAccountUsage }) {
  const { t } = useTranslation()
  return (
    <article className='bg-card group relative overflow-hidden rounded-xl border p-4 transition-[border-color,transform,box-shadow] hover:-translate-y-0.5 hover:border-emerald-500/25 hover:shadow-[0_16px_34px_rgba(16,185,129,0.07)]'>
      <div className='absolute inset-x-0 top-0 h-0.5 bg-[linear-gradient(90deg,transparent,rgba(16,185,129,.55),transparent)] opacity-0 transition-opacity group-hover:opacity-100' />
      <header className='flex items-start justify-between gap-3'>
        <div className='flex min-w-0 items-center gap-2.5'>
          <span className='flex size-9 shrink-0 items-center justify-center rounded-lg border border-emerald-500/15 bg-emerald-500/8 text-emerald-700 dark:text-emerald-300'>
            <CircleGauge className='size-4' />
          </span>
          <div className='min-w-0'>
            <div className='text-muted-foreground text-[10px] font-medium tracking-[0.08em] uppercase'>
              {t('Account identifier')}
            </div>
            <div
              className='mt-0.5 truncate font-mono text-sm font-semibold'
              title={account.code}
            >
              {account.code}
            </div>
          </div>
        </div>
        <div className='flex shrink-0 flex-col items-end gap-1.5'>
          <span className='relative isolate overflow-hidden rounded-full border border-slate-300/90 bg-[linear-gradient(135deg,#f8fafc_0%,#cbd5e1_38%,#ffffff_58%,#94a3b8_100%)] px-2.5 py-1 text-[10px] font-extrabold tracking-[0.1em] text-slate-800 uppercase shadow-[inset_0_1px_0_rgba(255,255,255,.95),0_2px_8px_rgba(71,85,105,.18)] after:pointer-events-none after:absolute after:inset-y-[-35%] after:left-[-75%] after:w-1/2 after:skew-x-[-18deg] after:bg-[linear-gradient(90deg,transparent,rgba(255,255,255,.95),transparent)] after:transition-transform after:duration-700 group-hover:after:translate-x-[420%] motion-reduce:after:transition-none dark:border-slate-500/80 dark:bg-[linear-gradient(135deg,#475569_0%,#cbd5e1_42%,#f8fafc_58%,#64748b_100%)] dark:text-slate-950'>
            <span className='relative z-10 drop-shadow-[0_1px_0_rgba(255,255,255,.75)]'>
              {account.plan_type || t('Unknown plan')}
            </span>
          </span>
          <span
            className={cn(
              'rounded-full border px-2 py-0.5 text-[10px] font-semibold',
              account.available
                ? 'border-emerald-500/25 bg-emerald-500/8 text-emerald-700 dark:text-emerald-300'
                : 'border-amber-500/25 bg-amber-500/8 text-amber-700 dark:text-amber-300'
            )}
          >
            {account.available ? t('Available') : t('Unavailable')}
          </span>
        </div>
      </header>

      <div className='mt-4 min-h-24 space-y-3'>
        {account.available && account.windows.length > 0 ? (
          account.windows
            .slice(0, 4)
            .map((window) => <QuotaRow key={window.id} window={window} />)
        ) : (
          <div className='bg-muted/45 text-muted-foreground flex min-h-24 items-center justify-center rounded-lg border border-dashed text-xs'>
            {!account.enabled
              ? t('Account disabled')
              : t('Quota temporarily unavailable')}
          </div>
        )}
      </div>
    </article>
  )
}

function MetricCard({
  title,
  value,
  description,
  icon: Icon,
  accent = false,
}: {
  title: string
  value: string
  description: string
  icon: React.ComponentType<{ className?: string }>
  accent?: boolean
}) {
  return (
    <div
      className={cn(
        'group relative overflow-hidden rounded-xl border p-4 transition-[transform,border-color,box-shadow] duration-200 hover:-translate-y-1 hover:border-emerald-500/20 hover:shadow-[0_16px_34px_rgba(15,23,42,.08)] motion-reduce:transform-none',
        metricCardSurfaceClass(accent)
      )}
    >
      <span className='pointer-events-none absolute inset-x-6 top-0 h-px bg-[linear-gradient(90deg,transparent,rgba(16,185,129,.55),transparent)] opacity-0 transition-opacity group-hover:opacity-100' />
      <div className='flex items-start justify-between gap-4'>
        <div>
          <p className='text-muted-foreground text-[11px] font-semibold tracking-[0.08em] uppercase'>
            {title}
          </p>
          <p className='mt-2 font-serif text-2xl font-semibold tracking-[-0.03em] tabular-nums'>
            {value}
          </p>
        </div>
        <Icon
          className={cn(
            'mt-0.5 size-4',
            accent ? 'text-emerald-600' : 'text-muted-foreground'
          )}
        />
      </div>
      <p className='text-muted-foreground mt-2 text-[11px]'>{description}</p>
    </div>
  )
}

function ModelsTable({
  models,
  loading,
}: {
  models: CPAModelUsage[]
  loading: boolean
}) {
  const { t } = useTranslation()
  if (loading) {
    return (
      <div className='space-y-2 p-4'>
        {Array.from({ length: 5 }).map((_, index) => (
          <Skeleton key={index} className='h-12 w-full' />
        ))}
      </div>
    )
  }
  if (models.length === 0) {
    return (
      <div className='text-muted-foreground flex min-h-44 flex-col items-center justify-center gap-2 text-sm'>
        <Boxes className='size-7 opacity-35' />
        {t('No CPA model usage today')}
      </div>
    )
  }
  return (
    <div className='overflow-x-auto'>
      <table className='w-full min-w-[1120px] border-collapse text-left'>
        <thead>
          <tr className='text-muted-foreground border-b text-[10px] tracking-[0.08em] uppercase'>
            <th className='px-4 py-3 font-semibold'>{t('Model')}</th>
            <th className='px-3 py-3 font-semibold'>{t('Alias')}</th>
            <th className='px-3 py-3 font-semibold'>{t('Provider')}</th>
            <th className='px-3 py-3 text-right font-semibold'>
              {t('Requests')}
            </th>
            <th className='px-3 py-3 text-right font-semibold'>
              {t('Total Tokens')}
            </th>
            <th className='px-3 py-3 text-right font-semibold'>{t('Cost')}</th>
            <th className='px-3 py-3 text-right font-semibold'>
              {t('Performance')}
            </th>
            <th className='px-3 py-3 text-right font-semibold'>{t('Input')}</th>
            <th className='px-3 py-3 text-right font-semibold'>
              {t('Output')}
            </th>
            <th className='px-3 py-3 text-right font-semibold'>{t('Cache')}</th>
            <th className='px-4 py-3 text-right font-semibold'>
              {t('Cache Hit Rate')}
            </th>
          </tr>
        </thead>
        <tbody>
          {models.map((model) => (
            <tr
              key={`${model.provider}:${model.model}:${model.alias}`}
              className='hover:bg-muted/30 border-b last:border-0'
            >
              <td className='px-4 py-3'>
                <div className='font-mono text-xs font-semibold'>
                  {model.model}
                </div>
              </td>
              <td className='px-3 py-3 font-mono text-xs'>
                {model.alias || '—'}
              </td>
              <td className='text-muted-foreground px-3 py-3 text-xs'>
                {model.provider || 'Codex'}
              </td>
              <td className='px-3 py-3 text-right font-mono text-xs font-semibold tabular-nums'>
                {formatCompactNumber(model.requests)}
                {model.failed > 0 && (
                  <span className='text-destructive mt-0.5 block text-[10px]'>
                    {model.failed} {t('failed')}
                  </span>
                )}
              </td>
              <td className='px-3 py-3 text-right font-mono text-xs tabular-nums'>
                {formatCompactNumber(model.total_tokens)}
              </td>
              <td className='px-3 py-3 text-right font-mono text-xs tabular-nums'>
                {formatModelCost(model)}
              </td>
              <td className='px-3 py-3 text-right font-mono text-xs tabular-nums'>
                <span
                  className={cn(
                    'font-semibold',
                    model.avg_latency_ms >= 12000 && 'text-destructive',
                    model.avg_latency_ms > 0 &&
                      model.avg_latency_ms < 12000 &&
                      'text-emerald-600 dark:text-emerald-300'
                  )}
                >
                  {formatLatency(model.avg_latency_ms)}
                </span>
                <span className='text-muted-foreground mt-0.5 block text-[10px]'>
                  {t('TTFT')} {formatLatency(model.avg_ttft_ms)} ·{' '}
                  {model.output_tokens_per_second > 0
                    ? `${model.output_tokens_per_second.toFixed(1)} t/s`
                    : '—'}
                </span>
              </td>
              <td className='text-muted-foreground px-3 py-3 text-right font-mono text-xs tabular-nums'>
                {formatCompactNumber(model.input_tokens)}
              </td>
              <td className='text-muted-foreground px-3 py-3 text-right font-mono text-xs tabular-nums'>
                {formatCompactNumber(model.output_tokens)}
              </td>
              <td className='text-muted-foreground px-3 py-3 text-right font-mono text-xs tabular-nums'>
                {formatCompactNumber(modelCacheTokens(model))}
              </td>
              <td className='text-muted-foreground px-4 py-3 text-right font-mono text-xs tabular-nums'>
                {modelCacheRate(model).toFixed(1)}%
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

export function PlatformUsage() {
  const { t } = useTranslation()
  const [accountPage, setAccountPage] = useState(1)
  const query = useQuery({
    queryKey: ['platform-usage'],
    queryFn: async () => {
      const response = await getPlatformUsage()
      if (!response.success || !response.data)
        throw new Error(response.message || 'Failed to load platform usage')
      return response.data
    },
    staleTime: 30_000,
    refetchInterval: 60_000,
    refetchOnWindowFocus: false,
    retry: false,
  })
  const data = query.data
  const cpa = data?.cpa
  const accounts = cpa?.accounts ?? []
  const {
    currentPage: currentAccountPage,
    totalPages: accountPageCount,
    accounts: visibleAccounts,
  } = getPlatformAccountPage(accounts, accountPage)
  const availableAccounts = accounts.filter(
    (account) => account.available
  ).length
  const cacheHitRate = aggregateCacheRate(cpa?.models ?? [])

  return (
    <SectionPageLayout fixedContent variant='editorial'>
      <SectionPageLayout.Title>{t('Platform Usage')}</SectionPageLayout.Title>
      <SectionPageLayout.Description>
        {t('Live site consumption and sanitized CPA account capacity')}
      </SectionPageLayout.Description>
      <SectionPageLayout.Content>
        <div className='h-full overflow-y-auto pb-2'>
          <section className='relative overflow-hidden rounded-2xl border bg-[radial-gradient(circle_at_85%_20%,rgba(16,185,129,.13),transparent_32%),linear-gradient(145deg,var(--card),color-mix(in_oklch,var(--card)_94%,var(--muted)))] p-5 sm:p-6'>
            <div className='pointer-events-none absolute inset-0 [background-image:linear-gradient(to_right,currentColor_1px,transparent_1px),linear-gradient(to_bottom,currentColor_1px,transparent_1px)] [background-size:26px_26px] opacity-[0.045]' />
            <div className='relative flex flex-col justify-between gap-5 @4xl/content:flex-row @4xl/content:items-end'>
              <div className='max-w-2xl'>
                <div className='mb-3 flex flex-wrap items-center gap-2'>
                  <span className='inline-flex items-center gap-1.5 rounded-full border border-emerald-500/20 bg-emerald-500/8 px-2.5 py-1 text-[11px] font-semibold text-emerald-700 dark:text-emerald-300'>
                    <ShieldCheck className='size-3' />{' '}
                    {t('Server-side sanitized')}
                  </span>
                  <SyncBadge cpa={cpa} />
                </div>
                <h2 className='font-serif text-2xl font-semibold tracking-[-0.035em] sm:text-3xl'>
                  {t("Today's capacity ledger")}
                </h2>
                <p className='text-muted-foreground mt-2 max-w-xl text-sm leading-6'>
                  {t(
                    'Pre-discount usage removes entrance and group multipliers. CPA data is synchronized in the background every 10 minutes.'
                  )}
                </p>
              </div>
              <div className='text-muted-foreground flex flex-wrap gap-x-5 gap-y-2 text-[11px]'>
                <span className='inline-flex items-center gap-1.5'>
                  <Clock3 className='size-3' />
                  {t('Updated')} {formatSyncTime(cpa?.updated_at ?? 0)}
                </span>
                <span className='inline-flex items-center gap-1.5'>
                  <DatabaseZap className='size-3' />
                  {t('Next sync')} {formatSyncTime(cpa?.next_refresh_at ?? 0)}
                </span>
              </div>
            </div>
          </section>

          {query.isError && (
            <div className='border-destructive/25 bg-destructive/5 text-destructive mt-3 flex items-center justify-between rounded-lg border px-3 py-2 text-sm'>
              <span>
                {t('Platform usage could not be loaded. Please try again.')}
              </span>
              <Button
                variant='ghost'
                size='sm'
                onClick={() => void query.refetch()}
              >
                {t('Retry')}
              </Button>
            </div>
          )}

          <div className='mt-3 grid gap-3 @3xl/content:grid-cols-2 @6xl/content:grid-cols-5'>
            <MetricCard
              title={t('Pre-discount usage')}
              value={
                query.isPending
                  ? '—'
                  : formatQuota(data?.site.pre_discount_quota ?? 0)
              }
              description={t('All site channels · today')}
              icon={Coins}
              accent
            />
            <MetricCard
              title={t('Total Tokens')}
              value={
                query.isPending
                  ? '—'
                  : formatCompactNumber(data?.site.total_tokens ?? 0)
              }
              description={t(
                'Input plus output tokens, cache is not added twice'
              )}
              icon={Layers3}
            />
            <MetricCard
              title={t('Successful requests')}
              value={
                query.isPending
                  ? '—'
                  : formatCompactNumber(data?.site.request_count ?? 0)
              }
              description={t('Successful consumption logs across the site')}
              icon={Sparkles}
            />
            <MetricCard
              title={t('CPA accounts available')}
              value={
                query.isPending
                  ? '—'
                  : `${availableAccounts} / ${accounts.length}`
              }
              description={t(
                'Only active accounts with a fresh quota response'
              )}
              icon={Activity}
            />
            <MetricCard
              title={t('Cache Hit Rate')}
              value={query.isPending ? '—' : `${cacheHitRate.toFixed(1)}%`}
              description={t('CPA input cache · weighted by tokens')}
              icon={DatabaseZap}
            />
          </div>

          <section className='mt-3'>
            <div className='mb-3 flex items-end justify-between gap-3 px-1'>
              <div>
                <h3 className='font-serif text-lg font-semibold tracking-[-0.02em]'>
                  {t('CPA account capacity')}
                </h3>
                <p className='text-muted-foreground mt-1 text-xs'>
                  {t(
                    'Account identifiers are anonymized before leaving the server; email addresses are never displayed'
                  )}
                </p>
              </div>
              <span className='text-muted-foreground hidden text-[11px] @3xl/content:block'>
                {t('No manual refresh · background snapshot only')}
              </span>
            </div>
            {query.isPending ? (
              <div className='grid gap-3 @3xl/content:grid-cols-2 @6xl/content:grid-cols-4'>
                {Array.from({ length: 8 }).map((_, index) => (
                  <Skeleton key={index} className='h-44 rounded-xl' />
                ))}
              </div>
            ) : accounts.length === 0 ? (
              <div className='bg-card text-muted-foreground flex min-h-36 items-center justify-center rounded-xl border text-sm'>
                {cpa?.configured
                  ? t('Account snapshot is syncing')
                  : t('CPA usage integration is not configured')}
              </div>
            ) : (
              <>
                <div className='grid gap-3 @3xl/content:grid-cols-2 @6xl/content:grid-cols-4'>
                  {visibleAccounts.map((account) => (
                    <AccountCard key={account.code} account={account} />
                  ))}
                </div>
                {accountPageCount > 1 && (
                  <div className='mt-4 flex items-center justify-center gap-3'>
                    <Button
                      variant='outline'
                      size='icon-sm'
                      aria-label={t('Previous page')}
                      disabled={currentAccountPage <= 1}
                      onClick={() =>
                        setAccountPage(Math.max(1, currentAccountPage - 1))
                      }
                    >
                      <ChevronLeft className='size-4' />
                    </Button>
                    <span className='text-muted-foreground text-xs tabular-nums'>
                      {t('Page {{current}} of {{total}}', {
                        current: currentAccountPage,
                        total: accountPageCount,
                      })}
                    </span>
                    <Button
                      variant='outline'
                      size='icon-sm'
                      aria-label={t('Next page')}
                      disabled={currentAccountPage >= accountPageCount}
                      onClick={() =>
                        setAccountPage(
                          Math.min(accountPageCount, currentAccountPage + 1)
                        )
                      }
                    >
                      <ChevronRight className='size-4' />
                    </Button>
                  </div>
                )}
              </>
            )}
          </section>

          <section className='bg-card mt-3 overflow-hidden rounded-xl border'>
            <header className='flex flex-col justify-between gap-3 border-b px-4 py-4 sm:flex-row sm:items-center'>
              <div>
                <h3 className='font-serif text-lg font-semibold tracking-[-0.02em]'>
                  {t('CPA model traffic today')}
                </h3>
                <p className='text-muted-foreground mt-1 text-xs'>
                  {t(
                    'This table covers CPA traffic only; the site totals above include every channel'
                  )}
                </p>
              </div>
              <div className='text-muted-foreground flex gap-3 text-[10px]'>
                <span className='inline-flex items-center gap-1'>
                  <ArrowDownToLine className='size-3' />
                  {t('Input')}
                </span>
                <span className='inline-flex items-center gap-1'>
                  <ArrowUpFromLine className='size-3' />
                  {t('Output')}
                </span>
              </div>
            </header>
            <ModelsTable models={cpa?.models ?? []} loading={query.isPending} />
          </section>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
