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
import { useCallback, useEffect, useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Crown, HandCoins, TrendingUp } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import type { AuthUser } from '@/stores/auth-store'
import { getSelf } from '@/lib/api'
import dayjs from '@/lib/dayjs'
import { formatQuota } from '@/lib/format'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { SectionPageLayout } from '@/components/layout'
import { getUserWithdraws } from '@/features/withdraw/api'
import { WithdrawRequestSheet } from '@/features/withdraw/components/withdraw-request-sheet'
import {
  WITHDRAW_STATUS,
  WITHDRAW_TYPE,
  type WithdrawStatus,
} from '@/features/withdraw/types'

export function Dividend() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [user, setUser] = useState<AuthUser | null>(null)
  const [loading, setLoading] = useState(true)
  const [sheetOpen, setSheetOpen] = useState(false)

  const fetchSelf = useCallback(async () => {
    setLoading(true)
    try {
      const res = await getSelf()
      if (res.success && res.data) setUser(res.data as AuthUser)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchSelf()
  }, [fetchSelf])

  const { data: withdraws } = useQuery({
    queryKey: ['my-withdraws'],
    queryFn: async () => (await getUserWithdraws(1, 50)).data?.data ?? [],
  })

  const balance = user?.dividend_balance ?? 0
  const total = user?.dividend_total ?? 0

  const refreshAll = () => {
    fetchSelf()
    queryClient.invalidateQueries({ queryKey: ['my-withdraws'] })
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Dividend Account')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='mx-auto flex w-full max-w-5xl flex-col gap-4'>
          <div className='grid gap-4 sm:grid-cols-2'>
            <StatCard
              icon={Crown}
              label={t('Dividend Balance')}
              value={loading ? null : formatQuota(balance)}
              desc={t('Withdrawable balance')}
            />
            <StatCard
              icon={TrendingUp}
              label={t('Total Dividend')}
              value={loading ? null : formatQuota(total)}
              desc={t('Cumulative dividends earned')}
            />
          </div>

          <div className='flex justify-end'>
            <Button
              onClick={() => {
                if (balance <= 0) {
                  toast.error('暂无可提现的分红余额')
                  return
                }
                setSheetOpen(true)
              }}
              disabled={loading}
            >
              <HandCoins className='size-4' /> {t('Withdraw Dividend')}
            </Button>
          </div>

          <div className='overflow-hidden rounded-lg border'>
            <div className='border-b px-4 py-3 text-sm font-semibold'>
              {t('Withdrawal History')}
            </div>
            <div className='divide-y'>
              {(withdraws ?? []).length === 0 ? (
                <div className='text-muted-foreground px-4 py-8 text-center text-sm'>
                  {t('No withdrawal history')}
                </div>
              ) : (
                (withdraws ?? []).map((w) => (
                  <div
                    key={w.id}
                    className='flex items-center justify-between px-4 py-3 text-sm'
                  >
                    <div>
                      <div className='flex flex-wrap items-center gap-2'>
                        <Badge
                          variant={
                            w.type === WITHDRAW_TYPE.DIVIDEND
                              ? 'outline'
                              : 'secondary'
                          }
                          className='text-xs'
                        >
                          {w.type === WITHDRAW_TYPE.DIVIDEND
                            ? t('Dividend')
                            : t('Principal')}
                        </Badge>
                        <span className='font-mono'>
                          {formatQuota(w.amount)}
                        </span>
                        <span className='text-muted-foreground text-xs'>
                          ({t('actual')} {formatQuota(w.actual_amount)})
                        </span>
                      </div>
                      <div className='text-muted-foreground mt-0.5 text-xs'>
                        {dayjs(w.created_at * 1000).format('YYYY-MM-DD HH:mm')}
                      </div>
                    </div>
                    <StatusBadge status={w.status} t={t} />
                  </div>
                ))
              )}
            </div>
          </div>
        </div>
      </SectionPageLayout.Content>

      <WithdrawRequestSheet
        open={sheetOpen}
        onOpenChange={setSheetOpen}
        type={WITHDRAW_TYPE.DIVIDEND}
        maxAmount={balance}
        onSuccess={refreshAll}
      />
    </SectionPageLayout>
  )
}

function StatCard({
  icon: Icon,
  label,
  value,
  desc,
}: {
  icon: typeof Crown
  label: string
  value: string | null
  desc: string
}) {
  return (
    <div className='rounded-lg border p-5'>
      <div className='flex items-center gap-2'>
        <Icon className='text-muted-foreground/60 size-4' />
        <span className='text-muted-foreground text-xs font-medium tracking-wider uppercase'>
          {label}
        </span>
      </div>
      <div className='mt-2 font-mono text-2xl font-bold tabular-nums'>
        {value === null ? <Skeleton className='h-8 w-28' /> : value}
      </div>
      <div className='text-muted-foreground/60 mt-1 text-xs'>{desc}</div>
    </div>
  )
}

function StatusBadge({
  status,
  t,
}: {
  status: WithdrawStatus
  t: (k: string) => string
}) {
  if (status === WITHDRAW_STATUS.PENDING)
    return (
      <Badge variant='outline' className='border-amber-500 text-amber-600'>
        {t('Pending')}
      </Badge>
    )
  if (status === WITHDRAW_STATUS.APPROVED)
    return (
      <Badge className='bg-emerald-600 hover:bg-emerald-600'>
        {t('Approved')}
      </Badge>
    )
  return (
    <Badge variant='secondary' className='text-rose-600'>
      {t('Rejected')}
    </Badge>
  )
}
