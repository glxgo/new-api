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
import { useEffect, useMemo, useState } from 'react'
import { VChart } from '@visactor/react-vchart'
import { PieChart } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { VCHART_OPTION } from '@/lib/vchart'
import { useTheme } from '@/context/theme-provider'
import { Skeleton } from '@/components/ui/skeleton'
import type { ProfitSummary } from '../types'

// ThemeManager is lazily loaded once (shared across chart instances).
let themeManagerPromise: Promise<
  (typeof import('@visactor/vchart'))['ThemeManager']
> | null = null

interface ProfitChartProps {
  summary: ProfitSummary | null
  loading?: boolean
}

// Pie chart showing how settled gross profit is split across rebate / dividends / net.
export function ProfitChart({ summary, loading }: ProfitChartProps) {
  const { t } = useTranslation()
  const { resolvedTheme } = useTheme()
  const [themeReady, setThemeReady] = useState(false)

  useEffect(() => {
    const updateTheme = async () => {
      setThemeReady(false)
      if (!themeManagerPromise) {
        themeManagerPromise = import('@visactor/vchart').then(
          (m) => m.ThemeManager
        )
      }
      const ThemeManager = await themeManagerPromise
      ThemeManager.setCurrentTheme(resolvedTheme === 'dark' ? 'dark' : 'light')
      setThemeReady(true)
    }
    updateTheme()
  }, [resolvedTheme])

  // 从 CSS 变量读 chart token，明暗切换时重读，避免硬编码 hex 跟主题脱节
  const chartColors = useMemo(() => {
    if (typeof window === 'undefined') return []
    const root = document.documentElement
    const read = (n: string) =>
      getComputedStyle(root).getPropertyValue(n).trim()
    return [
      read('--chart-1'),
      read('--chart-2'),
      read('--chart-3'),
      read('--chart-4'),
    ].filter(Boolean) as string[]
  }, [resolvedTheme])

  const values = useMemo(() => {
    if (!summary) return []
    return [
      { name: t('Referral Rebate'), value: summary.affiliate_rebate },
      { name: t('Admin Dividend'), value: summary.admin_dividend },
      { name: t('Root Dividend'), value: summary.root_dividend },
      { name: t('Net Profit'), value: summary.net_profit },
    ].filter((d) => d.value > 0)
  }, [summary, t])

  const spec = useMemo(
    () => ({
      type: 'pie' as const,
      data: [{ id: 'profit', values }],
      valueField: 'value',
      categoryField: 'name',
      outerRadius: 0.8,
      innerRadius: 0.5,
      title: { visible: false },
      label: { visible: true },
      legends: { visible: true, orient: 'right', position: 'start' },
      tooltip: true,
      color: chartColors.length
        ? chartColors
        : ['#f59e0b', '#3b82f6', '#a855f7', '#10b981'],
    }),
    [values, chartColors]
  )

  return (
    <div className='overflow-hidden rounded-lg border'>
      <div className='flex flex-wrap items-center gap-2 border-b px-3 py-2 sm:px-5 sm:py-3'>
        <PieChart className='text-muted-foreground/60 size-4' />
        <div className='text-sm font-semibold'>{t('Profit Distribution')}</div>
        <span className='text-muted-foreground text-xs'>
          {t('How settled gross profit is split')}
        </span>
      </div>
      <div className='h-[300px] p-1.5 sm:h-96 sm:p-2'>
        {loading ? (
          <Skeleton className='h-full w-full' />
        ) : themeReady && values.length > 0 ? (
          <VChart
            key={values.map((v) => v.value).join('-') + '|' + resolvedTheme}
            spec={{
              ...spec,
              theme: resolvedTheme === 'dark' ? 'dark' : 'light',
              background: 'transparent',
            }}
            option={VCHART_OPTION}
          />
        ) : (
          <div className='text-muted-foreground flex h-full items-center justify-center text-sm'>
            {t('No settled profit data yet')}
          </div>
        )}
      </div>
    </div>
  )
}
