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
import { z } from 'zod'
import { useForm, type Resolver } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
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
import { Switch } from '@/components/ui/switch'
import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

// Rates are stored as decimals (0.10 = 10%). Bounds 0..1.
const schema = z.object({
  directRate: z.coerce.number().min(0).max(1),
  indirectRate: z.coerce.number().min(0).max(1),
  rootDividendRate: z.coerce.number().min(0).max(1),
  adminIndirectRate: z.coerce.number().min(0).max(1),
  settleEnabled: z.boolean(),
  settleHour: z.coerce.number().int().min(0).max(23),
})
type Values = z.infer<typeof schema>

interface DividendSettingsDefaultValues {
  directRate: number
  indirectRate: number
  rootDividendRate: number
  adminIndirectRate: number
  settleEnabled: boolean
  settleHour: number
}

export function DividendSettingsSection({
  defaultValues,
}: {
  defaultValues: DividendSettingsDefaultValues
}) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const form = useForm<Values>({
    resolver: zodResolver(schema) as unknown as Resolver<Values>,
    defaultValues,
  })
  const { isDirty, isSubmitting } = form.formState
  const settleEnabled = form.watch('settleEnabled')

  async function onSubmit(values: Values) {
    const updates: Array<{ key: string; value: string }> = []
    if (values.directRate !== defaultValues.directRate)
      updates.push({
        key: 'AffiliateDirectRate',
        value: String(values.directRate),
      })
    if (values.indirectRate !== defaultValues.indirectRate)
      updates.push({
        key: 'AffiliateIndirectRate',
        value: String(values.indirectRate),
      })
    if (values.rootDividendRate !== defaultValues.rootDividendRate)
      updates.push({
        key: 'RootDividendRate',
        value: String(values.rootDividendRate),
      })
    if (values.adminIndirectRate !== defaultValues.adminIndirectRate)
      updates.push({
        key: 'AffiliateAdminIndirectRate',
        value: String(values.adminIndirectRate),
      })
    if (values.settleEnabled !== defaultValues.settleEnabled)
      updates.push({
        key: 'affiliate_settle_setting.enabled',
        value: String(values.settleEnabled),
      })
    if (values.settleHour !== defaultValues.settleHour)
      updates.push({
        key: 'affiliate_settle_setting.settle_hour',
        value: String(values.settleHour),
      })

    if (updates.length === 0) {
      toast.info(t('No changes to save'))
      return
    }
    for (const update of updates) {
      await updateOption.mutateAsync(update)
    }
    form.reset(values)
  }

  return (
    <SettingsSection title={t('Dividend & Rebate Settings')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)} autoComplete='off'>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending || isSubmitting}
            isSaveDisabled={!isDirty}
            saveLabel='Save dividend settings'
          />

          <div className='grid gap-6 sm:grid-cols-2 lg:grid-cols-4'>
            <FormField
              control={form.control}
              name='directRate'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Direct Referral Rate')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      step='0.01'
                      min={0}
                      max={1}
                      placeholder='0.10'
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Share of gross profit to the direct referrer (0.10 = 10%)'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='indirectRate'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Indirect Referral Rate')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      step='0.01'
                      min={0}
                      max={1}
                      placeholder='0.05'
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Share of gross profit to the indirect referrer (0.05 = 5%)'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='rootDividendRate'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Root Dividend Rate')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      step='0.01'
                      min={0}
                      max={1}
                      placeholder='0.10'
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Share of all gross profit to the super admin (0.10 = 10%)'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='adminIndirectRate'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Admin Indirect Rate')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      step='0.01'
                      min={0}
                      max={1}
                      placeholder='0.22'
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Admin dividend for indirect/3rd-tier+ invitees (0.22 = 22%); direct invitees still use the admin personal rate'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <FormField
            control={form.control}
            name='settleEnabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable T+1 Settlement')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Run the daily profit settlement task. Enable only after configuring model costs and rates.'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                    disabled={updateOption.isPending || isSubmitting}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          {settleEnabled && (
            <FormField
              control={form.control}
              name='settleHour'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Settlement Hour')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={0}
                      max={23}
                      placeholder='2'
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      "Hour of day (0-23, local time) to settle the previous day's profit"
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          )}
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
