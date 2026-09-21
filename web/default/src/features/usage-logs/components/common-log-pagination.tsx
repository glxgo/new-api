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
import { getRouteApi } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { PageFooterPortal } from '@/components/layout'

const route = getRouteApi('/_authenticated/usage-logs/$section')

export function CommonLogPagination(props: {
  hasMore: boolean
  nextCursor?: string
  loading: boolean
  pageSize: number
}) {
  const { t } = useTranslation()
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const page = search.page ?? 1
  const { page: _page, cursor: _cursor, ...filters } = search
  const scope = JSON.stringify({ ...filters, pageSize: props.pageSize })
  const [history, setHistory] = useState<{
    scope: string
    cursors: Record<number, string>
  }>({ scope: '', cursors: {} })
  const cursors = history.scope === scope ? history.cursors : { 1: '' }
  const go = (target: number, cursor?: string) => {
    setHistory({
      scope,
      cursors: {
        ...cursors,
        [page]: search.cursor ?? '',
        [target]: cursor ?? '',
      },
    })
    void navigate({
      search: { ...search, page: target, cursor: cursor || undefined },
    })
  }

  return (
    <PageFooterPortal>
      <div className='flex flex-wrap items-center justify-end gap-2 text-sm'>
        <label className='text-muted-foreground flex items-center gap-2'>
          {t('Rows per page')}
          <select
            aria-label={t('Rows per page')}
            className='bg-background h-8 rounded-md border px-2'
            value={props.pageSize}
            disabled={props.loading}
            onChange={(event) => {
              void navigate({
                search: {
                  ...search,
                  page: 1,
                  cursor: undefined,
                  pageSize: Number(event.target.value),
                },
              })
            }}
          >
            {[20, 50, 100].map((size) => (
              <option key={size} value={size}>
                {size}
              </option>
            ))}
          </select>
        </label>
        <Button
          variant='outline'
          size='sm'
          disabled={props.loading || page === 1}
          onClick={() => go(1)}
        >
          {t('Go to first page')}
        </Button>
        <Button
          variant='outline'
          size='sm'
          disabled={
            props.loading || page === 1 || cursors[page - 1] === undefined
          }
          onClick={() => go(page - 1, cursors[page - 1])}
        >
          {t('Go to previous page')}
        </Button>
        <span className='px-2 tabular-nums'>{page}</span>
        <Button
          variant='outline'
          size='sm'
          disabled={props.loading || !props.hasMore || !props.nextCursor}
          onClick={() => go(page + 1, props.nextCursor)}
        >
          {t('Go to next page')}
        </Button>
      </div>
    </PageFooterPortal>
  )
}
