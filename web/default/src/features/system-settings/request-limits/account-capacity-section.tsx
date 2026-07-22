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
import type { ChangeEvent } from 'react'
import * as z from 'zod'
import type { Resolver } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Alert, AlertDescription } from '@/components/ui/alert'
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
import { FormDirtyIndicator } from '../components/form-dirty-indicator'
import { FormNavigationGuard } from '../components/form-navigation-guard'
import {
  SettingsForm,
  SettingsFormGrid,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useSettingsForm } from '../hooks/use-settings-form'
import { useUpdateOption } from '../hooks/use-update-option'

const accountCapacitySchema = z.object({
  DefaultUserConcurrencyLimit: z.coerce.number().int().min(1).max(10000),
  DefaultUserRPMLimit: z.coerce.number().int().min(1).max(10000000),
})

export type AccountCapacityFormValues = z.infer<typeof accountCapacitySchema>

type AccountCapacitySectionProps = {
  defaultValues: AccountCapacityFormValues
}

export function AccountCapacitySection({
  defaultValues,
}: AccountCapacitySectionProps) {
  const updateOption = useUpdateOption()
  const handleNumberChange =
    (onChange: (value: number | string) => void) =>
    (event: ChangeEvent<HTMLInputElement>) => {
      onChange(
        event.target.value === '' ? '' : event.currentTarget.valueAsNumber
      )
    }

  const { form, handleSubmit, isDirty, isSubmitting } =
    useSettingsForm<AccountCapacityFormValues>({
      resolver: zodResolver(accountCapacitySchema) as Resolver<
        AccountCapacityFormValues,
        unknown,
        AccountCapacityFormValues
      >,
      defaultValues,
      onSubmit: async (_data, changedFields) => {
        for (const [key, value] of Object.entries(changedFields)) {
          await updateOption.mutateAsync({
            key,
            value: value as string | number | boolean,
          })
        }
      },
    })

  return (
    <SettingsSection title='账号默认容量'>
      <FormNavigationGuard when={isDirty} />
      <Alert>
        <AlertDescription>
          这里修改的是继承默认值的账号。已经单独设置过并发或 RPM
          的账号，会分别保留对应的独立上限。
        </AlertDescription>
      </Alert>

      <Form {...form}>
        <SettingsForm onSubmit={handleSubmit}>
          <SettingsPageFormActions
            onSave={handleSubmit}
            isSaving={updateOption.isPending || isSubmitting}
          />
          <FormDirtyIndicator isDirty={isDirty} />
          <SettingsFormGrid>
            <FormField
              control={form.control}
              name='DefaultUserConcurrencyLimit'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>默认并发上限</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={1}
                      max={10000}
                      value={field.value ?? ''}
                      onChange={handleNumberChange(field.onChange)}
                      name={field.name}
                      onBlur={field.onBlur}
                      ref={field.ref}
                    />
                  </FormControl>
                  <FormDescription>
                    同一账号同时执行的 API 请求数量，默认 8。
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='DefaultUserRPMLimit'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>默认 RPM 上限</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={1}
                      max={10000000}
                      value={field.value ?? ''}
                      onChange={handleNumberChange(field.onChange)}
                      name={field.name}
                      onBlur={field.onBlur}
                      ref={field.ref}
                    />
                  </FormControl>
                  <FormDescription>
                    账号最近滚动 60 秒允许的请求数量，默认
                    12；不再根据并发自动计算。
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </SettingsFormGrid>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
