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
import { Crown } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatQuota } from '@/lib/format'
import { cn } from '@/lib/utils'
import { TitledCard } from '@/components/ui/titled-card'
import { SpecularCard } from '@/components/reactbits/specular-card'
import '@/components/reactbits/sub-effects.css'
import {
  getPublicPlans,
  getSelfSubscriptionFull,
} from '@/features/subscriptions/api'
import { PLAN_VERSION_STYLES } from '@/features/subscriptions/constants'
import type {
  PlanRecord,
  UserSubscriptionRecord,
} from '@/features/subscriptions/types'

// 数据看板"我的订阅"详细卡片: 从订阅套餐页 SubscriptionPlansCard 抽出, 自拉数据
// (getSelfSubscriptionFull + getPublicPlans), 渲染每张订阅的详细用量 (标题/剩余天数/
// 起止时间/周期额度进度/总额度cap/可用端点/重置时间/能量条). 无任何订阅时不显示.
const quotaBarColor = (pct: number) =>
  pct >= 90 ? 'bg-destructive' : pct >= 70 ? 'bg-warning' : 'bg-success'

export function MySubscriptionsDetail() {
  const { t } = useTranslation()
  const [plans, setPlans] = useState<PlanRecord[]>([])
  const [allSubscriptions, setAllSubscriptions] = useState<
    UserSubscriptionRecord[]
  >([])
  const [subscriptionReferenceTime, setSubscriptionReferenceTime] = useState(0)

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
        setAllSubscriptions(res.data.subscriptions || [])
        setSubscriptionReferenceTime(Date.now() / 1000)
      }
    } catch {
      // ignore
    }
  }, [])

  useEffect(() => {
    const init = async () => {
      await Promise.all([fetchPlans(), fetchSelfSubscription()])
    }
    init()
  }, [fetchPlans, fetchSelfSubscription])

  const planTitleMap = useMemo(() => {
    const map = new Map<number, string>()
    for (const p of plans) {
      if (p?.plan?.id) {
        map.set(p.plan.id, p.plan.title || '')
      }
    }
    return map
  }, [plans])

  // plan_id -> plan_version, 用于卡片边框跟随套餐版本变色(与可购套餐卡片同款)
  const planVersionMap = useMemo(() => {
    const map = new Map<number, string>()
    for (const p of plans) {
      if (p?.plan?.id && p.plan.plan_version) {
        map.set(p.plan.id, p.plan.plan_version)
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

  if (allSubscriptions.length === 0) {
    return null
  }

  return (
    <div className='subscription-responsive-scope'>
      <TitledCard
        title={t('My Subscriptions')}
        icon={<Crown className='h-4 w-4' />}
        disableHoverEffect
        contentClassName='p-3 sm:p-5'
      >
        <div className='subscription-card-grid'>
          {allSubscriptions.map((sub) => {
            const subscription = sub.subscription
            const totalAmount = Number(subscription?.amount_total || 0)
            const usedAmount = Number(subscription?.amount_used || 0)
            const remainAmount =
              totalAmount > 0 ? Math.max(0, totalAmount - usedAmount) : 0
            const capTotal = Number(subscription?.amount_cap || 0)
            const capUsed = Number(subscription?.amount_cap_used || 0)
            const capRemain = capTotal > 0 ? Math.max(0, capTotal - capUsed) : 0
            const capPercent =
              capTotal > 0 ? Math.round((capUsed / capTotal) * 100) : 0
            const allowedGroup = subscription?.allowed_group || ''
            const planTitle =
              subscription?.plan_title ||
              planTitleMap.get(subscription?.plan_id) ||
              ''
            const planVersion =
              subscription?.plan_version ||
              planVersionMap.get(subscription?.plan_id) ||
              ''
            const remainDays = getRemainingDays(sub)
            const usagePercent = getUsagePercent(sub)
            const remainPercent =
              totalAmount > 0 ? Math.max(0, 100 - usagePercent) : 0
            const capRemainPercent =
              capTotal > 0 ? Math.max(0, 100 - capPercent) : 0
            const now = subscriptionReferenceTime
            const isExpired = (subscription?.end_time || 0) < now
            const isCancelled = subscription?.status === 'cancelled'
            const isActive = subscription?.status === 'active' && !isExpired

            const planVersionStyle =
              planVersion && PLAN_VERSION_STYLES[planVersion]
                ? PLAN_VERSION_STYLES[planVersion]
                : null

            return (
              <SpecularCard
                key={subscription?.id}
                specularColor={
                  planVersionStyle
                    ? `${planVersionStyle.accent}bb`
                    : 'rgba(255,255,255,0.45)'
                }
                specularRadius={180}
                specularIntensity={0.7}
                className={cn(
                  'rounded-xl border p-px transition-[transform,box-shadow] duration-300 hover:-translate-y-0.5',
                  planVersionStyle
                    ? cn(planVersionStyle.wrapper, planVersionStyle.ring)
                    : 'border-border bg-card'
                )}
              >
                <div
                  className={cn(
                    'bg-card/90 relative z-[3] flex h-full flex-col overflow-hidden rounded-xl border backdrop-blur-sm transition-colors',
                    planVersionStyle
                      ? planVersionStyle.inner
                      : 'border-border hover:border-primary/40'
                  )}
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
                          {isCancelled ? t('Cancelled') : t('Expired')}
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
                    {isActive && (subscription?.next_reset_time ?? 0) > 0 && (
                      <div className='text-muted-foreground text-[10px]'>
                        {t('Next reset')}:{' '}
                        {new Date(
                          subscription!.next_reset_time! * 1000
                        ).toLocaleString()}
                      </div>
                    )}
                  </div>

                  {/* 底部能量条已移除 */}
                </div>
              </SpecularCard>
            )
          })}
        </div>
      </TitledCard>
    </div>
  )
}
