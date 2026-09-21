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
import {
  Check,
  ChevronLeft,
  ChevronRight,
  History,
  RotateCcw,
  ShieldCheck,
  X,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatTimestampToDate } from '@/lib/format'
import { Button } from '@/components/ui/button'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { Dialog } from '@/components/dialog'
import {
  clientRequest,
  type ClientAudit,
  type ClientIdentity,
  type ClientPage,
} from './client-approval-api'
import { ClientGroupPolicies } from './client-group-policies'

export function ClientAuditHistory(props: { path: string }) {
  const { t } = useTranslation()
  const [page, setPage] = useState(1)
  const query = useQuery({
    queryKey: ['client-audit', props.path, page],
    queryFn: () =>
      clientRequest<ClientPage<ClientAudit>>(
        `${props.path}&p=${page}&page_size=10`
      ),
  })
  return (
    <div className='space-y-3'>
      {query.isPending && <p>{t('Loading...')}</p>}
      {query.error && <p role='alert'>{t('Failed to load records')}</p>}
      {query.data?.items.map((item) => (
        <div key={item.id} className='space-y-1 border-b pb-3 text-sm'>
          <p>
            {formatTimestampToDate(item.created_at)} · {t('Operator')} #
            {item.operator_id}{' '}
            {item.status && `· ${t(`client.status.${item.status}`)}`}
          </p>
          <p className='break-words whitespace-pre-wrap'>{item.reason}</p>
          {item.policy && (
            <details>
              <summary>{t('Group policy')}</summary>
              <pre className='max-h-48 overflow-auto text-xs break-all whitespace-pre-wrap'>
                {item.previous_policy}
                {'\n'}
                {item.policy}
              </pre>
            </details>
          )}
        </div>
      ))}
      {query.data?.total === 0 && <p>{t('No records')}</p>}
      <ClientPagination
        page={page}
        total={query.data?.total ?? 0}
        pageSize={10}
        onChange={setPage}
      />
    </div>
  )
}

export function ClientPagination(props: {
  page: number
  total: number
  pageSize: number
  onChange: (page: number) => void
}) {
  const { t } = useTranslation()
  return (
    <div className='flex items-center justify-end gap-2 py-2 text-xs'>
      <span>
        {props.page} / {Math.max(1, Math.ceil(props.total / props.pageSize))} ·{' '}
        {props.total}
      </span>
      <Button
        size='icon'
        variant='ghost'
        aria-label={t('Previous page')}
        disabled={props.page <= 1}
        onClick={() => props.onChange(props.page - 1)}
      >
        <ChevronLeft />
      </Button>
      <Button
        size='icon'
        variant='ghost'
        aria-label={t('Next page')}
        disabled={props.page * props.pageSize >= props.total}
        onClick={() => props.onChange(props.page + 1)}
      >
        <ChevronRight />
      </Button>
    </div>
  )
}

