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
import { Gauge, LockKeyhole, ShieldAlert } from 'lucide-react'
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
      <div className='bg-card rounded-xl border p-4'>
        <Skeleton className='h-5 w-40' />
        <Skeleton className='mt-4 h-12 w-full' />
        <Skeleton className='mt-4 h-14 w-full' />
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
    <section className='border-border bg-card overflow-hidden rounded-xl border p-4 shadow-sm sm:p-5'>
      <div className='flex flex-col justify-between gap-3 sm:flex-row sm:items-start'>
        <div className='min-w-0'>
          <div className='flex items-center gap-2'>
            <div className='text-muted-foreground flex items-center gap-2 text-xs font-semibold tracking-[0.18em] uppercase'>
              <Gauge className='size-3.5' />
              账号通行能力
            </div>
            <span className='text-muted-foreground hidden text-xs sm:inline'>
              · 累计有效充值自动解锁
            </span>
          </div>
          <div className='mt-2 flex items-baseline gap-2'>
            <h2 className='text-base font-semibold tracking-tight'>
              当前请求容量
            </h2>
            <span className='text-muted-foreground text-xs'>
              累计 {formatMoney(capacity.total_cents)}
            </span>
          </div>
        </div>

        <div className='bg-muted/20 grid shrink-0 grid-cols-2 overflow-hidden rounded-lg border'>
          <div className='flex items-baseline gap-1.5 border-r px-3 py-2'>
            <div className='font-mono text-xl font-semibold tabular-nums'>
              {capacity.concurrency_limit}
            </div>
            <div className='text-muted-foreground text-[10px] uppercase'>
              并发
            </div>
          </div>
          <div className='flex items-baseline gap-1.5 px-3 py-2'>
            <div className='font-mono text-xl font-semibold tabular-nums'>
              {capacity.rpm_limit}
            </div>
            <div className='text-muted-foreground text-[10px] uppercase'>
              RPM
            </div>
          </div>
        </div>
      </div>

      <Progress value={progressPercent} className='mt-3 gap-1.5'>
        <ProgressLabel>
          {next
            ? `下一档：并发 ${next.concurrency_limit} / RPM ${next.rpm_limit}`
            : '已解锁最高档'}
        </ProgressLabel>
        <ProgressValue>
          {() =>
            next ? `还差 ${formatMoney(capacity.remaining_cents)}` : '100%'
          }
        </ProgressValue>
      </Progress>

      {hasSecurityRestriction && (
        <div className='mt-3'>
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

      <div className='mt-3 grid gap-1.5 sm:grid-cols-2 lg:grid-cols-4'>
        {capacity.tiers.map((tier) => {
          const active =
            tier.minimum_cents === capacity.current_tier.minimum_cents
          const unlocked = capacity.total_cents >= tier.minimum_cents
          return (
            <div
              key={tier.minimum_cents}
              className={cn(
                'flex items-center justify-between gap-2 rounded-lg border px-2.5 py-2 text-xs',
                active
                  ? 'border-foreground bg-foreground text-background'
                  : unlocked
                    ? 'border-border bg-muted/45'
                    : 'bg-background border-dashed'
              )}
            >
              <span
                className={cn(
                  'font-mono font-semibold',
                  active ? 'text-background' : 'text-muted-foreground'
                )}
              >
                {tierRange(tier)}
              </span>
              <span className='flex items-center gap-1 font-mono tabular-nums'>
                {!unlocked && <LockKeyhole className='size-3' />}
                并发 {tier.concurrency_limit} · RPM {tier.rpm_limit}
              </span>
            </div>
          )
        })}
      </div>
    </section>
  )
}
