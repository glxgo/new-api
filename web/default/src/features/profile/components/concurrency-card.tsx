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
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Gauge, Loader2, Send, TimerReset } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Progress } from '@/components/ui/progress'
import { Textarea } from '@/components/ui/textarea'
import {
  createConcurrencyApplication,
  getSelfConcurrencyApplications,
} from '../api'
import { resolveCurrentRpm, resolveRpmLimit } from '../rpm'
import type { UserProfile } from '../types'

type Props = {
  profile: UserProfile | null
  loading: boolean
  compact?: boolean
}

export function ConcurrencyCard({ profile, loading, compact = false }: Props) {
  const queryClient = useQueryClient()
  const [open, setOpen] = useState(false)
  const [requestedLimit, setRequestedLimit] = useState(16)
  const [reason, setReason] = useState('')
  const [contact, setContact] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const { data } = useQuery({
    queryKey: ['self-concurrency-applications'],
    queryFn: getSelfConcurrencyApplications,
    enabled: !!profile,
  })
  const pending = data?.data?.items?.find((item) => item.status === 'pending')
  const limit = profile?.concurrency_limit || 8
  const current = profile?.current_concurrency || 0

  const openApplication = () => {
    setRequestedLimit(Math.max(limit + 1, limit * 2))
    setOpen(true)
  }

  const submit = async () => {
    if (requestedLimit <= limit) {
      toast.error('申请并发必须高于当前上限')
      return
    }
    if (reason.trim().length < 10 || contact.trim().length < 3) {
      toast.error('请填写至少 10 个字的理由和有效联系方式')
      return
    }
    setSubmitting(true)
    try {
      const result = await createConcurrencyApplication({
        requested_limit: requestedLimit,
        reason: reason.trim(),
        contact: contact.trim(),
      })
      if (!result.success) throw new Error(result.message || '提交失败')
      toast.success('申请已提交，管理员处理后会显示结果')
      setOpen(false)
      setReason('')
      await queryClient.invalidateQueries({
        queryKey: ['self-concurrency-applications'],
      })
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '提交失败')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <>
      <Card className='border-border/70 bg-background/75 overflow-hidden shadow-xs'>
        <CardHeader className={compact ? 'px-4 pt-4 pb-2' : 'pb-3'}>
          <CardTitle className='flex items-center gap-2 text-base'>
            <span className='bg-muted flex h-8 w-8 items-center justify-center rounded-lg border'>
              <Gauge className='h-4 w-4' />
            </span>
            API 并发额度
          </CardTitle>
        </CardHeader>
        <CardContent className={compact ? 'space-y-3 px-4 pb-4' : 'space-y-4'}>
          <div className='flex items-end justify-between'>
            <div>
              <div
                className={
                  compact
                    ? 'text-2xl font-semibold tracking-tight'
                    : 'text-3xl font-semibold tracking-tight'
                }
              >
                {loading ? '—' : limit}
              </div>
              <div className='text-muted-foreground text-xs'>账号并发上限</div>
            </div>
            <div className='text-right text-sm'>
              <div className='font-medium'>
                {current} / {limit}
              </div>
              <div className='text-muted-foreground text-xs'>当前使用</div>
            </div>
          </div>
          <Progress
            value={Math.min(100, (current / Math.max(1, limit)) * 100)}
          />
          {pending ? (
            <div className='rounded-lg border border-amber-500/25 bg-amber-500/10 px-3 py-2 text-xs text-amber-700 dark:text-amber-300'>
              已申请提升至 {pending.requested_limit}，等待管理员处理
            </div>
          ) : (
            <Button
              className='w-full'
              variant='outline'
              onClick={openApplication}
            >
              <Send className='mr-2 h-4 w-4' />
              申请提高并发
            </Button>
          )}
        </CardContent>
      </Card>

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className='sm:max-w-lg'>
          <DialogHeader>
            <DialogTitle>申请提高并发</DialogTitle>
            <DialogDescription>
              请说明真实使用场景。并发是同一时刻正在处理的请求数，不是每分钟请求数。
            </DialogDescription>
          </DialogHeader>
          <div className='space-y-4 py-2'>
            <div className='space-y-2'>
              <Label htmlFor='requested-concurrency'>期望并发数</Label>
              <Input
                id='requested-concurrency'
                type='number'
                min={limit + 1}
                max={10000}
                value={requestedLimit}
                onChange={(event) =>
                  setRequestedLimit(Number(event.target.value))
                }
              />
            </div>
            <div className='space-y-2'>
              <Label htmlFor='concurrency-reason'>申请理由</Label>
              <Textarea
                id='concurrency-reason'
                rows={4}
                maxLength={500}
                placeholder='例如：团队有 12 位成员同时使用 Codex，工作日高峰需要更高并发。'
                value={reason}
                onChange={(event) => setReason(event.target.value)}
              />
              <div className='text-muted-foreground text-right text-xs'>
                {reason.length}/500
              </div>
            </div>
            <div className='space-y-2'>
              <Label htmlFor='concurrency-contact'>联系方式</Label>
              <Input
                id='concurrency-contact'
                maxLength={120}
                placeholder='微信、QQ、邮箱或 Telegram'
                value={contact}
                onChange={(event) => setContact(event.target.value)}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant='outline' onClick={() => setOpen(false)}>
              取消
            </Button>
            <Button onClick={submit} disabled={submitting}>
              {submitting && <Loader2 className='mr-2 h-4 w-4 animate-spin' />}
              提交申请
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}

export function RpmCard({ profile, loading }: Omit<Props, 'compact'>) {
  const limit = resolveRpmLimit(profile)
  const current = resolveCurrentRpm(profile)

  return (
    <Card className='border-border/70 bg-background/75 overflow-hidden shadow-xs'>
      <CardHeader className='px-4 pt-4 pb-2'>
        <CardTitle className='flex items-center gap-2 text-base'>
          <span className='bg-muted flex h-8 w-8 items-center justify-center rounded-lg border'>
            <TimerReset className='h-4 w-4' />
          </span>
          API 每分钟请求
        </CardTitle>
      </CardHeader>
      <CardContent className='space-y-3 px-4 pb-4'>
        <div className='flex items-end justify-between'>
          <div>
            <div className='text-2xl font-semibold tracking-tight tabular-nums'>
              {loading ? '—' : limit}
            </div>
            <div className='text-muted-foreground text-xs'>账号 RPM 上限</div>
          </div>
          <div className='text-right text-sm'>
            <div className='font-medium tabular-nums'>
              {current ?? '—'} / {limit}
            </div>
            <div className='text-muted-foreground text-xs'>近 60 秒</div>
          </div>
        </div>
        <Progress
          value={
            current == null
              ? 0
              : Math.min(100, (current / Math.max(1, limit)) * 100)
          }
        />
        <p className='text-muted-foreground text-xs leading-5'>
          账号共享；上限自动按并发额度的 1.5 倍计算。
        </p>
      </CardContent>
    </Card>
  )
}
