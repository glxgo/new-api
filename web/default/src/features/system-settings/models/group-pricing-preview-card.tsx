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
import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { StaticDataTable } from '@/components/data-table'
import { getGroupPricingPreview } from '../api'
import { useSystemOptions } from '../hooks/use-system-options'

type PreviewItem = {
  model: string
  billing_mode: string
  has_price: boolean
  base_input_price_per_m?: number
  final_input_price_per_m?: number
  base_request_price?: number
  final_request_price?: number
  gross_margin?: number
  enabled_channel_count: number
  total_channel_count: number
  statuses: string[]
}

function formatMoney(n: number | undefined | null): string {
  if (n === undefined || n === null) return '-'
  if (n === 0) return '0'
  if (Math.abs(n) >= 1) return `$${n.toFixed(4)}`
  return `$${n.toFixed(6)}`
}

// GroupPricingPreviewCard previews sale pricing. Cost now depends on the final
// selected channel and therefore cannot be represented by one group-level value.
export function GroupPricingPreviewCard() {
  const { t } = useTranslation()
  const { data: resp } = useSystemOptions()

  const groups = useMemo(() => {
    const opt = resp?.data?.find((o) => o.key === 'GroupRatio')
    if (!opt) return ['default']
    try {
      const parsed = JSON.parse(opt.value) as Record<string, number>
      const keys = Object.keys(parsed)
      return keys.length > 0 ? keys : ['default']
    } catch {
      return ['default']
    }
  }, [resp])

  const [selectedGroup, setSelectedGroup] = useState('default')
  const [search, setSearch] = useState('')
  const [includeDisabled, setIncludeDisabled] = useState(false)
  const [statusFilter, setStatusFilter] = useState<string>('all')

  const { data, isLoading, isError } = useQuery({
    queryKey: ['group-pricing-preview', selectedGroup, includeDisabled],
    queryFn: () => getGroupPricingPreview(selectedGroup, includeDisabled),
    enabled: groups.length > 0,
  })

  const preview = data?.data
  const isAuto = preview?.is_auto === true

  const filteredItems = useMemo(() => {
    const items = (preview?.items ?? []) as PreviewItem[]
    return items.filter((item) => {
      if (
        search.trim() &&
        !item.model.toLowerCase().includes(search.trim().toLowerCase())
      ) {
        return false
      }
      if (statusFilter !== 'all' && !item.statuses.includes(statusFilter)) {
        return false
      }
      return true
    })
  }, [preview, search, statusFilter])

  const saleRatio = preview?.sale_ratio

  return (
    <Card className='shadow-sm ring-0'>
      <CardHeader className='bg-muted/20 border-b'>
        <div>
          <CardTitle>{t('Group model pricing preview')}</CardTitle>
          <CardDescription>
            {t(
              'Verify each model actual sale price under the selected group. Platform cost is configured per channel.'
            )}
          </CardDescription>
        </div>
      </CardHeader>
      <CardContent className='space-y-4 pt-4'>
        <div className='flex flex-wrap items-end gap-4'>
          <div className='space-y-1'>
            <Label className='text-xs'>{t('Group')}</Label>
            <select
              className='bg-background h-9 rounded-md border px-3 text-sm'
              value={selectedGroup}
              onChange={(e) => setSelectedGroup(e.target.value)}
            >
              {groups.map((g) => (
                <option key={g} value={g}>
                  {g}
                  {g === 'auto' ? ` (${t('auto')})` : ''}
                </option>
              ))}
            </select>
          </div>
          <div className='space-y-1'>
            <Label className='text-xs'>{t('Search model')}</Label>
            <Input
              className='h-9 w-56'
              placeholder={t('Model name')}
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
          </div>
          <div className='space-y-1'>
            <Label className='text-xs'>{t('Status')}</Label>
            <select
              className='bg-background h-9 rounded-md border px-3 text-sm'
              value={statusFilter}
              onChange={(e) => setStatusFilter(e.target.value)}
            >
              <option value='all'>{t('All')}</option>
              <option value='normal'>{t('Normal')}</option>
              <option value='missing_price'>{t('Missing price')}</option>
              <option value='no_enabled_channel'>
                {t('No available channel')}
              </option>
            </select>
          </div>
          <div className='flex items-center gap-2 pb-2'>
            <Switch
              checked={includeDisabled}
              onCheckedChange={setIncludeDisabled}
              id='include-disabled'
            />
            <Label htmlFor='include-disabled' className='text-sm'>
              {t('Include disabled channels')}
            </Label>
          </div>
        </div>

        {!isAuto && saleRatio !== undefined && (
          <div className='text-muted-foreground flex flex-wrap gap-x-6 gap-y-1 text-sm'>
            <span>
              {t('Sale ratio')}: <strong>{saleRatio}</strong>
            </span>
          </div>
        )}

        {isLoading && (
          <p className='text-muted-foreground text-sm'>{t('Loading...')}</p>
        )}
        {isError && (
          <p className='text-destructive text-sm'>
            {t('Failed to load preview')}
          </p>
        )}

        {isAuto ? (
          <p className='text-muted-foreground rounded-md border border-dashed p-4 text-sm'>
            {preview?.message ||
              t(
                'auto is an automatic group; select a concrete group to view final prices.'
              )}
          </p>
        ) : (
          !isLoading &&
          !isError && (
            <StaticDataTable
              data={filteredItems}
              getRowKey={(row) => row.model}
              emptyContent={t('No models found.')}
              columns={[
                {
                  id: 'model',
                  header: t('Model'),
                  className: 'min-w-40',
                  cell: (row) => (
                    <span className='font-medium'>{row.model}</span>
                  ),
                },
                {
                  id: 'mode',
                  header: t('Billing mode'),
                  className: 'w-28',
                  cell: (row) => (
                    <span className='text-muted-foreground text-xs'>
                      {row.billing_mode === 'fixed'
                        ? t('Per request')
                        : row.billing_mode === 'expression'
                          ? t('Expression')
                          : t('Per token')}
                    </span>
                  ),
                },
                {
                  id: 'base',
                  header: t('Official price'),
                  className: 'w-28',
                  cell: (row) => (
                    <span className='text-sm'>
                      {row.billing_mode === 'fixed'
                        ? formatMoney(row.base_request_price)
                        : row.billing_mode === 'expression'
                          ? '-'
                          : `${formatMoney(row.base_input_price_per_m)}/M`}
                    </span>
                  ),
                },
                {
                  id: 'sale',
                  header: t('Actual sale price'),
                  className: 'w-28',
                  cell: (row) => {
                    const sale =
                      row.billing_mode === 'fixed'
                        ? undefined
                        : row.billing_mode === 'expression'
                          ? undefined
                          : (row as PreviewItem).final_input_price_per_m
                    const finalRequest = (row as PreviewItem)
                      .final_request_price
                    return (
                      <span className='text-sm'>
                        {row.billing_mode === 'fixed'
                          ? formatMoney(finalRequest)
                          : row.billing_mode === 'expression'
                            ? '-'
                            : `${formatMoney(sale)}/M`}
                      </span>
                    )
                  },
                },
                {
                  id: 'channels',
                  header: t('Channels'),
                  className: 'w-24',
                  cell: (row) => (
                    <span className='text-muted-foreground text-xs'>
                      {includeDisabled
                        ? `${row.total_channel_count} (${row.enabled_channel_count} ${t('enabled')})`
                        : `${row.enabled_channel_count}`}
                    </span>
                  ),
                },
                {
                  id: 'status',
                  header: t('Status'),
                  className: 'w-32',
                  cell: (row) => {
                    const hasMissing = row.statuses.includes('missing_price')
                    const hasNoChannel =
                      row.statuses.includes('no_enabled_channel')
                    return (
                      <span
                        className={`text-xs ${hasMissing || hasNoChannel ? 'text-warning' : 'text-muted-foreground'}`}
                      >
                        {row.statuses
                          .map((s) =>
                            s === 'normal'
                              ? t('Normal')
                              : s === 'loss'
                                ? t('Channel-dependent cost')
                                : s === 'missing_price'
                                  ? t('Missing price')
                                  : s === 'no_enabled_channel'
                                    ? t('No available channel')
                                    : s === 'only_disabled_channels'
                                      ? t('Disabled channels only')
                                      : s === 'cost_ratio_inherited'
                                        ? t('Inherit sale ratio')
                                        : s === 'group_ratio_default'
                                          ? t('Default ratio')
                                          : s
                          )
                          .join(', ')}
                      </span>
                    )
                  },
                },
              ]}
            />
          )
        )}

        <p className='text-muted-foreground text-xs'>
          平台成本取决于请求最终成功渠道，本分组售价预览不再展示单一成本或毛利。
        </p>
      </CardContent>
    </Card>
  )
}
