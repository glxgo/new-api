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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Bell, CheckCheck } from 'lucide-react'
import dayjs from '@/lib/dayjs'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Markdown } from '@/components/ui/markdown'
import { SectionPageLayout } from '@/components/layout'
import { getUserAnnouncements, markUserAnnouncementsRead } from './api'
import { getAnnouncementTitle } from './types'

export function AnnouncementsPage() {
  const queryClient = useQueryClient()
  const query = useQuery({
    queryKey: ['user-announcements'],
    queryFn: getUserAnnouncements,
  })
  const data = query.data?.data
  const unreadIds = data?.unread.map((item) => item.id) ?? []
  const announcements = [...(data?.announcements ?? [])].sort(
    (left, right) =>
      dayjs(right.publishDate).valueOf() - dayjs(left.publishDate).valueOf()
  )
  const markRead = useMutation({
    mutationFn: () => markUserAnnouncementsRead(unreadIds),
    onSuccess: async () =>
      queryClient.invalidateQueries({ queryKey: ['user-announcements'] }),
  })

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>历史公告</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <Button
          variant='outline'
          disabled={unreadIds.length === 0 || markRead.isPending}
          onClick={() => markRead.mutate()}
        >
          <CheckCheck className='size-4' />
          全部标为已读
        </Button>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='mx-auto max-w-5xl space-y-3'>
          {query.isLoading ? (
            <div className='text-muted-foreground py-16 text-center'>
              加载中…
            </div>
          ) : announcements.length === 0 ? (
            <div className='text-muted-foreground flex flex-col items-center gap-3 rounded-2xl border py-16'>
              <Bell className='size-8' />
              暂无公告
            </div>
          ) : (
            announcements.map((item) => (
              <article key={item.id} className='bg-card rounded-2xl border p-5'>
                <div className='mb-4 flex flex-wrap items-center justify-between gap-2'>
                  <div className='flex items-center gap-2'>
                    <h2 className='font-semibold'>
                      {getAnnouncementTitle(item)}
                    </h2>
                    {item.unread && <Badge>未读</Badge>}
                  </div>
                  <time className='text-muted-foreground text-xs'>
                    {item.publishDate
                      ? dayjs(item.publishDate).format('YYYY-MM-DD HH:mm:ss')
                      : '发布时间未知'}
                  </time>
                </div>
                <Markdown>{item.content}</Markdown>
                {item.extra && (
                  <p className='text-muted-foreground mt-4 border-t pt-3 text-sm'>
                    {item.extra}
                  </p>
                )}
              </article>
            ))
          )}
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
