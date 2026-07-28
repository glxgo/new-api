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
import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Markdown } from '@/components/ui/markdown'
import { useUpdateOption } from '@/features/system-settings/hooks/use-update-option'
import { getSubscriptionIntro } from '../api'

interface SubscriptionIntroDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

// 编辑套餐订阅页顶部全局介绍(富文本 markdown), 存于系统选项 SubscriptionPlansIntro
export function SubscriptionIntroDialog({
  open,
  onOpenChange,
}: SubscriptionIntroDialogProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [value, setValue] = useState('')
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    if (!open) return
    setLoading(true)
    getSubscriptionIntro()
      .then((res) => {
        if (res.success && res.data) {
          setValue(res.data.intro || '')
        }
      })
      .finally(() => setLoading(false))
  }, [open])

  const handleSave = async () => {
    await updateOption.mutateAsync({
      key: 'SubscriptionPlansIntro',
      value,
    })
    onOpenChange(false)
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-w-2xl'>
        <DialogHeader>
          <DialogTitle>{t('Subscription Plans Intro')}</DialogTitle>
          <DialogDescription>
            {t(
              'Global intro shown at the top of the subscription plans page. Supports Markdown.'
            )}
          </DialogDescription>
        </DialogHeader>
        <div className='space-y-3'>
          <textarea
            value={value}
            onChange={(e) => setValue(e.target.value)}
            rows={8}
            disabled={loading}
            placeholder={t(
              'Welcome to our subscription plans! **Bold**, <span style="color:#e11d48">colored</span>, [link](url)'
            )}
            className='border-input bg-background focus-visible:ring-ring placeholder:text-muted-foreground flex min-h-[160px] w-full rounded-md border px-3 py-2 text-sm focus-visible:ring-2 focus-visible:outline-none disabled:opacity-50'
          />
          {value ? (
            <div className='bg-muted/30 rounded-md border p-3'>
              <div className='text-muted-foreground mb-1 text-xs'>
                {t('Preview')}
              </div>
              <div className='text-sm'>
                <Markdown>{value}</Markdown>
              </div>
            </div>
          ) : null}
        </div>
        <DialogFooter>
          <Button variant='ghost' onClick={() => onOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button
            onClick={handleSave}
            disabled={updateOption.isPending || loading}
          >
            {updateOption.isPending ? t('Saving...') : t('Save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
