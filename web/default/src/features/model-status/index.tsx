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
import { useCallback, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { PublicLayout } from '@/components/layout'
import { PageTransition } from '@/components/page-transition'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { usePricingData } from '@/features/pricing/hooks/use-pricing-data'
import { getPerfMetricsSummary } from '@/features/performance-metrics/api'
import { ModelStatusCard } from './components/model-status-card'
import { ModelStatusDrawer } from './components/model-status-drawer'

export function ModelStatus() {
  const { t } = useTranslation()
  const [selectedModelName, setSelectedModelName] = useState<string | null>(
    null
  )
  const [search, setSearch] = useState('')

  const { models, isLoading: pricingLoading } = usePricingData()

  const summaryQuery = useQuery({
    queryKey: ['perf-metrics-summary-model-status'],
    queryFn: () => getPerfMetricsSummary(24),
    staleTime: 60 * 1000,
  })

  const summaryMap = useMemo(() => {
    const map = new Map<
      string,
      {
        success_rate: number
        avg_latency_ms: number
        avg_tps: number
        request_count?: number
      }
    >()
    for (const m of summaryQuery.data?.data?.models ?? []) {
      map.set(m.model_name, {
        success_rate: m.success_rate,
        avg_latency_ms: m.avg_latency_ms,
        avg_tps: m.avg_tps,
        request_count: m.request_count,
      })
    }
    return map
  }, [summaryQuery.data])

  // 有汇总数据时按成功率排序（高的在前，无数据的靠后），否则保持模型清单顺序
  const sortedModels = useMemo(() => {
    const list = models ?? []
    if (summaryMap.size === 0) return list
    return [...list].sort((a, b) => {
      const ra = summaryMap.get(a.model_name)?.success_rate ?? -1
      const rb = summaryMap.get(b.model_name)?.success_rate ?? -1
      return rb - ra
    })
  }, [models, summaryMap])

  const filteredModels = useMemo(() => {
    const q = search.trim().toLowerCase()
    if (!q) return sortedModels
    return sortedModels.filter((m) =>
      m.model_name.toLowerCase().includes(q)
    )
  }, [sortedModels, search])

  const selectedModel = useMemo(
    () =>
      selectedModelName
        ? (models ?? []).find((m) => m.model_name === selectedModelName) ??
          null
        : null,
    [models, selectedModelName]
  )

  const handleModelClick = useCallback((name: string) => {
    setSelectedModelName(name)
  }, [])

  if (pricingLoading) {
    return (
      <PublicLayout showMainContainer={false}>
        <div className='mx-auto w-full max-w-[1800px] px-3 pt-16 pb-8 sm:px-6 sm:pt-20 sm:pb-10 xl:px-8'>
          <Skeleton className='mx-auto mb-8 h-10 w-56' />
          <div className='grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4'>
            {Array.from({ length: 8 }).map((_, i) => (
              <Skeleton key={i} className='h-32 w-full rounded-xl' />
            ))}
          </div>
        </div>
      </PublicLayout>
    )
  }

  return (
    <PublicLayout showMainContainer={false}>
      <PageTransition className='mx-auto w-full max-w-[1800px] px-3 pt-16 pb-8 sm:px-6 sm:pt-20 sm:pb-10 xl:px-8'>
        <header className='mx-auto mb-8 max-w-3xl pt-5 text-center sm:mb-10 sm:pt-10'>
          <h1 className='text-[clamp(2rem,5.5vw,3.5rem)] leading-[1.15] font-bold tracking-tight'>
            {t('Model Status')}
          </h1>
          <p className='text-muted-foreground/80 mt-3 text-sm sm:mt-4 sm:text-base'>
            {t('Real-time health of {{count}} models in the last 24 hours', {
              count: models?.length || 0,
            })}
          </p>
          <Input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder={t('Search model...')}
            className='mx-auto mt-4 max-w-2xl sm:mt-6'
          />
        </header>

        {filteredModels.length === 0 ? (
          <div className='text-muted-foreground py-20 text-center text-sm'>
            {t('No models found')}
          </div>
        ) : (
          <div className='grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4'>
            {filteredModels.map((model) => (
              <ModelStatusCard
                key={model.model_name}
                model={model}
                summary={summaryMap.get(model.model_name)}
                onClick={() => handleModelClick(model.model_name)}
              />
            ))}
          </div>
        )}

        {selectedModel && (
          <ModelStatusDrawer
            open={Boolean(selectedModel)}
            onOpenChange={(open) => {
              if (!open) setSelectedModelName(null)
            }}
            model={selectedModel}
          />
        )}
      </PageTransition>
    </PublicLayout>
  )
}
