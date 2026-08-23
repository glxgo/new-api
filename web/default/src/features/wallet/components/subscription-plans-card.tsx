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
import { useState, useEffect, useMemo, useCallback } from 'react'
import {
  Crown,
  RefreshCw,
  Check,
  Pencil,
  Settings2,
  RotateCw,
  ListOrdered,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatQuota } from '@/lib/format'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import { Markdown } from '@/components/ui/markdown'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import { TitledCard } from '@/components/ui/titled-card'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { GooeyButton } from '@/components/reactbits/gooey-button'
import { SpecularCard } from '@/components/reactbits/specular-card'
import '@/components/reactbits/sub-effects.css'
import { dotColorMap, textColorMap } from '@/components/status-badge'
import {
  getPublicPlans,
  getSelfSubscriptionFull,
  getSubscriptionRenewalPreview,
  setSelfSubscriptionHidden,
  updateBillingPreference,
} from '@/features/subscriptions/api'
import { SubscriptionConsumptionOrderDialog } from '@/features/subscriptions/components/dialogs/subscription-consumption-order-dialog'
import { SubscriptionInstanceManagementDialog } from '@/features/subscriptions/components/dialogs/subscription-instance-management-dialog'
import { SubscriptionInstanceRemarkDialog } from '@/features/subscriptions/components/dialogs/subscription-instance-remark-dialog'
import { SubscriptionPurchaseDialog } from '@/features/subscriptions/components/dialogs/subscription-purchase-dialog'
import {
  versionLabelOf,
  PLAN_VERSION_STYLES,
} from '@/features/subscriptions/constants'
import { formatDuration, formatResetPeriod } from '@/features/subscriptions/lib'
import type {
  PlanRecord,
  SubscriptionRenewalPreview,
  UserSubscription,
  UserSubscriptionRecord,
} from '@/features/subscriptions/types'
import type { PaymentMethod, TopupInfo } from '../types'

interface SubscriptionPlansCardProps {
  topupInfo: TopupInfo | null
  onAvailabilityChange?: (available: boolean) => void
  userQuota?: number
  onPurchaseSuccess?: () => void | Promise<void>
}

function getEpayMethods(payMethods: PaymentMethod[] = []): PaymentMethod[] {
  return payMethods.filter(
    (m) => m?.type && m.type !== 'stripe' && m.type !== 'creem'
  )
}

function getBillingPreferenceLabel(
  preference: string,
  t: (key: string) => string
): string {
  switch (preference) {
    case 'subscription_first':
      return t('Subscription First')
    case 'wallet_first':
      return t('Wallet First')
    case 'subscription_only':
      return t('Subscription Only')
    case 'wallet_only':
      return t('Wallet Only')
    default:
      return preference
  }
}

