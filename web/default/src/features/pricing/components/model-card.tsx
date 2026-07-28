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
import { memo, useMemo, useState } from 'react'
import { ChevronRight } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { getLobeIcon } from '@/lib/lobe-icon'
import { cn } from '@/lib/utils'
import { StatusBadge } from '@/components/status-badge'
import { DEFAULT_TOKEN_UNIT } from '../constants'
import { getDynamicPricingSummary } from '../lib/dynamic-price'
import { isTokenBasedModel } from '../lib/model-helpers'
import {
  formatFixedPrice,
  formatGroupPrice,
  formatOfficialPrice,
  formatPrice,
  formatRequestPrice,
  stripTrailingZeros,
} from '../lib/price'
import type { PriceType, PricingModel, TokenUnit } from '../types'

export interface ModelCardProps {
  model: PricingModel
  onClick: () => void
  priceRate?: number
  usdExchangeRate?: number
  tokenUnit?: TokenUnit
  showRechargePrice?: boolean
  groupRatios?: Record<string, number>
  perf?: unknown
}

type PriceItem = {
  label: string
  value: string | null
  original?: string | null
  unit: string
}

function formatContext(length?: number): string | null {
  if (!length || length <= 0) return null
  if (length >= 1000) return `>${Math.round(length / 1000)}K`
  return String(length)
}

function formatRatio(ratio: number): string {
  return Number.isInteger(ratio)
    ? ratio.toString()
    : ratio.toFixed(3).replace(/0+$/, '').replace(/\.$/, '')
}

function PriceBlock({ item }: { item: PriceItem }) {
  return (
    <div className='min-h-[82px] px-3 py-3 sm:px-4'>
      <div className='text-foreground/90 flex items-center gap-1.5 text-xs font-semibold'>
        <span className='bg-muted-foreground/70 h-4 w-0.5 rounded-full' />
        {item.label}
      </div>
      <div className='mt-1.5 flex items-baseline gap-1'>
        <span className='font-mono text-[22px] leading-none font-bold tracking-tight'>
          {item.value ?? '—'}
        </span>
        {item.value && (
          <span className='text-muted-foreground text-xs'>/{item.unit}</span>
        )}
      </div>
      {item.original && (
        <div className='text-muted-foreground mt-1 text-xs'>
          原价{' '}
          <span className='decoration-muted-foreground/50 font-mono line-through'>
            {item.original}
          </span>
          /{item.unit}
        </div>
      )}
    </div>
  )
}

