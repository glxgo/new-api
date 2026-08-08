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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { BellRing } from 'lucide-react'
import { toast } from 'sonner'
import dayjs from '@/lib/dayjs'
import { cn } from '@/lib/utils'
import { buttonVariants, Button } from '@/components/ui/button'
import { Markdown } from '@/components/ui/markdown'
import { Dialog } from '@/components/dialog'
import { getUserAnnouncements, markUserAnnouncementsRead } from './api'
import { getAnnouncementTitle } from './types'

export function UnreadAnnouncementDialog() {
  const queryClient = useQueryClient()
  const [dismissedSignature, setDismissedSignature] = useState('')
  const query = useQuery({
    queryKey: ['user-announcements'],
    queryFn: getUserAnnouncements,
    staleTime: 30_000,
  })
  const unread = [...(query.data?.data?.unread ?? [])].sort(
    (left, right) =>
      dayjs(right.publishDate).valueOf() - dayjs(left.publishDate).valueOf()
  )
  const unreadSignature = unread.map((item) => item.id).join(',')
  const open = unread.length > 0 && dismissedSignature !== unreadSignature
  const markRead = useMutation({
    mutationFn: () => markUserAnnouncementsRead(unread.map((item) => item.id)),
    onSuccess: async () => {
      setDismissedSignature(unreadSignature)
      await queryClient.invalidateQueries({ queryKey: ['user-announcements'] })
    },
    onError: () => toast.error('公告已读状态保存失败，请稍后重试'),
  })

  if (!query.data?.data?.enabled || unread.length === 0) return null

  return (
    <Dialog
      open={open}
      onOpenChange={(nextOpen) => {
        if (!nextOpen) setDismissedSignature(unreadSignature)
      }}
      title={
        <span className='flex items-center gap-2'>
          <BellRing className='size-5 text-amber-500' />
          未读公告（{unread.length}）
        </span>
      }
      description='这是你尚未读过的全部公告，确认后本账号不再重复弹出。'
      contentClassName='sm:max-w-2xl'
      contentHeight='min(78vh, 46rem)'
      footer={
        <div className='flex w-full items-center justify-between gap-3'>
          <Link
            to='/announcements'
            className={cn(buttonVariants({ variant: 'ghost' }))}
          >
            查看历史公告
          </Link>
          <Button
            onClick={() => markRead.mutate()}
            disabled={markRead.isPending}
          >
            全部已读
          </Button>
        </div>
      }
    >
      <div className='space-y-3 py-2'>
        {unread.map((item) => (
          <article key={item.id} className='rounded-xl border p-4'>
            <div className='text-muted-foreground mb-2 flex items-center justify-between gap-3 text-xs'>
              <span className='text-foreground font-medium'>
                {getAnnouncementTitle(item)}
              </span>
              <time>
                {item.publishDate
                  ? dayjs(item.publishDate).format('YYYY-MM-DD HH:mm')
                  : '发布时间未知'}
              </time>
            </div>
            <Markdown>{item.content}</Markdown>
            {item.extra && (
              <p className='text-muted-foreground mt-3 text-sm'>{item.extra}</p>
            )}
          </article>
        ))}
      </div>
    </Dialog>
  )
}
