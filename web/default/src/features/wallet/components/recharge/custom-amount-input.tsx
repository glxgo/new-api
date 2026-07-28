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
import { useTranslation } from 'react-i18next'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { formatCurrency } from '../../lib'

interface CustomAmountInputProps {
  topupAmount: number
  onTopupAmountChange: (amount: number) => void
  paymentAmount: number
  calculating: boolean
  minTopup: number
}

// 自定义金额输入 + 实付展示 + min_topup 提示：从 RechargeFormCard 抽出。
// 受控：父级持有 topupAmount，内部保留输入框字符串本地态以便自由编辑。
export function CustomAmountInput({
  topupAmount,
  onTopupAmountChange,
  paymentAmount,
  calculating,
  minTopup,
}: CustomAmountInputProps) {
  const { t } = useTranslation()
  const [amountInput, setAmountInput] = useState(() => ({
    amount: topupAmount,
    text: topupAmount.toString(),
  }))
  if (amountInput.amount !== topupAmount) {
    setAmountInput({ amount: topupAmount, text: topupAmount.toString() })
  }
  const localAmount = amountInput.text

  const handleAmountChange = (value: string) => {
    const numValue = parseInt(value) || 0
    setAmountInput({ amount: numValue, text: value })
    if (numValue >= 0) {
      onTopupAmountChange(numValue)
    }
  }

  return (
    <div className='space-y-2.5 sm:space-y-3'>
      <Label
        htmlFor='topup-amount'
        className='text-muted-foreground text-xs font-medium tracking-wider uppercase'
      >
        {t('Custom Amount')}
      </Label>
      <div className='grid grid-cols-[minmax(0,1fr)_minmax(110px,0.55fr)] gap-2 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-center'>
        <Input
          id='topup-amount'
          type='number'
          value={localAmount}
          onChange={(e) => handleAmountChange(e.target.value)}
          min={minTopup}
          placeholder={`Minimum ${minTopup}`}
          className='h-9 text-base sm:h-10 sm:text-lg'
        />
        <div className='bg-muted/30 flex min-h-9 items-center justify-between gap-2 rounded-md border px-3 lg:min-w-52'>
          <span className='text-muted-foreground truncate text-xs'>
            {t('Amount to pay:')}
          </span>
          {calculating ? (
            <Skeleton className='h-5 w-16' />
          ) : (
            <span className='text-sm font-semibold'>
              {'¥'}
              {formatCurrency(paymentAmount)}
            </span>
          )}
        </div>
      </div>
      {Number(localAmount) > 0 && Number(localAmount) < minTopup && (
        <p className='text-xs text-amber-600 dark:text-amber-400'>
          {t('Minimum topup amount: {{amount}}', { amount: minTopup })}
        </p>
      )}
    </div>
  )
}