export function ClientApprovalPanel() {
  const { t } = useTranslation()
  const cache = useQueryClient()
  const [open, setOpen] = useState(false)
  const [status, setStatus] = useState('pending')
  const [page, setPage] = useState(1)
  const [review, setReview] = useState<{
    client: ClientIdentity
    status: string
  } | null>(null)
  const [reason, setReason] = useState('')
  const query = useQuery({
    queryKey: ['client-identities', status, page],
    queryFn: () =>
      clientRequest<ClientPage<ClientIdentity>>(
        `?status=${status}&p=${page}&page_size=20`
      ),
    enabled: open && status !== 'groups',
  })
  const mutation = useMutation({
    mutationFn: () =>
      clientRequest('/review', {
        client_key: review?.client.client_key,
        revision: review?.client.revision,
        status: review?.status,
        reason: reason.trim(),
      }),
    onSuccess: () => {
      setReview(null)
      setReason('')
      void cache.invalidateQueries({ queryKey: ['client-identities'] })
      void cache.invalidateQueries({ queryKey: ['client-audit'] })
      toast.success(t('Saved'))
    },
    onError: () =>
      toast.error(t('Review failed. Refresh the list and try again.')),
  })
  return (
    <>
      <Dialog
        open={open}
        onOpenChange={setOpen}
        title={t('Coding client allowlist')}
        contentClassName='sm:max-w-4xl'
        description={t(
          'UA can be spoofed. Classification and admission do not verify official identity or guarantee cache hit rates.'
        )}
        trigger={
          <Button size='sm' variant='outline'>
            <ShieldCheck className='size-3.5' />
            {t('Client approvals')}
          </Button>
        }
      >
        <Tabs
          value={status}
          onValueChange={(value) => {
            setStatus(value)
            setPage(1)
          }}
        >
          <TabsList className='mb-4 flex h-auto flex-wrap'>
            {['pending', 'approved', 'rejected', 'groups'].map((item) => (
              <TabsTrigger key={item} value={item}>
                {t(`client.status.${item}`)}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>
        {status === 'groups' ? (
          <ClientGroupPolicies />
        ) : (
          <>
            {query.isPending && <p>{t('Loading...')}</p>}
            {query.error && (
              <div role='alert'>
                {t('Failed to load records')}
                <Button variant='ghost' onClick={() => void query.refetch()}>
                  <RotateCcw />
                  {t('Refresh')}
                </Button>
              </div>
            )}
            {query.data?.total === 0 && (
              <p className='text-muted-foreground py-8 text-center'>
                {t('No records')}
              </p>
            )}
            <div className='divide-y'>
              {query.data?.items.map((client) => (
                <div key={client.client_key} className='min-w-0 space-y-2 py-3'>
                  <div className='flex flex-wrap items-center justify-between gap-2'>
                    <div className='min-w-0'>
                      <p className='text-sm font-medium'>
                        {t(client.display_name)}
                      </p>
                      <p className='text-muted-foreground font-mono text-xs break-all'>
                        {client.client_key}
                      </p>
                    </div>
                    <div className='flex gap-1'>
                      {[
                        {
                          status: 'approved',
                          label: 'Approve client',
                          Icon: Check,
                        },
                        { status: 'rejected', label: 'Reject client', Icon: X },
                        {
                          status: 'pending',
                          label: 'Revoke approval',
                          Icon: RotateCcw,
                        },
                      ]
                        .filter((action) => action.status !== client.status)
                        .map((action) => (
                          <Tooltip key={action.status}>
                            <TooltipTrigger
                              render={
                                <Button
                                  size='icon'
                                  variant='ghost'
                                  disabled={
                                    client.family === 'unknown' &&
                                    action.status === 'approved'
                                  }
                                  aria-label={t(action.label)}
                                  onClick={() => {
                                    setReview({ client, status: action.status })
                                    setReason('')
                                  }}
                                />
                              }
                            >
                              <action.Icon />
                            </TooltipTrigger>
                            <TooltipContent>{t(action.label)}</TooltipContent>
                          </Tooltip>
                        ))}
                      <Dialog
                        title={t('Approval history')}
                        trigger={
                          <Button
                            size='icon'
                            variant='ghost'
                            aria-label={t('Approval history')}
                          >
                            <History />
                          </Button>
                        }
                      >
                        <ClientAuditHistory
                          path={`/reviews?client_key=${encodeURIComponent(client.client_key)}`}
                        />
                      </Dialog>
                    </div>
                  </div>
                  <p className='text-muted-foreground text-xs'>
                    {t('Requests')}: {client.request_count} · {t('Last seen')}:{' '}
                    {formatTimestampToDate(client.last_seen)}
                  </p>
                  <pre
                    className='bg-muted max-h-24 overflow-auto rounded p-2 text-xs break-all whitespace-pre-wrap'
                    dir='ltr'
                  >
                    {client.user_agent || t('Empty User-Agent')}
                  </pre>
                  {client.truncated && (
                    <p className='text-muted-foreground text-xs'>
                      {t(
                        'User-Agent truncated to 2048 bytes; control characters removed.'
                      )}
                    </p>
                  )}
                </div>
              ))}
            </div>
            <ClientPagination
              page={page}
              pageSize={20}
              total={query.data?.total ?? 0}
              onChange={setPage}
            />
          </>
        )}
      </Dialog>
      <Dialog
        open={!!review}
        onOpenChange={(value) => {
          if (!value && !mutation.isPending) setReview(null)
        }}
        title={t('Review client')}
        description={
          review
            ? `${t(review.client.display_name)} · ${t(`client.status.${review.status}`)}`
            : undefined
        }
        footer={
          <Button
            disabled={!reason.trim() || mutation.isPending}
            onClick={() => mutation.mutate()}
          >
            <Check />
            {t('Save review')}
          </Button>
        }
      >
        <label htmlFor='client-review-reason' className='mb-2 block text-sm'>
          {t('Reason (required)')}
        </label>
        <Textarea
          id='client-review-reason'
          maxLength={500}
          value={reason}
          onChange={(e) => setReason(e.target.value)}
          rows={4}
        />
      </Dialog>
    </>
  )
}
