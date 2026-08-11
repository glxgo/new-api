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
import { type ColumnDef } from '@tanstack/react-table'
import { useTranslation } from 'react-i18next'
import { formatQuota, formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'
import { Checkbox } from '@/components/ui/checkbox'
import { Progress } from '@/components/ui/progress'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { StatusBadge } from '@/components/status-badge'
import { API_KEY_STATUSES } from '../constants'
import { type ApiKey } from '../types'
import {
  ApiKeyCell,
  ApiKeyGroupCell,
  ModelLimitsCell,
  IpRestrictionsCell,
} from './api-keys-cells'
import { DataTableRowActions } from './data-table-row-actions'

function getQuotaProgressColor(percentage: number): string {
  if (percentage <= 10) return '[&_[data-slot=progress-indicator]]:bg-rose-500'
  if (percentage <= 30) return '[&_[data-slot=progress-indicator]]:bg-amber-500'
  return '[&_[data-slot=progress-indicator]]:bg-emerald-500'
}

export function useApiKeysColumns(): ColumnDef<ApiKey>[] {
  const { t } = useTranslation()
  return [
    {
      id: 'select',
      header: ({ table }) => (
        <Checkbox
          checked={table.getIsAllPageRowsSelected()}
          indeterminate={table.getIsSomePageRowsSelected()}
          onCheckedChange={(value) => table.toggleAllPageRowsSelected(!!value)}
          aria-label='Select all'
          className='translate-y-[2px]'
        />
      ),
      cell: ({ row }) => (
        <Checkbox
          checked={row.getIsSelected()}
          onCheckedChange={(value) => row.toggleSelected(!!value)}
          aria-label='Select row'
          className='translate-y-[2px]'
        />
      ),
      enableSorting: false,
      enableHiding: false,
      size: 40,
    },
    {
      accessorKey: 'name',
      header: t('Name'),
      cell: ({ row }) => (
        <span className='font-medium'>{row.getValue('name')}</span>
      ),
      size: 180,
      meta: { mobileTitle: true },
    },
    {
      accessorKey: 'status',
      header: t('Status'),
      cell: ({ row }) => {
        const statusConfig = API_KEY_STATUSES[row.getValue('status') as number]
        if (!statusConfig) return null
        return (
          <StatusBadge
            label={t(statusConfig.label)}
            variant={statusConfig.variant}
            copyable={false}
            className='-ml-1.5'
          />
        )
      },
      filterFn: (row, id, value) => value.includes(String(row.getValue(id))),
      size: 120,
      meta: { mobileBadge: true },
    },
    {
      id: 'key',
      accessorKey: 'key',
      header: t('API Key'),
      cell: ({ row }) => <ApiKeyCell apiKey={row.original} />,
      enableSorting: false,
      size: 260,
    },
    {
      id: 'quota',
      accessorKey: 'remain_quota',
      header: t('Quota'),
      cell: ({ row }) => {
        const apiKey = row.original
        if (apiKey.unlimited_quota) {
          return (
            <StatusBadge
              label={t('Unlimited')}
              variant='neutral'
              copyable={false}
              className='-ml-1.5'
            />
          )
        }

        const used = apiKey.used_quota
        const remaining = apiKey.remain_quota
        const total = used + remaining
        const percentage = total > 0 ? (remaining / total) * 100 : 0

        return (
          <Tooltip>
            <TooltipTrigger render={<div className='w-[150px] space-y-1' />}>
              <div className='flex justify-between text-xs'>
                <span className='font-medium tabular-nums'>
                  {formatQuota(remaining)}
                </span>
                <span className='text-muted-foreground tabular-nums'>
                  {formatQuota(total)}
                </span>
              </div>
              <Progress
                value={percentage}
                className={cn('h-1.5', getQuotaProgressColor(percentage))}
              />
            </TooltipTrigger>
            <TooltipContent>
              <div className='space-y-1 text-xs'>
                <div>
                  {t('Used:')} {formatQuota(used)}
                </div>
                <div>
                  {t('Remaining:')} {formatQuota(remaining)} (
                  {percentage.toFixed(1)}%)
                </div>
                <div>
                  {t('Total:')} {formatQuota(total)}
                </div>
              </div>
            </TooltipContent>
          </Tooltip>
        )
      },
      size: 170,
    },
    {
      accessorKey: 'group',
      header: t('Group'),
      cell: ({ row }) => <ApiKeyGroupCell apiKey={row.original} />,
      size: 160,
      meta: { mobileHidden: true },
    },
    {
      id: 'subscription_binding',
      header: t('Quota ownership'),
      cell: ({ row }) => {
        const apiKey = row.original
        if (apiKey.routing_mode === 'custom') {
          return (
            <StatusBadge
              label='API Key 消耗路由策略'
              variant='info'
              copyable={false}
              className='-ml-1.5'
            />
          )
        }
        const isBound =
          apiKey.subscription_mode === 'instance' && apiKey.subscription_id > 0
        if (!isBound) {
          return (
            <StatusBadge
              label={t('Automatic allocation')}
              variant='neutral'
              copyable={false}
              className='-ml-1.5'
            />
          )
        }
        const policies = [
          apiKey.subscription_allow_renewal && t('Renewed successor'),
          apiKey.subscription_allow_same_group && t('Same-group instance'),
          apiKey.subscription_allow_wallet && t('Wallet'),
        ].filter(Boolean)
        return (
          <Tooltip>
            <TooltipTrigger
              render={
                <div className='flex max-w-[180px] flex-col items-start gap-1' />
              }
            >
              <StatusBadge
                label={`${t('Instance')} #${apiKey.subscription_id}`}
                variant='success'
                copyable={false}
                className='-ml-1.5'
              />
              <span className='text-muted-foreground max-w-full truncate text-[10px]'>
                {policies.length > 0
                  ? policies.join(' → ')
                  : t('Stop after exhaustion')}
              </span>
            </TooltipTrigger>
            <TooltipContent className='max-w-72'>
              <div className='space-y-1 text-xs'>
                <div>
                  {t('Configured ownership')}:{' '}
                  <span className='font-mono'>#{apiKey.subscription_id}</span>
                </div>
                <div>
                  {t('Continuation')}:{' '}
                  {policies.length > 0
                    ? policies.join(' → ')
                    : t('Stop after exhaustion')}
                </div>
                {apiKey.planned_subscription_id > 0 && (
                  <div>
                    {t('Scheduled successor')}:{' '}
                    <span className='font-mono'>
                      #{apiKey.planned_subscription_id}
                    </span>
                  </div>
                )}
                {apiKey.subscription_allow_wallet && (
                  <div>
                    {t('Wallet fallback used')}:{' '}
                    {formatQuota(apiKey.subscription_wallet_used)} /{' '}
                    {formatQuota(apiKey.subscription_wallet_limit)}
                  </div>
                )}
              </div>
            </TooltipContent>
          </Tooltip>
        )
      },
      size: 190,
      meta: { mobileHidden: true },
    },
    {
      id: 'model_limits',
      accessorKey: 'model_limits',
      header: t('Models'),
      cell: ({ row }) => <ModelLimitsCell apiKey={row.original} />,
      enableSorting: false,
      size: 160,
      meta: { mobileHidden: true },
    },
    {
      id: 'allow_ips',
      accessorKey: 'allow_ips',
      header: t('IP Restriction'),
      cell: ({ row }) => <IpRestrictionsCell apiKey={row.original} />,
      enableSorting: false,
      size: 160,
      meta: { mobileHidden: true },
    },
    {
      accessorKey: 'created_time',
      header: t('Created'),
      cell: ({ row }) => (
        <span className='text-muted-foreground block truncate font-mono text-xs tabular-nums'>
          {formatTimestampToDate(row.getValue('created_time'))}
        </span>
      ),
      size: 180,
      meta: { mobileHidden: true },
    },
    {
      accessorKey: 'accessed_time',
      header: t('Last Used'),
      cell: ({ row }) => {
        const accessedTime = row.getValue('accessed_time') as number
        if (!accessedTime) {
          return <span className='text-muted-foreground text-xs'>-</span>
        }
        return (
          <span className='text-muted-foreground block truncate font-mono text-xs tabular-nums'>
            {formatTimestampToDate(accessedTime)}
          </span>
        )
      },
      size: 180,
      meta: { mobileHidden: true },
    },
    {
      accessorKey: 'expired_time',
      header: t('Expires'),
      cell: ({ row }) => {
        const expiredTime = row.getValue('expired_time') as number
        if (expiredTime === -1) {
          return (
            <StatusBadge
              label={t('Never')}
              variant='neutral'
              copyable={false}
              className='-ml-1.5'
            />
          )
        }
        const isExpired = expiredTime * 1000 < Date.now()
        return (
          <span
            className={cn(
              'block truncate font-mono text-xs tabular-nums',
              isExpired ? 'text-destructive' : 'text-muted-foreground'
            )}
          >
            {formatTimestampToDate(expiredTime)}
          </span>
        )
      },
      size: 180,
      meta: { mobileHidden: true },
    },
    {
      id: 'actions',
      header: () => t('Actions'),
      cell: ({ row }) => <DataTableRowActions row={row} />,
      meta: { pinned: 'right' as const },
      size: 88,
    },
  ]
}
