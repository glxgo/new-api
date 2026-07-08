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
import { Skeleton } from '@/components/ui/skeleton'
import { CountUp } from '@/components/ui/count-up'
import type { UserWalletData } from '../types'

// 能量条渐变（每卡一色），与 dashboard 统计卡一致
const CARD_ACCENTS = [
  'from-sky-500 to-blue-500',
  'from-violet-500 to-purple-500',
  'from-emerald-500 to-teal-500',
  'from-amber-500 to-orange-500',
] as const

interface WalletStatsCardProps {
  user: UserWalletData | null
  loading?: boolean
}

export function WalletStatsCard(props: WalletStatsCardProps) {
  const { t } = useTranslation()
  if (props.loading) {
    return (
      <div className='grid grid-cols-1 gap-3 sm:grid-cols-3'>
        {Array.from({ length: 3 }).map((_, i) => (
          <div key={i} className='rounded-xl border p-4 shadow-sm'>
            <Skeleton className='h-3.5 w-20' />
            <Skeleton className='mt-2 h-7 w-28' />
            <Skeleton className='mt-1.5 h-3.5 w-24' />
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
    <div className='grid grid-cols-1 gap-3 sm:grid-cols-3'>
      {stats.map((item, idx) => {
        const Icon = item.icon
        const accent = CARD_ACCENTS[idx % CARD_ACCENTS.length]
        return (
          <div
            key={item.label}
            className='group rounded-xl border border-border bg-card p-4 shadow-sm transition-all duration-300 hover:scale-[1.03] hover:shadow-md'
          >
            <div className='flex items-center gap-2'>
              <Icon className='text-muted-foreground/70 size-3.5 shrink-0' />
              <div className='text-muted-foreground truncate text-xs font-medium tracking-wider uppercase'>
                {item.label}
              </div>
            </div>

            <div className='text-foreground mt-1.5 font-mono text-base font-bold tracking-tight break-all tabular-nums sm:mt-2 sm:text-2xl'>
              <CountUp value={item.rawValue} format={item.format} />
            </div>

            {item.detail ? (
              <div className='text-muted-foreground mt-1 text-xs md:block'>
                {item.detail}
              </div>
            ) : (
              <div className='text-muted-foreground/60 mt-1 hidden text-xs md:block'>
                {item.description}
              </div>
            )}

            {/* 能量条：默认 50%，hover 充到 80%（与 dashboard 统计卡一致）*/}
            <div className='mt-3 h-1.5 w-full overflow-hidden rounded-full bg-muted'>
              <div
                className={`h-full w-full origin-left scale-x-50 rounded-full bg-gradient-to-r ${accent} transition-transform duration-500 ease-out group-hover:scale-x-[0.8]`}
              />
            </div>
          </div>
        )
      })}
    </div>
  )
}
