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
import { useTranslation } from 'react-i18next'
import { formatNumber } from '@/lib/format'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import {
  calculatePresetPricing,
  formatCurrency,
  getDiscountLabel,
} from '../../lib'
import type { PresetAmount, TopupInfo } from '../../types'

interface PresetAmountGroupProps {
  presetAmounts: PresetAmount[]
  selectedPreset: number | null
  onSelectPreset: (preset: PresetAmount) => void
  topupInfo: TopupInfo | null
  priceRatio: number
  usdExchangeRate: number
}

// 预设档位组：从 RechargeFormCard 抽出，快捷充值弹框与原表单卡共用。
export function PresetAmountGroup({
  presetAmounts,
  selectedPreset,
  onSelectPreset,
  topupInfo,
  priceRatio,
  usdExchangeRate,
}: PresetAmountGroupProps) {
  const { t } = useTranslation()

  if (!presetAmounts || presetAmounts.length === 0) return null

  return (
    <div className='space-y-2.5 sm:space-y-3'>
      <Label className='text-muted-foreground text-xs font-medium tracking-wider uppercase'>
        {t('Amount')}
      </Label>
      <div className='grid grid-cols-2 gap-1.5 sm:gap-3 md:grid-cols-4'>
        {presetAmounts.map((preset, index) => {
          const discount =
            preset.discount || topupInfo?.discount?.[preset.value] || 1.0
          const { displayValue, actualPrice, savedAmount, hasDiscount } =
            calculatePresetPricing(
              preset.value,
              priceRatio,
              discount,
              usdExchangeRate
            )
          return (
            <Button
              key={index}
              variant='outline'
              className={cn(
                'flex min-h-16 flex-col items-start rounded-lg px-3 py-2.5 text-left whitespace-normal sm:min-h-[72px] sm:p-4',
                selectedPreset === preset.value
                  ? 'border-foreground bg-foreground/5 dark:border-foreground dark:bg-foreground/10'
                  : 'border-muted'
              )}
              onClick={() => onSelectPreset(preset)}
            >
              <div className='flex w-full items-center justify-between'>
                <div className='text-base font-semibold sm:text-lg'>
                  {'$'}
                  {formatNumber(displayValue)}
                </div>
                {hasDiscount && (
                  <div className='text-xs font-medium text-green-600'>
                    {getDiscountLabel(discount)}
                  </div>
                )}
              </div>
              <div className='text-muted-foreground mt-1.5 w-full text-xs sm:mt-2'>
                {t('Pay')} {'¥'}
                {formatCurrency(actualPrice)}
                {hasDiscount && savedAmount > 0 && (
                  <span className='text-green-600'>
                    {' '}
                    • 节省 {'¥'}
                    {formatCurrency(savedAmount)}
                  </span>
                )}
              </div>
            </Button>
          )
        })}
      </div>
    </div>
  )
}
