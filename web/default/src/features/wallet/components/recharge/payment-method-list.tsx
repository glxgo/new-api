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
import { Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { getPaymentIcon } from '../../lib'
import { getStandardPaymentMethods } from '../../lib/payment-methods'
import type { PaymentMethod, TopupInfo } from '../../types'

interface PaymentMethodListProps {
  topupInfo: TopupInfo | null
  topupAmount: number
  onPaymentMethodSelect: (method: PaymentMethod) => void
  paymentLoading: string | null
  hideEmpty?: boolean
}

// 标准支付方式列表：从 RechargeFormCard 抽出。仅渲染
// topupInfo.pay_methods；Creem / Waffo 由各自的专用流程渲染。
export function PaymentMethodList({
  topupInfo,
  topupAmount,
  onPaymentMethodSelect,
  paymentLoading,
  hideEmpty = false,
}: PaymentMethodListProps) {
  const { t } = useTranslation()
  const standardPaymentMethods = getStandardPaymentMethods(
    topupInfo?.pay_methods
  )
  const hasStandardPaymentMethods = standardPaymentMethods.length > 0

  return (
    <div className='space-y-2.5 sm:space-y-3'>
      <Label className='text-muted-foreground text-xs font-medium tracking-wider uppercase'>
        {t('Payment Method')}
      </Label>
      {hasStandardPaymentMethods ? (
        <div className='grid grid-cols-2 gap-1.5 sm:gap-3 lg:grid-cols-3'>
          {standardPaymentMethods.map((method) => {
            const methodMin = method.min_topup || 0
            const disabled = methodMin > topupAmount

            const button = (
              <Button
                key={method.type}
                variant='outline'
                onClick={() => onPaymentMethodSelect(method)}
                disabled={disabled || !!paymentLoading}
                className='h-9 min-w-0 justify-start gap-2 rounded-lg px-3'
              >
                {paymentLoading === method.type ? (
                  <Loader2 className='h-4 w-4 animate-spin' />
                ) : (
                  getPaymentIcon(
                    method.type,
                    'h-4 w-4',
                    method.icon,
                    method.name
                  )
                )}
                <span className='truncate'>{method.name}</span>
                {method.fee_rate && method.fee_rate > 0 ? (
                  <span className='text-muted-foreground shrink-0 text-[10px]'>
                    +{method.fee_rate}%
                  </span>
                ) : null}
              </Button>
            )

            return disabled ? (
              <TooltipProvider key={method.type}>
                <Tooltip>
                  <TooltipTrigger render={button}></TooltipTrigger>
                  <TooltipContent>
                    {t('Minimum topup amount: {{amount}}', {
                      amount: methodMin,
                    })}
                  </TooltipContent>
                </Tooltip>
              </TooltipProvider>
            ) : (
              button
            )
          })}
        </div>
      ) : hideEmpty ? null : (
        <Alert>
          <AlertDescription>
            {t('No payment methods available. Please contact administrator.')}
          </AlertDescription>
        </Alert>
      )}
    </div>
  )
}
