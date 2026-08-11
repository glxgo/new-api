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
import {
  BadgeDollarSign,
  Coins,
  Crown,
  HandCoins,
  Receipt,
  Wallet,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatQuota } from '@/lib/format'
import { Skeleton } from '@/components/ui/skeleton'
import type { ProfitSummary } from '../types'

interface ProfitStatCardsProps {
  summary: ProfitSummary | null
  loading?: boolean
}

type StatItem = {
  label: string
  value: string
  description: string
  icon: typeof Wallet
  tone: 'default' | 'accent'
}

export function ProfitStatCards({ summary, loading }: ProfitStatCardsProps) {
  const { t } = useTranslation()

  if (loading || !summary) {
    return (
      <div className='overflow-hidden rounded-lg border'>
        <div className='divide-border/60 grid grid-cols-2 divide-x sm:grid-cols-3 lg:grid-cols-4'>
          {Array.from({ length: 6 }).map((_, i) => (
            <div key={i} className='px-3 py-3 sm:px-5 sm:py-4'>
              <Skeleton className='h-3.5 w-20' />
              <Skeleton className='mt-2 h-7 w-24' />
            </div>
          ))}
        </div>
      </div>
    )
  }

  const stats: StatItem[] = [
    {
      label: t('实付充值'),
      value: new Intl.NumberFormat('zh-CN', {
        style: 'currency',
        currency: 'CNY',
      }).format(summary.paid_recharge_cents / 100),
      description: t('仅统计真实外部付款'),
      icon: Receipt,
      tone: 'default',
    },
    {
      label: t('实付订单'),
      value: String(summary.paid_order_count),
      description: t('钱包余额购买不重复计入'),
      icon: Coins,
      tone: 'default',
    },
    {
      label: t('邀新返利'),
      value: formatQuota(summary.affiliate_rebate),
      description: t('普通用户 5%/2%，代理 8%/4%'),
      icon: HandCoins,
      tone: 'accent',
    },
    {
      label: t('管理员分润'),
      value: formatQuota(summary.admin_dividend),
      description: t('直属 15%，二级 5%'),
      icon: Wallet,
      tone: 'accent',
    },
    {
      label: t('超管分润'),
      value: formatQuota(summary.root_dividend),
      description: t('全部真实付款固定 5%'),
      icon: Crown,
      tone: 'accent',
    },
    {
      label: t('本期分润合计'),
      value: formatQuota(summary.total_commission),
      description: t('仅统计新系统上线后的充值分润'),
      icon: BadgeDollarSign,
      tone: 'accent',
    },
  ]

  const toneClass: Record<StatItem['tone'], string> = {
    default: 'text-foreground',
    accent: 'text-primary',
  }

  return (
    <div className='overflow-hidden rounded-lg border'>
      <div className='divide-border/60 grid grid-cols-2 divide-x sm:grid-cols-3 lg:grid-cols-4'>
        {stats.map((item) => (
          <div key={item.label} className='px-3 py-3 sm:px-5 sm:py-4'>
            <div className='flex items-center gap-2'>
              <item.icon className='text-muted-foreground/60 size-3.5 shrink-0' />
              <div className='text-muted-foreground truncate text-xs font-medium tracking-wider uppercase'>
                {item.label}
              </div>
            </div>
            <div
              className={`mt-1.5 font-mono text-base font-bold tracking-tight break-all tabular-nums sm:mt-2 sm:text-2xl ${toneClass[item.tone]}`}
            >
              {item.value}
            </div>
            <div className='text-muted-foreground/60 mt-1 hidden text-xs md:block'>
              {item.description}
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