export const ModelCard = memo(function ModelCard(props: ModelCardProps) {
  const { t } = useTranslation()
  const tokenUnit = props.tokenUnit ?? DEFAULT_TOKEN_UNIT
  const priceRate = props.priceRate ?? 1
  const usdExchangeRate = props.usdExchangeRate ?? 1
  const showRechargePrice = props.showRechargePrice ?? false
  const isTokenBased = isTokenBasedModel(props.model)
  const tokenUnitLabel = tokenUnit === 'K' ? 'K' : 'M'
  const requestUnitLabel = t('request')
  const modelIconKey = props.model.icon || props.model.vendor_icon
  const modelIcon = modelIconKey ? getLobeIcon(modelIconKey, 18) : null
  const initial = props.model.model_name?.charAt(0).toUpperCase() || '?'
  const contextLabel = formatContext(props.model.context_length)
  const groupRatios = props.groupRatios ?? props.model.group_ratio ?? {}
  const groups = useMemo(
    () => (props.model.enable_groups?.length ? props.model.enable_groups : []),
    [props.model.enable_groups]
  )
  const [selectedGroup, setSelectedGroup] = useState(groups[0] ?? '')
  const currentGroup = groups.includes(selectedGroup)
    ? selectedGroup
    : (groups[0] ?? '')
  const currentRatio = currentGroup ? (groupRatios[currentGroup] ?? 1) : 1
  const isDynamicPricing =
    props.model.billing_mode === 'tiered_expr' &&
    Boolean(props.model.billing_expr)
  const dynamicSummary = isDynamicPricing
    ? getDynamicPricingSummary(props.model, {
        tokenUnit,
        showRechargePrice,
        priceRate,
        usdExchangeRate,
        groupRatioMultiplier: currentRatio,
      })
    : null

  const getDynamicEntry = (field: string) =>
    dynamicSummary && !dynamicSummary.isSpecialExpression
      ? dynamicSummary.entries.find((entry) => entry.field === field)?.formatted
      : null

  const groupPrice = (type: PriceType) =>
    currentGroup
      ? formatGroupPrice(
          props.model,
          currentGroup,
          type,
          tokenUnit,
          showRechargePrice,
          priceRate,
          usdExchangeRate,
          groupRatios
        )
      : formatPrice(
          props.model,
          type,
          tokenUnit,
          showRechargePrice,
          priceRate,
          usdExchangeRate
        )

  const priceItems: PriceItem[] = isTokenBased
    ? [
        {
          label: t('Input'),
          value: stripTrailingZeros(
            getDynamicEntry('inputPrice') ?? groupPrice('input')
          ),
          original:
            props.model.official_input !== undefined
              ? stripTrailingZeros(
                  formatOfficialPrice(
                    props.model,
                    'input',
                    tokenUnit,
                    showRechargePrice,
                    priceRate,
                    usdExchangeRate
                  )
                )
              : null,
          unit: tokenUnitLabel,
        },
        {
          label: t('Output'),
          value: stripTrailingZeros(
            getDynamicEntry('outputPrice') ?? groupPrice('output')
          ),
          original:
            props.model.official_output !== undefined
              ? stripTrailingZeros(
                  formatOfficialPrice(
                    props.model,
                    'output',
                    tokenUnit,
                    showRechargePrice,
                    priceRate,
                    usdExchangeRate
                  )
                )
              : null,
          unit: tokenUnitLabel,
        },
        {
          label: `${t('Cache Write')} 5min`,
          value:
            props.model.create_cache_ratio == null
              ? null
              : stripTrailingZeros(
                  getDynamicEntry('cacheCreatePrice') ??
                    groupPrice('create_cache')
                ),
          original:
            props.model.official_cache_write !== undefined
              ? stripTrailingZeros(String(props.model.official_cache_write))
              : null,
          unit: tokenUnitLabel,
        },
        {
          label: t('Cached'),
          value:
            props.model.cache_ratio == null
              ? null
              : stripTrailingZeros(
                  getDynamicEntry('cacheReadPrice') ?? groupPrice('cache')
                ),
          original:
            props.model.official_cache_read !== undefined
              ? stripTrailingZeros(String(props.model.official_cache_read))
              : null,
          unit: tokenUnitLabel,
        },
      ]
    : [
        {
          label: t('Per Request'),
          value: stripTrailingZeros(
            currentGroup
              ? formatFixedPrice(
                  props.model,
                  currentGroup,
                  showRechargePrice,
                  priceRate,
                  usdExchangeRate,
                  groupRatios
                )
              : formatRequestPrice(
                  props.model,
                  showRechargePrice,
                  priceRate,
                  usdExchangeRate
                )
          ),
          original:
            props.model.official_request_price !== undefined
              ? stripTrailingZeros(String(props.model.official_request_price))
              : null,
          unit: requestUnitLabel,
        },
      ]

  return (
    <div
      className={cn(
        'group bg-card text-card-foreground relative flex min-h-[250px] flex-col overflow-hidden rounded-xl border shadow-sm transition-all duration-200',
        'hover:border-primary/30 hover:-translate-y-0.5 hover:shadow-md'
      )}
    >
      {contextLabel && (
        <span className='border-chart-4/30 bg-chart-4/10 text-chart-4 absolute top-3 right-3 rounded-full border px-2 py-0.5 font-mono text-xs font-semibold'>
          {contextLabel}
        </span>
      )}

      <div className='flex min-h-[88px] items-start gap-3 px-4 pt-4 pb-3'>
        <div className='bg-background flex size-9 shrink-0 items-center justify-center rounded-xl border shadow-sm'>
          {modelIcon || (
            <span className='text-muted-foreground text-sm font-bold'>
              {initial}
            </span>
          )}
        </div>
        <div className='min-w-0 flex-1 pr-16'>
          <h3 className='truncate font-mono text-base leading-6 font-bold'>
            {props.model.model_name}
          </h3>
          <div className='mt-1.5 flex flex-wrap items-center gap-1.5'>
            <span className='border-success/30 bg-success/10 text-success inline-flex items-center rounded-full border px-2.5 py-1 text-xs font-bold'>
              {isTokenBased ? t('Token-based') : t('Per Request')}
            </span>
            <span className='border-info/30 bg-info/10 text-info inline-flex items-center rounded-full border px-2.5 py-1 text-xs font-bold'>
              倍率 {formatRatio(currentRatio)}
            </span>
            {dynamicSummary?.isSpecialExpression && (
              <StatusBadge
                label={t('Special billing expression')}
                variant='warning'
                copyable={false}
                size='sm'
              />
            )}
          </div>
        </div>
        <button
          type='button'
          onClick={props.onClick}
          className='text-muted-foreground hover:text-foreground hover:bg-muted absolute top-12 right-3 inline-flex items-center gap-1 rounded-md border px-2 py-1 text-xs transition-colors'
        >
          {t('Details')}
          <ChevronRight className='size-3.5' />
        </button>
      </div>

      <div
        className={cn(
          'grid border-y',
          priceItems.length === 1 ? 'grid-cols-1' : 'grid-cols-2'
        )}
      >
        {priceItems.map((item, index) => (
          <div
            key={`${item.label}-${index}`}
            className={cn(
              index % 2 === 1 && 'border-l',
              index > 1 && 'border-t'
            )}
          >
            <PriceBlock item={item} />
          </div>
        ))}
      </div>

      <div className='text-muted-foreground mt-auto flex min-h-12 flex-wrap items-center gap-1.5 px-4 py-3 text-xs'>
        <span className='mr-1'>{t('Channel Price')}</span>
        {groups.length > 0 ? (
          groups.map((group) => {
            const ratio = groupRatios[group] ?? 1
            const active = group === currentGroup
            return (
              <button
                key={group}
                type='button'
                onClick={() => setSelectedGroup(group)}
                className={cn(
                  'inline-flex max-w-full items-center rounded-full border px-2.5 py-1 font-semibold transition-colors',
                  active
                    ? 'border-success/30 bg-success/10 text-success'
                    : 'border-border bg-background text-muted-foreground hover:text-foreground'
                )}
                title={group}
              >
                <span className='truncate'>{group}</span>
                <span className='ml-1'>×{formatRatio(ratio)}</span>
              </button>
            )
          })
        ) : (
          <span className='rounded-full border px-2.5 py-1 font-medium'>
            默认渠道 ×1
          </span>
        )}
      </div>
    </div>
  )
})
