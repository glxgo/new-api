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
import { useEffect, useState } from 'react'
import { Gauge, RotateCcw, Settings2, Users } from 'lucide-react'
import { toast } from 'sonner'
import { formatQuotaAsUSD } from '@/lib/currency'
import { formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { TitledCard } from '@/components/ui/titled-card'
import {
  activeResetVirtualMembership,
  getVirtualMembershipPage,
  payVirtualMembershipActiveResetEpay,
} from '@/features/virtual-membership/api'
import { VirtualMembershipManagementDialog } from '@/features/virtual-membership/components/virtual-membership-management-dialog'
import type { UserVirtualMembership } from '@/features/virtual-membership/types'

const quotaBarColor = (percent: number) =>
  percent >= 90 ? 'bg-destructive' : percent >= 70 ? 'bg-warning' : 'bg-success'

export function MyVirtualMembershipsDetail() {
  const [memberships, setMemberships] = useState<UserVirtualMembership[]>([])
  const [renderedAt] = useState(() => Date.now() / 1000)
  const [manageMembership, setManageMembership] =
    useState<UserVirtualMembership | null>(null)
  const [manageOpen, setManageOpen] = useState(false)
  const [resettingId, setResettingId] = useState<number | null>(null)

  const openEpayForm = (result: {
    url?: string
    data?: Record<string, unknown>
  }) => {
    if (!result.url) return false
    const form = document.createElement('form')
    form.action = result.url
    form.method = 'POST'
    const isSafari =
      typeof navigator !== 'undefined' &&
      /^((?!chrome|android).)*safari/i.test(navigator.userAgent)
    if (!isSafari) {
      form.target = '_blank'
    }
    Object.entries(result.data || {}).forEach(([key, value]) => {
      const input = document.createElement('input')
      input.type = 'hidden'
      input.name = key
      input.value = String(value)
      form.appendChild(input)
    })
    document.body.appendChild(form)
    form.submit()
    form.remove()
    return true
  }

  const activeReset = async (membership: UserVirtualMembership) => {
    if (resettingId !== null) return
    setResettingId(membership.id)
    try {
      if (membership.active_reset_credits > 0) {
        if (
          !window.confirm(
            '确认立即重置周限额和 5 小时限额吗？将消耗 1 次主动重置次数。'
          )
        )
          return
        const result = await activeResetVirtualMembership(membership.id)
        if (!result.success) {
          toast.error(result.message || '主动重置失败')
          return
        }
        toast.success('已主动重置额度')
        const refreshed = await getVirtualMembershipPage()
        if (refreshed.success) {
          setMemberships(
            (refreshed.data?.memberships || []).filter(
              (item) => item.status === 'active'
            )
          )
        }
        return
      }
      const method = (await getVirtualMembershipPage()).data?.epay_methods?.[0]
        ?.type
      const price = membership.active_reset_price_amount || 0
      if (!method || price < 0.01) {
        toast.error('主动重置支付暂不可用，请联系管理员')
        return
      }
      if (
        !window.confirm(
          `主动重置次数不足，是否购买 1 次？价格 ¥${price.toFixed(2)}`
        )
      )
        return
      const result = await payVirtualMembershipActiveResetEpay({
        membership_id: membership.id,
        payment_method: method,
      })
      if (result.message !== 'success' || !openEpayForm(result)) {
        toast.error(result.message || '支付请求失败')
        return
      }
      toast.success('支付页面已打开，支付成功后将获得 1 次主动重置')
    } catch {
      toast.error('主动重置请求失败，请稍后重试')
    } finally {
      setResettingId(null)
    }
  }

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
    <>
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
                  {membership.start_time > renderedAt ? '待生效' : '生效中'}
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
                <div className='border-border/70 bg-muted/30 flex items-center justify-between rounded-lg border px-2.5 py-2'>
                  <span className='text-muted-foreground text-[10px]'>
                    购买后累计已使用
                  </span>
                  <span className='text-xs font-semibold tabular-nums'>
                    {formatQuotaAsUSD(membership.lifetime_used || 0)}
                  </span>
                </div>
              </div>
              <div className='text-muted-foreground mt-3 text-[10px]'>
                {membership.start_time > renderedAt ? '生效时间' : '有效期至'}{' '}
                {formatTimestampToDate(
                  membership.start_time > renderedAt
                    ? membership.start_time
                    : membership.end_time
                )}{' '}
                · 会员 #{membership.id}
              </div>
              <Button
                type='button'
                variant='outline'
                size='sm'
                className='mt-3 h-7 w-full gap-1.5 text-[11px]'
                onClick={() => {
                  setManageMembership(membership)
                  setManageOpen(true)
                }}
              >
                <Settings2 className='size-3' />
                管理
              </Button>
              <Button
                type='button'
                variant='outline'
                size='sm'
                className='mt-2 h-7 w-full gap-1.5 text-[11px] text-violet-700'
                disabled={
                  resettingId === membership.id ||
                  membership.start_time > renderedAt ||
                  membership.status !== 'active'
                }
                onClick={() => void activeReset(membership)}
              >
                <RotateCcw
                  className={cn(
                    'size-3',
                    resettingId === membership.id && 'animate-spin'
                  )}
                />
                {membership.active_reset_credits > 0
                  ? `主动重置（剩余 ${membership.active_reset_credits} 次）`
                  : `购买主动重置次数（¥${(membership.active_reset_price_amount || 0).toFixed(2)}）`}
              </Button>
            </div>
          ))}
        </div>
      </TitledCard>
      <VirtualMembershipManagementDialog
        open={manageOpen}
        onOpenChange={setManageOpen}
        membership={manageMembership}
        onHidden={async () => {
          const response = await getVirtualMembershipPage()
          if (response.success) {
            setMemberships(
              (response.data?.memberships || []).filter(
                (membership) => membership.status === 'active'
              )
            )
          }
        }}
      />
    </>
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
          剩余 {formatQuotaAsUSD(Math.max(0, total - used))} /{' '}
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
