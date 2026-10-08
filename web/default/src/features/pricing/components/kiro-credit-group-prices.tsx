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
import { formatKiroCreditPrice, getKiroCreditPrice } from '../lib/kiro-price'
import type { PricingModel } from '../types'

export function KiroCreditGroupPrices(props: { model: PricingModel }) {
  const { t } = useTranslation()
  const groups = props.model.enable_groups.filter((group) =>
    getKiroCreditPrice(props.model, group)
  )
  return (
    <section className='space-y-3'>
      <h3 className='text-sm font-semibold'>{t('Kiro Credit Price')}</h3>
      {groups.map((group) => {
        const quote = getKiroCreditPrice(props.model, group)!
        return (
          <div
            key={group}
            className='bg-muted/20 flex flex-wrap items-center justify-between gap-2 rounded-lg border p-3'
          >
            <div className='min-w-0'>
              <p className='text-sm font-medium break-all'>{group}</p>
              <p className='text-muted-foreground text-xs'>
                {t('Credit-based')} · ×{quote.group_ratio}
              </p>
            </div>
            <p className='font-mono text-lg font-semibold'>
              {formatKiroCreditPrice(quote)}
              <span className='text-muted-foreground ml-1 text-xs font-normal'>
                / {t('credit')}
              </span>
            </p>
          </div>
        )
      })}
      <p className='text-muted-foreground text-xs'>
        {t(
          'Billed from the upstream Kiro credit receipt. The price already includes the group multiplier.'
        )}
      </p>
    </section>
  )
}
