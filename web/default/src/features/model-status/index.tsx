/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import {
  Activity,
  ArrowRight,
  Boxes,
  Clock3,
  Gauge,
  RadioTower,
  Search,
  ShieldCheck,
  Sparkles,
  TimerReset,
  Zap,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { useIsAdmin } from '@/hooks/use-admin'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { Dialog } from '@/components/dialog'
import { SectionPageLayout } from '@/components/layout'
import { getPerfMetricsGroupSummary } from '@/features/performance-metrics/api'
import type {
  GroupCacheSummary,
  PerformanceSeriesPoint,
} from '@/features/performance-metrics/types'
import { usePricingData } from '@/features/pricing/hooks/use-pricing-data'
import type { PricingModel } from '@/features/pricing/types'
import {
  hasCompleteHealthMetrics,
  resolveModelStatusGroups,
  summarizeAvailabilitySeries,
} from './compat'
import { ChannelProbeDialog } from './components/channel-probe-dialog'
import {
  formatProbeTime,
  probeErrorLabel,
  probeLabel,
  probeTone,
} from './probe'
import {
  availabilityBarClass,
  HEALTHY_AVAILABILITY_THRESHOLD,
  UNSTABLE_AVAILABILITY_THRESHOLD,
} from './visuals'

type HealthTone = 'healthy' | 'unstable' | 'critical' | 'empty'

const TIME_RANGES = [
  { hours: 24, label: '24小时', shortLabel: '24H' },
  { hours: 24 * 7, label: '7天', shortLabel: '7D' },
  { hours: 24 * 30, label: '30天', shortLabel: '30D' },
] as const

function healthTone(summary?: GroupCacheSummary): HealthTone {
  if (!summary?.request_count || !hasCompleteHealthMetrics(summary))
    return 'empty'
  if ((summary.success_rate ?? 0) >= HEALTHY_AVAILABILITY_THRESHOLD)
    return 'healthy'
  if ((summary.success_rate ?? 0) >= UNSTABLE_AVAILABILITY_THRESHOLD)
    return 'unstable'
  return 'critical'
}

function healthLabel(tone: HealthTone, hasSamples: boolean) {
  if (tone === 'healthy') return '运行正常'
  if (tone === 'unstable') return '偶有波动'
  if (tone === 'critical') return '需要关注'
  if (hasSamples) return '健康指标待同步'
  return '暂无数据'
}

function metric(value: number | undefined, unit: string, digits = 1) {
  if (!value || value <= 0) return '—'
  return `${value.toFixed(digits)}${unit}`
}

function AvailabilityBars({
  series,
  hours,
}: {
  series?: PerformanceSeriesPoint[]
  hours: number
}) {
  if (!series?.length) {
    return (
      <div className='border-border/60 bg-muted/15 text-muted-foreground flex h-10 items-center justify-center rounded-lg border border-dashed text-[11px]'>
        暂无趋势数据
      </div>
    )
  }

  const segments = summarizeAvailabilitySeries(series, hours)

  return (
    <div className='flex h-10 items-end gap-1' aria-label='所选周期可用率趋势'>
      {segments.map((point) => (
        <span
          key={point.ts}
          title={`${new Date(point.ts * 1000).toLocaleString()} · ${point.successRate.toFixed(1)}%`}
          className={cn(
            'min-w-1 flex-1 rounded-[2px] transition-[height,opacity] duration-200 hover:opacity-60',
            availabilityBarClass(point.successRate)
          )}
          style={{ height: `${Math.max(18, point.successRate)}%` }}
        />
      ))}
    </div>
  )
}

function ProbeStatusBand({ summary }: { summary?: GroupCacheSummary }) {
  const probe = summary?.probe
  const tone = probeTone(probe)
  const series = probe?.series ?? []

  return (
    <div className='bg-muted/15 rounded-xl border border-dashed p-3'>
      <div className='flex items-start justify-between gap-3'>
        <div className='min-w-0'>
          <div className='text-muted-foreground flex items-center gap-1.5 text-[10px] tracking-[0.08em] uppercase'>
            <RadioTower className='size-3' /> 主动探测
          </div>
          <div className='mt-1.5 flex items-center gap-2'>
            <span
              className={cn(
                'size-1.5 rounded-full',
                tone === 'unhealthy'
                  ? 'bg-destructive'
                  : tone === 'degraded'
                    ? 'bg-warning'
                    : tone === 'unknown'
                      ? 'bg-muted-foreground/35'
                      : 'bg-success'
              )}
            />
            <p
              className={cn(
                'text-sm font-semibold',
                tone === 'unhealthy'
                  ? 'text-destructive'
                  : tone === 'degraded'
                    ? 'text-warning'
                    : tone === 'healthy'
                      ? 'text-success'
                      : 'text-foreground'
              )}
            >
              {probeLabel(tone, probe)}
            </p>
          </div>
        </div>
        <div className='shrink-0 text-right'>
          <p className='font-mono text-sm font-semibold tabular-nums'>
            {probe?.total_channels
              ? `${probe.healthy_channels} / ${probe.total_channels}`
              : '—'}
          </p>
          <p className='text-muted-foreground mt-0.5 text-[9px]'>正常渠道</p>
        </div>
      </div>

      {series.length ? (
        <div
          className='mt-3 flex h-5 items-end gap-0.5'
          aria-label='主动探测趋势'
        >
          {series.map((point) => (
            <span
              key={point.ts}
              title={`${new Date(point.ts * 1000).toLocaleString()} · ${point.success_rate.toFixed(1)}%`}
              className={cn(
                'min-w-1 flex-1 rounded-[2px]',
                availabilityBarClass(point.success_rate)
              )}
              style={{ height: `${Math.max(20, point.success_rate)}%` }}
            />
          ))}
        </div>
      ) : (
        <div className='bg-border/50 mt-3 h-px' />
      )}

      <div className='text-muted-foreground mt-2 flex items-center justify-between gap-3 text-[10px]'>
        <span>{formatProbeTime(probe?.last_probe_ts)}</span>
        <span className='truncate text-right'>
          {probe?.last_error_category
            ? probeErrorLabel(probe.last_error_category)
            : probe?.avg_latency_ms
              ? `平均 ${probe.avg_latency_ms}ms`
              : '独立于真实请求统计'}
        </span>
      </div>
    </div>
  )
}

function GroupHealthCard({
  group,
  description,
  summary,
  models,
  rangeHours,
  rangeLabel,
  rangeName,
  onViewModels,
}: {
  group: string
  description?: string
  summary?: GroupCacheSummary
  models: PricingModel[]
  rangeHours: number
  rangeLabel: string
  rangeName: string
  onViewModels: () => void
}) {
  const tone = healthTone(summary)
  const hasHealth = hasCompleteHealthMetrics(summary)
  const successCount = summary?.success_count
  const requestCount = summary?.request_count ?? 0

  return (
    <article className='border-border/70 bg-background/80 group hover:border-foreground/25 overflow-hidden rounded-2xl border transition-[transform,box-shadow,border-color] duration-200 hover:-translate-y-0.5 hover:shadow-[0_8px_24px_rgba(0,0,0,0.08)] dark:hover:shadow-[0_8px_24px_rgba(0,0,0,0.4)]'>
      <div className='flex items-start justify-between gap-4 border-b border-dashed px-5 py-4'>
        <div className='min-w-0'>
          <div className='flex items-center gap-2.5'>
            <span
              className={cn(
                'size-2 rounded-full',
                tone === 'critical'
                  ? 'bg-destructive'
                  : tone === 'empty'
                    ? 'bg-muted-foreground/35'
                    : 'bg-foreground'
              )}
            />
            <h2 className='truncate text-lg font-semibold tracking-tight'>
              {group}
            </h2>
          </div>
          <p className='text-muted-foreground mt-1.5 line-clamp-1 text-xs'>
            {description || `该分组关联渠道最近${rangeName}的真实请求表现`}
          </p>
        </div>
        <div className='shrink-0 text-right'>
          <p className='text-sm font-semibold'>
            {healthLabel(tone, requestCount > 0)}
          </p>
          <p className='text-muted-foreground mt-1 text-[10px] tracking-[0.14em] uppercase'>
            {rangeLabel} Health
          </p>
        </div>
      </div>

      <div className='grid grid-cols-3 divide-x border-b'>
        <div className='px-4 py-3'>
          <div className='text-muted-foreground flex items-center gap-1.5 text-[10px]'>
            <Clock3 className='size-3' /> 延迟
          </div>
          <p className='mt-1.5 font-mono text-sm font-semibold tabular-nums'>
            {metric((summary?.avg_latency_ms ?? 0) / 1000, 's', 2)}
          </p>
        </div>
        <div className='px-4 py-3'>
          <div className='text-muted-foreground flex items-center gap-1.5 text-[10px]'>
            <TimerReset className='size-3' /> 首 Token
          </div>
          <p className='mt-1.5 font-mono text-sm font-semibold tabular-nums'>
            {metric((summary?.avg_ttft_ms ?? 0) / 1000, 's', 2)}
          </p>
        </div>
        <div className='px-4 py-3'>
          <div className='text-muted-foreground flex items-center gap-1.5 text-[10px]'>
            <Gauge className='size-3' /> TPS
          </div>
          <p className='mt-1.5 font-mono text-sm font-semibold tabular-nums'>
            {metric(summary?.avg_tps, '', 1)}
          </p>
        </div>
      </div>

      <div className='space-y-4 p-5'>
        <div className='flex items-end justify-between gap-4'>
          <div>
            <p className='text-muted-foreground text-[10px] tracking-[0.12em] uppercase'>
              渠道真实请求可用率
            </p>
            <p className='mt-1 font-mono text-3xl font-semibold tracking-tight tabular-nums'>
              {requestCount && hasHealth
                ? `${summary?.success_rate?.toFixed(2)}%`
                : '—'}
            </p>
          </div>
          <p className='text-muted-foreground pb-1 text-right text-xs'>
            {requestCount && successCount != null
              ? `${successCount} / ${requestCount} 次成功`
              : requestCount
                ? `${requestCount} 次请求已统计`
                : '尚无请求样本'}
          </p>
        </div>

        <AvailabilityBars series={summary?.series} hours={rangeHours} />

        <ProbeStatusBand summary={summary} />

        <div className='grid grid-cols-2 gap-2'>
          <div className='bg-muted/20 rounded-xl border px-3 py-2.5'>
            <div className='text-muted-foreground flex items-center gap-1.5 text-[10px]'>
              <Zap className='size-3' /> 缓存命中
            </div>
            <p className='mt-1 font-mono text-sm font-semibold tabular-nums'>
              {summary && Number.isFinite(summary.cache_rate)
                ? `${summary.cache_rate.toFixed(1)}%`
                : '—'}
            </p>
          </div>
          <div className='bg-muted/20 rounded-xl border px-3 py-2.5'>
            <div className='text-muted-foreground flex items-center gap-1.5 text-[10px]'>
              <Boxes className='size-3' /> 可用模型
            </div>
            <p className='mt-1 font-mono text-sm font-semibold tabular-nums'>
              {models.length}
            </p>
          </div>
        </div>

        <Button
          type='button'
          variant='ghost'
          className='group/button -mx-2 w-[calc(100%+1rem)] justify-between'
          onClick={onViewModels}
        >
          查看该分组模型
          <ArrowRight className='size-4 transition-transform group-hover/button:translate-x-1' />
        </Button>
      </div>
    </article>
  )
}

export function ModelStatus() {
  const { t } = useTranslation()
  const isAdmin = useIsAdmin()
  const [search, setSearch] = useState('')
  const [rangeHours, setRangeHours] = useState(24)
  const [selectedGroup, setSelectedGroup] = useState<string | null>(null)
  const [probeDialogOpen, setProbeDialogOpen] = useState(false)
  const { models, usableGroup, isLoading: pricingLoading } = usePricingData()
  const summaryQuery = useQuery({
    queryKey: ['perf-metrics-group-summary-model-status', rangeHours],
    queryFn: () => getPerfMetricsGroupSummary(rangeHours),
    staleTime: 60 * 1000,
  })

  const activeRange =
    TIME_RANGES.find((range) => range.hours === rangeHours) ?? TIME_RANGES[0]

  const summaryMap = useMemo(
    () =>
      new Map(
        (summaryQuery.data?.data?.groups ?? []).map((summary) => [
          summary.group,
          summary,
        ])
      ),
    [summaryQuery.data]
  )

  const groups = useMemo(() => {
    const available = resolveModelStatusGroups(
      summaryQuery.data?.data,
      Object.keys(usableGroup)
    )
    return available
      .filter((group) =>
        group.toLowerCase().includes(search.trim().toLowerCase())
      )
      .sort((a, b) => {
        const requestsA = summaryMap.get(a)?.request_count ?? 0
        const requestsB = summaryMap.get(b)?.request_count ?? 0
        return requestsB - requestsA || a.localeCompare(b)
      })
  }, [search, summaryMap, summaryQuery.data, usableGroup])

  const modelsByGroup = useMemo(() => {
    const map = new Map<string, PricingModel[]>()
    for (const group of groups) {
      map.set(
        group,
        models.filter((model) => model.enable_groups?.includes(group))
      )
    }
    return map
  }, [groups, models])

  const selectedModels = selectedGroup
    ? (modelsByGroup.get(selectedGroup) ?? [])
    : []
  const loading = pricingLoading || summaryQuery.isLoading

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>{t('Model Status')}</SectionPageLayout.Title>
        <SectionPageLayout.Description>
          按分组并排查看最近{activeRange.label}
          的关联渠道真实请求表现与独立渠道探测，共用渠道的真实样本会同步计入对应分组，探测结果不参与渠道禁用。
        </SectionPageLayout.Description>
        <SectionPageLayout.Content>
          <div className='space-y-5'>
            <div className='flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between'>
              <div className='text-muted-foreground flex items-center gap-2 text-xs'>
                <Activity className='text-foreground size-4' />
                数据每分钟更新；只统计最终请求结果，重试中的中间错误不会重复计入
              </div>
              <div className='flex w-full flex-col gap-2 sm:flex-row lg:w-auto'>
                {isAdmin ? (
                  <Button
                    type='button'
                    variant='outline'
                    className='justify-start sm:justify-center'
                    onClick={() => setProbeDialogOpen(true)}
                  >
                    <ShieldCheck className='size-4' />
                    管理员诊断
                  </Button>
                ) : null}
                <div
                  className='bg-muted/20 inline-flex w-fit rounded-lg border p-0.5'
                  aria-label='数据周期'
                >
                  {TIME_RANGES.map((range) => (
                    <Button
                      key={range.hours}
                      type='button'
                      size='sm'
                      variant={
                        rangeHours === range.hours ? 'secondary' : 'ghost'
                      }
                      className='h-8 px-3 text-xs'
                      onClick={() => setRangeHours(range.hours)}
                    >
                      {range.label}
                    </Button>
                  ))}
                </div>
                <div className='relative w-full sm:w-72'>
                  <Search className='text-muted-foreground absolute top-1/2 left-3 size-4 -translate-y-1/2' />
                  <Input
                    value={search}
                    onChange={(event) => setSearch(event.target.value)}
                    placeholder='搜索分组…'
                    className='pl-9'
                  />
                </div>
              </div>
            </div>

            {loading ? (
              <div className='grid gap-4 xl:grid-cols-3'>
                {Array.from({ length: 6 }).map((_, index) => (
                  <Skeleton key={index} className='h-[35rem] rounded-2xl' />
                ))}
              </div>
            ) : groups.length ? (
              <div className='grid gap-4 xl:grid-cols-3'>
                {groups.map((group) => (
                  <GroupHealthCard
                    key={group}
                    group={group}
                    description={usableGroup[group]?.desc}
                    summary={summaryMap.get(group)}
                    models={modelsByGroup.get(group) ?? []}
                    rangeHours={rangeHours}
                    rangeLabel={activeRange.shortLabel}
                    rangeName={activeRange.label}
                    onViewModels={() => setSelectedGroup(group)}
                  />
                ))}
              </div>
            ) : (
              <div className='flex min-h-64 flex-col items-center justify-center rounded-2xl border border-dashed text-center'>
                <Sparkles className='text-muted-foreground mb-3 size-6' />
                <p className='text-sm font-medium'>没有找到匹配的分组</p>
                <p className='text-muted-foreground mt-1 text-xs'>
                  换一个关键词试试
                </p>
              </div>
            )}
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <Dialog
        open={Boolean(selectedGroup)}
        onOpenChange={(open) => {
          if (!open) setSelectedGroup(null)
        }}
        title={selectedGroup ? `${selectedGroup} · 可用模型` : '可用模型'}
        description={`当前共 ${selectedModels.length} 个模型可在该分组使用`}
        contentClassName='sm:max-w-2xl'
        bodyClassName='max-h-[60vh] overflow-y-auto'
      >
        {selectedModels.length ? (
          <div className='grid gap-2 sm:grid-cols-2'>
            {selectedModels.map((model) => (
              <div
                key={model.model_name}
                className='bg-muted/15 flex min-w-0 items-center gap-3 rounded-xl border px-3 py-2.5'
              >
                <span className='bg-background flex size-8 shrink-0 items-center justify-center rounded-lg border'>
                  <Boxes className='size-4' />
                </span>
                <div className='min-w-0'>
                  <p className='truncate text-sm font-medium'>
                    {model.model_name}
                  </p>
                  <p className='text-muted-foreground truncate text-[10px]'>
                    {model.vendor_name || '模型服务'}
                  </p>
                </div>
              </div>
            ))}
          </div>
        ) : (
          <div className='text-muted-foreground rounded-xl border border-dashed py-10 text-center text-sm'>
            该分组暂未公开模型
          </div>
        )}
      </Dialog>

      {isAdmin ? (
        <ChannelProbeDialog
          open={probeDialogOpen}
          onOpenChange={setProbeDialogOpen}
        />
      ) : null}
    </>
  )
}
