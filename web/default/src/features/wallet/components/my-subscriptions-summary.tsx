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
import { useState, useEffect } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { ChevronRight } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatQuota } from '@/lib/format'
import { cn } from '@/lib/utils'
import { Card, CardContent } from '@/components/ui/card'
import { Progress } from '@/components/ui/progress'
import {
  getSelfSubscriptionFull,
  getPublicPlans,
} from '@/features/subscriptions/api'
import type {
  PlanRecord,
  UserSubscriptionRecord,
} from '@/features/subscriptions/types'

// 钱包页"我的订阅"摘要: 仅当有活跃订阅时显示, 每张订阅一行彩色进度条, 点击跳套餐页
export function MySubscriptionsSummary() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [activeSubs, setActiveSubs] = useState<UserSubscriptionRecord[]>([])
  const [planMap, setPlanMap] = useState<Map<number, string>>(new Map())

  useEffect(() => {
    Promise.all([getSelfSubscriptionFull(), getPublicPlans()])
      .then(([selfRes, plansRes]) => {
        if (selfRes.success && selfRes.data?.subscriptions) {
          setActiveSubs(selfRes.data.subscriptions)
        }
        if (plansRes.success && plansRes.data) {
          const m = new Map<number, string>()
          plansRes.data.forEach((p: PlanRecord) => {
            if (p?.plan) {
              m.set(p.plan.id, p.plan.title)
            }
          })
          setPlanMap(m)
        }
      })
      .catch(() => {})
  }, [])

  if (activeSubs.length === 0) {
    return null
  }

  return (
    <Card
      className='hover:bg-accent/40 cursor-pointer gap-0 py-0 transition-colors'
      onClick={() => navigate({ to: '/subscription-plans' })}
    >
      <CardContent className='p-4 sm:p-5'>
        <div className='mb-3 flex items-center justify-between'>
          <div className='flex items-center gap-2'>
            <span className='text-sm font-semibold'>
              {t('My Subscriptions')}
            </span>
            <span className='bg-primary/10 text-primary rounded-full px-2 py-0.5 text-xs font-medium'>
              {activeSubs.length} {t('active')}
            </span>
          </div>
          <ChevronRight className='text-muted-foreground h-4 w-4' />
        </div>
        <div className='space-y-3'>
          {activeSubs.map((sub) => {
            const s = sub.subscription
            if (!s) {
              return null
            }
            const total = Number(s.amount_total || 0)
            const used = Number(s.amount_used || 0)
            const remain = total > 0 ? Math.max(0, total - used) : 0
            const pct = total > 0 ? Math.round((used / total) * 100) : 0
            const title =
              planMap.get(s.plan_id) || `${t('Subscription')} #${s.id}`
            return (
              <div key={s.id}>
                <div className='mb-1 flex items-baseline justify-between gap-2'>
                  <span className='truncate text-xs font-medium'>{title}</span>
                  {total > 0 ? (
                    <span className='text-primary shrink-0 text-xs font-bold'>
                      {formatQuota(remain)} {t('Remaining')}
                    </span>
                  ) : (
                    <span className='text-muted-foreground shrink-0 text-xs'>
                      {t('Unlimited')}
                    </span>
                  )}
                </div>
                {total > 0 && (
                  <Progress
                    value={pct}
                    className={cn(
                      '[&_[data-slot=progress-track]]:h-1.5',
                      pct >= 90
                        ? '[&_[data-slot=progress-indicator]]:bg-rose-500'
                        : pct >= 70
                          ? '[&_[data-slot=progress-indicator]]:bg-amber-500'
                          : '[&_[data-slot=progress-indicator]]:bg-emerald-500'
                    )}
                  />
                )}
              </div>
            )
          })}
        </div>
      </CardContent>
    </Card>
  )
}
