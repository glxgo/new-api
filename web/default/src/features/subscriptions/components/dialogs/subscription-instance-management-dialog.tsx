/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { useEffect, useMemo, useState } from 'react'
import { EyeOff, Gauge, KeyRound, Link2, Unlink2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import {
  formatQuota,
  parseQuotaFromDollars,
  quotaUnitsToDollars,
} from '@/lib/format'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { Dialog } from '@/components/dialog'
import {
  getSubscriptionTokenBindings,
  replaceSubscriptionTokenBindings,
  setSelfSubscriptionHidden,
  updateSubscriptionSpendLimit,
} from '../../api'
import type {
  SubscriptionTokenBindingItem,
  UserSubscription,
} from '../../types'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  subscription: UserSubscription | null
  onSaved?: () => void | Promise<void>
}

export function SubscriptionInstanceManagementDialog({
  open,
  onOpenChange,
  subscription,
  onSaved,
}: Props) {
  const { t } = useTranslation()
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [reviewing, setReviewing] = useState(false)
  const [items, setItems] = useState<SubscriptionTokenBindingItem[]>([])
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set())
  const [keepPlannedIds, setKeepPlannedIds] = useState<Set<number>>(new Set())
  const [allowRenewal, setAllowRenewal] = useState(true)
  const [allowSameGroup, setAllowSameGroup] = useState(false)
  const [allowWallet, setAllowWallet] = useState(false)
  const [walletLimit, setWalletLimit] = useState(0)
  const [hiding, setHiding] = useState(false)
  const [hideConfirmOpen, setHideConfirmOpen] = useState(false)
  const [spendLimitEnabled, setSpendLimitEnabled] = useState(false)
  const [spendLimitPeriod, setSpendLimitPeriod] = useState<'hour' | 'day'>(
    'day'
  )
  const [spendLimitAmount, setSpendLimitAmount] = useState(0)
  const [savingSpendLimit, setSavingSpendLimit] = useState(false)

  useEffect(() => {
    if (!open || !subscription?.id) return
    let active = true
    void Promise.resolve().then(async () => {
      if (!active) return
      setLoading(true)
      setReviewing(false)
      setKeepPlannedIds(new Set())
      setSpendLimitEnabled(!!subscription.spend_limit_quota)
      setSpendLimitPeriod(
        subscription.spend_limit_period === 'hour' ? 'hour' : 'day'
      )
      setSpendLimitAmount(
        quotaUnitsToDollars(subscription.spend_limit_quota || 0)
      )
      try {
        const res = await getSubscriptionTokenBindings(subscription.id)
        if (!active) return
        if (!res.success) {
          toast.error(res.message || t('Request failed'))
          return
        }
        const nextItems = res.data || []
        const bound = nextItems.filter(
          (item) =>
            item.subscription_mode === 'instance' &&
            item.subscription_id === subscription.id
        )
        const first = bound[0]
        setItems(nextItems)
        setSelectedIds(new Set(bound.map((item) => item.id)))
        setAllowRenewal(first?.subscription_allow_renewal ?? true)
        setAllowSameGroup(first?.subscription_allow_same_group ?? false)
        setAllowWallet(first?.subscription_allow_wallet ?? false)
        setWalletLimit(
          quotaUnitsToDollars(first?.subscription_wallet_limit || 0)
        )
      } catch {
        toast.error(t('Request failed'))
      } finally {
        if (active) setLoading(false)
      }
    })
    return () => {
      active = false
    }
  }, [
    open,
    subscription?.id,
    subscription?.spend_limit_period,
    subscription?.spend_limit_quota,
    t,
  ])

  const originallyBound = useMemo(
    () =>
      new Set(
        items
          .filter(
            (item) =>
              item.subscription_mode === 'instance' &&
              item.subscription_id === subscription?.id
          )
          .map((item) => item.id)
      ),
    [items, subscription?.id]
  )
  const newlyBound = items.filter(
    (item) => selectedIds.has(item.id) && !originallyBound.has(item.id)
  )
  const newlyUnbound = items.filter(
    (item) => !selectedIds.has(item.id) && originallyBound.has(item.id)
  )

  const toggleSelected = (item: SubscriptionTokenBindingItem) => {
    if (!item.compatible && !selectedIds.has(item.id)) return
    setReviewing(false)
    setSelectedIds((current) => {
      const next = new Set(current)
      if (next.has(item.id)) {
        next.delete(item.id)
      } else {
        next.add(item.id)
      }
      return next
    })
  }

  const submit = async () => {
    if (!subscription?.id) return
    if (allowWallet && walletLimit <= 0) {
      toast.error(t('Wallet fallback requires a positive limit'))
      return
    }
    if (!reviewing) {
      setReviewing(true)
      return
    }
    setSaving(true)
    try {
      const res = await replaceSubscriptionTokenBindings(subscription.id, {
        token_ids: Array.from(selectedIds),
        subscription_allow_renewal: allowRenewal,
        subscription_allow_same_group: allowSameGroup,
        subscription_allow_wallet: allowWallet,
        subscription_wallet_limit: allowWallet
          ? parseQuotaFromDollars(walletLimit)
          : 0,
        keep_planned_token_ids: Array.from(keepPlannedIds),
      })
      if (!res.success) {
        toast.error(res.message || t('Update failed'))
        return
      }
      toast.success(t('API Key bindings updated'))
      await onSaved?.()
      onOpenChange(false)
    } catch {
      toast.error(t('Request failed'))
    } finally {
      setSaving(false)
    }
  }

  const hideSubscription = async () => {
    if (!subscription?.id) return
    setHiding(true)
    try {
      const res = await setSelfSubscriptionHidden(subscription.id, true)
      if (!res.success) {
        toast.error(res.message || t('Update failed'))
        return
      }
      toast.success(t('Subscription hidden; administrators can restore it'))
      await onSaved?.()
      setHideConfirmOpen(false)
      onOpenChange(false)
    } catch {
      toast.error(t('Request failed'))
    } finally {
      setHiding(false)
    }
  }

  const persistSpendLimit = async (
    enabled: boolean,
    period: 'hour' | 'day',
    amount: number
  ) => {
    if (!subscription?.id) return
    if (enabled && amount <= 0) {
      toast.error(t('Spending limit must be greater than 0'))
      return
    }
    setSavingSpendLimit(true)
    try {
      const res = await updateSubscriptionSpendLimit(subscription.id, {
        period: enabled ? period : '',
        quota: enabled ? parseQuotaFromDollars(amount) : 0,
      })
      if (!res.success) {
        toast.error(res.message || t('Update failed'))
        return
      }
      setSpendLimitEnabled(enabled)
      if (!enabled) setSpendLimitAmount(0)
      toast.success(t('Spending limit updated'))
      await onSaved?.()
    } catch {
      toast.error(t('Request failed'))
    } finally {
      setSavingSpendLimit(false)
    }
  }

  const saveSpendLimit = () =>
    persistSpendLimit(spendLimitEnabled, spendLimitPeriod, spendLimitAmount)

  const clearSpendLimit = () => persistSpendLimit(false, spendLimitPeriod, 0)

  return (
    <>
      <Dialog
        open={open}
        onOpenChange={onOpenChange}
        title={
          <span className='flex items-center gap-2'>
            <KeyRound className='h-5 w-5' />
            {t('Manage Subscription')}
          </span>
        }
        description={`${subscription?.plan_title || t('Subscription')} #${subscription?.id || ''}`}
        contentClassName='sm:max-w-3xl'
        contentHeight='min(68vh, 680px)'
        footer={
          <>
            <Button
              variant='outline'
              onClick={() => onOpenChange(false)}
              disabled={saving}
            >
              {t('Cancel')}
            </Button>
            <Button onClick={submit} disabled={loading || saving}>
              {reviewing ? t('Confirm changes') : t('Review changes')}
            </Button>
          </>
        }
      >
        <div className='space-y-4'>
          <Alert>
            <AlertDescription>
              {t(
                'A Key can formally belong to only one subscription instance. Selecting a Key already bound elsewhere will move it here; deselecting a bound Key returns it to automatic allocation.'
              )}
            </AlertDescription>
          </Alert>

          <div className='border-primary/20 bg-primary/5 space-y-3 rounded-lg border p-3'>
            <div className='flex items-start justify-between gap-3'>
              <div className='min-w-0'>
                <div className='flex items-center gap-2 text-sm font-medium'>
                  <Gauge className='size-4' />
                  {t('Periodic spending limit')}
                </div>
                <p className='text-muted-foreground mt-1 text-xs'>
                  {t(
                    "Limit this subscription's consumption in each selected time window. It does not reduce the remaining package quota; time is based on Beijing time."
                  )}
                </p>
              </div>
              <Switch
                checked={spendLimitEnabled}
                onCheckedChange={(checked) => setSpendLimitEnabled(checked)}
                disabled={savingSpendLimit}
              />
            </div>

            {spendLimitEnabled && (
              <>
                <div className='grid gap-3 sm:grid-cols-[150px_1fr]'>
                  <div className='border-input flex rounded-lg border p-0.5'>
                    <Button
                      type='button'
                      size='sm'
                      variant={
                        spendLimitPeriod === 'hour' ? 'default' : 'ghost'
                      }
                      className='flex-1'
                      onClick={() => setSpendLimitPeriod('hour')}
                      disabled={savingSpendLimit}
                    >
                      {t('Hourly')}
                    </Button>
                    <Button
                      type='button'
                      size='sm'
                      variant={spendLimitPeriod === 'day' ? 'default' : 'ghost'}
                      className='flex-1'
                      onClick={() => setSpendLimitPeriod('day')}
                      disabled={savingSpendLimit}
                    >
                      {t('Daily')}
                    </Button>
                  </div>
                  <Input
                    type='number'
                    min={0.01}
                    step={0.01}
                    value={spendLimitAmount || ''}
                    onChange={(event) =>
                      setSpendLimitAmount(Number(event.target.value) || 0)
                    }
                    placeholder={t('Maximum spend')}
                    disabled={savingSpendLimit}
                  />
                </div>
                <div className='text-muted-foreground text-xs'>
                  {t('Used this period')}:{' '}
                  {formatQuota(subscription?.spend_limit_used || 0)}
                </div>
              </>
            )}

            <div className='flex justify-end gap-2'>
              {!!subscription?.spend_limit_period && (
                <Button
                  type='button'
                  variant='ghost'
                  size='sm'
                  onClick={clearSpendLimit}
                  disabled={savingSpendLimit}
                >
                  {t('Cancel limit')}
                </Button>
              )}
              <Button
                type='button'
                variant='outline'
                size='sm'
                onClick={saveSpendLimit}
                disabled={savingSpendLimit}
              >
                {savingSpendLimit ? t('Saving...') : t('Save limit')}
              </Button>
            </div>
          </div>

          <div className='space-y-2'>
            <div className='flex items-center justify-between'>
              <div>
                <div className='text-sm font-medium'>
                  {t('Choose API Keys')} ({selectedIds.size})
                </div>
                <div className='text-muted-foreground text-xs'>
                  {t(
                    'Disabled items use a group incompatible with this instance.'
                  )}
                </div>
              </div>
            </div>
            <div className='max-h-64 space-y-2 overflow-y-auto rounded-lg border p-2'>
              {loading ? (
                <div className='text-muted-foreground p-6 text-center text-sm'>
                  {t('Loading...')}
                </div>
              ) : items.length === 0 ? (
                <div className='text-muted-foreground p-6 text-center text-sm'>
                  {t('No API Keys')}
                </div>
              ) : (
                items.map((item) => {
                  const selected = selectedIds.has(item.id)
                  const moving =
                    selected &&
                    item.subscription_mode === 'instance' &&
                    item.subscription_id !== subscription?.id
                  const unbinding = !selected && originallyBound.has(item.id)
                  return (
                    <div
                      key={item.id}
                      className='bg-card flex items-start gap-3 rounded-md border p-3'
                    >
                      <Checkbox
                        checked={selected}
                        disabled={!item.compatible && !selected}
                        onCheckedChange={() => toggleSelected(item)}
                        aria-label={item.name}
                      />
                      <button
                        type='button'
                        className='min-w-0 flex-1 text-left'
                        onClick={() => toggleSelected(item)}
                        disabled={!item.compatible && !selected}
                      >
                        <div className='flex flex-wrap items-center gap-2'>
                          <span className='truncate text-sm font-medium'>
                            {item.name}
                          </span>
                          <span className='bg-muted rounded px-1.5 py-0.5 text-[10px]'>
                            {item.group}
                          </span>
                          {moving && (
                            <span className='text-warning text-[10px]'>
                              {t('Will move from instance #{{id}}', {
                                id: item.subscription_id,
                              })}
                            </span>
                          )}
                          {unbinding && (
                            <span className='text-warning text-[10px]'>
                              {t('Will return to automatic allocation')}
                            </span>
                          )}
                        </div>
                        {!item.compatible && (
                          <div className='text-destructive mt-1 text-xs'>
                            {item.incompatibility_reason}
                          </div>
                        )}
                      </button>
                      {unbinding && item.planned_subscription_id > 0 && (
                        <label className='flex shrink-0 items-center gap-2 text-xs'>
                          <Checkbox
                            checked={keepPlannedIds.has(item.id)}
                            onCheckedChange={(checked) =>
                              setKeepPlannedIds((current) => {
                                const next = new Set(current)
                                if (checked) next.add(item.id)
                                else next.delete(item.id)
                                return next
                              })
                            }
                          />
                          {t('Keep scheduled renewal')}
                        </label>
                      )}
                    </div>
                  )
                })
              )}
            </div>
          </div>

          <div className='space-y-3 rounded-lg border p-3'>
            <div>
              <div className='text-sm font-medium'>
                {t('Continuation order')}
              </div>
              <div className='text-muted-foreground text-xs'>
                {t(
                  'When this instance is unavailable: renewal successor → same-group instance → wallet cap → stop.'
                )}
              </div>
            </div>
            <label className='flex items-center justify-between gap-3 text-sm'>
              <span>
                {t('Same subscription instance automatic continuation')}
              </span>
              <Switch
                checked={allowRenewal}
                onCheckedChange={(checked) => {
                  setAllowRenewal(checked)
                  setReviewing(false)
                }}
              />
            </label>
            <label className='flex items-center justify-between gap-3 text-sm'>
              <span>{t('Allow another instance in the same group')}</span>
              <Switch
                checked={allowSameGroup}
                onCheckedChange={(checked) => {
                  setAllowSameGroup(checked)
                  setReviewing(false)
                }}
              />
            </label>
            <label className='flex items-center justify-between gap-3 text-sm'>
              <span>{t('Allow limited wallet continuation')}</span>
              <Switch
                checked={allowWallet}
                onCheckedChange={(checked) => {
                  setAllowWallet(checked)
                  setReviewing(false)
                }}
              />
            </label>
            {allowWallet && (
              <div className='space-y-1.5'>
                <label className='text-xs font-medium'>
                  {t('Wallet continuation cap (USD)')}
                </label>
                <Input
                  type='number'
                  min={0.01}
                  step={0.01}
                  value={walletLimit || ''}
                  onChange={(event) => {
                    setWalletLimit(Number(event.target.value) || 0)
                    setReviewing(false)
                  }}
                />
                <p className='text-warning text-xs'>
                  {t(
                    'This cap limits wallet charges for the current binding cycle. The independent API Key quota remains a hard limit.'
                  )}
                </p>
              </div>
            )}
          </div>

          <div className='border-warning/30 bg-warning/5 flex flex-wrap items-center justify-between gap-3 rounded-lg border p-3'>
            <div className='min-w-0'>
              <div className='text-sm font-medium'>
                {t('Manage remaining usage display')}
              </div>
              <div className='text-muted-foreground text-xs'>
                {t(
                  'Only hides this card from your remaining usage page. Quota, bindings and actual usage are unaffected.'
                )}
              </div>
            </div>
            <Button
              type='button'
              variant='outline'
              size='sm'
              className='gap-1.5'
              onClick={() => setHideConfirmOpen(true)}
              disabled={hiding || saving}
            >
              <EyeOff className='size-3.5' />
              {hiding ? t('Processing...') : t('Hide subscription')}
            </Button>
          </div>

          {reviewing && (
            <Alert>
              <AlertDescription className='space-y-1'>
                <div className='flex items-center gap-2 font-medium'>
                  {newlyBound.length > 0 ? (
                    <Link2 className='h-4 w-4' />
                  ) : (
                    <Unlink2 className='h-4 w-4' />
                  )}
                  {t('Please confirm the visible changes before saving')}
                </div>
                <p>
                  {t(
                    '{{bind}} Key(s) will be bound or moved; {{unbind}} Key(s) will return to automatic allocation.',
                    {
                      bind: newlyBound.length,
                      unbind: newlyUnbound.length,
                    }
                  )}
                </p>
                {newlyUnbound.some(
                  (item) =>
                    item.planned_subscription_id > 0 &&
                    !keepPlannedIds.has(item.id)
                ) && (
                  <p className='text-warning'>
                    {t(
                      'Scheduled renewal bindings for unbound Keys will be cancelled unless explicitly kept above.'
                    )}
                  </p>
                )}
              </AlertDescription>
            </Alert>
          )}
        </div>
      </Dialog>
      <ConfirmDialog
        open={hideConfirmOpen}
        onOpenChange={setHideConfirmOpen}
        title={t('Confirm hiding this subscription?')}
        desc={t(
          'This card will be hidden from your remaining usage page. Billing, bindings and quota are unaffected. Administrators can restore it.'
        )}
        confirmText={t('Hide subscription')}
        destructive
        isLoading={hiding}
        handleConfirm={hideSubscription}
      />
    </>
  )
}
