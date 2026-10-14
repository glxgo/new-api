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
import type { KiroCreditPrice, PricingModel } from '../types'

export function getKiroCreditPrice(model: PricingModel, group: string) {
  if (!model.enable_groups?.includes(group)) return undefined
  return model.kiro_credit_prices?.[group]
}

export function getDefaultPricingGroup(model: PricingModel): string {
  const groups = model.enable_groups ?? []
  return (
    groups.find((group) => getKiroCreditPrice(model, group)) ?? groups[0] ?? ''
  )
}

// Kiro prices are already frozen in CNY by the backend. Never treat them as
// USD token prices or apply recharge/group multipliers a second time.
export function formatKiroCreditPrice(price: KiroCreditPrice): string {
  return `¥${price.effective_price_per_credit}`
}
