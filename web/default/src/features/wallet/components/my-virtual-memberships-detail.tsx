import { useEffect, useState } from 'react'
import { Gauge, RotateCcw, Users } from 'lucide-react'
import { formatQuotaAsUSD } from '@/lib/currency'
import { formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'
import { TitledCard } from '@/components/ui/titled-card'
import { getVirtualMembershipPage } from '@/features/virtual-membership/api'
import type { UserVirtualMembership } from '@/features/virtual-membership/types'

const quotaBarColor = (percent: number) =>
  percent >= 90 ? 'bg-destructive' : percent >= 70 ? 'bg-warning' : 'bg-success'

export function MyVirtualMembershipsDetail() {
  const [memberships, setMemberships] = useState<UserVirtualMembership[]>([])

  useEffect(() => {
    let active = true
    getVirtualMembershipPage()
      .then((response) => {
        if (active && response.success) {
          setMemberships(
            (response.data?.memberships || []).filter(
              (membership) => membership.status === 'active'
            )
          )
        }
      })
      .catch(() => {
        if (active) setMemberships([])
      })
    return () => {
      active = false
    }
  }, [])

  if (memberships.length === 0) return null

  return (
    <TitledCard
      title='我的虚拟会员'
      icon={<Gauge className='h-4 w-4' />}
      disableHoverEffect
      contentClassName='p-3 sm:p-5'
    >
      <div className='grid gap-3 lg:grid-cols-2'>
        {memberships.map((membership) => (
          <div
            key={membership.id}
            className='bg-card rounded-xl border p-4 shadow-sm'
          >
            <div className='flex items-start justify-between gap-3'>
              <div className='min-w-0'>
                <div className='truncate text-sm font-semibold'>
                  {membership.plan_title}
                </div>
                <div className='text-muted-foreground mt-1 flex items-center gap-1 text-[10px]'>
                  <Users className='size-3' />
                  {membership.group_size === 1
                    ? '单独购买'
                    : `${membership.group_size} 人团 · 自动均分额度`}
                </div>
              </div>
              <span className='bg-success/10 text-success shrink-0 rounded-full px-2 py-0.5 text-[10px] font-medium'>
                生效中
              </span>
            </div>

            <div className='mt-4 space-y-3'>
              <QuotaLine
                label='周限额'
                used={membership.weekly_used}
                total={membership.weekly_quota}
                percent={membership.weekly_percent}
                resetAt={membership.weekly_reset_at}
              />
              {membership.five_hour_enabled && (
                <QuotaLine
                  label='5 小时限额'
                  used={membership.five_hour_used}
                  total={membership.five_hour_quota}
                  percent={membership.five_hour_percent}
                  resetAt={membership.five_hour_reset_at}
                />
              )}
            </div>
            <div className='text-muted-foreground mt-3 text-[10px]'>
              有效期至 {formatTimestampToDate(membership.end_time)} · 会员 #
              {membership.id}
            </div>
          </div>
        ))}
      </div>
    </TitledCard>
  )
}

function QuotaLine({
  label,
  used,
  total,
  percent,
  resetAt,
}: {
  label: string
  used: number
  total: number
  percent: number
  resetAt: number
}) {
  return (
    <div>
      <div className='mb-1 flex items-baseline justify-between gap-2'>
        <span className='text-muted-foreground text-[10px]'>{label}</span>
        <span className='text-xs tabular-nums'>
          {formatQuotaAsUSD(Math.max(0, total - used))} /{' '}
          {formatQuotaAsUSD(total)}
        </span>
      </div>
      <div className='bg-muted h-1.5 overflow-hidden rounded-full'>
        <div
          className={cn(
            'h-full rounded-full transition-all',
            quotaBarColor(percent)
          )}
          style={{ width: `${Math.min(100, Math.max(0, percent))}%` }}
        />
      </div>
      <div className='text-muted-foreground mt-1 flex items-center gap-1 text-[10px]'>
        <RotateCcw className='size-3' />
        重置于 {formatTimestampToDate(resetAt)}
      </div>
    </div>
  )
}
