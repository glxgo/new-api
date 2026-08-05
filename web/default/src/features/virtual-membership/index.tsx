import { useEffect, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Gift, Gauge, Users, Zap } from 'lucide-react'
import { toast } from 'sonner'
import { formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Markdown } from '@/components/ui/markdown'
import { SectionPageLayout } from '@/components/layout'
import {
  getVirtualMembershipPage,
  payVirtualMembershipEpay,
  purchaseVirtualMembership,
} from './api'
import type { UserVirtualMembership, VirtualMembershipPlan } from './types'

function quotaLabel(value: number) {
  return value.toLocaleString('en-US')
}

function UsageBar({
  value,
  tone = 'bg-emerald-500',
}: {
  value: number
  tone?: string
}) {
  return (
    <div className='bg-muted h-2 overflow-hidden rounded-full'>
      <div
        className={cn('h-full rounded-full transition-all', tone)}
        style={{ width: `${Math.min(100, value)}%` }}
      />
    </div>
  )
}

function MembershipCard({ membership }: { membership: UserVirtualMembership }) {
  return (
    <div className='bg-card/90 rounded-2xl border p-5 shadow-sm'>
      <div className='flex items-start justify-between gap-3'>
        <div>
          <p className='text-lg font-semibold'>{membership.plan_title}</p>
          <p className='text-muted-foreground text-xs'>
            {membership.group_size === 1
              ? '单独购买'
              : `${membership.group_size} 人团 · 自动均分额度`}
          </p>
        </div>
        <span className='rounded-full bg-emerald-500/10 px-2.5 py-1 text-xs font-medium text-emerald-600'>
          {membership.status === 'active' ? '生效中' : membership.status}
        </span>
      </div>
      <div className='mt-5 space-y-4'>
        <div>
          <div className='mb-1.5 flex items-center justify-between text-sm'>
            <span className='font-medium'>周限额</span>
            <span className='text-muted-foreground tabular-nums'>
              {quotaLabel(membership.weekly_remaining)} /{' '}
              {quotaLabel(membership.weekly_quota)}
            </span>
          </div>
          <UsageBar value={membership.weekly_percent} tone='bg-amber-500' />
          <p className='text-muted-foreground mt-1 text-[11px]'>
            重置：{formatTimestampToDate(membership.weekly_reset_at)}
          </p>
        </div>
        {membership.five_hour_enabled && (
          <div>
            <div className='mb-1.5 flex items-center justify-between text-sm'>
              <span className='font-medium'>5 小时限额</span>
              <span className='text-muted-foreground tabular-nums'>
                {quotaLabel(membership.five_hour_remaining)} /{' '}
                {quotaLabel(membership.five_hour_quota)}
              </span>
            </div>
            <UsageBar value={membership.five_hour_percent} />
            <p className='text-muted-foreground mt-1 text-[11px]'>
              滚动重置：{formatTimestampToDate(membership.five_hour_reset_at)}
            </p>
          </div>
        )}
      </div>
      <p className='text-muted-foreground mt-4 text-xs'>
        有效期至 {formatTimestampToDate(membership.end_time)}
      </p>
    </div>
  )
}

