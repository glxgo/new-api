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
import { useState, useCallback, useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Check, ChevronDown, Copy, Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { getUserGroups } from '@/lib/api'
import { copyToClipboard } from '@/lib/copy-to-clipboard'
import { cn } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from '@/components/ui/command'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { BadgeCell } from '@/components/data-table'
import { StatusBadge } from '@/components/status-badge'
import { getSelfSubscriptionFull } from '@/features/subscriptions/api'
import { getApiKey, updateApiKey } from '../api'
import { ERROR_MESSAGES, SUCCESS_MESSAGES } from '../constants'
import {
  transformApiKeyToFormDefaults,
  transformFormDataToPayload,
} from '../lib'
import { type ApiKey } from '../types'
import { useApiKeys } from './api-keys-provider'

export function ApiKeyCell({ apiKey }: { apiKey: ApiKey }) {
  const { t } = useTranslation()
  const {
    resolveRealKey,
    resolvedKeys,
    loadingKeys,
    copiedKeyId,
    markKeyCopied,
  } = useApiKeys()
  const [popoverOpen, setPopoverOpen] = useState(false)

  const isLoading = !!loadingKeys[apiKey.id]
  const resolvedFullKey = resolvedKeys[apiKey.id]
  const isCopied = copiedKeyId === apiKey.id
  const maskedKey = `sk-${apiKey.key}`

  const handlePopoverOpen = useCallback(
    (open: boolean) => {
      setPopoverOpen(open)
      if (open && !resolvedFullKey) {
        resolveRealKey(apiKey.id)
      }
    },
    [resolvedFullKey, resolveRealKey, apiKey.id]
  )

  const handleCopy = useCallback(async () => {
    const realKey = resolvedFullKey || (await resolveRealKey(apiKey.id))
    if (realKey) {
      const ok = await copyToClipboard(realKey)
      if (ok) markKeyCopied(apiKey.id)
    }
  }, [resolvedFullKey, resolveRealKey, apiKey.id, markKeyCopied])

  return (
    <div className='flex max-w-full min-w-0 items-center'>
      <Popover open={popoverOpen} onOpenChange={handlePopoverOpen}>
        <PopoverTrigger
          render={
            <Button
              variant='ghost'
              size='sm'
              className='text-muted-foreground h-7 max-w-full min-w-0 justify-start truncate px-0 font-mono text-xs hover:bg-transparent aria-expanded:bg-transparent'
            />
          }
        >
          <span className='truncate'>{maskedKey}</span>
        </PopoverTrigger>
        <PopoverContent
          className='w-auto max-w-[min(90vw,28rem)]'
          align='start'
        >
          <div className='space-y-2'>
            <p className='text-muted-foreground text-xs'>{t('Full API Key')}</p>
            {isLoading ? (
              <div className='flex items-center gap-2 py-2'>
                <Loader2 className='size-3.5 animate-spin' />
                <span className='text-muted-foreground text-xs'>
                  {t('Loading...')}
                </span>
              </div>
            ) : (
              <input
                readOnly
                value={resolvedFullKey || maskedKey}
                autoFocus
                onFocus={(e) => e.target.select()}
                className='bg-muted/50 w-full min-w-[280px] rounded-md border px-3 py-2 font-mono text-xs outline-none'
              />
            )}
          </div>
        </PopoverContent>
      </Popover>
      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              variant='ghost'
              size='icon'
              className='size-7 shrink-0'
              onClick={handleCopy}
              disabled={isLoading}
            />
          }
        >
          {isLoading ? (
            <Loader2 className='size-3.5 animate-spin' />
          ) : isCopied ? (
            <Check className='size-3.5 text-green-600' />
          ) : (
            <Copy className='size-3.5' />
          )}
        </TooltipTrigger>
        <TooltipContent>
          {isLoading
            ? t('Loading...')
            : isCopied
              ? t('Copied!')
              : t('Copy API key')}
        </TooltipContent>
      </Tooltip>
    </div>
  )
}

export function ModelLimitsCell({ apiKey }: { apiKey: ApiKey }) {
  const { t } = useTranslation()

  if (!apiKey.model_limits_enabled || !apiKey.model_limits) {
    return (
      <StatusBadge
        label={t('Unlimited')}
        variant='neutral'
        copyable={false}
        className='-ml-1.5'
      />
    )
  }

  const models = apiKey.model_limits.split(',').filter(Boolean)

  return (
    <Tooltip>
      <TooltipTrigger render={<BadgeCell />}>
        <StatusBadge
          label={t('{{count}} model(s)', { count: models.length })}
          variant='neutral'
          copyable={false}
        />
      </TooltipTrigger>
      <TooltipContent side='top' className='max-w-xs'>
        <div className='max-h-[200px] space-y-0.5 overflow-y-auto text-xs'>
          {models.map((m) => (
            <div key={m} className='font-mono'>
              {m}
            </div>
          ))}
        </div>
      </TooltipContent>
    </Tooltip>
  )
}

