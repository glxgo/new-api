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
import { Check, Clock3, Contact, Loader2, X } from 'lucide-react'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import {
  getConcurrencyApplications,
  reviewConcurrencyApplication,
} from '../api'
import type { AdminConcurrencyApplication } from '../types'

export function ConcurrencyApplicationsPanel() {
  const queryClient = useQueryClient()
  const [limits, setLimits] = useState<Record<number, number>>({})
  const [notes, setNotes] = useState<Record<number, string>>({})
  const { data, isLoading } = useQuery({
    queryKey: ['admin-concurrency-applications', 'pending'],
    queryFn: () => getConcurrencyApplications('pending'),
  })
  const applications = data?.data?.items || []
  const mutation = useMutation({
    mutationFn: async ({
      item,
      approve,
    }: {
      item: AdminConcurrencyApplication
      approve: boolean
    }) => {
      const result = await reviewConcurrencyApplication(item.id, {
        approve,
        approved_limit: limits[item.id] || item.requested_limit,
        admin_note: notes[item.id]?.trim(),
      })
      if (!result.success) throw new Error(result.message || '处理失败')
      return result
    },
    onSuccess: async () => {
      toast.success('申请已处理')
      await queryClient.invalidateQueries({
        queryKey: ['admin-concurrency-applications'],
      })
      await queryClient.invalidateQueries({ queryKey: ['users'] })
    },
    onError: (error) =>
      toast.error(error instanceof Error ? error.message : '处理失败'),
  })

  if (isLoading) {
    return (
      <div className='flex h-52 items-center justify-center'>
        <Loader2 className='h-6 w-6 animate-spin' />
      </div>
    )
  }

  if (applications.length === 0) {
    return (
      <div className='bg-muted/20 flex min-h-64 flex-col items-center justify-center rounded-xl border border-dashed text-center'>
        <Clock3 className='text-muted-foreground mb-3 h-8 w-8' />
        <div className='font-medium'>暂无待处理的并发申请</div>
        <div className='text-muted-foreground mt-1 text-sm'>
          用户提交后会显示在这里
        </div>
      </div>
    )
  }

  return (
    <div className='grid gap-4 xl:grid-cols-2'>
      {applications.map((item) => (
        <Card key={item.id} className='border-violet-500/15'>
          <CardHeader className='pb-3'>
            <div className='flex items-start justify-between gap-3'>
              <div>
                <CardTitle className='text-base'>
                  {item.username || `用户 #${item.user_id}`}
                </CardTitle>
                <div className='text-muted-foreground mt-1 text-xs'>
                  申请编号 #{item.id} ·{' '}
                  {new Date(item.created_at * 1000).toLocaleString()}
                </div>
              </div>
              <Badge variant='secondary'>待处理</Badge>
            </div>
          </CardHeader>
          <CardContent className='space-y-4'>
            <div className='bg-muted/55 grid grid-cols-[1fr_auto_1fr] items-center gap-3 rounded-xl px-4 py-3 text-center'>
              <div>
                <div className='text-xl font-semibold'>
                  {item.current_limit}
                </div>
                <div className='text-muted-foreground text-xs'>当前</div>
              </div>
              <div className='text-muted-foreground'>→</div>
              <div>
                <div className='text-xl font-semibold text-violet-500'>
                  {item.requested_limit}
                </div>
                <div className='text-muted-foreground text-xs'>申请</div>
              </div>
            </div>
            <div>
              <div className='text-muted-foreground mb-1 text-xs'>申请理由</div>
              <p className='text-sm leading-6'>{item.reason}</p>
            </div>
            <div className='flex items-center gap-2 rounded-lg border px-3 py-2 text-sm'>
              <Contact className='text-muted-foreground h-4 w-4' />
              {item.contact}
            </div>
            <div className='grid gap-3 sm:grid-cols-[150px_1fr]'>
              <div className='space-y-1.5'>
                <Label>批准并发数</Label>
                <Input
                  type='number'
                  min={1}
                  max={10000}
                  value={limits[item.id] || item.requested_limit}
                  onChange={(e) =>
                    setLimits((old) => ({
                      ...old,
                      [item.id]: Number(e.target.value),
                    }))
                  }
                />
              </div>
              <div className='space-y-1.5'>
                <Label>管理员备注（可选）</Label>
                <Textarea
                  rows={2}
                  maxLength={500}
                  value={notes[item.id] || ''}
                  onChange={(e) =>
                    setNotes((old) => ({ ...old, [item.id]: e.target.value }))
                  }
                />
              </div>
            </div>
            <div className='flex justify-end gap-2'>
              <Button
                variant='outline'
                disabled={mutation.isPending}
                onClick={() => mutation.mutate({ item, approve: false })}
              >
                <X className='mr-2 h-4 w-4' />
                拒绝
              </Button>
              <Button
                disabled={mutation.isPending}
                onClick={() => mutation.mutate({ item, approve: true })}
              >
                <Check className='mr-2 h-4 w-4' />
                批准
              </Button>
            </div>
          </CardContent>
        </Card>
      ))}
    </div>
  )
}
