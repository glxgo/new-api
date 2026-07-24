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
export const BILLING_SECTION_IDS = [
  'quota',
  'currency',
  'model-pricing',
  'group-pricing',
  'payment',
  'checkin',
  'dividend',
] as const

export type BillingSectionId = (typeof BILLING_SECTION_IDS)[number]
export const BILLING_DEFAULT_SECTION: BillingSectionId = 'quota'
export const BILLING_SECTION_TITLES = {
  quota: 'Quota Settings',
  currency: 'Currency & Display',
  'model-pricing': 'Model Pricing',
  'group-pricing': 'Group Pricing',
  payment: 'Payment Gateway',
  checkin: 'Check-in Rewards',
  dividend: 'Dividend & Rebate',
} satisfies Record<BillingSectionId, string>
