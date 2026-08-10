import { useEffect, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import {
  Gift,
  Gauge,
  Loader2,
  RefreshCw,
  ShieldCheck,
  Users,
  Zap,
} from 'lucide-react'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import { DEFAULT_CURRENCY_CONFIG } from '@/stores/system-config-store'
import { formatQuotaAsUSD } from '@/lib/currency'
import { formatQuota, formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'
import { useSystemConfig } from '@/hooks/use-system-config'
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
  return formatQuota(value)
}

function membershipQuotaLabel(value: number) {
  return formatQuotaAsUSD(value, {
    digitsLarge: 2,
    digitsSmall: 4,
    abbreviate: true,
  })
}

function capacityLabel(value: number) {
  return value > 0 ? value.toLocaleString('en-US') : '不限'
}

function UsageBar({
  value,
  tone = 'bg-emerald-500',
}: {
  value: number
  tone?: string
}) {
  return (
    <div className='bg-muted h-1.5 overflow-hidden rounded-full'>
      <div
        className={cn('h-full rounded-full transition-all', tone)}
        style={{ width: `${Math.min(100, value)}%` }}
      />
    </div>
  )
}

function MembershipCard({ membership }: { membership: UserVirtualMembership }) {
  return (
    <div className='bg-card/90 rounded-xl border p-4 shadow-sm'>
      <div className='flex items-start justify-between gap-3'>
        <div className='min-w-0'>
          <p className='truncate text-base font-semibold'>
            {membership.plan_title}
          </p>
          <p className='text-muted-foreground text-xs'>
            {membership.group_size === 1
              ? '单独购买'
              : `${membership.group_size} 人团 · 自动均分额度`}
          </p>
        </div>
        <span className='shrink-0 rounded-full bg-emerald-500/10 px-2 py-0.5 text-[10px] font-medium text-emerald-600'>
          {membership.status === 'active' ? '生效中' : membership.status}
        </span>
      </div>
      <div className='mt-3 space-y-3'>
        <div>
          <div className='mb-1 flex items-center justify-between text-xs'>
            <span className='font-medium'>周限额</span>
            <span className='text-muted-foreground tabular-nums'>
              {membershipQuotaLabel(membership.weekly_remaining)} /{' '}
              {membershipQuotaLabel(membership.weekly_quota)}
            </span>
          </div>
          <UsageBar value={membership.weekly_percent} tone='bg-amber-500' />
          <p className='text-muted-foreground mt-1 text-[11px]'>
            重置：{formatTimestampToDate(membership.weekly_reset_at)}
          </p>
        </div>
        {membership.five_hour_enabled && (
          <div>
            <div className='mb-1 flex items-center justify-between text-xs'>
              <span className='font-medium'>5 小时限额</span>
              <span className='text-muted-foreground tabular-nums'>
                {membershipQuotaLabel(membership.five_hour_remaining)} /{' '}
                {membershipQuotaLabel(membership.five_hour_quota)}
              </span>
            </div>
            <UsageBar value={membership.five_hour_percent} />
            <p className='text-muted-foreground mt-1 text-[11px]'>
              滚动重置：{formatTimestampToDate(membership.five_hour_reset_at)}
            </p>
          </div>
        )}
        <div className='border-border/70 bg-muted/30 flex items-center justify-between rounded-lg border px-2.5 py-2 text-xs'>
          <span className='text-muted-foreground'>购买后累计已使用</span>
          <span className='font-semibold tabular-nums'>
            {membershipQuotaLabel(membership.lifetime_used || 0)}
          </span>
        </div>
        <div className='grid grid-cols-2 gap-2 text-xs'>
          <div className='bg-muted/40 rounded-lg px-2.5 py-2'>
            <p className='text-muted-foreground text-[10px]'>会员并发</p>
            <p className='mt-0.5 font-semibold'>
              {capacityLabel(membership.concurrency_limit)}
            </p>
          </div>
          <div className='bg-muted/40 rounded-lg px-2.5 py-2'>
            <p className='text-muted-foreground text-[10px]'>会员 RPM</p>
            <p className='mt-0.5 font-semibold'>
              {capacityLabel(membership.rpm_limit)}
            </p>
          </div>
        </div>
      </div>
      <p className='text-muted-foreground mt-3 text-[11px]'>
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
  userQuota,
  onConfirm,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  plan: VirtualMembershipPlan | null
  groupSize: number
  epayMethods: { type: string; name?: string }[]
  userQuota: number
  onConfirm: (payment: PaymentSelection) => Promise<boolean>
}) {
  const { currency } = useSystemConfig()
  const [selectedEpayMethod, setSelectedEpayMethod] = useState('')
  const [submitting, setSubmitting] = useState(false)

  useEffect(() => {
    let active = true
    void Promise.resolve().then(() => {
      if (!active) return
      setSelectedEpayMethod(
        open && epayMethods.length > 0 ? epayMethods[0].type : ''
      )
    })
    return () => {
      active = false
    }
  }, [open, epayMethods])

  if (!plan) return null

  const variant =
    plan.variants.find((item) => item.group_size === groupSize) ??
    plan.variants[0]
  const groupLabel =
    variant?.label ?? (groupSize === 1 ? '单独购买' : `${groupSize} 人团`)
  const selectedEpayMethodLabel =
    epayMethods.find((item) => item.type === selectedEpayMethod)?.name ||
    selectedEpayMethod ||
    '请选择付款方式'
  const quotaPerUnit =
    currency?.quotaPerUnit && currency.quotaPerUnit > 0
      ? currency.quotaPerUnit
      : DEFAULT_CURRENCY_CONFIG.quotaPerUnit
  const balanceCost = Math.max(
    0,
    Math.ceil((variant?.price_amount ?? 0) * quotaPerUnit)
  )
  const availableQuota = Math.max(0, userQuota)
  const insufficientBalance = availableQuota < balanceCost

  const confirm = async (payment: PaymentSelection) => {
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
        <div className='flex items-center justify-between gap-3 text-sm'>
          <span className='text-muted-foreground'>会员并发 / RPM</span>
          <span className='font-medium'>
            {capacityLabel(variant?.concurrency_limit ?? 0)} /{' '}
            {capacityLabel(variant?.rpm_limit ?? 0)}
          </span>
        </div>
      </div>

      <div className='flex flex-col gap-2 rounded-xl border p-4'>
        <div className='flex items-center justify-between gap-2 text-xs'>
          <span className='text-muted-foreground'>必需</span>
          <span>{formatQuota(balanceCost)}</span>
        </div>
        <div className='flex items-center justify-between gap-2 text-xs'>
          <span className='text-muted-foreground'>可用</span>
          <span>{formatQuota(availableQuota)}</span>
        </div>
        {insufficientBalance && (
          <p className='text-destructive rounded-lg border px-3 py-2 text-sm'>
            钱包余额不足
          </p>
        )}
        <Button
          type='button'
          variant='outline'
          onClick={() => void confirm({ type: 'balance' })}
          disabled={submitting || insufficientBalance}
        >
          使用余额支付
        </Button>
      </div>

      {epayMethods.length > 0 && (
        <div className='space-y-3'>
          <p className='text-muted-foreground text-xs'>选择支付方式</p>
          <div className='grid grid-cols-[minmax(0,1fr)_auto] gap-2'>
            <Select
              items={epayMethods.map((method) => ({
                value: method.type,
                label: method.name || method.type,
              }))}
              value={selectedEpayMethod}
              onValueChange={(value) =>
                value !== null && setSelectedEpayMethod(value)
              }
            >
              <SelectTrigger className='w-full'>
                <SelectValue>{selectedEpayMethodLabel}</SelectValue>
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
              onClick={() =>
                void confirm({ type: 'epay', method: selectedEpayMethod })
              }
              disabled={submitting || !selectedEpayMethod}
            >
              {submitting && <Loader2 className='animate-spin' />}
              支付
            </Button>
          </div>
        </div>
      )}
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
              <span className='mt-1 flex flex-wrap items-baseline gap-2'>
                <span className='text-primary text-base font-semibold'>
                  ¥{item.price_amount.toFixed(2)}
                </span>
                {item.original_price_amount > item.price_amount && (
                  <span className='text-muted-foreground text-xs tabular-nums line-through decoration-1'>
                    ¥{item.original_price_amount.toFixed(2)}
                  </span>
                )}
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
              <span className='text-muted-foreground mt-1 block text-[11px]'>
                并发 {capacityLabel(item.concurrency_limit)} · RPM{' '}
                {capacityLabel(item.rpm_limit)}
              </span>
            </span>
          </button>
        ))}
      </div>
      <Button
        type='button'
        className='mt-5 w-full rounded-xl bg-emerald-600 hover:bg-emerald-700'
        onClick={(event) => {
          event.preventDefault()
          onPurchase(plan, selectedGroup)
        }}
        aria-label={`购买 ${plan.title}`}
      >
        <Zap className='size-4' /> 立即购买
      </Button>
    </div>
  )
}