function PlanCard({
  plan,
  onPurchase,
  onEpay,
  epayEnabled,
}: {
  plan: VirtualMembershipPlan
  onPurchase: (plan: VirtualMembershipPlan, groupSize: number) => void
  onEpay: (plan: VirtualMembershipPlan, groupSize: number) => void
  epayEnabled: boolean
}) {
  const [selectedGroup, setSelectedGroup] = useState(1)
  const variant =
    plan.variants.find((item) => item.group_size === selectedGroup) ??
    plan.variants[0]
  return (
    <div className='from-card to-muted/30 relative overflow-hidden rounded-2xl border bg-gradient-to-br p-5 shadow-sm'>
      {plan.recommended && (
        <span className='absolute top-4 right-4 rounded-full bg-emerald-500/10 px-2.5 py-1 text-xs font-medium text-emerald-600'>
          推荐
        </span>
      )}
      <div className='flex items-start gap-3'>
        <div className='flex size-11 items-center justify-center rounded-xl bg-emerald-500/10 text-emerald-600'>
          <Gift className='size-5' />
        </div>
        <div>
          <h3 className='text-xl font-semibold'>{plan.title}</h3>
          <p className='text-muted-foreground text-sm'>
            {plan.subtitle || `有效期 ${plan.duration_days} 天`}
          </p>
        </div>
      </div>
      {plan.description && (
        <p className='text-muted-foreground mt-4 text-sm'>{plan.description}</p>
      )}
      <div className='mt-5 grid grid-cols-2 gap-3'>
        <div className='rounded-xl bg-emerald-500/5 p-3'>
          <p className='text-muted-foreground text-xs'>周额度</p>
          <p className='mt-1 text-lg font-semibold'>
            {quotaLabel(variant?.weekly_quota ?? 0)}
          </p>
        </div>
        <div className='rounded-xl bg-blue-500/5 p-3'>
          <p className='text-muted-foreground text-xs'>5 小时额度</p>
          <p className='mt-1 text-lg font-semibold'>
            {plan.five_hour_enabled
              ? quotaLabel(variant?.five_hour_quota ?? 0)
              : '未开启'}
          </p>
        </div>
      </div>
      <div className='mt-4 grid grid-cols-2 gap-2 sm:grid-cols-4'>
        {plan.variants.map((item) => (
          <button
            key={item.group_size}
            type='button'
            onClick={() => setSelectedGroup(item.group_size)}
            className={cn(
              'rounded-xl border px-2 py-2 text-left transition',
              selectedGroup === item.group_size
                ? 'border-emerald-500 bg-emerald-500/10'
                : 'hover:bg-muted'
            )}
          >
            <span className='block text-xs font-medium'>{item.label}</span>
            <span className='mt-1 block text-sm font-semibold'>
              ¥{item.price_amount.toFixed(2)}
            </span>
          </button>
        ))}
      </div>
      <div className='mt-5 grid gap-2 sm:grid-cols-2'>
        <Button
          variant='outline'
          className='rounded-xl'
          onClick={() => onPurchase(plan, selectedGroup)}
        >
          钱包余额购买
        </Button>
        {epayEnabled && (
          <Button
            className='rounded-xl bg-emerald-600 hover:bg-emerald-700'
            onClick={() => onEpay(plan, selectedGroup)}
          >
            <Zap className='size-4' /> 支付宝支付
          </Button>
        )}
      </div>
    </div>
  )
}

