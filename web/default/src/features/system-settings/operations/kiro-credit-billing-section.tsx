/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { useEffect, useMemo } from 'react'
import * as z from 'zod'
import { useFieldArray, useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { AlertTriangle, Plus, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { SettingsForm } from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

const OPTION_KEY = 'KiroCreditBilling'
const fixedPoint = /^[0-9]{1,18}(\.[0-9]{1,9})?$/

const channelCostSchema = z.object({
  channelId: z.string(),
  cost: z.string(),
})

const kiroSchema = z
  .object({
    mode: z.enum(['off', 'shadow', 'charge']),
    groups: z.string(),
    pricePerCredit: z.string(),
    costPerCredit: z.string(),
    quotaPerCurrency: z.string(),
    reserveCredits: z.string(),
    trustedChannels: z.string(),
    channelCosts: z.array(channelCostSchema),
  })
  .superRefine((values, ctx) => {
    if (values.mode === 'off') return

    if (
      !values.groups
        .split(/[\n,]/)
        .map((item) => item.trim())
        .filter(Boolean).length
    ) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ['groups'],
        message: 'Add at least one service group.',
      })
    }
    if (listFromText(values.groups).includes('auto')) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ['groups'],
        message:
          'Names must exactly match the channel service group. The auto group is not supported.',
      })
    }
    const trustedChannels = listFromText(values.trustedChannels)
    if (!trustedChannels.length) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ['trustedChannels'],
        message: 'Add at least one trusted channel ID.',
      })
    } else if (
      trustedChannels.some(
        (item) =>
          !/^\d+$/.test(item) ||
          !Number.isSafeInteger(Number(item)) ||
          Number(item) <= 0
      )
    ) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ['trustedChannels'],
        message: 'Channel IDs must be positive integers.',
      })
    }

    const positiveFields: Array<[keyof typeof values, string]> = [
      ['pricePerCredit', 'Retail price must be greater than zero.'],
      ['reserveCredits', 'Reserve credits must be greater than zero.'],
      ['quotaPerCurrency', 'Quota conversion must be a positive integer.'],
    ]
    for (const [field, message] of positiveFields) {
      const number = Number(values[field])
      const valid =
        Number.isFinite(number) &&
        number > 0 &&
        (field === 'quotaPerCurrency'
          ? Number.isSafeInteger(number)
          : fixedPoint.test(String(values[field]).trim()))
      if (!valid)
        ctx.addIssue({ code: z.ZodIssueCode.custom, path: [field], message })
    }

    const cost = Number(values.costPerCredit)
    if (
      !fixedPoint.test(values.costPerCredit.trim()) ||
      !Number.isFinite(cost) ||
      cost < 0
    ) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ['costPerCredit'],
        message: 'Internal cost must be zero or greater.',
      })
    }

    const seenChannels = new Set<number>()
    for (const [index, row] of values.channelCosts.entries()) {
      if (!row.channelId.trim() && !row.cost.trim()) continue
      if (!row.channelId.trim() || !row.cost.trim()) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          path: [
            'channelCosts',
            index,
            row.channelId.trim() ? 'cost' : 'channelId',
          ],
          message: 'Enter both a channel ID and a cost.',
        })
        continue
      }
      const id = Number(row.channelId)
      const rowCost = Number(row.cost)
      if (
        !/^\d+$/.test(row.channelId.trim()) ||
        !Number.isSafeInteger(id) ||
        id <= 0
      ) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['channelCosts', index, 'channelId'],
          message: 'Enter a positive channel ID.',
        })
      } else if (seenChannels.has(id)) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['channelCosts', index, 'channelId'],
          message: 'Channel IDs must be unique.',
        })
      } else {
        seenChannels.add(id)
      }
      if (
        !fixedPoint.test(row.cost.trim()) ||
        !Number.isFinite(rowCost) ||
        rowCost < 0
      ) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['channelCosts', index, 'cost'],
          message: 'Cost must be zero or greater.',
        })
      }
    }
  })

type KiroFormValues = z.infer<typeof kiroSchema>
type KiroSettings = { KiroCreditBilling: string; GroupRatio: string }

function listFromText(value: string) {
  return Array.from(
    new Set(
      value
        .split(/[\n,]/)
        .map((item) => item.trim())
        .filter(Boolean)
    )
  )
}

function idsFromText(value: string) {
  return Array.from(new Set(listFromText(value).map((item) => Number(item))))
}