export function VirtualMembership() {
  const userQuota = useAuthStore((state) => state.auth.user?.quota ?? 0)
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
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>虚拟会员</SectionPageLayout.Title>
        <SectionPageLayout.Content>
          <div className='space-y-5'>
            {page?.announcement?.trim() && (
              <div className='bg-card rounded-2xl border border-emerald-500/20 bg-gradient-to-br from-emerald-500/10 p-5'>
                <Markdown>{page.announcement}</Markdown>
              </div>
            )}
            <div className='grid gap-3 sm:grid-cols-2 xl:grid-cols-4'>
              <div className='bg-card h-full rounded-2xl border p-4'>
                <Gauge className='size-5 text-emerald-600' />
                <p className='mt-3 text-sm font-semibold'>站内会员权益映射</p>
                <p className='text-muted-foreground mt-1 text-xs leading-relaxed'>
                  不提供实际 GPT
                  会员，由本站进行托管，提供对应额度与可用的生图功能。无需复杂网络环境，无需复杂支付环境
                </p>
              </div>
              <div className='bg-card h-full rounded-2xl border p-4'>
                <Users className='size-5 text-blue-600' />
                <p className='mt-3 text-sm font-semibold'>自动成团</p>
                <p className='text-muted-foreground mt-1 text-xs leading-relaxed'>
                  拼团均分额度的同时，还均分正常一个账号所对应的并发和rpm限制，无需其他成员，即可自动成团。
                </p>
              </div>
              <div className='bg-card h-full rounded-2xl border p-4'>
                <RefreshCw className='size-5 text-amber-600' />
                <p className='mt-3 text-sm font-semibold'>同步官方周期重置</p>
                <p className='text-muted-foreground mt-1 text-xs leading-relaxed'>
                  oai官方重置套餐额度，我们也重置相对应的套餐额度
                </p>
              </div>
              <div className='bg-card h-full rounded-2xl border p-4'>
                <ShieldCheck className='size-5 text-rose-600' />
                <p className='mt-3 text-sm font-semibold'>站内承担风险</p>
                <p className='text-muted-foreground mt-1 text-xs leading-relaxed'>
                  本站承担托管账号的全部风险与成本（家宽费、流量费等），出现封号等情况，损失由本站自行承担
                </p>
              </div>
            </div>
            {memberships.length > 0 && (
              <div>
                <h2 className='mb-3 text-lg font-semibold'>我的虚拟会员</h2>
                <div className='grid gap-3 md:grid-cols-2 xl:grid-cols-3'>
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
      </SectionPageLayout>
      <VirtualMembershipPurchaseDialog
        open={purchaseOpen}
        onOpenChange={setPurchaseOpen}
        plan={purchasePlan}
        groupSize={purchaseGroupSize}
        epayMethods={page?.epay_methods ?? []}
        userQuota={userQuota}
        onConfirm={handlePurchase}
      />
    </>
  )
}
