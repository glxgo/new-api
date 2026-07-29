/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { useEffect, useState } from 'react'
import { History } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Dialog } from '@/components/dialog'
import { getApiKeySubscriptionHistory } from '../../api'
import type { ApiKey, ApiKeySubscriptionHistory } from '../../types'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  apiKey: ApiKey
}

const ACTION_LABELS: Record<string, string> = {
  bind: 'Bound',
  rebind: 'Rebound',
  unbind: 'Returned to automatic allocation',
  auto_renew: 'Continued to renewal successor',
  auto_same_group: 'Continued to same-group instance',
  renewal_scheduled: 'Renewal scheduled',
  renewal_activated: 'Renewal activated',
  renewal_cancelled: 'Scheduled renewal cancelled',
  group_changed: 'Group changed',
}

export function ApiKeySubscriptionHistoryDialog({
  open,
  onOpenChange,
  apiKey,
}: Props) {
  const { t } = useTranslation()
  const [loading, setLoading] = useState(false)
  const [items, setItems] = useState<ApiKeySubscriptionHistory[]>([])

  useEffect(() => {
    if (!open) return
    let active = true
    void Promise.resolve().then(async () => {
      if (!active) return
      setLoading(true)
      try {
        const res = await getApiKeySubscriptionHistory(apiKey.id)
        if (!active) return
        if (!res.success) {
          toast.error(res.message || t('Request failed'))
          return
        }
        setItems(res.data || [])
      } catch {
        toast.error(t('Request failed'))
      } finally {
        if (active) setLoading(false)
      }
    })
    return () => {
      active = false
    }
  }, [apiKey.id, open, t])

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={
        <span className='flex items-center gap-2'>
          <History className='h-5 w-5' />
          {t('Subscription ownership history')}
        </span>
      }
      description={`${apiKey.name} · #${apiKey.id}`}
      contentClassName='sm:max-w-2xl'
      contentHeight='min(65vh, 620px)'
    >
      <div className='space-y-3'>
        {loading ? (
          <div className='text-muted-foreground p-8 text-center text-sm'>
            {t('Loading...')}
          </div>
        ) : items.length === 0 ? (
          <div className='rounded-lg border border-dashed p-8 text-center'>
            <div className='text-sm font-medium'>{t('No history yet')}</div>
            <div className='text-muted-foreground mt-1 text-xs'>
              {t(
                'Existing API Keys keep their prior automatic behavior until you change them.'
              )}
            </div>
          </div>
        ) : (
          items.map((item) => (
            <div key={item.id} className='relative rounded-lg border p-3'>
              <div className='flex flex-wrap items-center justify-between gap-2'>
                <span className='text-sm font-medium'>
                  {t(ACTION_LABELS[item.action] || item.action)}
                </span>
                <span className='text-muted-foreground text-xs'>
                  {new Date(item.created_at * 1000).toLocaleString()}
                </span>
              </div>
              <div className='text-muted-foreground mt-2 grid gap-1 text-xs sm:grid-cols-2'>
                <div>
                  {t('Instance')}:{' '}
                  {item.from_subscription_id > 0
                    ? `#${item.from_subscription_id}`
                    : t('Automatic')}
                  {' → '}
                  {item.to_subscription_id > 0
                    ? `#${item.to_subscription_id}`
                    : t('Automatic')}
                </div>
                <div>
                  {t('Group')}: {item.from_group || '—'} →{' '}
                  {item.to_group || '—'}
                </div>
                <div className='sm:col-span-2'>
                  {t('Continuation')}: {item.continuation_summary || '—'}
                </div>
                {item.reason && (
                  <div className='sm:col-span-2'>
                    {t('Reason')}: {item.reason}
                  </div>
                )}
              </div>
            </div>
          ))
        )}
      </div>
    </Dialog>
  )
}