function parseDefaults(value: string): KiroFormValues {
  const fallback: KiroFormValues = {
    mode: 'off',
    groups: '',
    pricePerCredit: '0.11',
    costPerCredit: '0',
    quotaPerCurrency: '500000',
    reserveCredits: '10',
    trustedChannels: '',
    channelCosts: [],
  }

  try {
    const raw = JSON.parse(value || '{}') as Record<string, unknown>
    const costs =
      raw.channel_cost_per_credit &&
      typeof raw.channel_cost_per_credit === 'object'
        ? Object.entries(
            raw.channel_cost_per_credit as Record<string, unknown>
          ).map(([channelId, cost]) => ({ channelId, cost: String(cost) }))
        : []
    return {
      mode: raw.mode === 'shadow' || raw.mode === 'charge' ? raw.mode : 'off',
      groups: Array.isArray(raw.groups)
        ? raw.groups.map(String).join('\n')
        : '',
      pricePerCredit: String(raw.price_per_credit ?? fallback.pricePerCredit),
      costPerCredit: String(raw.cost_per_credit ?? fallback.costPerCredit),
      quotaPerCurrency: String(
        raw.quota_per_currency ?? fallback.quotaPerCurrency
      ),
      reserveCredits: String(raw.reserve_credits ?? fallback.reserveCredits),
      trustedChannels: Array.isArray(raw.trusted_channels)
        ? raw.trusted_channels.map(String).join(', ')
        : '',
      channelCosts: costs,
    }
  } catch {
    return fallback
  }
}

function hasMissingGroupRatios(groups: string, groupRatio: string) {
  const selected = listFromText(groups)
  if (!selected.length || !groupRatio.trim()) return false
  try {
    const configured = JSON.parse(groupRatio) as Record<string, unknown>
    return selected.some(
      (group) => !Object.prototype.hasOwnProperty.call(configured, group)
    )
  } catch {
    return false
  }
}

