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
import { useCallback, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { CalendarClock, Check, ChevronDown, Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatQuota } from '@/lib/format'
import { cn } from '@/lib/utils'
import { useIsMobile } from '@/hooks/use-mobile'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Command,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
} from '@/components/ui/command'
import {
  Drawer,
  DrawerContent,
  DrawerHeader,
  DrawerTitle,
  DrawerTrigger,
} from '@/components/ui/drawer'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import { getSelfSubscriptionFull } from '@/features/subscriptions/api'
import type { UserSubscription } from '@/features/subscriptions/types'
import { getApiKey, updateApiKey } from '../api'
import { ERROR_MESSAGES, SUCCESS_MESSAGES } from '../constants'
import {
  transformApiKeyToFormDefaults,
  transformFormDataToPayload,
} from '../lib'
import type { ApiKey } from '../types'
import { useApiKeys } from './api-keys-provider'

type PendingChoice =
  | { mode: 'auto'; subscription: null }
  | { mode: 'instance'; subscription: UserSubscription }

function isUsableCompatibleSubscription(
  subscription: UserSubscription,
  group: string,
  now: number
) {
  const total = Number(subscription.amount_total || 0)
  const used = Number(subscription.amount_used || 0)
  const cap = Number(subscription.amount_cap || 0)
  const capUsed = Number(subscription.amount_cap_used || 0)
  const compatible =
    !subscription.allowed_group || subscription.allowed_group === group
  return (
    compatible &&
    subscription.status === 'active' &&
    subscription.start_time <= now &&
    subscription.end_time > now &&
    (total <= 0 || used < total) &&
    (cap <= 0 || capUsed < cap)
  )
}

function subscriptionDisplayName(
  subscription: UserSubscription | undefined,
  fallbackId: number,
  fallback: string
) {
  if (!subscription) return `${fallback} #${fallbackId}`
  return `${subscription.plan_title || fallback} · #${subscription.id}`
}

