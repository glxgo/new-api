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
import { lazy, Suspense, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import dayjs from '@/lib/dayjs'
import { Skeleton } from '@/components/ui/skeleton'
import { SectionPageLayout } from '@/components/layout'
import { getProfitSummary } from './api'
import { DividendRecordsTable } from './components/dividend-records-table'
import { ProfitStatCards } from './components/profit-stat-cards'

const LazyProfitChart = lazy(() =>
  import('./components/profit-chart').then((module) => ({
    default: module.ProfitChart,
  }))
)

type RangeKey = 'month' | 'last_month' | 'last_week' | 'all'

const RANGES: { key: RangeKey; label: string }[] = [
  { key: 'month', label: 'This Month' },
  { key: 'last_month', label: 'Last Month' },
  { key: 'last_week', label: 'Last Week' },
  { key: 'all', label: 'All Time' },
]

// range → {start,end} unix 秒。累计 start=0(从头, end 缺省=now); 其余按自然月/周。
function rangeToTimes(range: RangeKey): { start?: number; end?: number } {
  switch (range) {
    case 'all':
      return { start: 0 }
    case 'last_month':
      return {
        start: dayjs().subtract(1, 'month').startOf('month').unix(),
        end: dayjs().startOf('month').unix(),
      }
    case 'last_week':
      return {
        start: dayjs().subtract(1, 'week').startOf('week').unix(),
        end: dayjs().startOf('week').unix(),
      }
    case 'month':
    default:
      return { start: dayjs().startOf('month').unix() }
  }
}

export function Profit() {
  const { t } = useTranslation()
  const [range, setRange] = useState<RangeKey>('month')
  const { start, end } = useMemo(() => rangeToTimes(range), [range])

  const { data: summary, isLoading } = useQuery({
    queryKey: ['profit-summary', range, start, end],
    queryFn: async () => {
      const res = await getProfitSummary(start, end)
      if (!res.success || !res.data) {
        return null
      }
      return res.data
    },
  })

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('充值分润审计')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='mx-auto flex w-full max-w-7xl flex-col gap-4'>
          <div className='flex flex-wrap items-center justify-between gap-2'>
            <p className='text-muted-foreground text-sm'>
              {t(
                '外部付款、人工补单和管理员充值按固定比例分润；余额购买和免费会员发放不重复计入。'
              )}
            </p>
            <div className='bg-muted inline-flex rounded-lg p-0.5'>
              {RANGES.map((r) => (
                <button
                  key={r.key}
                  type='button'
                  onClick={() => setRange(r.key)}
                  className={`rounded-md px-3 py-1 text-xs font-medium transition-colors ${
                    range === r.key
                      ? 'bg-background text-foreground shadow-sm'
                      : 'text-muted-foreground'
                  }`}
                >
                  {t(r.label)}
                </button>
              ))}
            </div>
          </div>
          <ProfitStatCards summary={summary ?? null} loading={isLoading} />
          <Suspense
            fallback={<Skeleton className='h-[320px] w-full rounded-lg' />}
          >
            <LazyProfitChart summary={summary ?? null} loading={isLoading} />
          </Suspense>
          <div className='overflow-hidden rounded-lg border'>
            <div className='border-b px-4 py-3 text-sm font-semibold'>
              {t('分润记录')}
            </div>
            <div className='p-3'>
              <DividendRecordsTable />
            </div>
          </div>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