export function KiroCreditBillingSection({
  defaultValues,
}: {
  defaultValues: KiroSettings
}) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const initialValues = useMemo(
    () => parseDefaults(defaultValues.KiroCreditBilling),
    [defaultValues.KiroCreditBilling]
  )
  const form = useForm<KiroFormValues>({
    resolver: zodResolver(kiroSchema),
    defaultValues: initialValues,
  })
  const { fields, append, remove } = useFieldArray({
    control: form.control,
    name: 'channelCosts',
  })
  const mode = form.watch('mode')
  const groups = form.watch('groups')

  useEffect(() => {
    form.reset(initialValues)
  }, [form, initialValues])

  const onSubmit = async (values: KiroFormValues) => {
    const config =
      values.mode === 'off'
        ? {
            ...JSON.parse(defaultValues.KiroCreditBilling || '{}'),
            mode: 'off',
          }
        : {
            mode: values.mode,
            groups: listFromText(values.groups),
            price_per_credit: values.pricePerCredit.trim(),
            cost_per_credit: values.costPerCredit.trim(),
            quota_per_currency: Number(values.quotaPerCurrency),
            reserve_credits: values.reserveCredits.trim(),
            trusted_channels: idsFromText(values.trustedChannels),
            channel_cost_per_credit: Object.fromEntries(
              values.channelCosts
                .filter((row) => row.channelId.trim() || row.cost.trim())
                .map((row) => [String(Number(row.channelId)), row.cost.trim()])
            ),
          }
    updateOption.mutate(
      {
        key: OPTION_KEY,
        value: JSON.stringify(config),
      },
      {
        onSuccess: (result) => {
          if (result.success) form.reset(values)
        },
      }
    )
  }

  return (
    <SettingsSection title={t('Kiro Credit Billing')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            onReset={() => form.reset(initialValues)}
            isSaving={updateOption.isPending}
            isSaveDisabled={!form.formState.isDirty}
          />

          {mode === 'charge' ? (
            <Alert variant='destructive'>
              <AlertTriangle />
              <AlertTitle>{t('Charge mode is live billing')}</AlertTitle>
              <AlertDescription>
                {t(
                  'Every matching Kiro request will settle against the user balance. Verify shadow mode first.'
                )}
              </AlertDescription>
            </Alert>
          ) : null}
          {mode === 'shadow' ? (
            <Alert>
              <AlertTitle>{t('Shadow mode')}</AlertTitle>
              <AlertDescription>
                {t(
                  'Shadow records Kiro credits for comparison. Existing request billing still applies; no additional credit charge is made.'
                )}
              </AlertDescription>
            </Alert>
          ) : null}

          <FormField
            control={form.control}
            name='mode'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Billing mode')}</FormLabel>
                <Select value={field.value} onValueChange={field.onChange}>
                  <FormControl>
                    <SelectTrigger>
                      <SelectValue>
                        {t(
                          { off: 'Off', shadow: 'Shadow', charge: 'Charge' }[
                            field.value
                          ]
                        )}
                      </SelectValue>
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent>
                    <SelectItem value='off'>{t('Off')}</SelectItem>
                    <SelectItem value='shadow'>{t('Shadow')}</SelectItem>
                    <SelectItem value='charge'>{t('Charge')}</SelectItem>
                  </SelectContent>
                </Select>
                <FormDescription>
                  {t(
                    'Off disables Kiro billing. Shadow records evidence only; Charge settles credits.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='pricePerCredit'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Retail price per credit (CNY)')}</FormLabel>
                <FormControl>
                  <Input
                    inputMode='decimal'
                    {...field}
                    disabled={mode === 'off'}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='costPerCredit'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Default cost per credit (CNY)')}</FormLabel>
                <FormControl>
                  <Input
                    inputMode='decimal'
                    {...field}
                    disabled={mode === 'off'}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Used for internal cost records; channel overrides can be added below.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='quotaPerCurrency'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Quota units per CNY')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min='1'
                    step='1'
                    {...field}
                    disabled={mode === 'off'}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'For example, 500000 means 1 CNY reserves 500000 quota units.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='reserveCredits'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Reserve credits per request')}</FormLabel>
                <FormControl>
                  <Input
                    inputMode='decimal'
                    {...field}
                    disabled={mode === 'off'}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Credits reserved before the upstream response is complete.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='groups'
            render={({ field }) => (
              <FormItem data-settings-form-span='full'>
                <FormLabel>{t('Service groups')}</FormLabel>
                <FormControl>
                  <Textarea
                    rows={3}
                    placeholder={t('One exact group name per line')}
                    {...field}
                    disabled={mode === 'off'}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Names must exactly match the channel service group. The auto group is not supported.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
          {hasMissingGroupRatios(groups, defaultValues.GroupRatio) ? (
            <Alert>
              <AlertTitle>{t('Check group ratios')}</AlertTitle>
              <AlertDescription>
                {t(
                  'At least one selected group has no exact GroupRatio entry. Its effective ratio will fall back to 1.'
                )}
              </AlertDescription>
            </Alert>
          ) : null}
          <FormField
            control={form.control}
            name='trustedChannels'
            render={({ field }) => (
              <FormItem data-settings-form-span='full'>
                <FormLabel>{t('Trusted channel IDs')}</FormLabel>
                <FormControl>
                  <Textarea
                    rows={2}
                    placeholder={t('For example: 19, 89')}
                    {...field}
                    disabled={mode === 'off'}
                  />
                </FormControl>
                <FormDescription>
                  {t('Only these channel IDs may use shadow or charge mode.')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <div data-settings-form-span='full' className='space-y-3'>
            <div>
              <h4 className='text-sm font-medium'>
                {t('Channel cost overrides')}
              </h4>
              <p className='text-muted-foreground text-xs'>
                {t(
                  'Optional procurement cost by channel. Leave empty when the default cost applies.'
                )}
              </p>
            </div>
            {fields.map((item, index) => (
              <div
                key={item.id}
                className='grid gap-3 sm:grid-cols-[1fr_1fr_auto]'
              >
                <FormField
                  control={form.control}
                  name={`channelCosts.${index}.channelId`}
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Channel ID')}</FormLabel>
                      <FormControl>
                        <Input {...field} disabled={mode === 'off'} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name={`channelCosts.${index}.cost`}
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Cost per credit (CNY)')}</FormLabel>
                      <FormControl>
                        <Input
                          inputMode='decimal'
                          {...field}
                          disabled={mode === 'off'}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <Button
                  type='button'
                  variant='ghost'
                  size='icon'
                  className='self-end'
                  onClick={() => remove(index)}
                  disabled={mode === 'off'}
                  aria-label={t('Remove channel cost')}
                  title={t('Remove channel cost')}
                >
                  <Trash2 />
                </Button>
              </div>
            ))}
            <Button
              type='button'
              variant='outline'
              size='sm'
              onClick={() => append({ channelId: '', cost: '' })}
              disabled={mode === 'off'}
            >
              <Plus data-icon='inline-start' />
              {t('Add channel override')}
            </Button>
          </div>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
