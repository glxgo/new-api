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
import { BarChart3 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Skeleton } from '@/components/ui/skeleton'
import { TitledCard } from '@/components/ui/titled-card'
import { ConsumptionDistributionChart } from '@/features/dashboard/components/models/consumption-distribution-chart'
import type { QuotaDataItem } from '@/features/dashboard/types'

interface WalletChartsProps {
  data: QuotaDataItem[]
  loading?: boolean
}

// 本月模型消费分布：复用 dashboard 的 ConsumptionDistributionChart。
// 数据由父级 useWalletMonthUsage 拉取（与「本月消费」统计卡共用一次请求）。
export function WalletCharts({ data, loading }: WalletChartsProps) {
  const { t } = useTranslation()

  return (
    <TitledCard
      title={t('Monthly Model Usage')}
      description={t('Quota usage by model this month')}
      icon={<BarChart3 className='h-4 w-4' />}
      disableHoverEffect
    >
      {loading ? (
        <div className='grid grid-cols-1 gap-3 lg:grid-cols-2'>
          <Skeleton className='h-[330px] rounded-lg' />
          <Skeleton className='h-[330px] rounded-lg' />
        </div>
      ) : data.length === 0 ? (
        <div className='text-muted-foreground flex min-h-40 flex-col items-center justify-center py-10 text-center'>
          <p className='text-sm font-medium'>
            {t('No usage data for this month yet')}
          </p>
          <p className='mt-1 text-xs'>
            {t(
              'Your model consumption will appear here once you start calling'
            )}
          </p>
        </div>
      ) : (
        <ConsumptionDistributionChart
          data={data}
          loading={loading}
          timeGranularity='day'
        />
      )}
    </TitledCard>
  )
}
