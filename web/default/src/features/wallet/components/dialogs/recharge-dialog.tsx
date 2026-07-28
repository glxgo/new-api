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
import { Info, Receipt } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Dialog } from '@/components/dialog'
import { useRedemption } from '../../hooks/use-redemption'
import type {
  CreemProduct,
  PaymentMethod,
  PresetAmount,
  TopupInfo,
  WaffoPayMethod,
} from '../../types'
import { RechargeFormCard } from '../recharge-form-card'

interface RechargeDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  topupInfo: TopupInfo | null
  presetAmounts: PresetAmount[]
  selectedPreset: number | null
  onSelectPreset: (preset: PresetAmount) => void
  topupAmount: number
  onTopupAmountChange: (amount: number) => void
  paymentAmount: number
  calculating: boolean
  onPaymentMethodSelect: (method: PaymentMethod) => void
  paymentLoading: string | null
  priceRatio?: number
  usdExchangeRate?: number
  onOpenBilling?: () => void
  onCreemProductSelect: (product: CreemProduct) => void
  onWaffoMethodSelect: (method: WaffoPayMethod, index: number) => void
}

export function RechargeDialog({
  open,
  onOpenChange,
  topupInfo,
  presetAmounts,
  selectedPreset,
  onSelectPreset,
  topupAmount,
  onTopupAmountChange,
  paymentAmount,
  calculating,
  onPaymentMethodSelect,
  paymentLoading,
  priceRatio = 1,
  usdExchangeRate = 1,
  onOpenBilling,
  onCreemProductSelect,
  onWaffoMethodSelect,
}: RechargeDialogProps) {
  const { t } = useTranslation()
  const [redemptionCode, setRedemptionCode] = useState('')
  const { redeeming, redeemCode } = useRedemption()

  const handleRedeem = async () => {
    const ok = await redeemCode(redemptionCode)
    if (ok) {
      setRedemptionCode('')
      onOpenChange(false)
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={t('Quick Top Up')}
      description={t('Choose an amount and payment method')}
      contentClassName='max-sm:w-[calc(100vw-1.5rem)] sm:max-w-2xl'
      contentHeight='min(85vh, 54rem)'
      bodyClassName='space-y-4'
    >
      <div className='space-y-4 py-3 sm:space-y-6 sm:py-4'>
        <RechargeFormCard
          topupInfo={topupInfo}
          presetAmounts={presetAmounts}
          selectedPreset={selectedPreset}
          onSelectPreset={onSelectPreset}
          topupAmount={topupAmount}
          onTopupAmountChange={onTopupAmountChange}
          paymentAmount={paymentAmount}
          calculating={calculating}
          onPaymentMethodSelect={onPaymentMethodSelect}
          paymentLoading={paymentLoading}
          redemptionCode={redemptionCode}
          onRedemptionCodeChange={setRedemptionCode}
          onRedeem={handleRedeem}
          redeeming={redeeming}
          topupLink={topupInfo?.topup_link}
          priceRatio={priceRatio}
          usdExchangeRate={usdExchangeRate}
          onOpenBilling={onOpenBilling}
          creemProducts={topupInfo?.creem_products}
          enableCreemTopup={topupInfo?.enable_creem_topup}
          onCreemProductSelect={onCreemProductSelect}
          enableWaffoTopup={topupInfo?.enable_waffo_topup}
          waffoPayMethods={topupInfo?.waffo_pay_methods}
          waffoMinTopup={topupInfo?.waffo_min_topup}
          onWaffoMethodSelect={onWaffoMethodSelect}
          enableWaffoPancakeTopup={topupInfo?.enable_waffo_pancake_topup}
        />

        <Alert>
          <Info className='h-4 w-4' />
          <AlertDescription>
            {t('Consumption is billed in USD. Recharge ratio 1¥ : 1$.')}
          </AlertDescription>
        </Alert>
        <Alert>
          <Receipt className='h-4 w-4' />
          <AlertDescription>
            {t('For invoice service after topup, contact group admin.')}
          </AlertDescription>
        </Alert>
        <Alert>
          <AlertDescription>
            {t(
              'If unsatisfied, the balance can be refunded at "Withdraw"; the buyer covers the payment channel fees.'
            )}
          </AlertDescription>
        </Alert>
      </div>
    </Dialog>
  )
}
