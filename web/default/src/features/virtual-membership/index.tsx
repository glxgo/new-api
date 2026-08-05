import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Gift, Gauge, Loader2, Users, Zap } from 'lucide-react'
import { toast } from 'sonner'
import { formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Markdown } from '@/components/ui/markdown'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Dialog } from '@/components/dialog'
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

type PaymentSelection = { type: 'balance' } | { type: 'epay'; method: string }

function VirtualMembershipPurchaseDialog({
  open,
  onOpenChange,
  plan,
  groupSize,
  epayMethods,
  onConfirm,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  plan: VirtualMembershipPlan | null
  groupSize: number
  epayMethods: { type: string; name?: string }[]
  onConfirm: (payment: PaymentSelection) => Promise<boolean>
}) {
  const [payment, setPayment] = useState<PaymentSelection>({ type: 'balance' })
  const [submitting, setSubmitting] = useState(false)

  if (!plan) return null

  const variant =
    plan.variants.find((item) => item.group_size === groupSize) ??
    plan.variants[0]
  const groupLabel =
    variant?.label ?? (groupSize === 1 ? '单独购买' : `${groupSize} 人团`)
  const paymentLabel =
    payment.type === 'balance'
      ? '钱包余额'
      : epayMethods.find((item) => item.type === payment.method)?.name ||
        payment.method

  const confirm = async () => {
    setSubmitting(true)
    try {
      if (await onConfirm(payment)) onOpenChange(false)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title='立即购买'
      description='请选择付款方式，确认后将进入对应的支付流程。'
      contentClassName='max-sm:w-[calc(100vw-1.5rem)] sm:max-w-md'
      bodyClassName='space-y-4'
    >
      <div className='bg-muted/40 space-y-2 rounded-xl border p-4'>
        <div className='flex items-center justify-between gap-3'>
          <span className='text-muted-foreground text-sm'>方案</span>
          <span className='font-medium'>{plan.title}</span>
        </div>
        <div className='flex items-center justify-between gap-3'>
          <span className='text-muted-foreground text-sm'>购买档位</span>
          <span className='font-medium'>{groupLabel}</span>
        </div>
        <div className='flex items-center justify-between gap-3'>
          <span className='text-muted-foreground text-sm'>应付金额</span>
          <span className='text-primary text-xl font-bold'>
            ¥{(variant?.price_amount ?? 0).toFixed(2)}
          </span>
        </div>
        <div className='flex items-center justify-between gap-3'>
          <span className='text-muted-foreground text-sm'>获得周额度</span>
          <span className='font-medium'>
            {quotaLabel(variant?.weekly_quota ?? 0)}
          </span>
        </div>
        {plan.five_hour_enabled && (
          <div className='flex items-center justify-between gap-3'>
            <span className='text-muted-foreground text-sm'>
              获得 5 小时额度
            </span>
            <span className='font-medium'>
              {quotaLabel(variant?.five_hour_quota ?? 0)}
            </span>
          </div>
        )}
      </div>

      <div className='space-y-2'>
        <p className='text-sm font-medium'>付款方式</p>
        <div className='grid gap-2'>
          <Button
            type='button'
            variant={payment.type === 'balance' ? 'default' : 'outline'}
            className='justify-start'
            onClick={() => setPayment({ type: 'balance' })}
          >
            钱包余额
          </Button>
          {epayMethods.length > 0 && (
            <div className='grid grid-cols-[minmax(0,1fr)_auto] gap-2'>
              <Select
                items={epayMethods.map((method) => ({
                  value: method.type,
                  label: method.name || method.type,
                }))}
                value={payment.type === 'epay' ? payment.method : ''}
                onValueChange={(value) =>
                  value && setPayment({ type: 'epay', method: value })
                }
              >
                <SelectTrigger
                  className={cn(
                    'w-full',
                    payment.type === 'epay' &&
                      'border-primary bg-primary/5 text-primary'
                  )}
                  onClick={() => {
                    if (payment.type !== 'epay') {
                      setPayment({ type: 'epay', method: epayMethods[0].type })
                    }
                  }}
                >
                  <SelectValue>{paymentLabel}</SelectValue>
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    {epayMethods.map((method) => (
                      <SelectItem key={method.type} value={method.type}>
                        {method.name || method.type}
                      </SelectItem>
                    ))}
                  </SelectGroup>
                </SelectContent>
              </Select>
              <Button
                type='button'
                variant={payment.type === 'epay' ? 'default' : 'outline'}
                onClick={() =>
                  setPayment({ type: 'epay', method: epayMethods[0].type })
                }
              >
                选择
              </Button>
            </div>
          )}
        </div>
        <p className='text-muted-foreground text-xs'>
          当前选择：{paymentLabel}
        </p>
      </div>

      <Button className='w-full' onClick={confirm} disabled={submitting}>
        {submitting && <Loader2 className='animate-spin' />}
        确认支付
      </Button>
    </Dialog>
  )
}

function PlanCard({
  plan,
  onPurchase,
}: {
  plan: VirtualMembershipPlan
  onPurchase: (plan: VirtualMembershipPlan, groupSize: number) => void
}) {
  const [selectedGroup, setSelectedGroup] = useState(1)
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
      {plan.description?.trim() && (
        <Markdown className='text-muted-foreground mt-4 text-sm'>
          {plan.description}
        </Markdown>
      )}
      <div className='bg-background/60 mt-5 overflow-hidden rounded-2xl border'>
        <div className='text-muted-foreground bg-muted/30 grid grid-cols-[minmax(0,1fr)_auto] gap-3 border-b px-4 py-3 text-xs font-medium'>
          <span>购买档位 / 价格</span>
          <span>周额度</span>
        </div>
        {plan.variants.map((item) => (
          <button
            key={item.group_size}
            type='button'
            onClick={() => setSelectedGroup(item.group_size)}
            className={cn(
              'grid w-full grid-cols-[minmax(0,1fr)_auto] items-center gap-3 border-b px-4 py-3 text-left transition last:border-b-0',
              selectedGroup === item.group_size
                ? 'bg-emerald-500/10'
                : 'hover:bg-muted/70'
            )}
          >
            <span>
              <span className='block text-sm font-medium'>{item.label}</span>
              <span className='text-primary mt-1 block text-base font-semibold'>
                ¥{item.price_amount.toFixed(2)}
              </span>
            </span>
            <span className='text-right'>
              <span className='block text-base font-semibold'>
                {quotaLabel(item.weekly_quota)}
              </span>
              {plan.five_hour_enabled && (
                <span className='text-muted-foreground mt-1 block text-[11px]'>
                  5h {quotaLabel(item.five_hour_quota)}
                </span>
              )}
            </span>
          </button>
        ))}
      </div>
      <Button
        className='mt-5 w-full rounded-xl bg-emerald-600 hover:bg-emerald-700'
        onClick={() => onPurchase(plan, selectedGroup)}
      >
        <Zap className='size-4' /> 立即购买
      </Button>
    </div>
  )
}

