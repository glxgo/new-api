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
import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { History, Save } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { api } from '@/lib/api'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { Dialog } from '@/components/dialog'
import {
  clientRequest,
  type ClientIdentity,
  type ClientPage,
  type ClientPolicy,
} from './client-approval-api'
import { ClientAuditHistory } from './client-approval-panel'

function PolicyEditor(props: {
  policy: ClientPolicy
  clients: ClientIdentity[]
}) {
  const { t } = useTranslation()
  const cache = useQueryClient()
  const [draft, setDraft] = useState(props.policy)
  const [reason, setReason] = useState('')
  const mutation = useMutation({
    mutationFn: () =>
      clientRequest('/groups', { ...draft, reason: reason.trim() }, 'put'),
    onSuccess: () => {
      void cache.invalidateQueries({ queryKey: ['client-policies'] })
      void cache.invalidateQueries({ queryKey: ['client-audit'] })
      setReason('')
      toast.success(t('Saved'))
    },
    onError: () =>
      toast.error(t('Review failed. Refresh the list and try again.')),
  })
  const choices = new Map(
    props.clients.map((client) => [client.client_key, client.display_name])
  )
  draft.supported_clients.forEach((key) => {
    if (!choices.has(key)) choices.set(key, key)
  })
  return (
    <div className='space-y-4'>
      <label className='flex items-center gap-2 text-sm'>
        <Checkbox
          checked={draft.is_coding}
          onCheckedChange={(checked) =>
            setDraft({ ...draft, is_coding: checked })
          }
        />
        {t('Coding group')}
      </label>
      <fieldset
        disabled={!draft.is_coding}
        className='space-y-2 disabled:opacity-50'
      >
        <legend className='mb-2 text-sm font-medium'>
          {t('Supported clients')}
        </legend>
        <div className='grid max-h-64 gap-3 overflow-auto p-1 sm:grid-cols-2'>
          {[...choices].map(([key, name]) => (
            <label key={key} className='flex items-start gap-2 text-sm'>
              <Checkbox
                checked={draft.supported_clients.includes(key)}
                onCheckedChange={(checked) =>
                  setDraft({
                    ...draft,
                    supported_clients: checked
                      ? [...draft.supported_clients, key]
                      : draft.supported_clients.filter((item) => item !== key),
                  })
                }
              />
              <span className='min-w-0 break-words'>
                {t(name)}
                <span className='text-muted-foreground block font-mono text-xs break-all'>
                  {key}
                </span>
              </span>
            </label>
          ))}
        </div>
      </fieldset>
      <label htmlFor='client-policy-reason' className='block text-sm'>
        {t('Reason (required)')}
      </label>
      <Textarea
        id='client-policy-reason'
        maxLength={500}
        value={reason}
        onChange={(e) => setReason(e.target.value)}
      />
      <div className='flex flex-wrap justify-end gap-2'>
        <Dialog
          title={t('Approval history')}
          trigger={
            <Button variant='outline'>
              <History />
              {t('Approval history')}
            </Button>
          }
        >
          <ClientAuditHistory
            path={`/group-reviews?group_name=${encodeURIComponent(draft.group_name)}`}
          />
        </Dialog>
        <Button
          disabled={!reason.trim() || mutation.isPending}
          onClick={() => mutation.mutate()}
        >
          <Save />
          {t('Save group policy')}
        </Button>
      </div>
    </div>
  )
}

export function ClientGroupPolicies() {
  const { t } = useTranslation()
  const [group, setGroup] = useState('')
  const policies = useQuery({
    queryKey: ['client-policies'],
    queryFn: () => clientRequest<ClientPolicy[]>('/groups'),
  })
  const groups = useQuery({
    queryKey: ['client-policy-groups'],
    queryFn: async () => {
      const response = await api.get('/api/group/')
      if (!response.data.success) throw new Error('Request failed')
      return response.data.data as string[]
    },
  })
  const clients = useQuery({
    queryKey: ['client-identities', 'approved-all'],
    queryFn: async () => {
      const items: ClientIdentity[] = []
      for (let page = 1; ; page++) {
        const result = await clientRequest<ClientPage<ClientIdentity>>(
          `?status=approved&p=${page}&page_size=100`
        )
        items.push(...result.items)
        if (items.length >= result.total || result.items.length === 0)
          return items
      }
    },
  })
  if (policies.isPending || groups.isPending || clients.isPending)
    return <p>{t('Loading...')}</p>
  if (policies.error || groups.error || clients.error)
    return <p role='alert'>{t('Failed to load records')}</p>
  const policy = policies.data?.find((item) => item.group_name === group) ?? {
    group_name: group,
    is_coding: group.toLowerCase().includes('coding'),
    supported_clients: [],
    revision: 0,
  }
  return (
    <div className='space-y-4'>
      <Select
        items={(groups.data ?? []).map((name) => ({
          value: name,
          label: name,
        }))}
        value={group}
        onValueChange={(value) => setGroup(value || '')}
      >
        <SelectTrigger aria-label={t('Group')} className='w-full'>
          <SelectValue placeholder={t('Select group')} />
        </SelectTrigger>
        <SelectContent>
          {groups.data?.map((name) => (
            <SelectItem key={name} value={name}>
              {name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      {group && (
        <PolicyEditor
          key={`${group}:${policy.revision}`}
          policy={{
            ...policy,
            supported_clients: policy.supported_clients || [],
          }}
          clients={clients.data ?? []}
        />
      )}
    </div>
  )
}
