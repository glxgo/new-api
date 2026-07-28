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
import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Users } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Skeleton } from '@/components/ui/skeleton'
import {
  sideDrawerContentClassName,
  sideDrawerHeaderClassName,
} from '@/components/drawer-layout'
import { getSubscriptionSubscribers } from '../api'
import { UserSubscriptionsDialog } from './dialogs/user-subscriptions-dialog'

interface SubscribersDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

// 订阅用户列表: 超管查看所有买过套餐的用户, 点击某用户展开其订阅详情
export function SubscribersDialog({
  open,
  onOpenChange,
}: SubscribersDialogProps) {
  const { t } = useTranslation()
  const [selectedUserId, setSelectedUserId] = useState<number | null>(null)

  const { data, isLoading } = useQuery({
    queryKey: ['subscription-subscribers'],
    queryFn: () => getSubscriptionSubscribers(),
    enabled: open,
  })

  const subscribers = data?.data ?? []

  return (
    <>
      <Sheet open={open} onOpenChange={onOpenChange}>
        <SheetContent className={sideDrawerContentClassName('sm:max-w-2xl')}>
          <SheetHeader className={sideDrawerHeaderClassName()}>
            <SheetTitle className='flex items-center gap-2'>
              <Users className='size-4' />
              {t('Subscription Subscribers')}
            </SheetTitle>
            <SheetDescription>
              {t('Users who have purchased subscription plans')}
            </SheetDescription>
          </SheetHeader>
          <div className='space-y-2 overflow-y-auto p-4'>
            {isLoading ? (
              Array.from({ length: 5 }).map((_, i) => (
                <Skeleton key={i} className='h-12 w-full' />
              ))
            ) : subscribers.length === 0 ? (
              <p className='text-muted-foreground py-8 text-center text-sm'>
                {t('No subscribers')}
              </p>
            ) : (
              subscribers.map((s) => (
                <button
                  key={s.user_id}
                  type='button'
                  onClick={() => setSelectedUserId(s.user_id)}
                  className='hover:bg-muted/50 flex w-full items-center justify-between rounded-lg border p-3 text-left transition-colors'
                >
                  <span className='font-medium'>{s.username}</span>
                  <span className='text-muted-foreground flex items-center gap-3 text-xs'>
                    <span>
                      {t('Total')}: <strong>{s.total_count}</strong>
                    </span>
                    <span>
                      {t('Active')}:{' '}
                      <strong className='text-emerald-600 dark:text-emerald-400'>
                        {s.active_count}
                      </strong>
                    </span>
                  </span>
                </button>
              ))
            )}
          </div>
        </SheetContent>
      </Sheet>

      {selectedUserId !== null && (
        <UserSubscriptionsDialog
          user={{ id: selectedUserId } as never}
          open={selectedUserId !== null}
          onOpenChange={(v) => {
            if (!v) setSelectedUserId(null)
          }}
        />
      )}
    </>
  )
}
