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
import { useTranslation } from 'react-i18next'
import { SectionPageLayout } from '@/components/layout'
import { Markdown } from '@/components/ui/markdown'
import { SubscriptionPlansCard } from '@/features/wallet/components/subscription-plans-card'
import { getSubscriptionIntro } from '@/features/subscriptions/api'
import { useTopupInfo } from '@/features/wallet/hooks'
import { useAuthStore } from '@/stores/auth-store'

// 订阅套餐独立页(从钱包移出), 展示可购买的月卡/周卡套餐 + 我的订阅 + 计费偏好
export function SubscriptionPlans() {
  const { t } = useTranslation()
  const { topupInfo } = useTopupInfo()
  const user = useAuthStore((state) => state.auth.user)
  const [intro, setIntro] = useState('')

  useEffect(() => {
    getSubscriptionIntro()
      .then((res) => {
        if (res.success && res.data?.intro) {
          setIntro(res.data.intro)
        }
      })
      .catch(() => {})
  }, [])

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Subscription Plans')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='flex flex-col gap-4 sm:gap-5'>
          {intro && (
            <div className='bg-gradient-to-br from-primary/5 to-card rounded-xl border p-4 sm:p-5'>
              <Markdown>{intro}</Markdown>
            </div>
          )}
          <SubscriptionPlansCard topupInfo={topupInfo} userQuota={user?.quota} />
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
