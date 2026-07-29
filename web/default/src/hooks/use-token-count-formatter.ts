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
import { useCallback, useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { formatTokenCount, type TokenCountUnits } from '@/lib/token-format'

export function useTokenCountFormatter() {
  const { t, i18n } = useTranslation()
  const units = useMemo<TokenCountUnits>(
    () => ({
      tenThousand: t('Ten Thousand Token Unit'),
      hundredMillion: t('Hundred Million Token Unit'),
    }),
    [t]
  )

  return useCallback(
    (value: number | null | undefined) =>
      formatTokenCount(value, units, i18n.resolvedLanguage),
    [i18n.resolvedLanguage, units]
  )
}
