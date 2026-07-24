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
import { Gauge, LockKeyhole, ShieldAlert, Sparkles } from 'lucide-react'
import { cn } from '@/lib/utils'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import {
  Progress,
  ProgressLabel,
  ProgressValue,
} from '@/components/ui/progress'
import { Skeleton } from '@/components/ui/skeleton'
import type { RechargeCapacityTier, UserWalletData } from '../types'

interface RechargeCapacityCardProps {
  user: UserWalletData | null
  loading?: boolean
}

const formatMoney = (cents: number) =>
  new Intl.NumberFormat('zh-CN', {
    style: 'currency',
    currency: 'CNY',
    minimumFractionDigits: cents % 100 === 0 ? 0 : 2,
    maximumFractionDigits: 2,
  }).format(Math.max(0, cents) / 100)

const tierRange = (tier: RechargeCapacityTier) => {
  const min = tier.minimum_cents / 100
  if (!tier.maximum_cents) return `¥${min}+`
  return `¥${min}–${tier.maximum_cents / 100}`
}

const formatRestrictionStatus = (user: UserWalletData) => {
  if (user.security_permanent_ban) return '当前为永久限制，请联系管理员申诉。'
  if (!user.security_suspended_until) return ''
  const restoreAt = new Intl.DateTimeFormat('zh-CN', {
    dateStyle: 'medium',
    timeStyle: 'medium',
    timeZone: 'Asia/Shanghai',
  }).format(new Date(user.security_suspended_until * 1000))
  return `预计恢复时间：${restoreAt}。`
}

export function RechargeCapacityCard({
  user,
  loading,
}: RechargeCapacityCardProps) {
  if (loading) {
    return (
      <div className='bg-card rounded-2xl border p-5'>
        <Skeleton className='h-5 w-40' />
        <Skeleton className='mt-5 h-16 w-full' />
        <Skeleton className='mt-5 h-28 w-full' />
      </div>
    )
  }

  const capacity = user?.recharge_capacity
  if (!capacity?.enabled) return null

  const next = capacity.next_tier
  const progressPercent = Math.round(
    Math.min(1, Math.max(0, capacity.progress || 0)) * 100
  )
  const hasSecurityRestriction = Boolean(user?.security_restriction_active)

  return (
    <section className='border-border bg-card overflow-hidden rounded-2xl border shadow-sm'>
      <div className='border-border relative border-b bg-[radial-gradient(circle_at_top_right,var(--muted),transparent_42%)] p-5 sm:p-6'>
        <div className='flex flex-col justify-between gap-5 sm:flex-row sm:items-start'>
          <div>
            <div className='text-muted-foreground flex items-center gap-2 text-xs font-semibold tracking-[0.18em] uppercase'>
              <Gauge className='size-3.5' />
              账号通行能力
            </div>
            <h2 className='mt-2 text-xl font-semibold tracking-tight'>
              累计充值解锁并发
            </h2>
            <p className='text-muted-foreground mt-1 max-w-2xl text-sm'>
              已支付充值、管理员加额和在线支付订阅都会累计；赠金、兑换码、签到及余额购买不计入。
            </p>
          </div>

          <div className='bg-background/85 grid min-w-52 grid-cols-2 overflow-hidden rounded-xl border backdrop-blur'>
            <div className='border-r p-3 text-center'>
              <div className='text-muted-foreground text-[11px] tracking-wider uppercase'>
                并发
              </div>
              <div className='mt-1 font-mono text-2xl font-semibold tabular-nums'>
                {capacity.concurrency_limit}
              </div>
            </div>
            <div className='p-3 text-center'>
              <div className='text-muted-foreground text-[11px] tracking-wider uppercase'>
                RPM
              </div>
              <div className='mt-1 font-mono text-2xl font-semibold tabular-nums'>
                {capacity.rpm_limit}
              </div>
            </div>
          </div>
        </div>

        <div className='mt-6'>
          <Progress value={progressPercent} className='gap-2'>
            <ProgressLabel>
              累计 {formatMoney(capacity.total_cents)}
            </ProgressLabel>
            <ProgressValue>
              {() =>
                next
                  ? `距下一档还差 ${formatMoney(capacity.remaining_cents)}`
                  : '已解锁最高档'
              }
            </ProgressValue>
          </Progress>
          {next && (
            <div className='text-muted-foreground mt-2 flex items-center gap-1.5 text-xs'>
              <Sparkles className='size-3.5' />
              累计达到 {formatMoney(next.minimum_cents)}，解锁并发{' '}
              {next.concurrency_limit} / RPM {next.rpm_limit}
            </div>
          )}
        </div>
      </div>

      {hasSecurityRestriction && (
        <div className='px-5 pt-5 sm:px-6'>
          <Alert variant='destructive'>
            <ShieldAlert className='size-4' />
            <AlertTitle>API 安全限制生效中</AlertTitle>
            <AlertDescription>
              当前累计 {user?.security_strike_count || 0} 次有效警告。
              {user ? formatRestrictionStatus(user) : ''}
              充值容量不会绕过安全限制。
            </AlertDescription>
          </Alert>
        </div>
      )}

      <div className='p-5 sm:p-6'>
        <div className='mb-3 flex items-center justify-between'>
          <h3 className='text-sm font-semibold'>全部解锁档位</h3>
          <span className='text-muted-foreground text-xs'>
            达到左侧金额即生效
          </span>
        </div>
        <div className='grid gap-2 sm:grid-cols-2 lg:grid-cols-4'>
          {capacity.tiers.map((tier) => {
            const active =
              tier.minimum_cents === capacity.current_tier.minimum_cents
            const unlocked = capacity.total_cents >= tier.minimum_cents
            return (
              <div
                key={tier.minimum_cents}
                className={cn(
                  'relative rounded-xl border p-3.5 transition-colors',
                  active
                    ? 'border-foreground bg-foreground text-background'
                    : unlocked
                      ? 'border-border bg-muted/45'
                      : 'bg-background border-dashed'
                )}
              >
                <div className='flex items-center justify-between gap-2'>
                  <span
                    className={cn(
                      'font-mono text-xs font-semibold',
                      active ? 'text-background' : 'text-muted-foreground'
                    )}
                  >
                    {tierRange(tier)}
                  </span>
                  {!unlocked && (
                    <LockKeyhole className='text-muted-foreground size-3.5' />
                  )}
                </div>
                <div className='mt-3 flex items-end justify-between gap-3'>
                  <div>
                    <div className='font-mono text-lg font-semibold tabular-nums'>
                      {tier.concurrency_limit}
                    </div>
                    <div
                      className={cn(
                        'text-[11px]',
                        active ? 'text-background/70' : 'text-muted-foreground'
                      )}
                    >
                      并发
                    </div>
                  </div>
                  <div className='text-right'>
                    <div className='font-mono text-lg font-semibold tabular-nums'>
                      {tier.rpm_limit}
                    </div>
                    <div
                      className={cn(
                        'text-[11px]',
                        active ? 'text-background/70' : 'text-muted-foreground'
                      )}
                    >
                      RPM
                    </div>
                  </div>
                </div>
              </div>
            )
          })}
        </div>
      </div>
    </section>
  )
}
