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
import { BadgePercent, LockKeyhole, ShieldAlert } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import {
  Progress,
  ProgressLabel,
  ProgressValue,
} from '@/components/ui/progress'
import { Skeleton } from '@/components/ui/skeleton'
import type { UserWalletData } from '../types'

interface RechargeCapacityCardProps {
  user: UserWalletData | null
  loading?: boolean
}

export function RechargeCapacityCard(props: RechargeCapacityCardProps) {
  const { t, i18n } = useTranslation()
  const money = (cents: number) =>
    new Intl.NumberFormat(i18n.language, {
      style: 'currency',
      currency: 'CNY',
      maximumFractionDigits: 2,
    }).format(Math.max(0, cents) / 100)
  const rateLabel = (rate: number) =>
    rate >= 1
      ? t('Standard recharge price')
      : t('Recharge rate {{rate}}%', { rate: Math.round(rate * 100) })
  if (props.loading) return <Skeleton className='h-48 w-full rounded-xl' />
  const discount = props.user?.recharge_discount
  if (!discount) return null
  const next = discount.next_tier
  return (
    <section className='border-border bg-card overflow-hidden rounded-xl border p-4 shadow-sm sm:p-5'>
      <div className='flex flex-wrap items-start justify-between gap-3'>
        <div>
          <div className='text-muted-foreground flex items-center gap-2 text-xs font-semibold'>
            <BadgePercent className='size-3.5' aria-hidden />
            {t('Cumulative recharge discount')}
          </div>
          <h2 className='mt-2 text-base font-semibold'>
            {t('Recharge more, save more')}
          </h2>
          <p className='text-muted-foreground mt-1 text-xs'>
            {t('Qualified recharge total')}: {money(discount.total_cents)}
          </p>
        </div>
        <div className='bg-muted/20 rounded-lg border px-3 py-2 text-right'>
          <p className='text-muted-foreground text-[10px]'>
            {t('Current recharge rate')}
          </p>
          <span className='font-mono text-xl font-semibold tabular-nums'>
            {rateLabel(discount.current_tier.rate)}
          </span>
        </div>
      </div>
      <Progress
        value={Math.round(discount.progress * 100)}
        className='mt-4 gap-1.5'
      >
        <ProgressLabel>
          {next
            ? t('Next tier: {{rate}}', { rate: rateLabel(next.rate) })
            : t('Highest discount unlocked')}
        </ProgressLabel>
        <ProgressValue>
          {() =>
            next
              ? t('{{amount}} to go', {
                  amount: money(discount.remaining_cents),
                })
              : '100%'
          }
        </ProgressValue>
      </Progress>
      <div className='mt-3 grid grid-cols-2 gap-1.5 sm:grid-cols-5'>
        {discount.tiers.map((tier) => {
          const active =
            tier.minimum_cents === discount.current_tier.minimum_cents
          const unlocked = discount.total_cents >= tier.minimum_cents
          let style = 'bg-background border-dashed'
          if (active) style = 'border-foreground bg-foreground text-background'
          else if (unlocked) style = 'border-border bg-muted/45'
          return (
            <div
              key={tier.minimum_cents}
              className={cn('rounded-lg border px-3 py-2.5 text-xs', style)}
            >
              <div className='flex items-center justify-between gap-1 font-mono'>
                {money(tier.minimum_cents)}
                {!unlocked && <LockKeyhole className='size-3' aria-hidden />}
              </div>
              <p className='mt-1.5 font-semibold'>{rateLabel(tier.rate)}</p>
            </div>
          )
        })}
      </div>
      <p className='text-muted-foreground mt-3 text-xs leading-relaxed'>
        {t(
          'Recharge discounts stack with amount discounts and coupons. Completed recharge unlocks the rate for your next payment.'
        )}
      </p>
      <p className='text-muted-foreground mt-1 text-xs leading-relaxed'>
        {t(
          'All accounts include at least 200 concurrent requests and 1000 RPM. Higher limits are retained; qualified recharge of ¥1000 removes both limits.'
        )}
      </p>
      {props.user?.security_restriction_active && (
        <Alert variant='destructive' className='mt-3'>
          <ShieldAlert className='size-4' />
          <AlertTitle>{t('API security restriction active')}</AlertTitle>
          <AlertDescription>
            {t('Contact an administrator to review your account restriction.')}
          </AlertDescription>
        </Alert>
      )}
    </section>
  )
}