export function VirtualMembership() {
  const [refresh, setRefresh] = useState(0)
  const [selectedEpayMethod, setSelectedEpayMethod] = useState('')
  const { data, isLoading } = useQuery({
    queryKey: ['virtual-membership-page', refresh],
    queryFn: getVirtualMembershipPage,
  })
  const page = data?.data
  const memberships = useMemo(
    () => page?.memberships ?? [],
    [page?.memberships]
  )

  useEffect(() => {
    const method = page?.epay_methods?.[0]?.type || ''
    setSelectedEpayMethod((current) => current || method)
  }, [page?.epay_methods])

  const handlePurchase = async (
    plan: VirtualMembershipPlan,
    groupSize: number
  ) => {
    if (
      !window.confirm(
        `确认购买 ${plan.title}（${groupSize === 1 ? '单独购买' : `${groupSize} 人团`}）？将从钱包余额扣款。`
      )
    )
      return
    try {
      const result = await purchaseVirtualMembership({
        plan_id: plan.id,
        group_size: groupSize,
      })
      if (result.success) {
        toast.success('虚拟会员已开通')
        setRefresh((value) => value + 1)
      }
    } catch {
      // Global API interceptor already shows the server error.
    }
  }

  const handleEpay = async (plan: VirtualMembershipPlan, groupSize: number) => {
    if (!selectedEpayMethod) {
      toast.error('暂无可用的支付宝支付方式')
      return
    }
    if (
      !window.confirm(
        `确认购买 ${plan.title}（${groupSize === 1 ? '单独购买' : `${groupSize} 人团`}）并使用支付宝支付？`
      )
    )
      return
    try {
      const result = await payVirtualMembershipEpay({
        plan_id: plan.id,
        group_size: groupSize,
        payment_method: selectedEpayMethod,
      })
      if (result.message === 'success' && result.url) {
        const form = document.createElement('form')
        form.action = result.url
        form.method = 'POST'
        form.target = '_blank'
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
      } else {
        toast.error(result.message || '支付请求失败')
      }
    } catch {
      toast.error('支付请求失败')
    }
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>虚拟会员</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='space-y-5'>
          {page?.announcement && (
            <div className='to-card rounded-2xl border border-emerald-500/20 bg-gradient-to-br from-emerald-500/10 p-5'>
              <Markdown>{page.announcement}</Markdown>
            </div>
          )}
          {page?.epay_enabled && (page.epay_methods?.length ?? 0) > 0 && (
            <div className='flex flex-wrap items-center gap-3 rounded-2xl border border-emerald-500/20 bg-emerald-500/5 p-4'>
              <span className='text-sm font-medium'>支付宝支付方式</span>
              <select
                value={selectedEpayMethod}
                onChange={(event) => setSelectedEpayMethod(event.target.value)}
                className='bg-background rounded-lg border px-3 py-2 text-sm'
              >
                {page.epay_methods?.map((method) => (
                  <option key={method.type} value={method.type}>
                    {method.name || method.type}
                  </option>
                ))}
              </select>
              <span className='text-muted-foreground text-xs'>
                复用订阅套餐的 Epay 配置
              </span>
            </div>
          )}
          <div className='grid gap-3 sm:grid-cols-3'>
            <div className='bg-card rounded-2xl border p-4'>
              <Gauge className='size-5 text-emerald-600' />
              <p className='mt-3 text-sm font-semibold'>站内额度映射</p>
              <p className='text-muted-foreground mt-1 text-xs'>
                不是实际 GPT 会员，只提供对应额度。
              </p>
            </div>
            <div className='bg-card rounded-2xl border p-4'>
              <Users className='size-5 text-blue-600' />
              <p className='mt-3 text-sm font-semibold'>自动成团</p>
              <p className='text-muted-foreground mt-1 text-xs'>
                2/3/4 人档位只均分额度，不等待其他成员。
              </p>
            </div>
            <div className='bg-card rounded-2xl border p-4'>
              <Zap className='size-5 text-amber-600' />
              <p className='mt-3 text-sm font-semibold'>双周期额度</p>
              <p className='text-muted-foreground mt-1 text-xs'>
                管理员可按需开启 5 小时限额。
              </p>
            </div>
          </div>
          {memberships.length > 0 && (
            <div>
              <h2 className='mb-3 text-lg font-semibold'>我的虚拟会员</h2>
              <div className='grid gap-4 lg:grid-cols-2'>
                {memberships.map((item) => (
                  <MembershipCard key={item.id} membership={item} />
                ))}
              </div>
            </div>
          )}
          <div>
            <h2 className='mb-3 text-lg font-semibold'>选择方案</h2>
            {isLoading ? (
              <div className='text-muted-foreground rounded-2xl border p-8 text-center'>
                正在加载方案…
              </div>
            ) : (
              <div className='grid gap-4 lg:grid-cols-3'>
                {(page?.plans ?? []).map((plan) => (
                  <PlanCard
                    key={plan.id}
                    plan={plan}
                    onPurchase={handlePurchase}
                    onEpay={handleEpay}
                    epayEnabled={!!page?.epay_enabled}
                  />
                ))}
              </div>
            )}
          </div>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
