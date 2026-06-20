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
import type { ReactNode } from 'react'
import { Activity, Gauge, Medal } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatTokens } from '../lib/format'
import type { RankingBenchmark, RankingPerformance } from '../types'
import { ModelLink, VendorLink } from './entity-links'

type OpenRouterInsightsSectionProps = {
  performance: RankingPerformance[]
  benchmarks: RankingBenchmark[]
}

export function OpenRouterInsightsSection(
  props: OpenRouterInsightsSectionProps
) {
  const { t } = useTranslation()
  if (props.performance.length === 0 && props.benchmarks.length === 0) {
    return null
  }

  return (
    <section className='grid grid-cols-1 gap-4 lg:grid-cols-2'>
      <InsightCard
        title={t('OpenRouter performance')}
        description={t('Request-weighted latency and throughput snapshot')}
        icon={<Gauge className='text-primary size-4' />}
      >
        {props.performance.length === 0 ? (
          <InsightEmpty label={t('No performance data available')} />
        ) : (
          <ul>
            {props.performance.map((row) => (
              <li key={row.model_name} className='flex items-center gap-3 py-2'>
                <span className='text-muted-foreground/80 w-6 shrink-0 text-right font-mono text-xs tabular-nums'>
                  {row.rank}.
                </span>
                <div className='min-w-0 flex-1'>
                  <ModelLink
                    modelName={row.model_name}
                    title={row.model_name}
                    className='text-foreground block truncate font-mono text-xs font-medium'
                  >
                    {row.display_name || row.model_name}
                  </ModelLink>
                  <p className='text-muted-foreground/80 truncate text-[11px]'>
                    <VendorLink vendor={row.vendor}>
                      {row.vendor.toLowerCase()}
                    </VendorLink>{' '}
                    · {formatTokens(row.request_count)} {t('requests')}
                  </p>
                </div>
                <div className='shrink-0 text-right'>
                  <div className='text-foreground font-mono text-xs font-semibold tabular-nums'>
                    {Math.round(row.p50_latency)}ms
                  </div>
                  <div className='text-muted-foreground/80 font-mono text-[11px] tabular-nums'>
                    {Math.round(row.p50_throughput)} tok/s
                  </div>
                </div>
              </li>
            ))}
          </ul>
        )}
      </InsightCard>

      <InsightCard
        title={t('OpenRouter benchmarks')}
        description={t('Top benchmark scores grouped by capability')}
        icon={<Medal className='text-primary size-4' />}
      >
        {props.benchmarks.length === 0 ? (
          <InsightEmpty label={t('No benchmark data available')} />
        ) : (
          <div className='grid grid-cols-1 gap-x-6 md:grid-cols-2'>
            {groupBenchmarks(props.benchmarks).map(([category, rows]) => (
              <div key={category} className='min-w-0 py-1'>
                <h4 className='text-muted-foreground/80 mb-1 inline-flex items-center gap-1 text-[11px] font-medium tracking-widest uppercase'>
                  <Activity className='size-3' />
                  {t(category)}
                </h4>
                <ul>
                  {rows.map((row) => (
                    <li
                      key={`${row.category}-${row.model_name}`}
                      className='flex items-center gap-2 py-1.5'
                    >
                      <span className='text-muted-foreground/80 w-5 shrink-0 text-right font-mono text-[11px] tabular-nums'>
                        {row.rank}.
                      </span>
                      <ModelLink
                        modelName={row.model_name}
                        title={row.model_name}
                        className='text-foreground min-w-0 flex-1 truncate font-mono text-xs font-medium'
                      >
                        {row.display_name || row.model_name}
                      </ModelLink>
                      <span className='text-foreground shrink-0 font-mono text-xs font-semibold tabular-nums'>
                        {row.score.toFixed(1)}
                      </span>
                    </li>
                  ))}
                </ul>
              </div>
            ))}
          </div>
        )}
      </InsightCard>
    </section>
  )
}

function InsightCard(props: {
  title: string
  description: string
  icon: ReactNode
  children: ReactNode
}) {
  return (
    <div className='bg-card overflow-hidden rounded-lg border'>
      <header className='border-b px-4 py-3'>
        <h3 className='text-foreground inline-flex items-center gap-2 text-sm font-semibold'>
          {props.icon}
          {props.title}
        </h3>
        <p className='text-muted-foreground/80 mt-0.5 text-xs'>
          {props.description}
        </p>
      </header>
      <div className='px-4 py-2'>{props.children}</div>
    </div>
  )
}

function InsightEmpty(props: { label: string }) {
  return (
    <div className='text-muted-foreground/80 px-4 py-6 text-center text-xs'>
      {props.label}
    </div>
  )
}

function groupBenchmarks(rows: RankingBenchmark[]) {
  const groups = new Map<string, RankingBenchmark[]>()
  for (const row of rows) {
    const existing = groups.get(row.category) ?? []
    existing.push(row)
    groups.set(row.category, existing)
  }
  return [...groups.entries()]
}