export function ApiKeySubscriptionCombobox({ apiKey }: { apiKey: ApiKey }) {
  const { t } = useTranslation()
  const { triggerRefresh } = useApiKeys()
  const isMobile = useIsMobile()
  const [open, setOpen] = useState(false)
  const [searchValue, setSearchValue] = useState('')
  const [saving, setSaving] = useState(false)
  const [pendingChoice, setPendingChoice] = useState<PendingChoice | null>(null)
  const [keepPlanned, setKeepPlanned] = useState(true)

  const selfSubQuery = useQuery({
    queryKey: ['self-subscription-full'],
    queryFn: getSelfSubscriptionFull,
    staleTime: 0,
    enabled:
      apiKey.subscription_mode === 'instance' && apiKey.subscription_id > 0,
  })

  const allSubscriptions = useMemo(
    () =>
      (selfSubQuery.data?.data?.all_subscriptions || [])
        .map((record) => record.subscription)
        .filter((subscription): subscription is UserSubscription =>
          Boolean(subscription)
        ),
    [selfSubQuery.data]
  )

  const currentSubscription = allSubscriptions.find(
    (subscription) => subscription.id === apiKey.subscription_id
  )

  const selectableSubscriptions = useMemo(() => {
    const now = selfSubQuery.dataUpdatedAt / 1000
    const search = searchValue.trim().toLowerCase()
    return allSubscriptions
      .filter(
        (subscription) =>
          subscription.id !== apiKey.subscription_id &&
          isUsableCompatibleSubscription(subscription, apiKey.group || '', now)
      )
      .filter((subscription) => {
        if (!search) return true
        return [
          subscription.plan_title,
          subscription.remark,
          subscription.allowed_group,
          String(subscription.id),
        ].some((value) => value?.toLowerCase().includes(search))
      })
      .sort((a, b) => a.end_time - b.end_time || a.id - b.id)
  }, [
    allSubscriptions,
    apiKey.group,
    apiKey.subscription_id,
    searchValue,
    selfSubQuery.dataUpdatedAt,
  ])

  const currentName = subscriptionDisplayName(
    currentSubscription,
    apiKey.subscription_id,
    t('Subscription instance')
  )

  const continuation = [
    apiKey.subscription_allow_renewal && t('Renewed successor'),
    apiKey.subscription_allow_same_group && t('Same-group instance'),
    apiKey.subscription_allow_wallet && t('Wallet'),
  ].filter(Boolean)

  const closePicker = useCallback(() => {
    setOpen(false)
    setSearchValue('')
  }, [])

  const chooseAutomatic = useCallback(() => {
    closePicker()
    setKeepPlanned(true)
    setPendingChoice({ mode: 'auto', subscription: null })
  }, [closePicker])

  const chooseSubscription = useCallback(
    (subscription: UserSubscription) => {
      closePicker()
      setKeepPlanned(true)
      setPendingChoice({ mode: 'instance', subscription })
    },
    [closePicker]
  )

  const confirmSwitch = useCallback(async () => {
    if (!pendingChoice || saving) return
    setSaving(true)
    try {
      const detail = await getApiKey(apiKey.id)
      if (!detail.success || !detail.data) {
        toast.error(detail.message || t(ERROR_MESSAGES.UPDATE_FAILED))
        return
      }

      const base = transformApiKeyToFormDefaults(detail.data)
      if (pendingChoice.mode === 'auto') {
        base.subscription_mode = 'auto'
        base.subscription_id = 0
      } else {
        base.subscription_mode = 'instance'
        base.subscription_id = pendingChoice.subscription.id
      }
      const payload = transformFormDataToPayload(base)
      payload.cancel_planned_subscription =
        detail.data.planned_subscription_id > 0 && !keepPlanned

      const result = await updateApiKey({ ...payload, id: apiKey.id })
      if (!result.success) {
        toast.error(result.message || t(ERROR_MESSAGES.UPDATE_FAILED))
        return
      }

      toast.success(t(SUCCESS_MESSAGES.API_KEY_UPDATED))
      setPendingChoice(null)
      triggerRefresh()
    } catch {
      toast.error(t(ERROR_MESSAGES.UNEXPECTED))
    } finally {
      setSaving(false)
    }
  }, [apiKey.id, keepPlanned, pendingChoice, saving, t, triggerRefresh])

  if (apiKey.subscription_mode !== 'instance' || apiKey.subscription_id <= 0) {
    return null
  }

  const renderSubscriptionOption = (subscription: UserSubscription) => {
    const total = Number(subscription.amount_total || 0)
    const used = Number(subscription.amount_used || 0)
    const remaining = total > 0 ? Math.max(0, total - used) : 0
    return (
      <CommandItem
        key={subscription.id}
        value={`${subscription.id} ${subscription.plan_title || ''} ${subscription.remark || ''}`}
        disabled={saving}
        onSelect={() => chooseSubscription(subscription)}
        className='data-[selected=true]:bg-muted grid min-h-16 grid-cols-[1rem_minmax(0,1fr)_auto] items-start gap-x-3 rounded-lg px-3 py-3 [&>svg:last-child]:hidden'
      >
        <Check className='mt-0.5 size-4 opacity-0' />
        <span className='min-w-0'>
          <span className='flex flex-wrap items-center gap-1.5'>
            <span className='text-sm leading-5 font-medium'>
              {subscription.plan_title || t('Subscription instance')}
            </span>
            <span className='text-muted-foreground font-mono text-[10px]'>
              #{subscription.id}
            </span>
          </span>
          <span className='text-muted-foreground mt-0.5 block truncate text-xs'>
            {subscription.remark || t('No instance note yet')}
          </span>
        </span>
        <span className='text-right'>
          <span className='block font-mono text-xs font-semibold'>
            {total > 0 ? formatQuota(remaining) : t('Unlimited')}
          </span>
          <span className='text-muted-foreground mt-1 flex items-center justify-end gap-1 text-[10px]'>
            <CalendarClock className='size-3' />
            {new Date(subscription.end_time * 1000).toLocaleDateString()}
          </span>
        </span>
      </CommandItem>
    )
  }

  const pickerContent = (mobile = false) => (
    <Command
      shouldFilter={false}
      className={cn(
        mobile &&
          'min-h-0 flex-1 *:data-[slot=command-input]:text-base *:data-[slot=input-group]:h-11!'
      )}
    >
      <CommandInput
        placeholder={t('Search...')}
        value={searchValue}
        onValueChange={setSearchValue}
      />
      <CommandList className={mobile ? 'max-h-[55dvh] pb-2' : 'max-h-[360px]'}>
        <CommandGroup heading={t('Current instance')}>
          <CommandItem
            value={currentName}
            disabled
            className='grid min-h-14 grid-cols-[1rem_minmax(0,1fr)] items-start gap-x-3 rounded-lg px-3 py-3 [&>svg:last-child]:hidden'
          >
            <Check className='mt-0.5 size-4 opacity-100' />
            <span className='min-w-0'>
              <span className='block truncate text-sm font-medium'>
                {currentName}
              </span>
              <span className='text-muted-foreground mt-0.5 block truncate text-xs'>
                {currentSubscription?.remark || t('No instance note yet')}
              </span>
            </span>
          </CommandItem>
        </CommandGroup>
        <CommandSeparator />
        <CommandGroup heading={t('Available subscription instances')}>
          {selectableSubscriptions.map(renderSubscriptionOption)}
          {selectableSubscriptions.length === 0 && (
            <CommandItem
              value='no-compatible-subscription-instance'
              disabled
              className='text-muted-foreground justify-center py-5 text-xs [&>svg:last-child]:hidden'
            >
              {t('No compatible subscription instance')}
            </CommandItem>
          )}
        </CommandGroup>
        <CommandSeparator />
        <CommandGroup>
          <CommandItem
            value={t('Automatic allocation')}
            disabled={saving}
            onSelect={chooseAutomatic}
            className='min-h-12 rounded-lg px-3 [&>svg:last-child]:hidden'
          >
            <span className='size-4' />
            <span>
              <span className='block text-sm font-medium'>
                {t('Automatic allocation')}
              </span>
              <span className='text-muted-foreground block text-xs'>
                {t(
                  'Keep the current allocation behavior and let the system choose an available funding source.'
                )}
              </span>
            </span>
          </CommandItem>
        </CommandGroup>
      </CommandList>
    </Command>
  )

  const trigger = (
    <>
      <span
        className='max-w-[12rem] truncate'
        title={currentSubscription?.plan_title || t('Subscription instance')}
      >
        {currentSubscription?.plan_title || t('Subscription instance')}
      </span>
      <span className='shrink-0 font-mono text-[10px]'>
        #{apiKey.subscription_id}
      </span>
      <ChevronDown className='size-3 shrink-0 opacity-50' />
      {saving && <Loader2 className='size-3 shrink-0 animate-spin' />}
    </>
  )

  const triggerClassName = cn(
    'inline-flex h-6 max-w-full items-center gap-1 rounded-full border px-2.5 text-xs font-medium transition-colors',
    'border-emerald-200 bg-emerald-50 text-emerald-700 hover:bg-emerald-100',
    'disabled:cursor-wait disabled:opacity-70',
    'dark:border-emerald-900/60 dark:bg-emerald-950/40 dark:text-emerald-300 dark:hover:bg-emerald-950/60'
  )

  return (
    <>
      {isMobile ? (
        <Drawer open={open} onOpenChange={setOpen}>
          <DrawerTrigger asChild>
            <button
              type='button'
              role='combobox'
              aria-label={t('Change subscription instance')}
              aria-expanded={open}
              aria-busy={saving}
              disabled={saving}
              className={triggerClassName}
            >
              {trigger}
            </button>
          </DrawerTrigger>
          <DrawerContent className='max-h-[78dvh]'>
            <DrawerHeader className='pb-2 text-left'>
              <DrawerTitle>{t('Subscription instance')}</DrawerTitle>
            </DrawerHeader>
            <div className='flex min-h-0 flex-1 px-3 pb-[max(1rem,env(safe-area-inset-bottom))]'>
              {pickerContent(true)}
            </div>
          </DrawerContent>
        </Drawer>
      ) : (
        <Popover open={open} onOpenChange={setOpen}>
          <PopoverTrigger
            render={
              <button
                type='button'
                role='combobox'
                aria-label={t('Change subscription instance')}
                aria-expanded={open}
                aria-busy={saving}
                disabled={saving}
                className={triggerClassName}
              />
            }
          >
            {trigger}
          </PopoverTrigger>
          <PopoverContent
            className='data-closed:zoom-out-95 data-open:zoom-in-95 w-[28rem] max-w-[calc(100vw-1.5rem)] origin-(--transform-origin) overflow-hidden rounded-xl p-0 shadow-lg data-closed:duration-100 data-open:duration-150 motion-reduce:duration-0'
            align='end'
            collisionPadding={12}
            onWheel={(event) => event.stopPropagation()}
            onTouchMove={(event) => event.stopPropagation()}
            onPointerDown={(event) => event.stopPropagation()}
          >
            {pickerContent()}
          </PopoverContent>
        </Popover>
      )}

      <AlertDialog
        open={pendingChoice !== null}
        onOpenChange={(nextOpen) => {
          if (!nextOpen && !saving) setPendingChoice(null)
        }}
      >
        <AlertDialogContent className='sm:max-w-lg'>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Review changes')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                'You changed this existing Key’s group, subscription instance, or continuation policy. The new choice will replace the previous one after saving.'
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>

          <div className='grid gap-3 sm:grid-cols-2'>
            <div className='bg-muted/35 rounded-lg border p-3'>
              <p className='text-muted-foreground text-[10px] font-medium tracking-wide uppercase'>
                {t('Configured ownership')}
              </p>
              <p className='mt-1 text-sm font-semibold'>{currentName}</p>
              <p className='text-muted-foreground mt-1 text-xs'>
                {continuation.length > 0
                  ? continuation.join(' → ')
                  : t('Stop after exhaustion')}
              </p>
            </div>
            <div className='border-primary/30 bg-primary/5 rounded-lg border p-3'>
              <p className='text-muted-foreground text-[10px] font-medium tracking-wide uppercase'>
                {t('Review changes')}
              </p>
              <p className='mt-1 text-sm font-semibold'>
                {pendingChoice?.mode === 'instance'
                  ? subscriptionDisplayName(
                      pendingChoice.subscription,
                      pendingChoice.subscription.id,
                      t('Subscription instance')
                    )
                  : t('Automatic allocation')}
              </p>
              <p className='text-muted-foreground mt-1 text-xs'>
                {pendingChoice?.mode === 'instance'
                  ? continuation.length > 0
                    ? continuation.join(' → ')
                    : t('Stop after exhaustion')
                  : t(
                      'Keep the current allocation behavior and let the system choose an available funding source.'
                    )}
              </p>
            </div>
          </div>

          {apiKey.planned_subscription_id > 0 && (
            <label className='bg-warning/5 border-warning/30 flex cursor-pointer items-start gap-3 rounded-lg border p-3'>
              <Checkbox
                checked={keepPlanned}
                onCheckedChange={(checked) => setKeepPlanned(checked === true)}
                disabled={saving}
                className='mt-0.5'
              />
              <span>
                <span className='block text-sm font-medium'>
                  {t('Keep scheduled renewal')}
                </span>
                <span className='text-muted-foreground mt-0.5 block text-xs'>
                  {t('Scheduled successor')} #{apiKey.planned_subscription_id}
                </span>
              </span>
            </label>
          )}

          <AlertDialogFooter>
            <AlertDialogCancel disabled={saving}>
              {t('Cancel')}
            </AlertDialogCancel>
            <AlertDialogAction disabled={saving} onClick={confirmSwitch}>
              {saving && <Loader2 className='size-4 animate-spin' />}
              {t('Confirm changes')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
