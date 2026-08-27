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
import type { CPAAccountUsage } from './types'

export const PLATFORM_ACCOUNT_PAGE_SIZE = 8

export function getPlatformAccountPage(
  accounts: CPAAccountUsage[],
  requestedPage: number
) {
  const totalPages = Math.max(
    1,
    Math.ceil(accounts.length / PLATFORM_ACCOUNT_PAGE_SIZE)
  )
  const currentPage = Math.min(
    Math.max(1, Math.floor(requestedPage) || 1),
    totalPages
  )
  return {
    currentPage,
    totalPages,
    accounts: accounts.slice(
      (currentPage - 1) * PLATFORM_ACCOUNT_PAGE_SIZE,
      currentPage * PLATFORM_ACCOUNT_PAGE_SIZE
    ),
  }
}

export function metricCardSurfaceClass(accent: boolean) {
  return accent ? 'border-emerald-500/20 bg-white dark:bg-card' : 'bg-card'
}
