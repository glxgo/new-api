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

interface UserBalance {
  quota?: number | null
  gift_quota?: number | null
}

function finiteQuota(value: number | null | undefined): number {
  const quota = Number(value ?? 0)
  return Number.isFinite(quota) ? quota : 0
}

/** Total API-spendable wallet balance: withdrawable principal plus gift quota. */
export function getUserAvailableBalance(user?: UserBalance | null): number {
  return finiteQuota(user?.quota) + finiteQuota(user?.gift_quota)
}