export function SubscriptionPlansCard({
  topupInfo,
  onAvailabilityChange,
  userQuota,
  onPurchaseSuccess,
}: SubscriptionPlansCardProps) {
  const { t } = useTranslation()

  const [plans, setPlans] = useState<PlanRecord[]>([])
  const [activeSubscriptions, setActiveSubscriptions] = useState<
    UserSubscriptionRecord[]
  >([])
  const [allSubscriptions, setAllSubscriptions] = useState<
    UserSubscriptionRecord[]
  >([])
  const [billingPreference, setBillingPreference] =
    useState('subscription_first')
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const [subscriptionReferenceTime, setSubscriptionReferenceTime] = useState(0)

  const [purchaseOpen, setPurchaseOpen] = useState(false)
  const [selectedPlan, setSelectedPlan] = useState<PlanRecord | null>(null)
  const [renewalPreview, setRenewalPreview] =
    useState<SubscriptionRenewalPreview | null>(null)
  const [renewingId, setRenewingId] = useState<number | null>(null)
  const [selectedSubscription, setSelectedSubscription] =
    useState<UserSubscription | null>(null)
  const [managementOpen, setManagementOpen] = useState(false)
  const [remarkOpen, setRemarkOpen] = useState(false)
  const [consumptionOrderOpen, setConsumptionOrderOpen] = useState(false)
  const [descPlan, setDescPlan] = useState<PlanRecord | null>(null)
  const [restoringId, setRestoringId] = useState<number | null>(null)

  const enableStripe = !!topupInfo?.enable_stripe_topup
  const enableCreem = !!topupInfo?.enable_creem_topup
  const enableWaffoPancake = !!topupInfo?.enable_waffo_pancake_topup
  const enableOnlineTopUp = !!topupInfo?.enable_online_topup
  const epayMethods = useMemo(
    () => getEpayMethods(topupInfo?.pay_methods),
    [topupInfo?.pay_methods]
  )

  const fetchPlans = useCallback(async () => {
    try {
      const res = await getPublicPlans()
      if (res.success) {
        setPlans(res.data || [])
      }
    } catch {
      setPlans([])
    }
  }, [])

  const fetchSelfSubscription = useCallback(async () => {
    try {
      const res = await getSelfSubscriptionFull()
      if (res.success && res.data) {
        setSubscriptionReferenceTime(Date.now() / 1000)
        setBillingPreference(
          res.data.billing_preference || 'subscription_first'
        )
        setActiveSubscriptions(res.data.subscriptions || [])
        setAllSubscriptions(res.data.all_subscriptions || [])
      }
    } catch {
      // ignore
    }
  }, [])

  useEffect(() => {
    const init = async () => {
      setLoading(true)
      await Promise.all([fetchPlans(), fetchSelfSubscription()])
      setLoading(false)
    }
    init()
  }, [fetchPlans, fetchSelfSubscription])

  const handleRefresh = async () => {
    setRefreshing(true)
    try {
      await fetchSelfSubscription()
    } finally {
      setRefreshing(false)
    }
  }

  const handlePreferenceChange = async (pref: string) => {
    const previous = billingPreference
    setBillingPreference(pref)
    try {
      const res = await updateBillingPreference(pref)
      if (res.success) {
        toast.success(t('Updated successfully'))
        const normalized = res.data?.billing_preference || pref
        setBillingPreference(normalized)
      } else {
        toast.error(res.message || t('Update failed'))
        setBillingPreference(previous)
      }
    } catch {
      toast.error(t('Request failed'))
      setBillingPreference(previous)
    }
  }

  const hasActive = activeSubscriptions.length > 0
  const hasAny = allSubscriptions.length > 0
  const hiddenSubscriptions = allSubscriptions.filter(
    (sub) =>
      sub?.subscription?.hidden &&
      (sub.subscription?.end_time || 0) > subscriptionReferenceTime
  )
  const isAvailable = loading || plans.length > 0 || hasAny
  const disablePref = !hasActive
  const isSubPref =
    billingPreference === 'subscription_first' ||
    billingPreference === 'subscription_only'
  const displayPref =
    disablePref && isSubPref ? 'wallet_first' : billingPreference

  const planPurchaseCountMap = useMemo(() => {
    const map = new Map<number, number>()
    for (const sub of allSubscriptions) {
      const planId = sub?.subscription?.plan_id
      if (!planId) continue
      map.set(planId, (map.get(planId) || 0) + 1)
    }
    return map
  }, [allSubscriptions])

  useEffect(() => {
    onAvailabilityChange?.(isAvailable)
  }, [isAvailable, onAvailabilityChange])

  const planTitleMap = useMemo(() => {
    const map = new Map<number, string>()
    for (const p of plans) {
      if (p?.plan?.id) {
        map.set(p.plan.id, p.plan.title || '')
      }
    }
    return map
  }, [plans])

  const getRemainingDays = (sub: UserSubscriptionRecord) => {
    const endTime = sub?.subscription?.end_time || 0
    if (!endTime) return 0
    return Math.max(0, Math.ceil((endTime - subscriptionReferenceTime) / 86400))
  }

  const getUsagePercent = (sub: UserSubscriptionRecord) => {
    const total = Number(sub?.subscription?.amount_total || 0)
    const used = Number(sub?.subscription?.amount_used || 0)
    if (total <= 0) return 0
    return Math.round((used / total) * 100)
  }

  const handleRenew = async (subscription: UserSubscription) => {
    setRenewingId(subscription.id)
    try {
      const res = await getSubscriptionRenewalPreview(subscription.id)
      if (!res.success || !res.data) {
        toast.error(res.message || t('Renewal is not available'))
        return
      }
      setRenewalPreview(res.data)
      setSelectedPlan({ plan: res.data.plan })
      setPurchaseOpen(true)
    } catch {
      toast.error(t('Renewal is not available'))
    } finally {
      setRenewingId(null)
    }
  }

  const restoreSubscription = async (subscriptionId: number) => {
    setRestoringId(subscriptionId)
    try {
      const res = await setSelfSubscriptionHidden(subscriptionId, false)
      if (!res.success) {
        toast.error(res.message || t('Update failed'))
        return
      }
      toast.success('已恢复展示')
      await fetchSelfSubscription()
    } catch {
      toast.error(t('Request failed'))
    } finally {
      setRestoringId(null)
    }
  }

  if (loading) {
    return (
      <div className='subscription-responsive-scope'>
        <Card data-card-hover='false' className='gap-0 overflow-hidden py-0'>
          <CardHeader className='border-b p-3 !pb-3 sm:p-5 sm:!pb-5'>
            <Skeleton className='h-6 w-32' />
          </CardHeader>
          <CardContent className='space-y-4 p-3 sm:p-5'>
            <Skeleton className='h-20 w-full' />
            <div className='subscription-card-grid'>
              {Array.from({ length: 4 }).map((_, i) => (
                <Skeleton key={i} className='h-48 w-full' />
              ))}
            </div>
          </CardContent>
        </Card>
      </div>
    )
  }

  if (plans.length === 0 && !hasAny) {
    return null
  }

  return (
    <>
      <div className='subscription-responsive-scope'>
        <TitledCard
          title={t('Subscription Plans')}
          description={t('Subscribe to a plan for model access')}
          icon={<Crown className='h-4 w-4' />}
          disableHoverEffect
          contentClassName='space-y-4 sm:space-y-5'
        >
          {/* My subscriptions & billing preference */}
          <div className='rounded-xl border p-3 sm:p-4'>
            <div className='flex flex-wrap items-center justify-between gap-2.5 sm:gap-3'>
              <div className='flex min-w-0 flex-wrap items-center gap-2'>
                <span className='text-sm font-medium'>
                  {t('My Subscriptions')}
                </span>
                <span className='flex items-center gap-1.5 text-xs font-medium'>
                  <span
                    className={cn(
                      'size-1.5 shrink-0 rounded-full',
                      hasActive ? dotColorMap.success : dotColorMap.neutral
                    )}
                    aria-hidden='true'
                  />
                  {hasActive ? (
                    <span className={cn(textColorMap.success)}>
                      {activeSubscriptions.length} {t('active')}
                    </span>
                  ) : (
                    <span className='text-muted-foreground'>
                      {t('No Active')}
                    </span>
                  )}
                </span>
              </div>
              <div className='flex w-full items-center gap-2 sm:w-auto'>
                <Select
                  items={[
                    {
                      value: 'subscription_first',
                      label: (
                        <>
                          {getBillingPreferenceLabel('subscription_first', t)}
                          {disablePref ? ` (${t('No Active')})` : ''}
                        </>
                      ),
                    },
                    {
                      value: 'wallet_first',
                      label: getBillingPreferenceLabel('wallet_first', t),
                    },
                    {
                      value: 'subscription_only',
                      label: (
                        <>
                          {getBillingPreferenceLabel('subscription_only', t)}
                          {disablePref ? ` (${t('No Active')})` : ''}
                        </>
                      ),
                    },
                    {
                      value: 'wallet_only',
                      label: getBillingPreferenceLabel('wallet_only', t),
                    },
                  ]}
                  value={displayPref}
                  onValueChange={(v) => v !== null && handlePreferenceChange(v)}
                >
                  <SelectTrigger className='h-8 flex-1 text-xs sm:w-[140px] sm:flex-none'>
                    <SelectValue>
                      {getBillingPreferenceLabel(displayPref, t)}
                    </SelectValue>
                  </SelectTrigger>
                  <SelectContent alignItemWithTrigger={false}>
                    <SelectGroup>
                      <SelectItem
                        value='subscription_first'
                        disabled={disablePref}
                      >
                        {getBillingPreferenceLabel('subscription_first', t)}
                        {disablePref ? ` (${t('No Active')})` : ''}
                      </SelectItem>
                      <SelectItem value='wallet_first'>
                        {getBillingPreferenceLabel('wallet_first', t)}
                      </SelectItem>
                      <SelectItem
                        value='subscription_only'
                        disabled={disablePref}
                      >
                        {getBillingPreferenceLabel('subscription_only', t)}
                        {disablePref ? ` (${t('No Active')})` : ''}
                      </SelectItem>
                      <SelectItem value='wallet_only'>
                        {getBillingPreferenceLabel('wallet_only', t)}
                      </SelectItem>
                    </SelectGroup>
                  </SelectContent>
                </Select>
                <Button
                  variant='outline'
                  size='sm'
                  className='h-8 gap-1.5 px-2 text-xs'
                  onClick={() => setConsumptionOrderOpen(true)}
                  disabled={!hasActive}
                >
                  <ListOrdered className='size-3.5' />
                  消耗顺序
                </Button>
                <Button
                  variant='ghost'
                  size='icon'
                  className='h-8 w-8'
                  onClick={handleRefresh}
                  disabled={refreshing}
                >
                  <RefreshCw
                    className={`h-3.5 w-3.5 ${refreshing ? 'animate-spin' : ''}`}
                  />
                </Button>
              </div>
            </div>

            {disablePref && isSubPref && (
              <p className='text-muted-foreground mt-2 text-xs'>
                {t(
                  'Preference saved as {{pref}}, but no active subscription. Wallet will be used automatically.',
                  {
                    pref:
                      billingPreference === 'subscription_only'
                        ? t('Subscription Only')
                        : t('Subscription First'),
                  }
                )}
              </p>
            )}

            {hasActive && (
              <>
                <Separator className='my-3' />
                <div className='subscription-card-grid'>
                  {activeSubscriptions.map((sub) => {
                    const subscription = sub.subscription
                    const totalAmount = Number(subscription?.amount_total || 0)
                    const usedAmount = Number(subscription?.amount_used || 0)
                    const remainAmount =
                      totalAmount > 0
                        ? Math.max(0, totalAmount - usedAmount)
                        : 0
                    const capTotal = Number(subscription?.amount_cap || 0)
                    const capUsed = Number(subscription?.amount_cap_used || 0)
                    const capRemain =
                      capTotal > 0 ? Math.max(0, capTotal - capUsed) : 0
                    const capPercent =
                      capTotal > 0 ? Math.round((capUsed / capTotal) * 100) : 0
                    const allowedGroup = subscription?.allowed_group || ''
                    const planTitle =
                      subscription?.plan_title ||
                      planTitleMap.get(subscription?.plan_id) ||
                      ''
                    const remainDays = getRemainingDays(sub)
                    const usagePercent = getUsagePercent(sub)
                    const remainPercent =
                      totalAmount > 0 ? Math.max(0, 100 - usagePercent) : 0
                    const capRemainPercent =
                      capTotal > 0 ? Math.max(0, 100 - capPercent) : 0
                    const quotaBarColor = (pct: number) =>
                      pct >= 90
                        ? 'bg-destructive'
                        : pct >= 70
                          ? 'bg-warning'
                          : 'bg-success'
                    const now = subscriptionReferenceTime
                    const isExpired = (subscription?.end_time || 0) < now
                    const isUpcoming = (subscription?.start_time || 0) > now
                    const isCancelled = subscription?.status === 'cancelled'
                    const isActive =
                      subscription?.status === 'active' &&
                      !isExpired &&
                      !isUpcoming

                    return (
                      <div
                        key={subscription?.id}
                        className='bg-card hover:border-primary/40 relative flex flex-col overflow-hidden rounded-lg border transition-[transform,box-shadow,border-color] duration-200 hover:-translate-y-0.5 hover:shadow-[0_8px_24px_rgba(0,0,0,0.08)] dark:hover:shadow-[0_8px_24px_rgba(0,0,0,0.4)]'
                      >
                        <div className='flex flex-col gap-2 p-3'>
                          {/* 标题 + 剩余天数药丸 */}
                          <div className='flex items-start justify-between gap-2'>
                            <div className='min-w-0'>
                              <div className='truncate text-sm font-semibold'>
                                {planTitle ||
                                  `${t('Subscription')} #${subscription?.id}`}
                              </div>
                              <div className='text-muted-foreground text-[10px]'>
                                {t('Subscription')} #{subscription?.id}
                              </div>
                            </div>
                            {isActive ? (
                              <span className='bg-success/10 text-success shrink-0 rounded-full px-2 py-0.5 text-[10px] font-medium'>
                                {t('{{count}} days remaining', {
                                  count: remainDays,
                                })}
                              </span>
                            ) : (
                              <span className='bg-muted text-muted-foreground shrink-0 rounded-full px-2 py-0.5 text-[10px] font-medium'>
                                {isCancelled
                                  ? t('Cancelled')
                                  : isUpcoming
                                    ? t('Upcoming')
                                    : t('Expired')}
                              </span>
                            )}
                          </div>

                          {/* 获得时间 / 到期时间 */}
                          <div className='grid grid-cols-2 gap-2'>
                            <div>
                              <div className='text-muted-foreground text-[10px]'>
                                {t('Start Time')}
                              </div>
                              <div className='text-xs'>
                                {new Date(
                                  (subscription?.start_time || 0) * 1000
                                ).toLocaleDateString()}
                              </div>
                            </div>
                            <div>
                              <div className='text-muted-foreground text-[10px]'>
                                {isActive
                                  ? t('Until')
                                  : isCancelled
                                    ? t('Cancelled at')
                                    : t('Expired at')}
                              </div>
                              <div className='text-xs'>
                                {new Date(
                                  (subscription?.end_time || 0) * 1000
                                ).toLocaleDateString()}
                              </div>
                            </div>
                          </div>

                          {/* 可用端点 */}
                          {allowedGroup && (
                            <div className='flex items-center gap-1 text-[10px]'>
                              <span className='text-muted-foreground'>
                                {t('Available Endpoint')}
                              </span>
                              <span className='bg-muted rounded px-1.5 py-0.5 font-medium'>
                                {allowedGroup}
                              </span>
                            </div>
                          )}

                          {/* 本周/周期额度 */}
                          {totalAmount > 0 && (
                            <div className='space-y-1'>
                              <div className='flex items-baseline justify-between'>
                                <span className='text-muted-foreground text-[10px]'>
                                  {t('Period Quota')}
                                </span>
                                <span className='text-[10px]'>
                                  <span className='text-muted-foreground'>
                                    {t('Remaining')}{' '}
                                  </span>
                                  <span className='text-primary text-xs font-bold'>
                                    {formatQuota(remainAmount)}
                                  </span>
                                  <span className='text-muted-foreground'>
                                    {' '}
                                    / {formatQuota(totalAmount)}
                                  </span>
                                </span>
                              </div>
                              <div className='bg-primary/10 h-2.5 w-full overflow-hidden rounded-full'>
                                <div
                                  className={cn(
                                    'h-full rounded-full transition-all',
                                    quotaBarColor(usagePercent)
                                  )}
                                  style={{
                                    width: `${Math.min(100, remainPercent)}%`,
                                  }}
                                />
                              </div>
                            </div>
                          )}

                          {/* 月/总额度上限 */}
                          {capTotal > 0 && (
                            <div className='space-y-1'>
                              <div className='flex items-baseline justify-between'>
                                <span className='text-muted-foreground text-[10px]'>
                                  {t('Total Cap')}
                                </span>
                                <span className='text-[10px]'>
                                  <span className='text-muted-foreground'>
                                    {t('Remaining')}{' '}
                                  </span>
                                  <span className='text-primary text-xs font-bold'>
                                    {formatQuota(capRemain)}
                                  </span>
                                  <span className='text-muted-foreground'>
                                    {' '}
                                    / {formatQuota(capTotal)}
                                  </span>
                                </span>
                              </div>
                              <div className='bg-primary/10 h-2.5 w-full overflow-hidden rounded-full'>
                                <div
                                  className={cn(
                                    'h-full rounded-full transition-all',
                                    quotaBarColor(capPercent)
                                  )}
                                  style={{
                                    width: `${Math.min(100, capRemainPercent)}%`,
                                  }}
                                />
                              </div>
                            </div>
                          )}

                          {/* 周期 */}
                          {isActive &&
                            (subscription?.next_reset_time ?? 0) > 0 && (
                              <div className='text-muted-foreground text-[10px]'>
                                {t('Next reset')}:{' '}
                                {new Date(
                                  subscription!.next_reset_time! * 1000
                                ).toLocaleString()}
                              </div>
                            )}

                          {subscription?.remark && (
                            <div className='bg-muted/60 rounded-md px-2 py-1.5 text-[11px]'>
                              <span className='text-muted-foreground'>
                                {t('Remark')}：
                              </span>
                              {subscription.remark}
                            </div>
                          )}

                          <div className='grid grid-cols-3 gap-1.5 pt-1'>
                            <Button
                              type='button'
                              variant='outline'
                              size='sm'
                              className='h-7 px-2 text-[11px]'
                              disabled={!isActive}
                              onClick={() => {
                                setSelectedSubscription(subscription)
                                setManagementOpen(true)
                              }}
                            >
                              <Settings2 className='mr-1 h-3 w-3' />
                              {t('Manage')}
                            </Button>
                            <Button
                              type='button'
                              variant='outline'
                              size='sm'
                              className='h-7 px-2 text-[11px]'
                              onClick={() => {
                                setSelectedSubscription(subscription)
                                setRemarkOpen(true)
                              }}
                            >
                              <Pencil className='mr-1 h-3 w-3' />
                              {t('Remark')}
                            </Button>
                            <Button
                              type='button'
                              size='sm'
                              className='h-7 px-2 text-[11px]'
                              disabled={
                                isCancelled || renewingId === subscription.id
                              }
                              onClick={() => handleRenew(subscription)}
                            >
                              <RotateCw
                                className={cn(
                                  'mr-1 h-3 w-3',
                                  renewingId === subscription.id &&
                                    'animate-spin'
                                )}
                              />
                              {t('Renew')}
                            </Button>
                          </div>
                        </div>

                        {/* 底部能量条已移除 */}
                      </div>
                    )
                  })}
                </div>
              </>
            )}

            {hiddenSubscriptions.length > 0 && (
              <>
                <Separator className='my-3' />
                <div className='space-y-2'>
                  <div className='flex items-center gap-2 text-xs font-medium'>
                    <span>已隐藏的用量卡片</span>
                    <span className='bg-muted text-muted-foreground rounded-full px-2 py-0.5 text-[10px]'>
                      {hiddenSubscriptions.length}
                    </span>
                  </div>
                  <div className='subscription-card-grid'>
                    {hiddenSubscriptions.map((sub) => {
                      const subscription = sub.subscription
                      const planTitle =
                        subscription?.plan_title ||
                        planTitleMap.get(subscription?.plan_id) ||
                        ''
                      const remainDays = getRemainingDays(sub)
                      return (
                        <div
                          key={subscription?.id}
                          className='bg-card/70 relative flex flex-col overflow-hidden rounded-lg border border-dashed p-3 opacity-80'
                        >
                          <div className='flex items-start justify-between gap-2'>
                            <div className='min-w-0'>
                              <div className='truncate text-sm font-semibold'>
                                {planTitle ||
                                  `${t('Subscription')} #${subscription?.id}`}
                              </div>
                              <div className='text-muted-foreground text-[10px]'>
                                {t('Subscription')} #{subscription?.id}
                              </div>
                            </div>
                            <span className='bg-muted text-muted-foreground shrink-0 rounded-full px-2 py-0.5 text-[10px] font-medium'>
                              已隐藏
                            </span>
                          </div>
                          <div className='text-muted-foreground mt-2 text-[10px]'>
                            {t('Until')}{' '}
                            {new Date(
                              (subscription?.end_time || 0) * 1000
                            ).toLocaleDateString()}
                            {remainDays > 0 &&
                              ` · ${t('{{count}} days remaining', { count: remainDays })}`}
                          </div>
                          <Button
                            type='button'
                            variant='outline'
                            size='sm'
                            className='mt-3 h-7 gap-1.5 text-[11px]'
                            disabled={restoringId === subscription?.id}
                            onClick={() => {
                              if (subscription?.id) {
                                restoreSubscription(subscription.id)
                              }
                            }}
                          >
                            <RotateCw
                              className={cn(
                                'size-3',
                                restoringId === subscription?.id &&
                                  'animate-spin'
                              )}
                            />
                            {restoringId === subscription?.id
                              ? '恢复中…'
                              : '恢复展示'}
                          </Button>
                        </div>
                      )
                    })}
                  </div>
                </div>
              </>
            )}

            {!hasAny && (
              <p className='text-muted-foreground mt-2 text-xs'>
                {t('Subscribe to a plan for model access')}
              </p>
            )}
          </div>

          {/* Available plans grid */}
          {plans.length > 0 ? (
            <div className='subscription-card-grid'>
              {plans.map((p, index) => {
                const plan = p?.plan
                if (!plan) return null
                const totalAmount = Number(plan.total_amount || 0)
                const price = Number(plan.price_amount || 0).toFixed(2)
                const isPopular =
                  Boolean(plan.recommended) || (index === 0 && plans.length > 1)
                const limit = Number(plan.max_purchase_per_user || 0)
                const count = planPurchaseCountMap.get(plan.id) || 0
                const reached = limit > 0 && count >= limit

                // 限额标签按重置周期动态: daily→日限额/weekly→周限额/monthly→月限额
                const limitLabel =
                  plan.quota_reset_period === 'daily'
                    ? t('Daily Limit')
                    : plan.quota_reset_period === 'weekly'
                      ? t('Weekly Limit')
                      : plan.quota_reset_period === 'monthly'
                        ? t('Monthly Limit')
                        : t('Total Quota')
                // 三要素（时长/额度）抽到价格区突出展示，对照商务样图「价格/周期/额度」前置
                const validityLabel = formatDuration(plan, t)
                const quotaLabel =
                  totalAmount > 0 ? formatQuota(totalAmount) : t('Unlimited')
                const remainingBenefits = [
                  Number(plan.lucky_card_grant_count || 0) > 0
                    ? {
                        label: '立即获得幸运卡',
                        value: `${Number(plan.lucky_card_grant_count)} 张`,
                      }
                    : null,
                  plan.lucky_card_on_reset &&
                  plan.quota_reset_period !== 'never'
                    ? {
                        label: '周期幸运卡',
                        value: '每次重置获得 1 张',
                      }
                    : null,
                  formatResetPeriod(plan, t) !== t('No Reset')
                    ? {
                        label: t('Quota Reset'),
                        value: formatResetPeriod(plan, t),
                      }
                    : null,
                  plan.number_pool
                    ? { label: t('Number Pool'), value: plan.number_pool }
                    : null,
                  plan.model_limit
                    ? { label: t('Model Limit'), value: plan.model_limit }
                    : null,
                  plan.plan_version
                    ? {
                        label: t('Plan Version'),
                        value: t(versionLabelOf(plan.plan_version)),
                      }
                    : null,
                  plan.min_ratio
                    ? { label: t('Min Ratio'), value: `×${plan.min_ratio}` }
                    : null,
                  plan.allowed_group
                    ? { label: t('Allowed Group'), value: plan.allowed_group }
                    : null,
                  limit > 0
                    ? { label: t('Purchase Limit'), value: String(limit) }
                    : null,
                  plan.upgrade_group
                    ? { label: t('Upgrade Group'), value: plan.upgrade_group }
                    : null,
                ].filter(Boolean) as { label: string; value: string }[]

                const planVersionStyle =
                  plan.plan_version && PLAN_VERSION_STYLES[plan.plan_version]
                    ? PLAN_VERSION_STYLES[plan.plan_version]
                    : null
                const wrapperClass = planVersionStyle
                  ? planVersionStyle.wrapper
                  : isPopular
                    ? 'border border-primary/40 bg-card'
                    : 'border border-border bg-card'
                const innerClass = planVersionStyle
                  ? cn(
                      planVersionStyle.inner,
                      planVersionStyle.ring,
                      'border bg-card/85 backdrop-blur-sm'
                    )
                  : isPopular
                    ? 'border-0 bg-transparent ring-1 ring-primary/25'
                    : 'border-0 bg-transparent hover:border-primary/40'
                const specularColor = planVersionStyle
                  ? `${planVersionStyle.accent}cc`
                  : 'rgba(255,255,255,0.5)'

                return (
                  <SpecularCard
                    key={plan.id}
                    specularColor={specularColor}
                    specularRadius={220}
                    specularIntensity={0.85}
                    className={cn(
                      'rounded-2xl border p-px transition-[transform,box-shadow] duration-300 hover:-translate-y-1',
                      wrapperClass
                    )}
                  >
                    <Card
                      data-card-hover='false'
                      className={cn(
                        'relative h-full overflow-hidden rounded-2xl transition-colors duration-300',
                        innerClass
                      )}
                    >
                      <CardContent className='subscription-plan-card-content relative z-[3] flex h-full flex-col'>
                        {planVersionStyle && (
                          <span className='absolute top-3 right-3 z-[4] inline-block'>
                            <span className={planVersionStyle.badge}>
                              {t(versionLabelOf(plan.plan_version))}
                            </span>
                          </span>
                        )}
                        <div className='mb-3 text-center'>
                          <h4 className='truncate text-base font-semibold'>
                            {plan.title || t('Subscription Plans')}
                          </h4>
                          {plan.suitable_for && (
                            <p className='text-muted-foreground mt-1 text-xs'>
                              <span className='text-foreground/70'>
                                {t('Suitable for')}{' '}
                              </span>
                              {plan.suitable_for}
                            </p>
                          )}
                          {plan.subtitle && (
                            <p className='text-muted-foreground mt-1 line-clamp-3 text-xs'>
                              {plan.subtitle}
                            </p>
                          )}
                        </div>

                        {/* 价格 + 周期合体（对照样图：¥X/年） */}
                        <div className='text-center'>
                          <span className='text-foreground text-3xl font-bold tabular-nums'>
                            ${price}
                          </span>
                          <span className='text-muted-foreground text-sm'>
                            {' '}
                            / {validityLabel}
                          </span>
                        </div>

                        {/* 额度：周期额度 + 总额度 同行展示 */}
                        <div className='mt-1 flex flex-wrap items-center justify-center gap-x-2 gap-y-1 text-sm'>
                          <span className='text-foreground font-semibold tabular-nums'>
                            {quotaLabel}
                          </span>
                          <span className='text-muted-foreground text-xs'>
                            {limitLabel}
                          </span>
                          {plan.amount_cap ? (
                            <>
                              <span className='text-muted-foreground/40'>
                                ·
                              </span>
                              <span className='text-foreground font-semibold tabular-nums'>
                                {formatQuota(Number(plan.amount_cap))}
                              </span>
                              <span className='text-muted-foreground text-xs'>
                                {t('Total Cap')}
                              </span>
                            </>
                          ) : null}
                        </div>

                        {/* CTA 蓝色实心，放在权益列表上方（对照样图） */}
                        <div className='mt-4 mb-4'>
                          {reached ? (
                            <Tooltip>
                              <TooltipTrigger render={<div />}>
                                <Button
                                  variant='outline'
                                  className='w-full'
                                  disabled
                                >
                                  {t('Limit Reached')}
                                </Button>
                              </TooltipTrigger>
                              <TooltipContent>
                                {t('Purchase limit reached')} ({count}/{limit})
                              </TooltipContent>
                            </Tooltip>
                          ) : (
                            <GooeyButton
                              className='h-9 w-full rounded-lg bg-black text-sm font-semibold text-white shadow-[0_6px_18px_-6px_rgba(0,0,0,0.6)] transition-colors hover:bg-neutral-900 dark:bg-white dark:text-black dark:hover:bg-neutral-200'
                              onClick={() => {
                                if (
                                  plan.description &&
                                  !localStorage.getItem(
                                    `sub-desc-hide-${plan.id}`
                                  )
                                ) {
                                  setDescPlan(p)
                                } else {
                                  setRenewalPreview(null)
                                  setSelectedPlan(p)
                                  setPurchaseOpen(true)
                                }
                              }}
                            >
                              {t('Subscribe Now')}
                            </GooeyButton>
                          )}
                        </div>

                        {/* 权益列表：次要信息，下半部分，细线分隔 */}
                        <div className='flex-1 border-t pt-4'>
                          <div className='space-y-2'>
                            {remainingBenefits.map((b) => (
                              <div
                                key={b.label}
                                className='flex items-start gap-2 text-xs'
                              >
                                <Check className='text-foreground mt-0.5 h-3.5 w-3.5 shrink-0' />
                                <span className='text-muted-foreground'>
                                  {b.label}:
                                </span>
                                <span className='text-foreground font-medium'>
                                  {b.value}
                                </span>
                              </div>
                            ))}
                          </div>
                        </div>
                      </CardContent>
                    </Card>
                  </SpecularCard>
                )
              })}
            </div>
          ) : (
            <p className='text-muted-foreground py-4 text-center text-sm'>
              {t('No plans available')}
            </p>
          )}
        </TitledCard>
      </div>

      {/* 套餐介绍弹窗: 有 description 时点击订阅先弹, "已阅读"继续/"永不展示"localStorage 记住 */}
      {descPlan && (
        <div
          className='fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4'
          onClick={() => setDescPlan(null)}
        >
          <div
            className='bg-card max-w-lg rounded-2xl border p-6 shadow-lg'
            onClick={(e) => e.stopPropagation()}
          >
            <h3 className='mb-3 text-lg font-bold'>{descPlan.plan.title}</h3>
            <div className='mb-5 max-h-[40vh] overflow-y-auto text-sm'>
              <Markdown>{descPlan.plan.description || ''}</Markdown>
            </div>
            <div className='flex justify-end gap-2'>
              <Button
                variant='ghost'
                size='sm'
                onClick={() => {
                  localStorage.setItem(`sub-desc-hide-${descPlan.plan.id}`, '1')
                  setRenewalPreview(null)
                  setSelectedPlan(descPlan)
                  setPurchaseOpen(true)
                  setDescPlan(null)
                }}
              >
                {t('Never show again')}
              </Button>
              <Button
                size='sm'
                onClick={() => {
                  setRenewalPreview(null)
                  setSelectedPlan(descPlan)
                  setPurchaseOpen(true)
                  setDescPlan(null)
                }}
              >
                {t('Got it')}
              </Button>
            </div>
          </div>
        </div>
      )}

      <SubscriptionPurchaseDialog
        open={purchaseOpen}
        onOpenChange={(open) => {
          setPurchaseOpen(open)
          if (!open) {
            fetchSelfSubscription()
          }
        }}
        plan={selectedPlan}
        renewalPreview={renewalPreview}
        enableStripe={enableStripe}
        enableCreem={enableCreem}
        enableWaffoPancake={enableWaffoPancake}
        enableOnlineTopUp={enableOnlineTopUp}
        epayMethods={epayMethods}
        userQuota={userQuota}
        onPurchaseSuccess={onPurchaseSuccess}
        purchaseLimit={
          selectedPlan?.plan?.max_purchase_per_user
            ? Number(selectedPlan.plan.max_purchase_per_user)
            : undefined
        }
        purchaseCount={
          selectedPlan?.plan?.id
            ? planPurchaseCountMap.get(selectedPlan.plan.id)
            : undefined
        }
      />
      <SubscriptionInstanceManagementDialog
        open={managementOpen}
        onOpenChange={setManagementOpen}
        subscription={selectedSubscription}
        onSaved={fetchSelfSubscription}
      />
      <SubscriptionInstanceRemarkDialog
        open={remarkOpen}
        onOpenChange={setRemarkOpen}
        subscription={selectedSubscription}
        onSaved={fetchSelfSubscription}
      />
      <SubscriptionConsumptionOrderDialog
        open={consumptionOrderOpen}
        onOpenChange={setConsumptionOrderOpen}
        subscriptions={activeSubscriptions}
      />
    </>
  )
}
