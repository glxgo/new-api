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
import { formatQuota } from '@/lib/format'
import type { ApiKey } from '../types'

export function ApiKeyUsageValue(props: {
  apiKey: ApiKey
  field: 'today_used_quota' | 'lifetime_used_quota'
}) {
  const { t } = useTranslation()
  if (props.apiKey.usage_state === 'loading') {
    return (
      <span className='text-muted-foreground text-xs' aria-busy='true'>
        {t('Loading...')}
      </span>
    )
  }
  if (props.apiKey.usage_state === 'error') {
    return (
      <span className='text-muted-foreground text-xs'>{t('Unavailable')}</span>
    )
  }
  return (
    <>
      {formatQuota(props.apiKey[props.field])}
      {props.apiKey.usage_state === 'stale' && (
        <span className='text-muted-foreground ml-1 text-xs'>
          ({t('Cached')})
        </span>
      )}
    </>
  )
}