export function IpRestrictionsCell({ apiKey }: { apiKey: ApiKey }) {
  const { t } = useTranslation()
  const allowIps = apiKey.allow_ips?.trim()

  if (!allowIps) {
    return (
      <StatusBadge
        label={t('No restriction')}
        variant='neutral'
        copyable={false}
        className='-ml-1.5'
      />
    )
  }

  const ips = allowIps
    .split('\n')
    .map((ip) => ip.trim())
    .filter(Boolean)

  return (
    <Tooltip>
      <TooltipTrigger render={<BadgeCell />}>
        <StatusBadge
          label={t('{{count}} IP(s)', { count: ips.length })}
          variant='neutral'
          copyable={false}
        />
      </TooltipTrigger>
      <TooltipContent side='top' className='max-w-xs'>
        <div className='max-h-[200px] space-y-0.5 overflow-y-auto text-xs'>
          {ips.map((ip) => (
            <div key={ip} className='font-mono'>
              {ip}
            </div>
          ))}
        </div>
      </TooltipContent>
    </Tooltip>
  )
}

type GroupListOption = {
  value: string
  label: string
  desc?: string
  ratio?: number | string
}

function CellRatioBadge({ ratio }: { ratio: GroupListOption['ratio'] }) {
  if (ratio === undefined || ratio === null || ratio === '') return null
  const numeric = typeof ratio === 'number' ? ratio : Number(ratio)
  const tone =
    Number.isFinite(numeric) && numeric > 1
      ? 'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-900/60 dark:bg-amber-950/40 dark:text-amber-300'
      : 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-900/60 dark:bg-emerald-950/40 dark:text-emerald-300'
  return (
    <Badge
      variant='outline'
      className={cn('shrink-0 text-[10px] sm:text-xs', tone)}
    >
      {ratio}x
    </Badge>
  )
}

