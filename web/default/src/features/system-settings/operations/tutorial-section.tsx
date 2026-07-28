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
import { useEffect, useRef } from 'react'
import * as z from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import MDEditor from '@uiw/react-md-editor'
import { useTranslation } from 'react-i18next'
import { api } from '@/lib/api'
import { Form, FormMessage } from '@/components/ui/form'
import { SettingsForm } from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

const tutorialSchema = z.object({
  content: z.string().optional(),
})

type TutorialFormValues = z.infer<typeof tutorialSchema>

type TutorialSectionProps = {
  defaultValue: string
}

// TutorialSection 使用教程后台编辑(Markdown 编辑器 + 图片上传)。
// 教程内容存 option key "tutorial_setting.content", 前台 /tutorial 用 Markdown 渲染。
export function TutorialSection({ defaultValue }: TutorialSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const fileInputRef = useRef<HTMLInputElement>(null)
  const form = useForm<TutorialFormValues>({
    resolver: zodResolver(tutorialSchema),
    defaultValues: { content: defaultValue ?? '' },
  })

  useEffect(() => {
    form.reset({ content: defaultValue ?? '' })
  }, [defaultValue, form])

  const onSubmit = async (values: TutorialFormValues) => {
    const normalized = values.content ?? ''
    if (normalized === (defaultValue ?? '')) return
    await updateOption.mutateAsync({
      key: 'tutorial_setting.content',
      value: normalized,
    })
  }

  const handleImageUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    const formData = new FormData()
    formData.append('file', file)
    try {
      const res = await api.post('/api/option/upload', formData, {
        headers: { 'Content-Type': 'multipart/form-data' },
      })
      const url = (res.data as { data?: { url?: string } })?.data?.url
      if (url) {
        const current = form.getValues('content') ?? ''
        form.setValue('content', `${current}\n\n![${file.name}](${url})\n`, {
          shouldDirty: true,
        })
      }
    } catch {
      // 上传失败静默(响应拦截器会 toast)
    }
    if (fileInputRef.current) fileInputRef.current.value = ''
  }

  const content = form.watch('content') ?? ''

  return (
    <SettingsSection title={t('Usage Tutorial')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
            saveLabel='Save tutorial'
          />
          <div className='space-y-2'>
            <div className='flex items-center justify-between'>
              <span className='text-sm font-medium'>
                {t('Tutorial content (Markdown, supports images)')}
              </span>
              <div>
                <input
                  ref={fileInputRef}
                  type='file'
                  accept='image/*'
                  className='hidden'
                  onChange={handleImageUpload}
                />
                <button
                  type='button'
                  onClick={() => fileInputRef.current?.click()}
                  className='text-primary text-sm hover:underline'
                >
                  {t('Insert image')}
                </button>
              </div>
            </div>
            <MDEditor
              value={content}
              onChange={(val) =>
                form.setValue('content', val ?? '', { shouldDirty: true })
              }
              height={520}
            />
            <FormMessage />
          </div>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
