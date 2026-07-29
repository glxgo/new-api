/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Dialog } from '@/components/dialog'
import { updateSubscriptionRemark } from '../../api'
import type { UserSubscription } from '../../types'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  subscription: UserSubscription | null
  onSaved?: () => void | Promise<void>
}

export function SubscriptionInstanceRemarkDialog({
  open,
  onOpenChange,
  subscription,
  onSaved,
}: Props) {
  const { t } = useTranslation()
  const [remark, setRemark] = useState('')
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    let active = true
    if (open) {
      void Promise.resolve().then(() => {
        if (active) setRemark(subscription?.remark || '')
      })
    }
    return () => {
      active = false
    }
  }, [open, subscription?.remark])

  const save = async () => {
    if (!subscription?.id) return
    setSaving(true)
    try {
      const res = await updateSubscriptionRemark(subscription.id, remark)
      if (!res.success) {
        toast.error(res.message || t('Update failed'))
        return
      }
      toast.success(t('Remark updated'))
      await onSaved?.()
      onOpenChange(false)
    } catch {
      toast.error(t('Request failed'))
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={t('Subscription instance remark')}
      description={t(
        'A private note helps you distinguish multiple instances of the same plan when binding API Keys.'
      )}
      contentClassName='sm:max-w-md'
      footer={
        <>
          <Button variant='outline' onClick={() => onOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button onClick={save} disabled={saving}>
            {t('Save')}
          </Button>
        </>
      }
    >
      <div className='space-y-2'>
        <Input
          value={remark}
          maxLength={128}
          autoFocus
          placeholder={t('e.g. Project A / Finance team')}
          onChange={(event) => setRemark(event.target.value)}
        />
        <div className='text-muted-foreground text-right text-xs'>
          {Array.from(remark).length}/128
        </div>
      </div>
    </Dialog>
  )
}
