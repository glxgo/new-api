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
import { useEffect, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import { type Table as TanstackTable } from '@tanstack/react-table'
import {
  ArrowRightLeft,
  Check,
  Database,
  Edit3,
  Gauge,
  KeyRound,
  Loader2,
  Plus,
  Trash2,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { copyToClipboard } from '@/lib/copy-to-clipboard'
import { formatQuota, formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'
import { useTableUrlState } from '@/hooks/use-table-url-state'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import {
  DISABLED_ROW_DESKTOP,
  DISABLED_ROW_MOBILE,
  DataTablePage,
  useDebouncedColumnFilter,
  useDataTable,
} from '@/components/data-table'
import { StatusBadge } from '@/components/status-badge'
import {
  getAPIIngressProfiles,
  resolveAPIIngressBaseUrl,
  resolveAPIIngressEndpoint,
  type APIIngressProfile,
} from '@/features/api-ingress/api'
import { useChatPresets } from '@/features/chat/hooks/use-chat-presets'
import { resolveTutorialApiBaseUrl } from '@/features/tutorial/content'
import { getApiKeys, searchApiKeys } from '../api'
import {
  API_KEY_STATUS,
  API_KEY_STATUS_OPTIONS,
  API_KEY_STATUSES,
  ERROR_MESSAGES,
} from '../constants'
import { type ApiKey } from '../types'
import { ApiKeySubscriptionCombobox } from './api-key-subscription-combobox'
import {
  ApiKeyCell,
  ApiKeyGroupCell,
  IpRestrictionsCell,
  ModelLimitsCell,
} from './api-keys-cells'
import { useApiKeysColumns } from './api-keys-columns'
import { useApiKeys } from './api-keys-provider'
import { DataTableBulkActions } from './data-table-bulk-actions'
import { DataTableRowActions } from './data-table-row-actions'

const route = getRouteApi('/_authenticated/keys/')

function resolvePublicApiEndpoint(serverAddress?: string | null) {
  return resolveTutorialApiBaseUrl(
    serverAddress,
    typeof window === 'undefined' ? null : window.location.origin
  )
}

type APIIngressLatency = number | 'testing' | 'error'

function ApiIngressPreview({ fallbackEndpoint }: { fallbackEndpoint: string }) {
  const { data, isLoading } = useQuery({
    queryKey: ['api-ingress-profiles'],
    queryFn: getAPIIngressProfiles,
  })
  const { selectedIngressCode, setSelectedIngressCode } = useApiKeys()
  const [latencies, setLatencies] = useState<Record<string, APIIngressLatency>>(
    {}
  )
  const profiles = (data?.data ?? []).filter((profile) => profile.enabled)

  useEffect(() => {
    if (!profiles.length) return
    const selected =
      profiles.find((profile) => profile.code === selectedIngressCode) ??
      profiles.find((profile) => profile.default) ??
      profiles[0]
    if (selected && selected.code !== selectedIngressCode) {
      setSelectedIngressCode(selected.code)
    }
  }, [profiles, selectedIngressCode, setSelectedIngressCode])

  const endpointFor = (profile: APIIngressProfile) =>
    resolveAPIIngressEndpoint(profile, fallbackEndpoint)

  const copy = async (value: string) => {
    if (await copyToClipboard(value)) toast.success('已复制')
  }
  const measure = async (
    profile: APIIngressProfile
  ): Promise<number | 'error'> => {
    const base = resolveAPIIngressBaseUrl(profile, fallbackEndpoint)
    const ping = async () => {
      const controller = new AbortController()
      const timeout = window.setTimeout(() => controller.abort(), 8_000)
      try {
        const response = await fetch(`${base}/api/ingress/ping`, {
          // The probe is anonymous. Sending cookies cross-origin makes the
          // direct endpoint's wildcard CORS response fail in browsers.
          credentials: 'omit',
          cache: 'no-store',
          headers: { 'X-New-API-Ingress': profile.code },
          signal: controller.signal,
        })
        if (!response.ok) throw new Error('ping failed')
      } finally {
        window.clearTimeout(timeout)
      }
    }
    try {
      // 第一次请求可能包含 DNS/TLS/连接建立成本；先预热，再测量复用连接后的真实延迟。
      await ping()
      const started = performance.now()
      await ping()
      return Math.round(performance.now() - started)
    } catch {
      return 'error'
    }
  }

  const testAll = async () => {
    if (
      !profiles.length ||
      profiles.some((profile) => latencies[profile.code] === 'testing')
    ) {
      return
    }
    setLatencies(
      Object.fromEntries(
        profiles.map((profile) => [profile.code, 'testing'])
      ) as Record<string, APIIngressLatency>
    )
    const results = await Promise.all(
      profiles.map(async (profile) => ({
        profile,
        latency: await measure(profile),
      }))
    )
    const nextLatencies = Object.fromEntries(
      results.map(({ profile, latency }) => [profile.code, latency])
    ) as Record<string, APIIngressLatency>
    setLatencies(nextLatencies)

    const fastest = results
      .filter(
        (result): result is { profile: APIIngressProfile; latency: number } =>
          typeof result.latency === 'number'
      )
      .sort((a, b) => a.latency - b.latency)[0]
    if (fastest) {
      setSelectedIngressCode(fastest.profile.code)
      toast.success(
        `已自动选择 ${fastest.profile.display_name}（${fastest.latency} ms）`
      )
    } else {
      toast.error('两个 API 入口均测速失败，请稍后重试')
    }
  }

  if (isLoading)
    return (
      <div className='bg-muted/20 mt-3 rounded-xl border p-4 text-xs'>
        正在加载 API 入口…
      </div>
    )
  if (!profiles.length) return null
  return (
    <div className='border-border/70 bg-background/70 min-w-0 rounded-xl border p-3 shadow-xs'>
      <div className='mb-2 flex flex-wrap items-center justify-between gap-2'>
        <div className='text-muted-foreground flex items-center gap-2 text-[10px] font-medium tracking-wider uppercase'>
          <Gauge className='size-3.5' />
          API 请求入口
          <span className='tracking-normal normal-case'>
            测速后自动选择低延迟入口
          </span>
        </div>
        <Button
          type='button'
          variant='outline'
          size='sm'
          className='h-7 shrink-0 rounded-full px-3 text-xs'
          onClick={testAll}
          disabled={profiles.some(
            (profile) => latencies[profile.code] === 'testing'
          )}
        >
          {profiles.some((profile) => latencies[profile.code] === 'testing') ? (
            <Loader2 className='size-3.5 animate-spin' />
          ) : (
            <Gauge className='size-3.5' />
          )}
          一键测速
        </Button>
      </div>
      <div className='grid gap-2'>
        {profiles.map((profile) => {
          const latency = latencies[profile.code]
          const selected = profile.code === selectedIngressCode
          return (
            <div
              key={profile.code}
              className={cn(
                'border-border/70 bg-card flex min-w-0 items-stretch overflow-hidden rounded-lg border transition-colors',
                selected && 'border-emerald-500/70 bg-emerald-500/5'
              )}
            >
              <button
                type='button'
                aria-pressed={selected}
                onClick={() => setSelectedIngressCode(profile.code)}
                className='hover:bg-muted/50 min-w-0 flex-1 px-3 py-2 text-left transition-colors focus-visible:outline-none'
              >
                <div className='flex items-center justify-between gap-2'>
                  <span className='truncate text-xs font-semibold'>
                    {profile.display_name}
                  </span>
                  <span className='shrink-0 rounded-full bg-emerald-500/10 px-2 py-0.5 text-[10px] font-medium text-emerald-600'>
                    ×{profile.multiplier.toFixed(3)}
                  </span>
                </div>
                <code className='text-muted-foreground mt-1.5 block truncate font-mono text-[10px]'>
                  {endpointFor(profile)}
                </code>
                {profile.description?.trim() && (
                  <p className='text-muted-foreground mt-1 line-clamp-2 text-[10px] whitespace-pre-wrap'>
                    {profile.description.trim()}
                  </p>
                )}
                <div className='text-muted-foreground mt-1 flex items-center gap-2 text-[10px]'>
                  <span>
                    {profile.network_mode === 'line'
                      ? '线路机 → 落地机'
                      : '用户直连落地机'}
                  </span>
                  {typeof latency === 'number' && (
                    <span className='font-mono tabular-nums'>{latency} ms</span>
                  )}
                  {latency === 'error' && <span>测速失败</span>}
                </div>
              </button>
              <div className='border-border/60 flex shrink-0 flex-col items-center justify-center gap-1 border-l px-2'>
                {selected && <Check className='size-3.5 text-emerald-600' />}
                <button
                  type='button'
                  className='text-muted-foreground hover:text-foreground text-[10px]'
                  onClick={() => copy(endpointFor(profile))}
                >
                  复制
                </button>
              </div>
            </div>
          )
        })}
      </div>
      <div className='text-muted-foreground mt-2 text-[10px]'>
        当前使用：
        <span className='text-foreground font-medium'>
          {profiles.find((profile) => profile.code === selectedIngressCode)
            ?.display_name ?? profiles[0]?.display_name}
        </span>
        ，点击卡片可手动切换
      </div>
    </div>
  )
}

function isDisabledApiKeyRow(apiKey: ApiKey) {
  return apiKey.status !== API_KEY_STATUS.ENABLED
}

function ApiKeysMobileSkeleton() {
  return (
    <div className='divide-border overflow-hidden rounded-lg border'>
      {Array.from({ length: 5 }).map((_, index) => (
        <div
          key={index}
          className='space-y-2 border-b px-3 py-2.5 last:border-b-0'
        >
          <div className='flex items-center justify-between'>
            <Skeleton className='h-4 w-32' />
            <Skeleton className='h-5 w-16 rounded-md' />
          </div>
          <div className='flex items-center justify-between gap-3'>
            <Skeleton className='h-7 w-44' />
            <Skeleton className='h-8 w-16' />
          </div>
          <Skeleton className='h-3 w-28' />
        </div>
      ))}
    </div>
  )
}

function ApiKeysMobileList({
  table,
  isLoading,
}: {
  table: TanstackTable<ApiKey>
  isLoading: boolean
}) {
  const { t } = useTranslation()
  const rows = table.getRowModel().rows

  if (isLoading) return <ApiKeysMobileSkeleton />

  if (!rows.length) {
    return (
      <div className='rounded-lg border p-8'>
        <Empty className='border-none p-0'>
          <EmptyHeader>
            <EmptyMedia variant='icon'>
              <Database className='size-6' />
            </EmptyMedia>
            <EmptyTitle>{t('No API Keys Found')}</EmptyTitle>
            <EmptyDescription>
              {t(
                'No API keys available. Create your first API key to get started.'
              )}
            </EmptyDescription>
          </EmptyHeader>
        </Empty>
      </div>
    )
  }

  return (
    <div className='divide-border overflow-hidden rounded-lg border'>
      {rows.map((row) => {
        const apiKey = row.original
        const statusConfig = API_KEY_STATUSES[apiKey.status]
        const total = apiKey.used_quota + apiKey.remain_quota

        return (
          <div
            key={row.id}
            className={cn(
              'bg-card space-y-2.5 border-b px-3 py-2.5 last:border-b-0',
              isDisabledApiKeyRow(apiKey) && DISABLED_ROW_MOBILE
            )}
          >
            <div className='flex items-start justify-between gap-3'>
              <div className='min-w-0'>
                <div className='truncate text-sm font-semibold'>
                  {apiKey.name}
                </div>
                <div className='text-muted-foreground text-[11px]'>
                  {t('API Key')}
                </div>
              </div>
              {statusConfig && (
                <StatusBadge
                  label={t(statusConfig.label)}
                  variant={statusConfig.variant}
                  copyable={false}
                />
              )}
            </div>

            <div className='flex min-w-0 items-center justify-between gap-2'>
              <div className='min-w-0 flex-1 [&_button:first-child]:max-w-full [&_button:first-child]:truncate [&_button:first-child]:px-0'>
                <ApiKeyCell apiKey={apiKey} />
              </div>
              <DataTableRowActions row={row} />
            </div>

            <div className='flex items-center justify-between gap-2 text-xs'>
              <span className='text-muted-foreground'>{t('Quota')}</span>
              {apiKey.unlimited_quota ? (
                <span className='font-medium'>{t('Unlimited')}</span>
              ) : (
                <span className='font-medium tabular-nums'>
                  {formatQuota(apiKey.remain_quota)}
                  <span className='text-muted-foreground font-normal'>
                    {' / '}
                    {formatQuota(total)}
                  </span>
                </span>
              )}
            </div>
          </div>
        )
      })}
    </div>
  )
}

function ApiKeysDesktopSkeleton() {
  return (
    <div className='@container/api-key-workspace'>
      <div className='grid items-start gap-4 @2xl/api-key-workspace:grid-cols-[12rem_minmax(0,1fr)] @3xl/api-key-workspace:grid-cols-[12rem_minmax(18rem,1fr)_18rem]'>
        <Skeleton className='h-72 rounded-xl' />
        <Skeleton className='h-96 rounded-xl' />
        <div className='@2xl/api-key-workspace:col-start-2 @3xl/api-key-workspace:col-start-3 @3xl/api-key-workspace:row-start-1'>
          <Skeleton className='h-96 rounded-xl' />
        </div>
      </div>
    </div>
  )
}

function ApiKeysDesktopWorkspace({
  table,
  isLoading,
  apiEndpoint,
}: {
  table: TanstackTable<ApiKey>
  isLoading: boolean
  apiEndpoint: string
}) {
  const { t } = useTranslation()
  const [selectedKeyId, setSelectedKeyId] = useState<number | null>(null)
  const { setOpen, setCurrentRow, setResolvedKey, resolveRealKey } =
    useApiKeys()
  const rows = table.getRowModel().rows
  const selectedRow =
    rows.find((row) => row.original.id === selectedKeyId) ?? rows[0]

  if (isLoading) return <ApiKeysDesktopSkeleton />

  if (!selectedRow) {
    return (
      <div className='border-border/70 bg-background/75 flex h-full min-h-80 items-center justify-center rounded-2xl border p-8 shadow-xs'>
        <Empty className='border-none p-0'>
          <EmptyHeader>
            <EmptyMedia variant='icon'>
              <Database className='size-6' />
            </EmptyMedia>
            <EmptyTitle>{t('No API Keys Found')}</EmptyTitle>
            <EmptyDescription>
              {t(
                'No API keys available. Create your first API key to get started.'
              )}
            </EmptyDescription>
          </EmptyHeader>
          <Button
            size='sm'
            className='mt-4 rounded-full px-4'
            onClick={() => setOpen('create')}
          >
            <Plus className='size-4' />
            {t('Create API Key')}
          </Button>
        </Empty>
      </div>
    )
  }

  const apiKey = selectedRow.original
  const statusConfig = API_KEY_STATUSES[apiKey.status]
  const totalQuota = apiKey.used_quota + apiKey.remain_quota
  const lastUsed = apiKey.accessed_time
    ? formatTimestampToDate(apiKey.accessed_time)
    : '-'
  const expires =
    apiKey.expired_time === -1
      ? t('Never')
      : formatTimestampToDate(apiKey.expired_time)

  const openDialog = (type: 'update' | 'delete') => {
    setCurrentRow(apiKey)
    setOpen(type)
  }

  const openCcSwitch = async () => {
    const realKey = await resolveRealKey(apiKey.id)
    if (!realKey) return
    setResolvedKey(realKey)
    setCurrentRow(apiKey)
    setOpen('cc-switch')
  }

  return (
    <div className='@container/api-key-workspace'>
      <div className='grid items-start gap-4 @2xl/api-key-workspace:grid-cols-[12rem_minmax(0,1fr)] @3xl/api-key-workspace:grid-cols-[12rem_minmax(18rem,1fr)_18rem]'>
        <div className='self-start'>
          <div className='border-border/70 bg-background relative z-10 ml-4 flex w-28 items-center gap-2 rounded-t-xl border border-b-0 px-3 py-2'>
            <KeyRound className='size-3.5' />
            <span className='text-xs font-semibold'>{t('API Keys')}</span>
            <span className='text-muted-foreground text-[10px] tabular-nums'>
              {rows.length}
            </span>
          </div>
          <aside className='border-border/70 bg-background/80 -mt-px flex h-[26rem] flex-col overflow-hidden rounded-xl border shadow-xs'>
            <div className='border-border/60 flex items-center justify-between border-b px-3.5 py-2.5'>
              <div>
                <p className='text-xs font-semibold'>{t('API Keys')}</p>
                <p className='text-muted-foreground text-[10px] tabular-nums'>
                  {rows.length} {t('Total')}
                </p>
              </div>
              <Checkbox
                checked={table.getIsAllPageRowsSelected()}
                indeterminate={table.getIsSomePageRowsSelected()}
                onCheckedChange={(value) =>
                  table.toggleAllPageRowsSelected(!!value)
                }
                aria-label={t('Select all')}
              />
            </div>

            <div className='min-h-0 flex-1 space-y-1 overflow-y-auto p-2'>
              {rows.map((row) => {
                const rowApiKey = row.original
                const rowStatus = API_KEY_STATUSES[rowApiKey.status]
                const selected = rowApiKey.id === apiKey.id

                return (
                  <div
                    key={row.id}
                    className={cn(
                      'group flex items-center gap-2 rounded-lg px-2 py-0.5 transition-colors',
                      selected
                        ? 'border-border/60 bg-muted border'
                        : 'hover:bg-muted/60 border border-transparent',
                      isDisabledApiKeyRow(rowApiKey) && 'opacity-60'
                    )}
                  >
                    <button
                      type='button'
                      onClick={() => setSelectedKeyId(rowApiKey.id)}
                      className='focus-visible:ring-ring flex min-w-0 flex-1 items-center gap-2 rounded-md px-1 py-2 text-left focus-visible:ring-2 focus-visible:outline-none'
                    >
                      <span
                        className={cn(
                          'size-1.5 shrink-0 rounded-full border',
                          rowApiKey.status === API_KEY_STATUS.ENABLED
                            ? 'border-foreground bg-foreground'
                            : 'border-muted-foreground/60 bg-transparent'
                        )}
                      />
                      <span className='truncate text-sm font-medium'>
                        {rowApiKey.name}
                      </span>
                    </button>
                    <Checkbox
                      checked={row.getIsSelected()}
                      onCheckedChange={(value) => row.toggleSelected(!!value)}
                      aria-label={t('Select row')}
                      onClick={(event) => event.stopPropagation()}
                      className='opacity-45 transition-opacity group-hover:opacity-100'
                    />
                    <span className='sr-only'>
                      {rowStatus ? t(rowStatus.label) : ''}
                    </span>
                  </div>
                )
              })}
            </div>

            <div className='border-border/60 border-t p-2.5'>
              <Button
                type='button'
                variant='outline'
                size='sm'
                className='w-full justify-start border-dashed'
                onClick={() => setOpen('create')}
              >
                <Plus className='size-3.5' />
                {t('Create API Key')}
              </Button>
            </div>
          </aside>
        </div>

        <section className='border-border/70 bg-background/80 self-start rounded-xl border p-5 shadow-sm'>
          <div className='flex flex-wrap items-start justify-between gap-4'>
            <div className='min-w-0'>
              <div className='flex flex-wrap items-center gap-2.5'>
                <h3 className='truncate text-xl font-semibold tracking-tight'>
                  {apiKey.name}
                </h3>
                {statusConfig && (
                  <StatusBadge
                    label={t(statusConfig.label)}
                    variant={statusConfig.variant}
                    copyable={false}
                  />
                )}
              </div>
              <div className='mt-2 flex flex-wrap items-center gap-2'>
                <span className='font-mono text-xs tabular-nums'>
                  #{apiKey.id}
                </span>
              </div>
            </div>
            <span className='border-border/70 bg-muted/25 flex size-8 items-center justify-center rounded-full border'>
              <KeyRound className='size-3.5' />
            </span>
          </div>

          <div className='border-border/70 bg-muted/15 mt-5 space-y-3 rounded-xl border p-4'>
            <div className='grid gap-3 text-xs 2xl:grid-cols-2'>
              <div className='grid grid-cols-[5rem_minmax(0,1fr)] items-center gap-3'>
                <span className='text-muted-foreground'>{t('Group')}</span>
                {apiKey.routing_mode === 'custom' ? (
                  <span className='text-primary text-xs font-medium'>
                    API Key 消耗路由策略 · 首选 {apiKey.group}
                  </span>
                ) : (
                  <ApiKeyGroupCell apiKey={apiKey} />
                )}
              </div>
              {apiKey.subscription_mode === 'instance' &&
                apiKey.subscription_id > 0 && (
                  <div className='grid grid-cols-[5rem_minmax(0,1fr)] items-center gap-3 2xl:justify-self-end'>
                    <span className='text-muted-foreground'>
                      {t('Subscription instance')}
                    </span>
                    <ApiKeySubscriptionCombobox apiKey={apiKey} />
                  </div>
                )}
              {apiKey.virtual_membership_id > 0 && (
                <div className='grid grid-cols-[5rem_minmax(0,1fr)] items-center gap-3 2xl:justify-self-end'>
                  <span className='text-muted-foreground'>虚拟会员</span>
                  <span className='text-emerald-600'>
                    额度实例 #{apiKey.virtual_membership_id}
                  </span>
                </div>
              )}
            </div>
            <div className='grid grid-cols-[5rem_minmax(0,1fr)] items-center gap-3 text-xs'>
              <span className='text-muted-foreground'>{t('API Key')}</span>
              <ApiKeyCell apiKey={apiKey} />
            </div>
            <div className='grid grid-cols-2 gap-3 border-t border-dashed pt-3'>
              <div>
                <p className='text-muted-foreground mb-2 text-[10px]'>
                  {t('Models')}
                </p>
                <ModelLimitsCell apiKey={apiKey} />
              </div>
              <div>
                <p className='text-muted-foreground mb-2 text-[10px]'>
                  {t('IP Restriction')}
                </p>
                <IpRestrictionsCell apiKey={apiKey} />
              </div>
            </div>
          </div>

          <div className='mt-5 flex flex-wrap items-end justify-between gap-3'>
            <div>
              <div className='flex items-baseline gap-2'>
                <p className='font-mono text-2xl font-semibold tracking-tight tabular-nums'>
                  {formatQuota(apiKey.used_quota)}
                </p>
                <span className='text-muted-foreground text-xs'>
                  {t('Used')}
                </span>
              </div>
              <p className='text-muted-foreground mt-1 text-[10px] tabular-nums'>
                {t('Total')} {formatQuota(totalQuota)}
              </p>
            </div>
            <div className='text-right'>
              <p className='text-muted-foreground text-[10px]'>
                {t('Remaining')}
              </p>
              <p className='mt-1 font-mono text-xs font-semibold tabular-nums'>
                {apiKey.unlimited_quota
                  ? t('Unlimited')
                  : formatQuota(apiKey.remain_quota)}
              </p>
            </div>
          </div>

          <div className='border-border/60 text-muted-foreground mt-4 flex flex-wrap gap-x-5 gap-y-2 border-y border-dashed py-3 text-[10px]'>
            <span>
              {t('Last Used')}{' '}
              <b className='text-foreground font-mono'>{lastUsed}</b>
            </span>
            <span>
              {t('Created')}{' '}
              <b className='text-foreground font-mono'>
                {formatTimestampToDate(apiKey.created_time)}
              </b>
            </span>
            <span>
              {t('Expires')}{' '}
              <b className='text-foreground font-mono'>{expires}</b>
            </span>
          </div>

          <div className='mt-4 flex flex-wrap items-center gap-2'>
            <Button
              type='button'
              variant='outline'
              size='sm'
              className='rounded-full px-4'
              onClick={openCcSwitch}
            >
              <ArrowRightLeft className='size-3.5' />
              {t('One-click import to CC Switch')}
            </Button>
            <Button
              type='button'
              variant='outline'
              size='sm'
              className='rounded-full px-4'
              onClick={() => openDialog('update')}
            >
              <Edit3 className='size-3.5' />
              {t('Edit')}
            </Button>
            <DataTableRowActions
              row={selectedRow}
              display='compact'
              showMenu={false}
            />
            <Button
              type='button'
              variant='outline'
              size='sm'
              className='text-destructive hover:text-destructive rounded-full px-4'
              onClick={() => openDialog('delete')}
            >
              <Trash2 className='size-3.5' />
              {t('Delete')}
            </Button>
          </div>
        </section>

        <aside className='@2xl/api-key-workspace:col-start-2 @3xl/api-key-workspace:col-start-3 @3xl/api-key-workspace:row-start-1'>
          <ApiIngressPreview fallbackEndpoint={apiEndpoint} />
        </aside>
      </div>
    </div>
  )
}

export function ApiKeysTable() {
  const { t } = useTranslation()
  const { refreshTrigger } = useApiKeys()
  const { serverAddress } = useChatPresets()
  const apiEndpoint = resolvePublicApiEndpoint(serverAddress)
  const columns = useApiKeysColumns()
  const {
    globalFilter,
    onGlobalFilterChange,
    columnFilters,
    onColumnFiltersChange,
    pagination,
    onPaginationChange,
    ensurePageInRange,
  } = useTableUrlState({
    search: route.useSearch(),
    navigate: route.useNavigate(),
    pagination: { defaultPage: 1, defaultPageSize: 20 },
    globalFilter: { enabled: true, key: 'filter' },
    columnFilters: [
      { columnId: 'status', searchKey: 'status', type: 'array' },
      { columnId: '_tokenSearch', searchKey: 'token', type: 'string' },
    ],
  })

  const {
    value: tokenFilter,
    inputValue: tokenFilterInput,
    setInputValue: setTokenFilterInput,
  } = useDebouncedColumnFilter({
    columnFilters,
    columnId: '_tokenSearch',
    onColumnFiltersChange,
  })
  const shouldSearch = Boolean(globalFilter?.trim() || tokenFilter.trim())

  // Fetch data with React Query
  // eslint-disable-next-line @tanstack/query/exhaustive-deps
  const { data, isLoading, isFetching } = useQuery({
    queryKey: [
      'keys',
      pagination.pageIndex + 1,
      pagination.pageSize,
      globalFilter,
      tokenFilter,
      refreshTrigger,
    ],
    queryFn: async () => {
      const result = shouldSearch
        ? await searchApiKeys({
            keyword: globalFilter,
            token: tokenFilter,
            p: pagination.pageIndex + 1,
            size: pagination.pageSize,
          })
        : await getApiKeys({
            p: pagination.pageIndex + 1,
            size: pagination.pageSize,
          })

      if (!result.success) {
        toast.error(
          result.message ||
            t(
              shouldSearch
                ? ERROR_MESSAGES.SEARCH_FAILED
                : ERROR_MESSAGES.LOAD_FAILED
            )
        )
        return { items: [], total: 0 }
      }

      return {
        items: result.data?.items || [],
        total: result.data?.total || 0,
      }
    },
    placeholderData: (previousData) => previousData,
  })

  const apiKeys = data?.items || []

  const { table } = useDataTable({
    data: apiKeys,
    columns,
    enableRowSelection: true,
    columnFilters,
    globalFilter,
    pagination,
    globalFilterFn: () => true,
    onPaginationChange,
    onGlobalFilterChange,
    onColumnFiltersChange,
    manualPagination: true,
    totalCount: data?.total || 0,
    ensurePageInRange,
  })

  return (
    <DataTablePage
      table={table}
      columns={columns}
      isLoading={isLoading}
      isFetching={isFetching}
      emptyTitle={t('No API Keys Found')}
      emptyDescription={t(
        'No API keys available. Create your first API key to get started.'
      )}
      skeletonKeyPrefix='api-keys-skeleton'
      applyHeaderSize
      toolbarProps={{
        className: 'px-0 py-1',
        searchPlaceholder: t('Filter by name...'),
        additionalSearch: (
          <Input
            placeholder={t('Filter by API key...')}
            aria-label={t('Filter by API key...')}
            value={tokenFilterInput}
            onChange={(e) => setTokenFilterInput(e.target.value)}
            className='w-full sm:w-50 lg:w-60'
          />
        ),
        filters: [
          {
            columnId: 'status',
            title: t('Status'),
            options: API_KEY_STATUS_OPTIONS,
            singleSelect: true,
          },
        ],
      }}
      mobile={
        <div className='space-y-3'>
          <ApiIngressPreview fallbackEndpoint={apiEndpoint} />
          <ApiKeysMobileList table={table} isLoading={isLoading} />
        </div>
      }
      desktop={
        <ApiKeysDesktopWorkspace
          table={table}
          isLoading={isLoading}
          apiEndpoint={apiEndpoint}
        />
      }
      tableClassName='border-border/70 bg-background/75 rounded-xl shadow-xs'
      getRowClassName={(row) =>
        isDisabledApiKeyRow(row.original) ? DISABLED_ROW_DESKTOP : undefined
      }
      bulkActions={<DataTableBulkActions table={table} />}
    />
  )
}
