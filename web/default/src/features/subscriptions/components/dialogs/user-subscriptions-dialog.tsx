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
import { useCallback, useEffect, useMemo, useState } from 'react'
import { CalendarClock, Eye, EyeOff, Plus, RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatQuota } from '@/lib/format'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,
} from '@/components/ui/sheet'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { StaticDataTable } from '@/components/data-table'
import { DateTimePicker } from '@/components/datetime-picker'
import {
  sideDrawerContentClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
} from '@/components/drawer-layout'
import { StatusBadge } from '@/components/status-badge'
import { TableId } from '@/components/table-id'
import {
  getAdminPlans,
  getUserSubscriptions,
  createUserSubscription,
  renewUserSubscription,
  invalidateUserSubscription,
  deleteUserSubscription,
  setUserSubscriptionHidden,
} from '../../api'
import { formatTimestamp } from '../../lib'
import type { PlanRecord, UserSubscriptionRecord } from '../../types'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  user: { id: number; username?: string } | null
  onSuccess?: () => void
}

function SubscriptionStatusBadge(props: {
  sub: UserSubscriptionRecord['subscription']
  t: (key: string) => string
}) {
  // eslint-disable-next-line react-hooks/purity
  const now = Date.now() / 1000
  const isScheduled =
    props.sub.status === 'active' && (props.sub.start_time || 0) > now
  const isExpired = (props.sub.end_time || 0) > 0 && props.sub.end_time < now
  const isActive = props.sub.status === 'active' && !isExpired
  if (isScheduled)
    return (
      <StatusBadge
        label={props.t('Scheduled')}
        variant='warning'
        copyable={false}
      />
    )
  if (isActive)
    return (
      <StatusBadge
        label={props.t('Active')}
        variant='success'
        copyable={false}
      />
    )
  if (props.sub.status === 'cancelled')
    return (
      <StatusBadge
        label={props.t('Invalidated')}
        variant='neutral'
        copyable={false}
      />
    )
  return (
    <StatusBadge
      label={props.t('Expired')}
      variant='neutral'
      copyable={false}
    />
  )
}

