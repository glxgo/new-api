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
import z from 'zod'
import { createFileRoute, redirect } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/auth-store'
import { ROLE } from '@/lib/roles'
import { Channels } from '@/features/channels'

const channelsSearchSchema = z.object({
  page: z.number().optional().catch(1),
  pageSize: z.number().optional().catch(undefined),
  filter: z.string().optional().catch(''),
  status: z.array(z.string()).optional().catch([]),
  type: z.array(z.string()).optional().catch([]),
  group: z.array(z.string()).optional().catch([]),
  model: z.string().optional().catch(''),
})

export const Route = createFileRoute('/_authenticated/channels/')({
  beforeLoad: () => {
    const { auth } = useAuthStore.getState()

    if (!auth.user || auth.user.role < ROLE.ADMIN) {
      throw redirect({
        to: '/403',
      })
    }
  },
  validateSearch: channelsSearchSchema,
  loaderDeps: ({ search }) => search,
  loader: ({ context, deps }) => {
    void import('@/features/channels/lib/channel-queries')
      .then(
        ({
          channelGroupsQueryOptions,
          channelListQueryOptions,
          firstActiveFilter,
          firstActiveNumberFilter,
          getDefaultChannelPageSize,
          readChannelListPreferences,
        }) => {
          const preferences = readChannelListPreferences()
          const pageSize = deps.pageSize ?? getDefaultChannelPageSize()

          void context.queryClient.prefetchQuery(
            channelListQueryOptions({
              keyword: deps.filter || undefined,
              model: deps.model || undefined,
              group: firstActiveFilter(deps.group),
              status: firstActiveFilter(deps.status),
              type: firstActiveNumberFilter(deps.type),
              tag_mode: preferences.tagMode,
              id_sort: preferences.idSort,
              p: deps.page ?? 1,
              page_size: pageSize,
            })
          )
          void context.queryClient.prefetchQuery(channelGroupsQueryOptions)
        }
      )
      .catch(() => undefined)
  },
  component: Channels,
})
