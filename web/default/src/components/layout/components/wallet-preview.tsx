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
*/
import { Link } from '@tanstack/react-router'
import { WalletCards } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth-store'
import { normalizeCurrencyConfig } from '@/stores/system-config-store'
import { getUserAvailableBalance } from '@/lib/user-balance'
import { cn } from '@/lib/utils'
import { useSystemConfig } from '@/hooks/use-system-config'

function formatPreviewBalance(quota: number, currency: unknown) {
  const safeCurrency = normalizeCurrencyConfig(currency)
  if (safeCurrency.quotaDisplayType === 'TOKENS') {
    return new Intl.NumberFormat(undefined, {
      notation: 'compact',
      maximumFractionDigits: 1,
    }).format(quota)
  }

  const amount =
    (quota / safeCurrency.quotaPerUnit) *
    (safeCurrency.quotaDisplayType === 'CNY'
      ? safeCurrency.usdExchangeRate
      : safeCurrency.quotaDisplayType === 'CUSTOM'
        ? safeCurrency.customCurrencyExchangeRate
        : 1)
  const formattedAmount = amount.toFixed(2)

  if (safeCurrency.quotaDisplayType === 'CNY') return `¥${formattedAmount}`
  if (safeCurrency.quotaDisplayType === 'CUSTOM') {
    return `${safeCurrency.customCurrencySymbol}${formattedAmount}`
  }
  return `$${formattedAmount}`
}

export function WalletPreview() {
  const { t } = useTranslation()
  const user = useAuthStore((state) => state.auth.user)
  const { currency } = useSystemConfig()

  const balance = formatPreviewBalance(getUserAvailableBalance(user), currency)

  return (
    <div
      className={cn(
        'border-warning/60 bg-warning/5 flex h-8 items-center overflow-hidden rounded-full border shadow-sm transition-all',
        'hover:border-warning hover:bg-warning/10 dark:bg-warning/10 dark:hover:bg-warning/15'
      )}
      aria-label={t('Wallet')}
    >
      <Link
        to='/wallet'
        className='text-foreground hover:bg-warning/10 flex h-full min-w-0 items-center gap-1.5 px-2.5 transition-colors sm:px-3'
      >
        <WalletCards className='text-warning size-4 shrink-0' />
        <span className='truncate font-mono text-[13px] font-semibold tabular-nums'>
          {balance}
        </span>
      </Link>
      <Link
        to='/wallet'
        search={{ show_recharge: true }}
        className='bg-warning text-warning-foreground hover:bg-warning/90 me-0.5 flex h-7 shrink-0 items-center rounded-full px-2.5 text-xs font-semibold transition-colors sm:px-3'
      >
        {t('Recharge')}
      </Link>
    </div>
  )
}
