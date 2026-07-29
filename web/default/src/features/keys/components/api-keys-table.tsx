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
import { useQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import { type Table as TanstackTable } from '@tanstack/react-table'
import {
  ArrowRightLeft,
  Database,
  Edit3,
  KeyRound,
  Link2,
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
import { useChatPresets } from '@/features/chat/hooks/use-chat-presets'
import { getUserProfile } from '@/features/profile/api'
import {
  ConcurrencyCard,
  RpmCard,
} from '@/features/profile/components/concurrency-card'
import { resolveCurrentRpm, resolveRpmLimit } from '@/features/profile/rpm'
import type { UserProfile } from '@/features/profile/types'
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

type ConcurrencySnapshot = {
  current: number
  limit: number
  currentRpm: number | null
  rpmLimit: number
  loading: boolean
}

function resolvePublicApiEndpoint(serverAddress?: string | null) {
  return resolveTutorialApiBaseUrl(
    serverAddress,
    typeof window === 'undefined' ? null : window.location.origin
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
  concurrency,
}: {
  table: TanstackTable<ApiKey>
  isLoading: boolean
  concurrency: ConcurrencySnapshot
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
            <div className='flex items-center justify-between gap-2 text-xs'>
              <span className='text-muted-foreground'>RPM</span>
              <span className='font-mono font-medium tabular-nums'>
                {concurrency.loading
                  ? '—'
                  : `${concurrency.currentRpm ?? '—'} / ${concurrency.rpmLimit}`}
                <span className='text-muted-foreground ml-1 font-sans text-[10px] font-normal'>
                  {t('Account shared')}
                </span>
              </span>
            </div>
            <div className='flex items-center justify-between gap-2 text-xs'>
              <span className='text-muted-foreground'>
                {t('Current Concurrency')}
              </span>
              <span className='font-mono font-medium tabular-nums'>
                {concurrency.loading
                  ? '—'
                  : `${concurrency.current} / ${concurrency.limit}`}
                <span className='text-muted-foreground ml-1 font-sans text-[10px] font-normal'>
                  {t('Account shared')}
                </span>
              </span>
            </div>
          </div>
        )
      })}
    </div>
  )
}

function ApiKeysDesktopSkeleton() {
  return (
    <div className='grid items-start gap-5 md:grid-cols-[14rem_minmax(0,1fr)] xl:grid-cols-[14rem_minmax(0,1fr)_17rem]'>
      <Skeleton className='h-72 rounded-xl' />
      <Skeleton className='h-96 rounded-xl' />
      <div className='grid gap-3 md:col-start-2 md:grid-cols-2 xl:col-start-3 xl:row-start-1 xl:grid-cols-1'>
        <Skeleton className='h-48 rounded-xl' />
        <Skeleton className='h-40 rounded-xl' />
      </div>
    </div>
  )
}

function ApiKeysDesktopWorkspace({
  table,
  isLoading,
  profile,
  profileLoading,
}: {
  table: TanstackTable<ApiKey>
  isLoading: boolean
  profile: UserProfile | null
  profileLoading: boolean
}) {
  const { t } = useTranslation()
  const [selectedKeyId, setSelectedKeyId] = useState<number | null>(null)
  const { setOpen, setCurrentRow, setResolvedKey, resolveRealKey } =
    useApiKeys()
  const { serverAddress } = useChatPresets()
  const apiEndpoint = resolvePublicApiEndpoint(serverAddress)
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

  const copyEndpoint = async () => {
    const copied = await copyToClipboard(apiEndpoint)
    if (copied) toast.success(t('Copied'))
  }

  return (
    <div className='grid items-start gap-5 md:grid-cols-[14rem_minmax(0,1fr)] xl:grid-cols-[14rem_minmax(0,1fr)_17rem]'>
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
              <ApiKeyGroupCell apiKey={apiKey} />
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
          </div>
          <div className='grid grid-cols-[5rem_minmax(0,1fr)] items-center gap-3 text-xs'>
            <span className='text-muted-foreground'>{t('API Key')}</span>
            <ApiKeyCell apiKey={apiKey} />
          </div>
          <div className='grid grid-cols-[5rem_minmax(0,1fr)] items-center gap-3 text-xs'>
            <span className='text-muted-foreground'>API Base URL</span>
            <div className='flex min-w-0 items-center gap-2'>
              <code className='min-w-0 flex-1 truncate font-mono text-xs'>
                {apiEndpoint}
              </code>
              <Button
                type='button'
                variant='ghost'
                size='icon-sm'
                onClick={copyEndpoint}
                aria-label={t('Copy')}
              >
                <Link2 className='size-3.5' />
              </Button>
            </div>
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
              <span className='text-muted-foreground text-xs'>{t('Used')}</span>
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

      <aside className='grid gap-3 md:col-start-2 md:grid-cols-2 xl:col-start-3 xl:row-start-1 xl:grid-cols-1'>
        <ConcurrencyCard profile={profile} loading={profileLoading} compact />
        <RpmCard profile={profile} loading={profileLoading} />
      </aside>
    </div>
  )
}

export function ApiKeysTable() {
  const { t } = useTranslation()
  const { refreshTrigger } = useApiKeys()
  const { serverAddress } = useChatPresets()
  const apiEndpoint = resolvePublicApiEndpoint(serverAddress)
  const columns = useApiKeysColumns()
  const { data: profileResponse, isLoading: concurrencyLoading } = useQuery({
    queryKey: ['api-keys-concurrency'],
    queryFn: getUserProfile,
    refetchInterval: 10_000,
    refetchIntervalInBackground: false,
  })
  const profile = profileResponse?.data ?? null
  const concurrency: ConcurrencySnapshot = {
    current: profile?.current_concurrency ?? 0,
    limit: profile?.concurrency_limit ?? 8,
    currentRpm: resolveCurrentRpm(profile),
    rpmLimit: resolveRpmLimit(profile),
    loading: concurrencyLoading,
  }

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
        leftActions: (
          <button
            type='button'
            onClick={async () => {
              const copied = await copyToClipboard(apiEndpoint)
              if (copied) toast.success(t('Copied'))
            }}
            className='border-border/70 bg-background hover:bg-muted/60 flex max-w-full items-center gap-2 rounded-lg border px-3 py-1.5 text-xs transition-colors'
          >
            <span className='font-semibold'>API Base URL</span>
            <code className='text-muted-foreground max-w-72 truncate font-mono'>
              {apiEndpoint}
            </code>
            <Link2 className='size-3.5 shrink-0' />
          </button>
        ),
      }}
      mobile={
        <ApiKeysMobileList
          table={table}
          isLoading={isLoading}
          concurrency={concurrency}
        />
      }
      desktop={
        <ApiKeysDesktopWorkspace
          table={table}
          isLoading={isLoading}
          profile={profile}
          profileLoading={concurrencyLoading}
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
