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
import { useEffect, useMemo, useRef, useState } from 'react'
import { useForm, type SubmitErrorHandler } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery } from '@tanstack/react-query'
import {
  ArrowRight,
  CalendarClock,
  ChevronDown,
  CircleDollarSign,
  KeyRound,
  Link2,
  Settings2,
  ShieldCheck,
  WalletCards,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { getUserModels, getUserGroups } from '@/lib/api'
import { getCurrencyDisplay, getCurrencyLabel } from '@/lib/currency'
import { formatQuota } from '@/lib/format'
import { cn } from '@/lib/utils'
import { useStatus } from '@/hooks/use-status'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { DateTimePicker } from '@/components/datetime-picker'
import {
  SideDrawerSection,
  SideDrawerSectionHeader,
  sideDrawerSwitchItemClassName,
} from '@/components/drawer-layout'
import { MultiSelect } from '@/components/multi-select'
import Stepper, { Step } from '@/components/reactbits/stepper'
import { getSelfSubscriptionFull } from '@/features/subscriptions/api'
import type { UserSubscription } from '@/features/subscriptions/types'
import { getVirtualMembershipPage } from '@/features/virtual-membership/api'
import type { UserVirtualMembership } from '@/features/virtual-membership/types'
import { createApiKey, updateApiKey, getApiKey } from '../api'
import { ERROR_MESSAGES, SUCCESS_MESSAGES } from '../constants'
import {
  getApiKeyFormSchema,
  type ApiKeyFormValues,
  getApiKeyFormDefaultValues,
  transformFormDataToPayload,
  transformApiKeyToFormDefaults,
} from '../lib'
import { type ApiKey } from '../types'
import {
  ApiKeyGroupCombobox,
  type ApiKeyGroupOption,
} from './api-key-group-combobox'
import { ApiKeyRoutingPolicyDialog } from './api-key-routing-policy-dialog'
import { useApiKeys } from './api-keys-provider'

type ApiKeyMutateDrawerProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentRow?: ApiKey
}

function subscriptionBindingSignature(values: ApiKeyFormValues): string {
  return JSON.stringify({
    mode: values.subscription_mode,
    subscriptionId: values.subscription_id,
    allowRenewal: values.subscription_allow_renewal,
    allowSameGroup: values.subscription_allow_same_group,
    allowWallet: values.subscription_allow_wallet,
    walletLimit: values.subscription_wallet_limit_dollars,
    keepPlanned: values.keep_planned_subscription,
    virtualMembershipId: values.virtual_membership_id,
    virtualMembershipMode: values.virtual_membership_mode,
    routingMode: values.routing_mode,
    routingRevision: values.routing_revision,
    routeSteps: values.route_steps,
    group: values.group,
  })
}