// Group column cell: shows the current group (same badge styling as before)
// but the badge is now a popover trigger that lets the user pick a different
// group inline — without opening the full edit drawer. The change is persisted
// via updateApiKey (full token payload, group swapped).
export function ApiKeyGroupCell({ apiKey }: { apiKey: ApiKey }) {
  const { t } = useTranslation()
  const { triggerRefresh } = useApiKeys()
  const [open, setOpen] = useState(false)
  const [searchValue, setSearchValue] = useState('')
  const [saving, setSaving] = useState(false)

  const { data: groupsData } = useQuery({
    queryKey: ['user-groups'],
    queryFn: getUserGroups,
    staleTime: 0,
  })
  const selfSubQuery = useQuery({
    queryKey: ['self-subscription-full'],
    queryFn: getSelfSubscriptionFull,
    staleTime: 0,
  })

  const options: GroupListOption[] = useMemo(() => {
    const raw = groupsData?.data || {}
    return Object.entries(raw).map(([key, info]) => ({
      value: key,
      label: key,
      desc: info.desc || key,
      ratio: info.ratio,
    }))
  }, [groupsData])

  const subscribedSet = useMemo(() => {
    const subs = selfSubQuery.data?.data?.subscriptions || []
    const now = selfSubQuery.dataUpdatedAt / 1000
    const result = new Set<string>()
    for (const rec of subs) {
      const sub = rec?.subscription
      if (!sub?.allowed_group) continue
      if (sub.status === 'active' && (sub.end_time || 0) >= now) {
        result.add(sub.allowed_group)
      }
    }
    return result
  }, [selfSubQuery.data, selfSubQuery.dataUpdatedAt])

  const filteredOptions = useMemo(() => {
    const search = searchValue.trim().toLowerCase()
    const matched = search
      ? options.filter((option) => {
          const ratioText = String(option.ratio ?? '').toLowerCase()
          return (
            option.value.toLowerCase().includes(search) ||
            option.label.toLowerCase().includes(search) ||
            option.desc?.toLowerCase().includes(search) ||
            ratioText.includes(search)
          )
        })
      : options
    return [...matched].sort((a, b) => {
      const aSub = subscribedSet.has(a.value) ? 0 : 1
      const bSub = subscribedSet.has(b.value) ? 0 : 1
      return aSub - bSub
    })
  }, [options, searchValue, subscribedSet])

  const handleSelect = useCallback(
    async (nextGroup: string) => {
      if (nextGroup === apiKey.group || saving) {
        setOpen(false)
        setSearchValue('')
        return
      }
      setSaving(true)
      try {
        // Fetch fresh full token data, swap the group, and push the merged
        // payload back through updateApiKey — same field mapping the edit
        // drawer uses, so no behavior change beyond the group field.
        const detail = await getApiKey(apiKey.id)
        if (!detail.success || !detail.data) {
          toast.error(detail.message || t(ERROR_MESSAGES.UPDATE_FAILED))
          return
        }
        const base = transformApiKeyToFormDefaults(detail.data)
        base.group = nextGroup
        const payload = transformFormDataToPayload(base)
        const result = await updateApiKey({ ...payload, id: apiKey.id })
        if (result.success) {
          toast.success(t(SUCCESS_MESSAGES.API_KEY_UPDATED))
          setOpen(false)
          setSearchValue('')
          triggerRefresh()
        } else {
          toast.error(result.message || t(ERROR_MESSAGES.UPDATE_FAILED))
        }
      } catch {
        toast.error(t(ERROR_MESSAGES.UNEXPECTED))
      } finally {
        setSaving(false)
      }
    },
    [apiKey, saving, t, triggerRefresh]
  )

  const currentGroup = apiKey.group || ''
  const isAuto = currentGroup === 'auto'
  const currentRatio =
    !isAuto && currentGroup
      ? options.find((o) => o.value === currentGroup)?.ratio
      : undefined

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger
        render={
          <button
            type='button'
            aria-label={t('Change group')}
            className={cn(
              'inline-flex h-6 w-fit items-center gap-1 rounded-full border px-2.5 text-xs font-medium transition-colors',
              'border-emerald-200 bg-emerald-50 text-emerald-700 hover:bg-emerald-100',
              'dark:border-emerald-900/60 dark:bg-emerald-950/40 dark:text-emerald-300 dark:hover:bg-emerald-950/60'
            )}
          />
        }
      >
        <span className='max-w-[120px] truncate'>
          {currentGroup || t('Default')}
        </span>
        {typeof currentRatio === 'number' && (
          <span className='bg-muted rounded px-1 text-[10px] font-medium'>
            {currentRatio}x
          </span>
        )}
        <ChevronDown className='size-3 shrink-0 opacity-50' />
        {saving && <Loader2 className='size-3 shrink-0 animate-spin' />}
      </PopoverTrigger>
      <PopoverContent
        className='w-[var(--anchor-width)] min-w-[240px] overflow-hidden rounded-xl p-0 shadow-lg'
        align='start'
        onWheel={(event) => event.stopPropagation()}
        onTouchMove={(event) => event.stopPropagation()}
        onPointerDown={(event) => event.stopPropagation()}
      >
        <Command shouldFilter={false}>
          <CommandInput
            placeholder={t('Search...')}
            value={searchValue}
            onValueChange={setSearchValue}
          />
          <CommandList className='max-h-[300px]'>
            <CommandEmpty>{t('No group found.')}</CommandEmpty>
            <CommandGroup>
              {filteredOptions.map((option) => {
                const isSub = subscribedSet.has(option.value)
                return (
                  <CommandItem
                    key={option.value}
                    value={option.value}
                    disabled={saving}
                    onSelect={() => handleSelect(option.value)}
                    className='data-[selected=true]:bg-muted items-start gap-3 rounded-lg px-3 py-2.5 transition-colors'
                  >
                    <Check
                      className={cn(
                        'mt-0.5 h-4 w-4',
                        currentGroup === option.value
                          ? 'opacity-100'
                          : 'opacity-0'
                      )}
                    />
                    <span className='min-w-0 flex-1'>
                      <span className='block truncate font-medium'>
                        {option.label}
                      </span>
                      {option.desc && (
                        <span className='text-muted-foreground block truncate text-xs'>
                          {option.desc}
                        </span>
                      )}
                    </span>
                    <span className='flex items-center gap-1.5'>
                      {isSub && (
                        <Badge
                          variant='outline'
                          className='shrink-0 border-violet-200 bg-violet-50 text-[10px] text-violet-700 dark:border-violet-900/60 dark:bg-violet-950/40 dark:text-violet-300'
                        >
                          {t('Plan Group')}
                        </Badge>
                      )}
                      <CellRatioBadge ratio={option.ratio} />
                    </span>
                  </CommandItem>
                )
              })}
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  )
}
