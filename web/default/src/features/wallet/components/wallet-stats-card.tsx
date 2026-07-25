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
import { Activity, BarChart3, WalletCards } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatQuota } from '@/lib/format'
import { CountUp } from '@/components/ui/count-up'
import { Skeleton } from '@/components/ui/skeleton'
import type { UserWalletData } from '../types'

interface WalletStatsCardProps {
  user: UserWalletData | null
  loading?: boolean
}

export function WalletStatsCard(props: WalletStatsCardProps) {
  const { t } = useTranslation()
  if (props.loading) {
    return (
      <div className='bg-border grid min-h-36 grid-cols-1 gap-px overflow-hidden rounded-xl border sm:grid-cols-3 xl:grid-cols-1'>
        {Array.from({ length: 3 }).map((_, i) => (
          <div key={i} className='bg-card p-3.5'>
            <Skeleton className='h-3.5 w-20' />
            <Skeleton className='mt-2 h-6 w-28' />
          </div>
        ))}
      </div>
    )
  }

  // 当前余额 = 原余额（可提现）+ 赠金余额（不可提现）。赠金来自拉新返利/签到/兑换码。
  const quotaValue = Number(props.user?.quota ?? 0)
  const giftValue = Number(props.user?.gift_quota ?? 0)
  const totalValue = quotaValue + giftValue

  const stats = [
    {
      label: t('Current Balance'),
      rawValue: totalValue,
      format: (v: number) => formatQuota(v),
      description: t('Principal balance + gift balance'),
      detail: t('Principal {{principal}}, Gift {{gift}}', {
        principal: formatQuota(quotaValue),
        gift: formatQuota(giftValue),
      }),
      icon: WalletCards,
    },
    {
      label: t('Total Usage'),
      rawValue: Number(props.user?.used_quota ?? 0),
      format: (v: number) => formatQuota(v),
      description: t('Total consumed quota'),
      icon: BarChart3,
    },
    {
      label: t('API Requests'),
      rawValue: Number(props.user?.request_count ?? 0),
      format: undefined, // 整数千分位（CountUp 默认）
      description: t('Total requests made'),
      icon: Activity,
    },
  ]

  return (
    <section className='bg-border grid min-h-36 grid-cols-1 gap-px overflow-hidden rounded-xl border shadow-sm sm:grid-cols-3 xl:grid-cols-1'>
      {stats.map((item) => {
        const Icon = item.icon
        return (
          <div
            key={item.label}
            className='bg-card hover:bg-muted/25 flex min-w-0 items-center gap-3 p-3.5 transition-colors'
          >
            <div className='bg-background flex size-9 shrink-0 items-center justify-center rounded-lg border'>
              <Icon className='text-muted-foreground size-4' />
            </div>
            <div className='min-w-0 flex-1'>
              <div className='text-muted-foreground truncate text-[11px] font-medium tracking-wider uppercase'>
                {item.label}
              </div>
              <div className='text-foreground mt-0.5 truncate font-mono text-lg font-bold tracking-tight tabular-nums'>
                <CountUp value={item.rawValue} format={item.format} />
              </div>
              {item.detail ? (
                <div className='text-muted-foreground mt-0.5 truncate text-[10px]'>
                  {item.detail}
                </div>
              ) : null}
            </div>
          </div>
        )
      })}
    </section>
  )
}