export function ApiKeysMutateDrawer({
  open,
  onOpenChange,
  currentRow,
}: ApiKeyMutateDrawerProps) {
  const { t } = useTranslation()
  const isUpdate = !!currentRow
  const { triggerRefresh } = useApiKeys()
  const { status } = useStatus()
  const [isSubmitting, setIsSubmitting] = useState(false)
  const submittingRef = useRef(false)
  const [advancedOpen, setAdvancedOpen] = useState(false)
  const [routingPolicyOpen, setRoutingPolicyOpen] = useState(false)
  const [subscriptionChoiceConfirmed, setSubscriptionChoiceConfirmed] =
    useState(false)
  const [initialBindingSignature, setInitialBindingSignature] = useState('')
  const [changeAcknowledged, setChangeAcknowledged] = useState(false)
  // Create-mode Stepper: key bumps remount (reset) the Stepper; resumeStep picks
  // which step it lands on after a remount (used to recover from validation/API
  // failures so the user isn't stuck on the collapsed "completed" view).
  const [stepperKey, setStepperKey] = useState(0)
  const [resumeStep, setResumeStep] = useState(1)
  const defaultUseAutoGroup = status?.default_use_auto_group === true

  // Fetch models
  const { data: modelsData } = useQuery({
    queryKey: ['user-models'],
    queryFn: getUserModels,
    enabled: open,
    staleTime: 0,
  })

  // Fetch groups
  const { data: groupsData } = useQuery({
    queryKey: ['user-groups'],
    queryFn: getUserGroups,
    enabled: open,
    staleTime: 0,
  })

  // Fetch the user's active subscriptions so we can surface the groups that
  // their purchased plans unlock (shown first + badged as "Plan Group" in the
  // group selector).
  const selfSubQuery = useQuery({
    queryKey: ['self-subscription-full'],
    queryFn: getSelfSubscriptionFull,
    enabled: open,
    staleTime: 0,
  })

  const virtualMembershipQuery = useQuery({
    queryKey: ['virtual-membership-page'],
    queryFn: getVirtualMembershipPage,
    enabled: open,
    staleTime: 0,
  })

  const models = modelsData?.data || []
  const virtualMembershipGroups: string[] = useMemo(() => {
    const memberships = virtualMembershipQuery.data?.data?.memberships || []
    return Array.from(
      new Set(
        memberships
          .filter((membership) => membership.status === 'active')
          .map((membership) => membership.allowed_group?.trim() || '')
          .filter(Boolean)
      )
    )
  }, [virtualMembershipQuery.data])
  const groups: ApiKeyGroupOption[] = useMemo(() => {
    const groupsRaw = groupsData?.data || {}
    const options: ApiKeyGroupOption[] = Object.entries(groupsRaw).map(
      ([key, info]) => ({
        value: key,
        label: key,
        desc: info.desc || key,
        ratio: info.ratio,
      })
    )
    const known = new Set(options.map((option) => option.value))
    for (const group of virtualMembershipGroups) {
      if (known.has(group)) continue
      options.push({
        value: group,
        label: group,
        desc: '虚拟会员专属分组',
      })
    }
    return options
  }, [groupsData?.data, virtualMembershipGroups])
  const subscribedGroups: string[] = useMemo(() => {
    const subs = selfSubQuery.data?.data?.subscriptions || []
    const now = selfSubQuery.dataUpdatedAt / 1000
    const result = new Set<string>()
    for (const rec of subs) {
      const sub = rec?.subscription
      if (!sub) continue
      const group = sub.allowed_group
      if (!group) continue
      // Only treat currently-active subscriptions as "plan groups".
      if (sub.status === 'active' && (sub.end_time || 0) >= now) {
        result.add(group)
      }
    }
    return Array.from(result)
  }, [selfSubQuery.data, selfSubQuery.dataUpdatedAt])
  const backendHasAuto = groups.some((g) => g.value === 'auto')
  const schema = getApiKeyFormSchema(t)

  const form = useForm<ApiKeyFormValues>({
    resolver: zodResolver(schema),
    defaultValues: getApiKeyFormDefaultValues(defaultUseAutoGroup),
  })

  const resetStepperTo = (step: number) => {
    setResumeStep(step)
    setStepperKey((k) => k + 1)
  }

  // Load existing data when updating
  useEffect(() => {
    if (open && isUpdate && currentRow) {
      getApiKey(currentRow.id).then((result) => {
        if (result.success && result.data) {
          const defaults = transformApiKeyToFormDefaults(result.data)
          form.reset(defaults)
          setInitialBindingSignature(subscriptionBindingSignature(defaults))
          setChangeAcknowledged(false)
          setSubscriptionChoiceConfirmed(true)
        }
      })
    } else if (open && !isUpdate) {
      form.reset(
        getApiKeyFormDefaultValues(defaultUseAutoGroup && backendHasAuto)
      )
      // Start a fresh Stepper each time the create drawer opens.
      setResumeStep(1)
      setStepperKey((k) => k + 1)
      setSubscriptionChoiceConfirmed(false)
      setInitialBindingSignature('')
      setChangeAcknowledged(false)
    }
  }, [open, isUpdate, currentRow, form, defaultUseAutoGroup, backendHasAuto])

  // Correct group after groups load: if the form value is not in available groups, fall back
  useEffect(() => {
    if (groups.length === 0) return
    const currentGroup = form.getValues('group')
    if (currentGroup && !groups.some((g) => g.value === currentGroup)) {
      const fallback =
        groups.find((g) => g.value === 'default')?.value ??
        groups[0]?.value ??
        ''
      form.setValue('group', fallback)
      if (currentGroup === 'auto') {
        form.setValue('cross_group_retry', false)
      }
    }
  }, [groups, form])

  const onSubmit = async (data: ApiKeyFormValues) => {
    if (
      hasVirtualMembershipStep &&
      data.virtual_membership_mode === 'instance' &&
      data.virtual_membership_id <= 0
    ) {
      form.setError('virtual_membership_id', {
        type: 'manual',
        message: '请选择虚拟会员额度',
      })
      toast.error('请选择虚拟会员额度')
      if (!isUpdate) {
        resetStepperTo(2)
      }
      return
    }
    if (
      isUpdate &&
      hasSourceStep &&
      initialBindingSignature &&
      subscriptionBindingSignature(data) !== initialBindingSignature &&
      !changeAcknowledged
    ) {
      toast.error(t('Please acknowledge the subscription ownership change'))
      return
    }
    if (submittingRef.current) return
    submittingRef.current = true
    setIsSubmitting(true)
    try {
      const basePayload = transformFormDataToPayload(data)

      if (isUpdate && currentRow) {
        const result = await updateApiKey({
          ...basePayload,
          id: currentRow.id,
        })
        if (result.success) {
          toast.success(t(SUCCESS_MESSAGES.API_KEY_UPDATED))
          onOpenChange(false)
          triggerRefresh()
        } else {
          toast.error(result.message || t(ERROR_MESSAGES.UPDATE_FAILED))
        }
      } else {
        // Create mode - handle batch creation
        const count = data.tokenCount || 1
        let successCount = 0

        for (let i = 0; i < count; i++) {
          const result = await createApiKey({
            ...basePayload,
            name:
              i === 0 && data.name
                ? data.name
                : `${data.name || 'default'}-${Math.random().toString(36).slice(2, 8)}`,
          })
          if (result.success) {
            successCount++
          } else {
            toast.error(result.message || t(ERROR_MESSAGES.CREATE_FAILED))
            break
          }
        }

        if (successCount > 0) {
          toast.success(
            t('Successfully created {{count}} API Key(s)', {
              count: successCount,
            })
          )
          onOpenChange(false)
          triggerRefresh()
        } else {
          // Nothing got created — un-collapse the Stepper back to the final
          // step so the user can adjust and retry.
          resetStepperTo(hasSourceStep ? 4 : 3)
        }
      }
    } catch (_error) {
      toast.error(t(ERROR_MESSAGES.UNEXPECTED))
      if (!isUpdate) {
        resetStepperTo(hasSourceStep ? 4 : 3)
      }
    } finally {
      submittingRef.current = false
      setIsSubmitting(false)
    }
  }

  const onInvalid: SubmitErrorHandler<ApiKeyFormValues> = (errors) => {
    toast.error(t('Please fix the highlighted fields before saving'))
    if (!isUpdate) {
      // Reveal the earliest step that owns an errored field.
      const errorKeys = Object.keys(errors)
      const step1Keys = [
        'name',
        'group',
        'cross_group_retry',
        'expired_time',
        'tokenCount',
      ]
      const subscriptionKeys = [
        'subscription_mode',
        'subscription_id',
        'subscription_allow_renewal',
        'subscription_allow_same_group',
        'subscription_allow_wallet',
        'subscription_wallet_limit_dollars',
        'virtual_membership_id',
      ]
      const quotaKeys = ['remain_quota_dollars', 'unlimited_quota']
      const advancedKeys = ['model_limits', 'allow_ips']
      let target = 1
      if (!errorKeys.some((k) => step1Keys.includes(k))) {
        if (
          hasSourceStep &&
          errorKeys.some((k) => subscriptionKeys.includes(k))
        ) {
          target = 2
        } else if (errorKeys.some((k) => quotaKeys.includes(k))) {
          target = hasSourceStep ? 3 : 2
        } else if (errorKeys.some((k) => advancedKeys.includes(k))) {
          target = hasSourceStep ? 4 : 3
        }
      }
      resetStepperTo(target)
    }
  }

  const handleSetExpiry = (months: number, days: number, hours: number) => {
    if (months === 0 && days === 0 && hours === 0) {
      form.setValue('expired_time', undefined)
      return
    }

    const now = new Date()
    now.setMonth(now.getMonth() + months)
    now.setDate(now.getDate() + days)
    now.setHours(now.getHours() + hours)

    form.setValue('expired_time', now)
  }

  const { meta: currencyMeta } = getCurrencyDisplay()
  const currencyLabel = getCurrencyLabel()
  const tokensOnly = currencyMeta.kind === 'tokens'
  const quotaLabel = t('Quota ({{currency}})', { currency: currencyLabel })
  const quotaPlaceholder = tokensOnly
    ? t('Enter quota in tokens')
    : t('Enter quota in {{currency}}', { currency: currencyLabel })
  const selectedGroup = form.watch('group')
  const routingMode = form.watch('routing_mode')
  const routeSteps = form.watch('route_steps')
  const unlimitedQuota = form.watch('unlimited_quota')
  const subscriptionMode = form.watch('subscription_mode')
  const selectedSubscriptionId = form.watch('subscription_id')
  const allowRenewal = form.watch('subscription_allow_renewal')
  const allowSameGroup = form.watch('subscription_allow_same_group')
  const allowWallet = form.watch('subscription_allow_wallet')
  const hasSubscriptionStep =
    routingMode === 'single' && subscribedGroups.includes(selectedGroup)
  const hasVirtualMembershipStep =
    routingMode === 'single' && virtualMembershipGroups.includes(selectedGroup)
  const continuationEnabled = allowRenewal || allowSameGroup || allowWallet
  const currentBindingSignature = subscriptionBindingSignature(form.getValues())
  const bindingChanged =
    isUpdate &&
    initialBindingSignature !== '' &&
    currentBindingSignature !== initialBindingSignature

  useEffect(() => {
    setChangeAcknowledged(false)
  }, [currentBindingSignature])

  const subscriptionInstances = useMemo(() => {
    const records = selfSubQuery.data?.data?.all_subscriptions || []
    const now = Date.now() / 1000
    return records
      .map((record) => record.subscription)
      .filter((subscription): subscription is UserSubscription => {
        if (!subscription) return false
        const compatible =
          !subscription.allowed_group ||
          subscription.allowed_group === selectedGroup
        const isCurrent = isUpdate && subscription.id === selectedSubscriptionId
        if (!compatible && !isCurrent) return false
        const total = Number(subscription.amount_total || 0)
        const used = Number(subscription.amount_used || 0)
        const cap = Number(subscription.amount_cap || 0)
        const capUsed = Number(subscription.amount_cap_used || 0)
        const usable =
          subscription.status === 'active' &&
          subscription.start_time <= now &&
          subscription.end_time > now &&
          (total <= 0 || used < total) &&
          (cap <= 0 || capUsed < cap)
        return usable || isCurrent
      })
      .sort((a, b) => a.end_time - b.end_time || a.id - b.id)
  }, [selfSubQuery.data, selectedGroup, selectedSubscriptionId, isUpdate])

  const selectedSubscription = subscriptionInstances.find(
    (subscription) => subscription.id === selectedSubscriptionId
  )

  const virtualMemberships = useMemo(() => {
    const memberships = virtualMembershipQuery.data?.data?.memberships || []
    return memberships.filter(
      (membership): membership is UserVirtualMembership =>
        membership.status === 'active' &&
        (membership.allowed_group?.trim() || '') === selectedGroup
    )
  }, [selectedGroup, virtualMembershipQuery.data])
  const allSubscriptionInstances = useMemo(() => {
    const records = selfSubQuery.data?.data?.all_subscriptions || []
    const now = Date.now() / 1000
    return records
      .map((record) => record.subscription)
      .filter(
        (subscription): subscription is UserSubscription =>
          !!subscription &&
          subscription.status === 'active' &&
          subscription.start_time <= now &&
          subscription.end_time > now
      )
  }, [selfSubQuery.data])
  const allVirtualMemberships = useMemo(
    () =>
      (virtualMembershipQuery.data?.data?.memberships || []).filter(
        (membership) => membership.status === 'active'
      ),
    [virtualMembershipQuery.data]
  )
  const hasSourceStep = hasSubscriptionStep || hasVirtualMembershipStep

  useEffect(() => {
    if (!open || isUpdate) return
    setSubscriptionChoiceConfirmed(!hasSubscriptionStep)
  }, [hasSubscriptionStep, selectedGroup, open, isUpdate])

  useEffect(() => {
    if (!open) return
    if (!hasSubscriptionStep) {
      form.setValue('subscription_mode', 'auto')
      form.setValue('subscription_id', 0)
      form.setValue('subscription_allow_renewal', false)
      form.setValue('subscription_allow_same_group', false)
      form.setValue('subscription_allow_wallet', false)
      form.setValue('subscription_wallet_limit_dollars', 0)
    }
    if (!hasVirtualMembershipStep) {
      form.setValue('virtual_membership_id', 0)
    }
    if (
      subscriptionMode === 'instance' &&
      selectedSubscriptionId > 0 &&
      !subscriptionInstances.some(
        (subscription) => subscription.id === selectedSubscriptionId
      )
    ) {
      form.setValue('subscription_id', 0, { shouldValidate: true })
    }
  }, [
    form,
    hasSubscriptionStep,
    hasVirtualMembershipStep,
    isUpdate,
    open,
    selectedSubscriptionId,
    subscriptionInstances,
    subscriptionMode,
  ])

  // Shared field groups — rendered both in the edit-mode one-shot form and the
  // create-mode Stepper steps, so field/validation/submit logic stays single-source.
  const basicInfoFields = (
    <>
      <FormField
        control={form.control}
        name='name'
        render={({ field }) => (
          <FormItem>
            <FormLabel>{t('Name')}</FormLabel>
            <FormControl>
              <Input {...field} placeholder={t('Enter a name')} />
            </FormControl>
            <FormMessage />
          </FormItem>
        )}
      />

      <FormField
        control={form.control}
        name='group'
        render={({ field }) => (
          <FormItem>
            <FormLabel>{t('Group')}</FormLabel>
            <FormControl>
              <ApiKeyGroupCombobox
                options={groups}
                value={field.value}
                onValueChange={(value) => {
                  field.onChange(value)
                  form.setValue('routing_mode', 'single')
                  form.setValue('route_steps', [])
                }}
                placeholder={t('Select a group')}
                subscribedGroups={subscribedGroups}
                virtualMembershipGroups={virtualMembershipGroups}
                customRoutingSelected={routingMode === 'custom'}
                onCustomRouting={() => setRoutingPolicyOpen(true)}
              />
            </FormControl>
            <FormMessage />
          </FormItem>
        )}
      />

      {selectedGroup === 'auto' && (
        <FormField
          control={form.control}
          name='cross_group_retry'
          render={({ field }) => (
            <FormItem className={sideDrawerSwitchItemClassName()}>
              <div className='flex flex-col gap-0.5'>
                <FormLabel className='text-sm'>
                  {t('Cross-group retry')}
                </FormLabel>
                <FormDescription className='line-clamp-2 text-xs sm:line-clamp-none'>
                  {t(
                    'When enabled, if channels in the current group fail, it will try channels in the next group in order.'
                  )}
                </FormDescription>
              </div>
              <FormControl>
                <Switch
                  checked={!!field.value}
                  onCheckedChange={field.onChange}
                />
              </FormControl>
            </FormItem>
          )}
        />
      )}

      <FormField
        control={form.control}
        name='expired_time'
        render={({ field }) => (
          <FormItem>
            <FormLabel>{t('Expiration Time')}</FormLabel>
            <div className='grid gap-2 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center'>
              <FormControl>
                <DateTimePicker
                  value={field.value}
                  onChange={field.onChange}
                  placeholder={t('Never expires')}
                  className='min-w-0 [&_input[type=time]]:w-24 sm:[&_input[type=time]]:w-32'
                />
              </FormControl>
              <div className='grid grid-cols-4 gap-2 sm:flex'>
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  className='px-2 text-xs sm:px-3 sm:text-sm'
                  onClick={() => handleSetExpiry(0, 0, 0)}
                >
                  {t('Never')}
                </Button>
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  className='px-2 text-xs sm:px-3 sm:text-sm'
                  onClick={() => handleSetExpiry(1, 0, 0)}
                >
                  {t('1 Month')}
                </Button>
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  className='px-2 text-xs sm:px-3 sm:text-sm'
                  onClick={() => handleSetExpiry(0, 1, 0)}
                >
                  {t('1 Day')}
                </Button>
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  className='px-2 text-xs sm:px-3 sm:text-sm'
                  onClick={() => handleSetExpiry(0, 0, 1)}
                >
                  {t('1 Hour')}
                </Button>
              </div>
            </div>
            <FormMessage />
          </FormItem>
        )}
      />

      {!isUpdate && (
        <FormField
          control={form.control}
          name='tokenCount'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Quantity')}</FormLabel>
              <FormControl>
                <Input
                  {...field}
                  type='number'
                  min='1'
                  placeholder={t('Number of keys to create')}
                  onChange={(e) =>
                    field.onChange(parseInt(e.target.value, 10) || 1)
                  }
                />
              </FormControl>
              <FormDescription>
                {t(
                  'Create multiple API keys at once (random suffix will be added to names)'
                )}
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />
      )}
    </>
  )

  const subscriptionFields = (
    <div className='space-y-5'>
      {hasSubscriptionStep && (
        <FormField
          control={form.control}
          name='subscription_mode'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Quota source')}</FormLabel>
              <FormControl>
                <RadioGroup
                  value={
                    subscriptionChoiceConfirmed || isUpdate ? field.value : ''
                  }
                  onValueChange={(value) => {
                    field.onChange(value)
                    setSubscriptionChoiceConfirmed(true)
                    form.setValue('virtual_membership_id', 0)
                    if (value === 'auto') {
                      form.setValue('subscription_id', 0)
                    }
                  }}
                  className='grid gap-3 sm:grid-cols-2'
                >
                  <label
                    className={cn(
                      'border-border bg-card hover:border-primary/40 flex cursor-pointer gap-3 rounded-xl border p-4 transition-colors',
                      field.value === 'auto' &&
                        subscriptionChoiceConfirmed &&
                        'border-primary bg-primary/5'
                    )}
                  >
                    <RadioGroupItem value='auto' className='mt-0.5' />
                    <span className='space-y-1'>
                      <span className='flex flex-wrap items-center gap-2 text-sm font-semibold'>
                        <ShieldCheck className='text-primary size-4' />
                        {t('System automatic allocation')}
                        <Badge className='border-emerald-500/25 bg-emerald-500/10 px-1.5 py-0 text-[10px] font-semibold text-emerald-700 shadow-none dark:text-emerald-300'>
                          {t('Recommended')}
                        </Badge>
                      </span>
                      <span className='text-muted-foreground block text-xs leading-5'>
                        {t(
                          'Keep the current allocation behavior and let the system choose an available funding source.'
                        )}
                      </span>
                    </span>
                  </label>
                  <label
                    className={cn(
                      'border-border bg-card hover:border-primary/40 flex cursor-pointer gap-3 rounded-xl border p-4 transition-colors',
                      field.value === 'instance' &&
                        subscriptionChoiceConfirmed &&
                        'border-primary bg-primary/5'
                    )}
                  >
                    <RadioGroupItem value='instance' className='mt-0.5' />
                    <span className='space-y-1'>
                      <span className='flex items-center gap-2 text-sm font-semibold'>
                        <Link2 className='text-primary size-4' />
                        {t('Specify subscription instance')}
                      </span>
                      <span className='text-muted-foreground block text-xs leading-5'>
                        {t(
                          'Bind this API Key to one purchased subscription instance for project-level quota isolation.'
                        )}
                      </span>
                    </span>
                  </label>
                </RadioGroup>
              </FormControl>
              {!subscriptionChoiceConfirmed && !isUpdate && (
                <FormDescription className='text-warning'>
                  {t('Please make this choice yourself before continuing.')}
                </FormDescription>
              )}
            </FormItem>
          )}
        />
      )}

      {hasVirtualMembershipStep && (
        <div className='space-y-4'>
          <FormField
            control={form.control}
            name='virtual_membership_mode'
            render={({ field }) => (
              <FormItem>
                <FormLabel>会员额度分配</FormLabel>
                <FormControl>
                  <RadioGroup
                    value={field.value}
                    onValueChange={(value) => {
                      field.onChange(value)
                      if (value === 'auto') {
                        form.setValue('virtual_membership_id', 0)
                        form.clearErrors('virtual_membership_id')
                      }
                    }}
                    className='grid gap-3 sm:grid-cols-2'
                  >
                    <label
                      className={cn(
                        'border-border bg-card flex cursor-pointer gap-3 rounded-xl border p-4',
                        field.value === 'auto' && 'border-primary bg-primary/5'
                      )}
                    >
                      <RadioGroupItem value='auto' className='mt-0.5' />
                      <span>
                        <span className='flex items-center gap-2 text-sm font-semibold'>
                          自动分配
                          <Badge className='px-1.5 py-0 text-[10px]'>
                            推荐
                          </Badge>
                        </span>
                        <span className='text-muted-foreground mt-1 block text-xs leading-5'>
                          先用周限额最早到期的会员，用完自动换下一个。
                        </span>
                      </span>
                    </label>
                    <label
                      className={cn(
                        'border-border bg-card flex cursor-pointer gap-3 rounded-xl border p-4',
                        field.value === 'instance' &&
                          'border-primary bg-primary/5'
                      )}
                    >
                      <RadioGroupItem value='instance' className='mt-0.5' />
                      <span>
                        <span className='text-sm font-semibold'>
                          绑定指定会员
                        </span>
                        <span className='text-muted-foreground mt-1 block text-xs leading-5'>
                          与原有方式相同，这枚 Key 只消耗选定的会员实例。
                        </span>
                      </span>
                    </label>
                  </RadioGroup>
                </FormControl>
              </FormItem>
            )}
          />
          {form.watch('virtual_membership_mode') === 'instance' && (
            <FormField
              control={form.control}
              name='virtual_membership_id'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>虚拟会员额度</FormLabel>
                  <FormDescription>
                    绑定后，本 API Key 的请求会优先消耗所选虚拟会员额度。
                  </FormDescription>
                  <FormControl>
                    <RadioGroup
                      value={field.value > 0 ? String(field.value) : '0'}
                      onValueChange={(value) => {
                        const membershipId = Number(value)
                        field.onChange(membershipId)
                        if (membershipId > 0) {
                          form.clearErrors('virtual_membership_id')
                          form.setValue('subscription_mode', 'auto')
                          form.setValue('subscription_id', 0)
                          setSubscriptionChoiceConfirmed(true)
                        }
                      }}
                      className='max-h-64 gap-2 overflow-y-auto pr-1'
                    >
                      {virtualMemberships.map((membership) => {
                        const incompatible =
                          !!membership.allowed_group &&
                          membership.allowed_group !== selectedGroup
                        const unavailable =
                          incompatible || membership.weekly_remaining <= 0
                        return (
                          <label
                            key={membership.id}
                            className={cn(
                              'border-border bg-card grid cursor-pointer grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-3 rounded-xl border p-3',
                              field.value === membership.id &&
                                'border-primary bg-primary/5',
                              unavailable &&
                                'bg-muted/40 cursor-not-allowed opacity-65'
                            )}
                          >
                            <RadioGroupItem
                              value={String(membership.id)}
                              disabled={unavailable}
                            />
                            <span className='min-w-0'>
                              <span className='block truncate text-sm font-medium'>
                                {membership.plan_title} · #{membership.id}
                              </span>
                              <span className='text-muted-foreground mt-1 block text-xs'>
                                周余量{' '}
                                {formatQuota(membership.weekly_remaining)}
                                {membership.five_hour_enabled
                                  ? ` · 5h ${formatQuota(membership.five_hour_remaining)}`
                                  : ''}
                              </span>
                            </span>
                            <span className='text-muted-foreground text-[10px]'>
                              {membership.group_size === 1
                                ? '单独'
                                : `${membership.group_size} 人团`}
                            </span>
                          </label>
                        )
                      })}
                      {virtualMemberships.length === 0 && (
                        <div className='border-border bg-muted/30 rounded-xl border border-dashed p-6 text-center'>
                          <p className='text-sm font-medium'>
                            暂无可绑定的虚拟会员额度
                          </p>
                          <p className='text-muted-foreground mt-1 text-xs'>
                            请先购买或恢复该会员分组对应的有效额度。
                          </p>
                        </div>
                      )}
                    </RadioGroup>
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          )}
        </div>
      )}

      {hasSubscriptionStep && subscriptionMode === 'instance' && (
        <>
          <FormField
            control={form.control}
            name='subscription_id'
            render={({ field }) => (
              <FormItem>
                <div className='flex items-end justify-between gap-3'>
                  <div>
                    <FormLabel>{t('Subscription instance')}</FormLabel>
                    <FormDescription>
                      {t(
                        'Instance notes help distinguish subscriptions purchased for different projects.'
                      )}
                    </FormDescription>
                  </div>
                  <span className='text-muted-foreground text-xs'>
                    {t('{{count}} available', {
                      count: subscriptionInstances.length,
                    })}
                  </span>
                </div>
                <FormControl>
                  <RadioGroup
                    value={field.value > 0 ? String(field.value) : ''}
                    onValueChange={(value) => field.onChange(Number(value))}
                    className='max-h-72 gap-2 overflow-y-auto pr-1'
                  >
                    {subscriptionInstances.map((subscription) => {
                      const now = Date.now() / 1000
                      const total = Number(subscription.amount_total || 0)
                      const used = Number(subscription.amount_used || 0)
                      const remaining =
                        total > 0 ? Math.max(0, total - used) : 0
                      const incompatible =
                        !!subscription.allowed_group &&
                        subscription.allowed_group !== selectedGroup
                      const unavailable =
                        incompatible ||
                        subscription.status !== 'active' ||
                        subscription.start_time > now ||
                        subscription.end_time <= now ||
                        (total > 0 && remaining <= 0)
                      return (
                        <label
                          key={subscription.id}
                          className={cn(
                            'border-border bg-card grid cursor-pointer grid-cols-[auto_minmax(0,1fr)_auto] items-start gap-3 rounded-xl border p-3 transition-colors',
                            field.value === subscription.id &&
                              'border-primary bg-primary/5',
                            unavailable &&
                              'bg-muted/40 cursor-not-allowed opacity-65'
                          )}
                        >
                          <RadioGroupItem
                            value={String(subscription.id)}
                            disabled={unavailable}
                            className='mt-1'
                          />
                          <span className='min-w-0'>
                            <span className='flex flex-wrap items-center gap-2'>
                              <span className='font-mono text-xs font-semibold'>
                                #{subscription.id}
                              </span>
                              <span className='truncate text-sm font-medium'>
                                {subscription.plan_title ||
                                  t('Subscription instance')}
                              </span>
                            </span>
                            <span className='text-muted-foreground mt-1 block truncate text-xs'>
                              {subscription.remark || t('No instance note yet')}
                            </span>
                            {incompatible && (
                              <span className='text-destructive mt-1 block text-xs'>
                                {t(
                                  'This instance only supports group {{group}}. Choose another instance or automatic allocation before saving.',
                                  { group: subscription.allowed_group }
                                )}
                              </span>
                            )}
                          </span>
                          <span className='text-right'>
                            <span className='text-foreground block font-mono text-xs font-semibold'>
                              {total > 0
                                ? formatQuota(remaining)
                                : t('Unlimited')}
                            </span>
                            <span className='text-muted-foreground mt-1 flex items-center justify-end gap-1 text-[10px]'>
                              <CalendarClock className='size-3' />
                              {new Date(
                                subscription.end_time * 1000
                              ).toLocaleDateString()}
                            </span>
                          </span>
                        </label>
                      )
                    })}
                    {subscriptionInstances.length === 0 && (
                      <div className='border-border bg-muted/30 rounded-xl border border-dashed p-6 text-center'>
                        <p className='text-sm font-medium'>
                          {t('No compatible subscription instance')}
                        </p>
                        <p className='text-muted-foreground mt-1 text-xs'>
                          {t(
                            'Choose system automatic allocation or purchase a compatible subscription first.'
                          )}
                        </p>
                      </div>
                    )}
                  </RadioGroup>
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          {selectedSubscription && (
            <div className='border-border bg-muted/20 space-y-4 rounded-xl border p-4'>
              <div>
                <div className='text-sm font-semibold'>
                  {t('When this instance is exhausted')}
                </div>
                <div className='text-muted-foreground mt-1 text-xs'>
                  {t(
                    'This setting controls the funding source after the selected instance becomes unavailable.'
                  )}
                </div>
              </div>
              <RadioGroup
                value={continuationEnabled ? 'continue' : 'stop'}
                onValueChange={(value) => {
                  if (value === 'stop') {
                    form.setValue('subscription_allow_renewal', false)
                    form.setValue('subscription_allow_same_group', false)
                    form.setValue('subscription_allow_wallet', false)
                  } else if (!continuationEnabled) {
                    form.setValue('subscription_allow_renewal', true)
                  }
                }}
                className='grid gap-2 sm:grid-cols-2'
              >
                <label className='bg-background flex cursor-pointer items-center gap-3 rounded-lg border p-3'>
                  <RadioGroupItem value='stop' />
                  <span className='text-sm font-medium'>
                    {t('Stop requests immediately')}
                  </span>
                </label>
                <label className='bg-background flex cursor-pointer items-center gap-3 rounded-lg border p-3'>
                  <RadioGroupItem value='continue' />
                  <span className='text-sm font-medium'>
                    {t('Allow continuation')}
                  </span>
                </label>
              </RadioGroup>

              {continuationEnabled && (
                <div className='space-y-2'>
                  <FormField
                    control={form.control}
                    name='subscription_allow_renewal'
                    render={({ field }) => (
                      <FormItem className={sideDrawerSwitchItemClassName()}>
                        <div className='flex flex-col gap-0.5'>
                          <FormLabel className='text-sm'>
                            {t('Continue to renewed successor instance')}
                          </FormLabel>
                          <FormDescription className='text-xs'>
                            {t(
                              'Only follows the explicit successor created by Renew; it never guesses by plan name.'
                            )}
                          </FormDescription>
                        </div>
                        <FormControl>
                          <Switch
                            checked={field.value}
                            onCheckedChange={field.onChange}
                          />
                        </FormControl>
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='subscription_allow_same_group'
                    render={({ field }) => (
                      <FormItem className={sideDrawerSwitchItemClassName()}>
                        <div className='flex flex-col gap-0.5'>
                          <FormLabel className='text-sm'>
                            {t('Continue to another instance in this group')}
                          </FormLabel>
                          <FormDescription className='text-xs'>
                            {t(
                              'Uses the compatible instance that expires first after the renewal chain.'
                            )}
                          </FormDescription>
                        </div>
                        <FormControl>
                          <Switch
                            checked={field.value}
                            onCheckedChange={field.onChange}
                          />
                        </FormControl>
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='subscription_allow_wallet'
                    render={({ field }) => (
                      <FormItem className={sideDrawerSwitchItemClassName()}>
                        <div className='flex flex-col gap-0.5'>
                          <FormLabel className='flex items-center gap-2 text-sm'>
                            <CircleDollarSign className='size-4' />
                            {t('Continue with wallet balance')}
                          </FormLabel>
                          <FormDescription className='text-xs'>
                            {t(
                              'This may create charges outside the subscription and is always used last.'
                            )}
                          </FormDescription>
                        </div>
                        <FormControl>
                          <Switch
                            checked={field.value}
                            onCheckedChange={field.onChange}
                          />
                        </FormControl>
                      </FormItem>
                    )}
                  />
                  {allowWallet && (
                    <FormField
                      control={form.control}
                      name='subscription_wallet_limit_dollars'
                      render={({ field }) => (
                        <FormItem className='border-warning/40 bg-warning/5 rounded-lg border p-3'>
                          <FormLabel>
                            {t('Wallet fallback limit ({{currency}})', {
                              currency: currencyLabel,
                            })}
                          </FormLabel>
                          <FormControl>
                            <Input
                              type='number'
                              min='0'
                              step={tokensOnly ? 1 : 0.01}
                              value={field.value}
                              onChange={(event) =>
                                field.onChange(
                                  Number.parseFloat(event.target.value) || 0
                                )
                              }
                            />
                          </FormControl>
                          <FormDescription>
                            {t(
                              'Requests stop when this Key reaches the limit; unlimited fallback is not allowed.'
                            )}
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                  )}
                </div>
              )}

              <div className='bg-background text-muted-foreground flex flex-wrap items-center gap-1.5 rounded-lg border px-3 py-2 text-xs'>
                <span>{t('Order')}:</span>
                {allowRenewal && (
                  <>
                    <span className='text-foreground font-medium'>
                      {t('Renewed successor')}
                    </span>
                    <ArrowRight className='size-3' />
                  </>
                )}
                {allowSameGroup && (
                  <>
                    <span className='text-foreground font-medium'>
                      {t('Other group instance')}
                    </span>
                    <ArrowRight className='size-3' />
                  </>
                )}
                {allowWallet && (
                  <span className='text-foreground font-medium'>
                    {t('Wallet balance')}
                  </span>
                )}
                {!continuationEnabled && (
                  <span className='text-foreground font-medium'>
                    {t('Stop requests')}
                  </span>
                )}
              </div>
            </div>
          )}
        </>
      )}

      {hasSubscriptionStep &&
        subscriptionMode === 'auto' &&
        isUpdate &&
        (currentRow?.planned_subscription_id || 0) > 0 && (
          <FormField
            control={form.control}
            name='keep_planned_subscription'
            render={({ field }) => (
              <FormItem className='border-warning/40 bg-warning/5 rounded-xl border p-4'>
                <div className='flex items-start justify-between gap-3'>
                  <div>
                    <FormLabel>
                      {t('Keep the scheduled renewal binding')}
                    </FormLabel>
                    <FormDescription className='mt-1'>
                      {t(
                        'Default is off: returning to automatic allocation also cancels the scheduled successor. Turn this on only if you want this Key to bind again when instance #{{id}} becomes active.',
                        { id: currentRow?.planned_subscription_id }
                      )}
                    </FormDescription>
                  </div>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </div>
              </FormItem>
            )}
          />
        )}
    </div>
  )

  const quotaFields = (
    <>
      {!unlimitedQuota && (
        <FormField
          control={form.control}
          name='remain_quota_dollars'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{quotaLabel}</FormLabel>
              <FormControl>
                <Input
                  {...field}
                  type='number'
                  step={tokensOnly ? 1 : 0.01}
                  placeholder={quotaPlaceholder}
                  onChange={(e) =>
                    field.onChange(parseFloat(e.target.value) || 0)
                  }
                />
              </FormControl>
              <FormDescription>
                {tokensOnly
                  ? t('Enter the quota amount in tokens')
                  : t('Enter the quota amount in {{currency}}', {
                      currency: currencyLabel,
                    })}
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />
      )}

      <FormField
        control={form.control}
        name='unlimited_quota'
        render={({ field }) => (
          <FormItem className={sideDrawerSwitchItemClassName()}>
            <div className='flex flex-col gap-0.5'>
              <FormLabel className='text-sm'>{t('Unlimited Quota')}</FormLabel>
              <FormDescription className='text-xs'>
                {t('Enable unlimited quota for this API key')}
              </FormDescription>
            </div>
            <FormControl>
              <Switch checked={field.value} onCheckedChange={field.onChange} />
            </FormControl>
          </FormItem>
        )}
      />
    </>
  )

  const advancedFields = (
    <>
      <FormField
        control={form.control}
        name='model_limits'
        render={({ field }) => (
          <FormItem>
            <FormLabel>{t('Model Limits')}</FormLabel>
            <FormControl>
              <MultiSelect
                options={models.map((m) => ({
                  label: m,
                  value: m,
                }))}
                selected={field.value}
                onChange={field.onChange}
                placeholder={t('Select models (empty for allow all)')}
              />
            </FormControl>
            <FormDescription>
              {t('Limit which models can be used with this key')}
            </FormDescription>
            <FormMessage />
          </FormItem>
        )}
      />

      <FormField
        control={form.control}
        name='allow_ips'
        render={({ field }) => (
          <FormItem>
            <FormLabel>{t('IP Whitelist (supports CIDR)')}</FormLabel>
            <FormControl>
              <Textarea
                {...field}
                className='min-h-20 resize-none'
                placeholder={t('One IP per line (empty for no restriction)')}
                rows={3}
              />
            </FormControl>
            <FormDescription>
              {t(
                'Do not over-trust this feature. IP may be spoofed. Please use with nginx, CDN and other gateways.'
              )}
            </FormDescription>
            <FormMessage />
          </FormItem>
        )}
      />
    </>
  )

  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        onOpenChange(v)
        if (!v) {
          form.reset()
        }
      }}
    >
      <DialogContent className='max-h-[calc(100dvh-2rem)] overflow-y-auto sm:max-w-2xl'>
        <DialogHeader>
          <DialogTitle>
            {isUpdate ? t('Update API Key') : t('Create API Key')}
          </DialogTitle>
          <DialogDescription>
            {isUpdate
              ? t('Update the API key by providing necessary info.')
              : t('Add a new API key by providing necessary info.')}
          </DialogDescription>
        </DialogHeader>
        <Form {...form}>
          {isUpdate ? (
            // Update mode: keep the original one-shot form (editing a single
            // field is faster without step navigation).
            <form
              id='api-key-form'
              onSubmit={form.handleSubmit(onSubmit, onInvalid)}
              className='space-y-5'
            >
              <SideDrawerSection>
                <SideDrawerSectionHeader
                  title={t('Basic Information')}
                  description={t('Set API key basic information')}
                  icon={<KeyRound className='size-4' />}
                />
                {basicInfoFields}
              </SideDrawerSection>

              {hasSourceStep && (
                <SideDrawerSection>
                  <SideDrawerSectionHeader
                    title={
                      hasSubscriptionStep
                        ? t('Subscription ownership')
                        : '虚拟会员额度'
                    }
                    description={
                      hasSubscriptionStep
                        ? t('Choose the quota source and continuation behavior')
                        : '为会员分组绑定对应的可用额度'
                    }
                    icon={<Link2 className='size-4' />}
                  />
                  {subscriptionFields}
                  {bindingChanged && (
                    <Alert className='border-warning/50 bg-warning/5'>
                      <AlertDescription className='space-y-2'>
                        <p>
                          {t(
                            'You changed this existing Key’s group, subscription instance, or continuation policy. The new choice will replace the previous one after saving.'
                          )}
                        </p>
                        <label className='flex items-center gap-2 font-medium'>
                          <Checkbox
                            checked={changeAcknowledged}
                            onCheckedChange={(checked) =>
                              setChangeAcknowledged(checked === true)
                            }
                          />
                          {t('I have reviewed and accept this change')}
                        </label>
                      </AlertDescription>
                    </Alert>
                  )}
                </SideDrawerSection>
              )}

              <SideDrawerSection>
                <SideDrawerSectionHeader
                  title={t('Quota Settings')}
                  description={t('Set quota amount and limits')}
                  icon={<WalletCards className='size-4' />}
                />
                {quotaFields}
              </SideDrawerSection>

              <Collapsible open={advancedOpen} onOpenChange={setAdvancedOpen}>
                <SideDrawerSection>
                  <CollapsibleTrigger
                    render={
                      <button
                        type='button'
                        className='hover:bg-muted/40 flex w-full items-center gap-3 rounded-md py-1.5 text-left transition-colors'
                      />
                    }
                  >
                    <SideDrawerSectionHeader
                      className='flex-1'
                      title={t('Advanced Settings')}
                      description={t('Set API key access restrictions')}
                      icon={<Settings2 className='size-4' />}
                    />
                    <ChevronDown
                      className={cn(
                        'text-muted-foreground size-4 shrink-0 transition-transform',
                        advancedOpen && 'rotate-180'
                      )}
                    />
                  </CollapsibleTrigger>
                  <CollapsibleContent>
                    <div className='flex flex-col gap-4 pt-2'>
                      {advancedFields}
                    </div>
                  </CollapsibleContent>
                </SideDrawerSection>
              </Collapsible>
            </form>
          ) : (
            // Create mode: 3-step guided Stepper. The Stepper's Complete action
            // drives form submission via onFinalStepCompleted; Enter-key submit
            // is suppressed so the Stepper stays the only submission path.
            <form onSubmit={(e) => e.preventDefault()} className='space-y-5'>
              <Stepper
                key={stepperKey}
                initialStep={resumeStep}
                onFinalStepCompleted={form.handleSubmit(onSubmit, onInvalid)}
                nextButtonProps={{ disabled: isSubmitting }}
                onBeforeNext={async (step) => {
                  // Step 1 (Basic Information): name + group required before
                  // the user can proceed. form.trigger surfaces errors on the
                  // fields so FormMessage highlights them.
                  if (step === 1) {
                    const [okName, okGroup] = await Promise.all([
                      form.trigger('name'),
                      form.trigger('group'),
                    ])
                    return Boolean(okName && okGroup)
                  }
                  if (hasSourceStep && step === 2) {
                    if (hasSubscriptionStep && !subscriptionChoiceConfirmed) {
                      toast.error(t('Please choose the quota source yourself'))
                      return false
                    }
                    const fields: Array<
                      | 'subscription_mode'
                      | 'subscription_id'
                      | 'subscription_wallet_limit_dollars'
                      | 'virtual_membership_id'
                    > = []
                    if (hasSubscriptionStep) {
                      fields.push('subscription_mode')
                    }
                    if (
                      hasSubscriptionStep &&
                      form.getValues('subscription_mode') === 'instance'
                    ) {
                      fields.push('subscription_id')
                      if (form.getValues('subscription_allow_wallet')) {
                        fields.push('subscription_wallet_limit_dollars')
                      }
                    }
                    if (
                      hasVirtualMembershipStep &&
                      form.getValues('virtual_membership_mode') ===
                        'instance' &&
                      form.getValues('virtual_membership_id') <= 0
                    ) {
                      form.setError('virtual_membership_id', {
                        type: 'manual',
                        message: '请选择虚拟会员额度',
                      })
                      toast.error('请选择虚拟会员额度')
                      return false
                    }
                    if (hasVirtualMembershipStep) {
                      fields.push('virtual_membership_id')
                    }
                    return form.trigger(fields)
                  }
                  return true
                }}
                backButtonText={t('Previous')}
                nextButtonText={t('Next')}
                completeButtonText={t('Complete')}
              >
                <Step>
                  <SideDrawerSection>
                    <SideDrawerSectionHeader
                      title={t('Basic Information')}
                      description={t('Set API key basic information')}
                      icon={<KeyRound className='size-4' />}
                    />
                    {basicInfoFields}
                  </SideDrawerSection>
                </Step>
                {hasSourceStep && (
                  <Step>
                    <SideDrawerSection>
                      <SideDrawerSectionHeader
                        title={
                          hasSubscriptionStep
                            ? t('Subscription ownership')
                            : '虚拟会员额度'
                        }
                        description={
                          hasSubscriptionStep
                            ? t(
                                'Choose the instance and what happens after it is exhausted'
                              )
                            : '为会员分组绑定对应的可用额度'
                        }
                        icon={<Link2 className='size-4' />}
                      />
                      {subscriptionFields}
                    </SideDrawerSection>
                  </Step>
                )}
                <Step>
                  <SideDrawerSection>
                    <SideDrawerSectionHeader
                      title={t('Quota Settings')}
                      description={t('Set quota amount and limits')}
                      icon={<WalletCards className='size-4' />}
                    />
                    {quotaFields}
                  </SideDrawerSection>
                </Step>
                <Step>
                  <SideDrawerSection>
                    <SideDrawerSectionHeader
                      title={t('Advanced Settings')}
                      description={t('Set API key access restrictions')}
                      icon={<Settings2 className='size-4' />}
                    />
                    {advancedFields}
                  </SideDrawerSection>
                </Step>
              </Stepper>
            </form>
          )}
        </Form>
        {isUpdate && (
          <DialogFooter>
            <Button
              variant='outline'
              className='w-full sm:w-auto'
              onClick={() => onOpenChange(false)}
            >
              {t('Close')}
            </Button>
            <Button
              type='button'
              onClick={form.handleSubmit(onSubmit, onInvalid)}
              disabled={isSubmitting}
              className='w-full sm:w-auto'
            >
              {isSubmitting ? t('Saving...') : t('Save changes')}
            </Button>
          </DialogFooter>
        )}
      </DialogContent>
      {routingPolicyOpen && (
        <ApiKeyRoutingPolicyDialog
          open={routingPolicyOpen}
          onOpenChange={setRoutingPolicyOpen}
          groups={groups}
          subscribedGroups={subscribedGroups}
          virtualMembershipGroups={virtualMembershipGroups}
          subscriptions={allSubscriptionInstances}
          memberships={allVirtualMemberships}
          initialSteps={routeSteps}
          onSave={(steps) => {
            form.setValue('routing_mode', 'custom', { shouldDirty: true })
            form.setValue('route_steps', steps, { shouldDirty: true })
            form.setValue('group', steps[0]?.group || '', {
              shouldDirty: true,
              shouldValidate: true,
            })
            form.setValue('cross_group_retry', false)
            form.setValue('subscription_mode', 'auto')
            form.setValue('subscription_id', 0)
            form.setValue('virtual_membership_id', 0)
            form.setValue('virtual_membership_mode', 'instance')
          }}
        />
      )}
    </Dialog>
  )
}