export function UserSubscriptionsDialog(props: Props) {
  const { t } = useTranslation()
  const userId = props.user?.id
  const [loading, setLoading] = useState(false)
  const [creating, setCreating] = useState(false)
  const [actionSubmitting, setActionSubmitting] = useState(false)
  const [visibilitySubmittingId, setVisibilitySubmittingId] = useState<
    number | null
  >(null)
  const [plans, setPlans] = useState<PlanRecord[]>([])
  const [subs, setSubs] = useState<UserSubscriptionRecord[]>([])
  const [selectedPlanId, setSelectedPlanId] = useState<string>('')
  const [startTime, setStartTime] = useState<Date | undefined>()
  const [confirmAction, setConfirmAction] = useState<{
    type: 'renew' | 'invalidate' | 'delete'
    subId: number
  } | null>(null)

  const renewedSourceIds = useMemo(
    () =>
      new Set(
        subs
          .map((record) => record.subscription.renewed_from_id)
          .filter((id): id is number => typeof id === 'number' && id > 0)
      ),
    [subs]
  )

  const planTitleMap = useMemo(() => {
    const map = new Map<number, string>()
    plans.forEach((p) => {
      if (p.plan.id) map.set(p.plan.id, p.plan.title || `#${p.plan.id}`)
    })
    return map
  }, [plans])

  const loadData = useCallback(async () => {
    if (!userId) return
    setLoading(true)
    try {
      const [plansRes, subsRes] = await Promise.all([
        getAdminPlans(),
        getUserSubscriptions(userId),
      ])
      if (plansRes.success) setPlans(plansRes.data || [])
      if (subsRes.success) setSubs(subsRes.data || [])
    } catch {
      toast.error(t('Loading failed'))
    } finally {
      setLoading(false)
    }
  }, [userId, t])

  useEffect(() => {
    let active = true
    if (props.open && userId) {
      void Promise.resolve().then(() => {
        if (!active) return
        setSelectedPlanId('')
        setStartTime(undefined)
        void loadData()
      })
    }
    return () => {
      active = false
    }
  }, [props.open, userId, loadData])

  const handleCreate = async () => {
    if (!userId || !selectedPlanId) {
      toast.error(t('Please select a subscription plan'))
      return
    }
    setCreating(true)
    try {
      const res = await createUserSubscription(userId, {
        plan_id: Number(selectedPlanId),
        start_time: startTime
          ? Math.floor(startTime.getTime() / 1000)
          : undefined,
      })
      if (res.success) {
        toast.success(res.data?.message || t('Added successfully'))
        setSelectedPlanId('')
        setStartTime(undefined)
        await loadData()
        props.onSuccess?.()
      }
    } catch {
      toast.error(t('Request failed'))
    } finally {
      setCreating(false)
    }
  }

  const handleConfirmAction = async () => {
    if (!confirmAction) return
    setActionSubmitting(true)
    try {
      if (confirmAction.type === 'renew') {
        const res = await renewUserSubscription(confirmAction.subId)
        if (res.success) {
          toast.success(t('Renewal created successfully'))
          await loadData()
          props.onSuccess?.()
        }
      } else if (confirmAction.type === 'invalidate') {
        const res = await invalidateUserSubscription(confirmAction.subId)
        if (res.success) {
          toast.success(res.data?.message || t('Has been invalidated'))
          await loadData()
          props.onSuccess?.()
        }
      } else {
        const res = await deleteUserSubscription(confirmAction.subId)
        if (res.success) {
          toast.success(t('Deleted'))
          await loadData()
          props.onSuccess?.()
        }
      }
    } catch {
      toast.error(t('Operation failed'))
    } finally {
      setActionSubmitting(false)
      setConfirmAction(null)
    }
  }

  const toggleVisibility = async (
    sub: UserSubscriptionRecord['subscription']
  ) => {
    if (visibilitySubmittingId !== null) return
    setVisibilitySubmittingId(sub.id)
    try {
      const res = await setUserSubscriptionHidden(sub.id, !sub.hidden)
      if (res.success) {
        toast.success(sub.hidden ? '已恢复用户端展示' : '已从用户端隐藏')
        await loadData()
        props.onSuccess?.()
      }
    } catch {
      toast.error(t('Operation failed'))
    } finally {
      setVisibilitySubmittingId(null)
    }
  }

  return (
    <>
      <Sheet open={props.open} onOpenChange={props.onOpenChange}>
        <SheetContent className={sideDrawerContentClassName('sm:max-w-2xl')}>
          <SheetHeader className={sideDrawerHeaderClassName()}>
            <SheetTitle>{t('User Subscription Management')}</SheetTitle>
            <SheetDescription>
              {props.user?.username || '-'} (ID: {props.user?.id || '-'})
            </SheetDescription>
          </SheetHeader>

          <div className={sideDrawerFormClassName()}>
            <div className='border-border/70 bg-muted/25 space-y-3 rounded-xl border p-3'>
              <div className='flex items-start gap-2'>
                <div className='bg-background border-border flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border'>
                  <CalendarClock className='text-muted-foreground h-4 w-4' />
                </div>
                <div>
                  <div className='text-sm font-medium'>
                    {t('Add subscription')}
                  </div>
                  <p className='text-muted-foreground text-xs'>
                    {t(
                      'Leave the start time empty to activate immediately. A future subscription is not usable before that time.'
                    )}
                  </p>
                </div>
              </div>
              <div className='grid gap-2 sm:grid-cols-[minmax(0,1fr)_minmax(15rem,1fr)_auto]'>
                <Select
                  items={[
                    ...plans.map((p) => ({
                      value: String(p.plan.id),
                      label: (
                        <>
                          {p.plan.title}($
                          {Number(p.plan.price_amount || 0).toFixed(2)})
                        </>
                      ),
                    })),
                  ]}
                  value={selectedPlanId}
                  onValueChange={(v) => v !== null && setSelectedPlanId(v)}
                >
                  <SelectTrigger className='w-full'>
                    <SelectValue placeholder={t('Select subscription plan')} />
                  </SelectTrigger>
                  <SelectContent alignItemWithTrigger={false}>
                    <SelectGroup>
                      {plans.map((p) => (
                        <SelectItem key={p.plan.id} value={String(p.plan.id)}>
                          {p.plan.title} ($
                          {Number(p.plan.price_amount || 0).toFixed(2)})
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
                <DateTimePicker
                  value={startTime}
                  onChange={setStartTime}
                  placeholder={t('Start immediately')}
                  className='min-w-0'
                />
                <Button
                  onClick={handleCreate}
                  disabled={creating || !selectedPlanId}
                >
                  <Plus className='mr-1 h-4 w-4' />
                  {t('Add')}
                </Button>
              </div>
            </div>

            <StaticDataTable
              data={loading ? [] : subs}
              getRowKey={(record) => record.subscription.id}
              emptyClassName={loading ? 'py-8' : 'text-muted-foreground py-8'}
              emptyContent={
                loading ? t('Loading...') : t('No subscription records')
              }
              columns={[
                {
                  id: 'id',
                  header: t('ID'),
                  cell: (record) => <TableId value={record.subscription.id} />,
                },
                {
                  id: 'plan',
                  header: t('Plan'),
                  cell: (record) => {
                    const sub = record.subscription

                    return (
                      <div>
                        <div className='font-medium'>
                          {sub.plan_title ||
                            planTitleMap.get(sub.plan_id) ||
                            `#${sub.plan_id}`}
                        </div>
                        <div className='text-muted-foreground text-sm'>
                          {t('Source')}: {sub.source || '-'}
                        </div>
                        {sub.hidden && (
                          <div className='mt-1 text-xs text-amber-600'>
                            用户端已隐藏
                          </div>
                        )}
                      </div>
                    )
                  },
                },
                {
                  id: 'status',
                  header: t('Status'),
                  cell: (record) => (
                    <SubscriptionStatusBadge sub={record.subscription} t={t} />
                  ),
                },
                {
                  id: 'validity',
                  header: t('Validity'),
                  cell: (record) => {
                    const sub = record.subscription

                    return (
                      <div className='text-sm'>
                        <div>
                          {t('Start')}: {formatTimestamp(sub.start_time)}
                        </div>
                        <div>
                          {t('End')}: {formatTimestamp(sub.end_time)}
                        </div>
                      </div>
                    )
                  },
                },
                {
                  id: 'quota',
                  header: t('Usage'),
                  cell: (record) => {
                    const sub = record.subscription
                    const total = Number(sub.amount_total || 0)
                    const used = Number(sub.amount_used || 0)
                    if (total <= 0) {
                      return (
                        <span className='text-muted-foreground text-xs'>
                          {t('Unlimited')}
                        </span>
                      )
                    }
                    const remain = total - used
                    const percent = Math.round((used / total) * 100)
                    const barColor =
                      percent >= 90
                        ? 'bg-rose-500'
                        : percent >= 70
                          ? 'bg-amber-500'
                          : 'bg-emerald-500'
                    return (
                      <div className='w-full min-w-[120px]'>
                        <div className='mb-1 flex items-baseline justify-between'>
                          <span className='text-xs font-semibold'>
                            {formatQuota(remain)}
                          </span>
                          <span className='text-muted-foreground text-[10px]'>
                            {percent}% {t('Used')}
                          </span>
                        </div>
                        <div className='bg-muted h-2 w-full overflow-hidden rounded-full'>
                          <div
                            className={`h-full rounded-full transition-all ${barColor}`}
                            style={{ width: `${percent}%` }}
                          />
                        </div>
                        <div className='text-muted-foreground mt-1 flex justify-between text-[10px]'>
                          <span>
                            {formatQuota(used)} {t('Used')}
                          </span>
                          <span>
                            {formatQuota(total)} {t('Total')}
                          </span>
                        </div>
                      </div>
                    )
                  },
                },
                {
                  id: 'actions',
                  header: t('Actions'),
                  className: 'text-right',
                  cellClassName: 'text-right',
                  cell: (record) => {
                    const sub = record.subscription
                    const now = Date.now() / 1000
                    const isExpired =
                      (sub.end_time || 0) > 0 && sub.end_time < now
                    const isActive = sub.status === 'active' && !isExpired
                    const hasSuccessor = renewedSourceIds.has(sub.id)

                    return (
                      <div className='flex flex-wrap justify-end gap-1'>
                        <Button
                          size='sm'
                          variant='outline'
                          disabled={visibilitySubmittingId !== null}
                          onClick={() => void toggleVisibility(sub)}
                        >
                          {sub.hidden ? (
                            <Eye className='mr-1 h-3.5 w-3.5' />
                          ) : (
                            <EyeOff className='mr-1 h-3.5 w-3.5' />
                          )}
                          {sub.hidden ? '恢复展示' : '隐藏'}
                        </Button>
                        <Button
                          size='sm'
                          variant='outline'
                          disabled={hasSuccessor}
                          onClick={() =>
                            setConfirmAction({
                              type: 'renew',
                              subId: sub.id,
                            })
                          }
                        >
                          <RefreshCw className='mr-1 h-3.5 w-3.5' />
                          {t('Renew')}
                        </Button>
                        <Button
                          size='sm'
                          variant='outline'
                          disabled={!isActive}
                          onClick={() =>
                            setConfirmAction({
                              type: 'invalidate',
                              subId: sub.id,
                            })
                          }
                        >
                          {t('Invalidate')}
                        </Button>
                        <Button
                          size='sm'
                          variant='destructive'
                          onClick={() =>
                            setConfirmAction({
                              type: 'delete',
                              subId: sub.id,
                            })
                          }
                        >
                          {t('Delete')}
                        </Button>
                      </div>
                    )
                  },
                },
              ]}
            />
          </div>
        </SheetContent>
      </Sheet>

      {confirmAction && (
        <ConfirmDialog
          open
          onOpenChange={(v) => !v && setConfirmAction(null)}
          title={
            confirmAction.type === 'renew'
              ? t('Confirm Subscription Renewal')
              : confirmAction.type === 'invalidate'
                ? t('Confirm invalidate')
                : t('Confirm delete')
          }
          desc={
            confirmAction.type === 'renew'
              ? t(
                  'The administrator will create a linked successor without charging the user. It starts when the current subscription ends, or immediately if already expired, and bound API Keys will follow the existing renewal policy. Continue?'
                )
              : confirmAction.type === 'invalidate'
                ? t(
                    'After invalidating, this subscription will be immediately deactivated. Every API Key currently bound to it is affected: its next request will follow that Key’s configured continuation policy, or fail if no continuation is allowed. Historical records and scheduled renewal relations remain visible. Continue?'
                  )
                : t(
                    'Deleting permanently removes this instance. The system will block deletion when API Key bindings, billing records, audit history, or renewal relations exist; use Invalidate instead in that case. Continue?'
                  )
          }
          handleConfirm={handleConfirmAction}
          destructive={confirmAction.type === 'delete'}
          isLoading={actionSubmitting}
        />
      )}
    </>
  )
}
