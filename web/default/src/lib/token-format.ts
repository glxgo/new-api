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
export interface TokenCountUnits {
  tenThousand: string
  hundredMillion: string
}

const DEFAULT_TOKEN_COUNT_UNITS: TokenCountUnits = {
  tenThousand: '10K',
  hundredMillion: '100M',
}

export function formatTokenCount(
  value: number | null | undefined,
  units: TokenCountUnits = DEFAULT_TOKEN_COUNT_UNITS,
  locale?: string
): string {
  if (value == null || !Number.isFinite(value)) return '-'

  const absoluteValue = Math.abs(value)
  if (absoluteValue >= 100_000_000) {
    return `${(value / 100_000_000).toFixed(2)}${units.hundredMillion}`
  }
  if (absoluteValue >= 10_000) {
    return `${(value / 10_000).toFixed(2)}${units.tenThousand}`
  }

  return Intl.NumberFormat(locale, { maximumFractionDigits: 0 }).format(value)
}
