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
import { ChevronRight } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { getLobeIcon } from '@/lib/lobe-icon'
import {
  BadgeListCell,
  DataTableColumnHeader,
} from '@/components/data-table'
import { StatusBadge } from '@/components/status-badge'
import { DEFAULT_TOKEN_UNIT } from '../constants'
import {
  getDynamicDisplayGroupRatio,
  getDynamicPricingSummary,
} from '../lib/dynamic-price'
import { parseTags } from '../lib/filters'
import { isTokenBasedModel } from '../lib/model-helpers'
import {
  formatPrice,
  formatRequestPrice,
  stripTrailingZeros,
} from '../lib/price'
import type { PricingModel, TokenUnit } from '../types'

// ----------------------------------------------------------------------------
// Pricing Table Columns（krill-ai 风格：模型/能力/上下文/输入/输出/缓存/操作）
// ----------------------------------------------------------------------------

export interface PricingColumnsOptions {
  tokenUnit?: TokenUnit
  priceRate?: number
  usdExchangeRate?: number
  showRechargePrice?: boolean
  onModelClick?: (modelName: string) => void
}

function formatContext(length?: number): string {
  if (!length || length <= 0) return '—'
  if (length >= 1000) return `${Math.round(length / 1000)}K`
  return String(length)
}

function PriceCell({ value, label }: { value: string | null; label: string }) {
  return value ? (
    <div className='min-w-0'>
      <span className='font-mono text-sm tabular-nums'>{value}</span>
      <div className='text-muted-foreground/50 text-[10px]'>/ {label}</div>
    </div>
  ) : (
    <span className='text-muted-foreground/30 text-xs'>—</span>
  )
}

export function usePricingColumns(
  options: PricingColumnsOptions = {}
): ColumnDef<PricingModel>[] {
  const { t } = useTranslation()
  const {
    tokenUnit = DEFAULT_TOKEN_UNIT,
    priceRate = 1,
    usdExchangeRate = 1,
    showRechargePrice = false,
    onModelClick,
  } = options

  const tokenUnitLabel = tokenUnit === 'K' ? '1K' : '1M'

  // 计算单个价格字段（input/output/cache）的显示值，统一处理 dynamic / token / request
  const fieldPrice = (
    model: PricingModel,
    field: 'input' | 'output' | 'cache'
  ): string | null => {
    const dynamicSummary = getDynamicPricingSummary(model, {
      tokenUnit,
      showRechargePrice,
      priceRate,
      usdExchangeRate,
      groupRatioMultiplier: getDynamicDisplayGroupRatio(model),
    })
    if (dynamicSummary) {
      if (dynamicSummary.isSpecialExpression) return null
      const fieldMap: Record<typeof field, string> = {
        input: 'promptPrice',
        output: 'completionPrice',
        cache: 'cacheReadPrice',
      }
      const entry = dynamicSummary.entries.find(
        (e) => e.field === fieldMap[field]
      )
      return entry ? stripTrailingZeros(entry.formatted) : null
    }
    if (!isTokenBasedModel(model)) {
      // 按次计费模型：只在 output 列显示单价
      if (field === 'output') {
        return stripTrailingZeros(
          formatRequestPrice(
            model,
            showRechargePrice,
            priceRate,
            usdExchangeRate
          )
        )
      }
      return null
    }
    if (field === 'cache' && model.cache_ratio == null) return null
    return stripTrailingZeros(
      formatPrice(
        model,
        field,
        tokenUnit,
        showRechargePrice,
        priceRate,
        usdExchangeRate
      )
    )
  }

  return [
    // Model column（logo + 名字 + 描述/厂商）
    {
      accessorKey: 'model_name',
      meta: { label: t('Model') },
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Model')} />
      ),
      cell: ({ row }) => {
        const model = row.original
        const modelIconKey = model.icon || model.vendor_icon
        const modelIcon = modelIconKey ? getLobeIcon(modelIconKey, 18) : null
        const initial = model.model_name?.charAt(0).toUpperCase() || '?'
        return (
          <div className='flex max-w-full min-w-0 items-center gap-2.5'>
            <div className='bg-muted/50 flex size-9 shrink-0 items-center justify-center rounded-lg'>
              {modelIcon || (
                <span className='text-muted-foreground text-sm font-bold'>
                  {initial}
                </span>
              )}
            </div>
            <div className='min-w-0'>
              <div className='truncate font-mono text-sm font-semibold'>
                {model.model_name}
              </div>
              {(model.description || model.vendor_name) && (
                <div className='text-muted-foreground truncate text-xs'>
                  {model.description || model.vendor_name}
                </div>
              )}
            </div>
          </div>
        )
      },
      minSize: 240,
    },

    // Capabilities column（tags + endpoints 合并为能力 pill）
    {
      id: 'capabilities',
      header: t('Capabilities'),
      cell: ({ row }) => {
        const tags = parseTags(row.original.tags)
        const endpoints = row.original.supported_endpoint_types || []
        const items = [...endpoints, ...tags].slice(0, 4)
        return (
          <BadgeListCell
            items={items.map((item) => (
              <StatusBadge
                key={item}
                label={item}
                autoColor={item}
                size='sm'
                copyable={false}
              />
            ))}
          />
        )
      },
      size: 220,
      enableSorting: false,
    },

    // Context column（上下文长度）
    {
      accessorKey: 'context_length',
      header: t('Context'),
      cell: ({ row }) => (
        <span className='text-muted-foreground font-mono text-xs tabular-nums'>
          {formatContext(row.original.context_length)}
        </span>
      ),
      size: 90,
      enableSorting: false,
    },

    // Input price column
    {
      id: 'input_price',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Input')} />
      ),
      cell: ({ row }) => (
        <PriceCell value={fieldPrice(row.original, 'input')} label={tokenUnitLabel} />
      ),
      size: 110,
      enableSorting: false,
    },

    // Output price column
    {
      id: 'output_price',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Output')} />
      ),
      cell: ({ row }) => {
        const isRequest =
          !isTokenBasedModel(row.original) &&
          !getDynamicPricingSummary(row.original, {
            tokenUnit,
            showRechargePrice,
            priceRate,
            usdExchangeRate,
            groupRatioMultiplier: getDynamicDisplayGroupRatio(row.original),
          })
        return (
          <PriceCell
            value={fieldPrice(row.original, 'output')}
            label={isRequest ? t('request') : tokenUnitLabel}
          />
        )
      },
      size: 110,
      enableSorting: false,
    },

    // Cached price column
    {
      id: 'cached_price',
      header: t('Cached'),
      cell: ({ row }) => (
        <PriceCell value={fieldPrice(row.original, 'cache')} label={tokenUnitLabel} />
      ),
      size: 110,
      enableSorting: false,
    },

    // Action column（详情按钮）
    {
      id: 'action',
      header: () => <span className='text-muted-foreground font-medium text-xs'>{t('Action')}</span>,
      cell: ({ row }) => (
        <button
          type='button'
          onClick={(e) => {
            e.stopPropagation()
            onModelClick?.(row.original.model_name)
          }}
          className='text-muted-foreground hover:text-foreground hover:bg-muted hover:border-primary/40 inline-flex items-center gap-1 rounded-md border px-2 py-1 text-xs transition-colors'
        >
          {t('Details')}
          <ChevronRight className='size-3.5' />
        </button>
      ),
      size: 100,
      enableSorting: false,
    },
  ]
}