export function VirtualMembership() {
  const [refresh, setRefresh] = useState(0)
  const [purchasePlan, setPurchasePlan] =
    useState<VirtualMembershipPlan | null>(null)
  const [purchaseGroupSize, setPurchaseGroupSize] = useState(1)
  const [purchaseOpen, setPurchaseOpen] = useState(false)
  const { data, isLoading } = useQuery({
    queryKey: ['virtual-membership-page', refresh],
    queryFn: getVirtualMembershipPage,
  })
  const page = data?.data
  const memberships = useMemo(
    () => page?.memberships ?? [],
    [page?.memberships]
  )

  const openPurchase = (plan: VirtualMembershipPlan, groupSize: number) => {
    setPurchasePlan(plan)
    setPurchaseGroupSize(groupSize)
    setPurchaseOpen(true)
  }

  const handlePurchase = async (payment: PaymentSelection) => {
    if (!purchasePlan) return false
    try {
      if (payment.type === 'balance') {
        const result = await purchaseVirtualMembership({
          plan_id: purchasePlan.id,
          group_size: purchaseGroupSize,
        })
        if (result.success) {
          toast.success('虚拟会员已开通')
          setRefresh((value) => value + 1)
          return true
        }
        toast.error(result.message || '支付请求失败')
        return false
      }

      const result = await payVirtualMembershipEpay({
        plan_id: purchasePlan.id,
        group_size: purchaseGroupSize,
        payment_method: payment.method,
      })
      if (result.message !== 'success' || !result.url) {
        toast.error(result.message || '支付请求失败')
        return false
      }
      const form = document.createElement('form')
      form.action = result.url
      form.method = 'POST'
      const isSafari =
        typeof navigator !== 'undefined' &&
        /^((?!chrome|android).)*safari/i.test(navigator.userAgent)
      if (!isSafari) form.target = '_blank'
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
      toast.success('支付页面已打开')
      return true
    } catch {
      toast.error('支付请求失败')
      return false
    }
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>虚拟会员</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='space-y-5'>
          {page?.announcement?.trim() && (
            <div className='bg-card rounded-2xl border border-emerald-500/20 bg-gradient-to-br from-emerald-500/10 p-5'>
              <Markdown>{page.announcement}</Markdown>
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
                    onPurchase={openPurchase}
                  />
                ))}
              </div>
            )}
          </div>
        </div>
      </SectionPageLayout.Content>
      <VirtualMembershipPurchaseDialog
        open={purchaseOpen}
        onOpenChange={setPurchaseOpen}
        plan={purchasePlan}
        groupSize={purchaseGroupSize}
        epayMethods={page?.epay_methods ?? []}
        onConfirm={handlePurchase}
      />
    </SectionPageLayout>
  )
}
