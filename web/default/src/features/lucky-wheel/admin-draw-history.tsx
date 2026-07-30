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
import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  ChevronLeft,
  ChevronRight,
  Loader2,
  RotateCcw,
  Search,
  Trophy,
} from 'lucide-react'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { getLuckyAdminDraws, type LuckyAdminDrawFilters } from './api'
import type { LuckyAdminDraw } from './types'
import { PRIZE_NAMES } from './wheel-model'

const PAGE_SIZE = 10

const PRIZE_OPTIONS = [
  'quota_5',
  'quota_10',
  'quota_20',
  'quota_30',
  'quota_50',
  'quota_100',
  'gift_5',
  'gift_10',
  'gift_20',
  'subscription_double',
  'subscription_full_reset',
  'crazy_5h',
]

interface DrawSearchForm {
  keyword: string
  prizeType: string
  status: string
  startTime: string
  endTime: string
}

const EMPTY_FORM: DrawSearchForm = {
  keyword: '',
  prizeType: 'all',
  status: 'all',
  startTime: '',
  endTime: '',
}

function formatTime(timestamp: number) {
  if (!timestamp) return '—'
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  }).format(new Date(timestamp * 1000))
}

function formatUsd(micros: number) {
  return (micros / 1_000_000).toLocaleString('en-US', {
    minimumFractionDigits: 0,
    maximumFractionDigits: 2,
  })
}

function sourceLabel(sourceType: string) {
  const labels: Record<string, string> = {
    admin_compensation: '管理员补发',
    wallet_topup: '充值获得',
    subscription_purchase: '购买套餐',
    subscription_reset: '套餐周期赠送',
  }
  return labels[sourceType] || sourceType || '未知来源'
}

function statusMeta(status: string) {
  if (status === 'review_required') {
    return {
      label: '待人工审核',
      variant: 'destructive' as const,
    }
  }
  if (status === 'awarded') {
    return {
      label: '已发放',
      variant: 'secondary' as const,
    }
  }
  return {
    label: status || '未知',
    variant: 'outline' as const,
  }
}

function actualAwardText(draw: LuckyAdminDraw) {
  const amount =
    draw.actual_usd_micros > 0 ? `$${formatUsd(draw.actual_usd_micros)}` : ''
  if (draw.prize_type.startsWith('gift_')) {
    return `${amount || '—'} 钱包赠金`
  }
  if (draw.prize_type.startsWith('quota_')) {
    return `${amount || '—'} 套餐额度`
  }
  if (draw.prize_type === 'subscription_double') {
    return draw.reward_subscription_id
      ? `赠送订阅实例 #${draw.reward_subscription_id}`
      : '套餐双倍权益'
  }
  if (draw.prize_type === 'subscription_full_reset') {
    return draw.reward_subscription_id
      ? `新赠送周期 #${draw.reward_subscription_id}`
      : '套餐已全额重置'
  }
  if (draw.prize_type === 'crazy_5h') {
    return `${amount || '$600'} · 实例 #${draw.reward_subscription_id || '—'}`
  }
  return amount || '已发放'
}

function formToFilters(form: DrawSearchForm): LuckyAdminDrawFilters | null {
  const startTime = form.startTime
    ? Math.floor(new Date(form.startTime).getTime() / 1000)
    : undefined
  const endTime = form.endTime
    ? Math.floor(new Date(form.endTime).getTime() / 1000) + 59
    : undefined
  if (startTime && endTime && startTime > endTime) {
    return null
  }
  return {
    keyword: form.keyword.trim() || undefined,
    prize_type: form.prizeType === 'all' ? undefined : form.prizeType,
    status: form.status === 'all' ? undefined : form.status,
    start_time: startTime,
    end_time: endTime,
  }
}

export function LuckyDrawHistory() {
  const [form, setForm] = useState<DrawSearchForm>(EMPTY_FORM)
  const [activeFilters, setActiveFilters] = useState<LuckyAdminDrawFilters>({})
  const [items, setItems] = useState<LuckyAdminDraw[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(true)

  const pageCount = Math.max(1, Math.ceil(total / PAGE_SIZE))
  const awardedCount = useMemo(
    () => items.filter((item) => item.status === 'awarded').length,
    [items]
  )

  const load = useCallback(
    async (nextPage: number, filters: LuckyAdminDrawFilters) => {
      setLoading(true)
      try {
        const response = await getLuckyAdminDraws({
          ...filters,
          page: nextPage,
          page_size: PAGE_SIZE,
        })
        if (response.success) {
          setItems(response.data.items)
          setTotal(response.data.total)
          setPage(response.data.page)
        }
      } finally {
        setLoading(false)
      }
    },
    []
  )

  useEffect(() => {
    const timer = window.setTimeout(() => void load(1, activeFilters), 0)
    return () => window.clearTimeout(timer)
  }, [activeFilters, load])

  function search() {
    const filters = formToFilters(form)
    if (!filters) {
      toast.error('结束时间不能早于开始时间')
      return
    }
    if (JSON.stringify(filters) === JSON.stringify(activeFilters)) {
      void load(1, filters)
      return
    }
    setActiveFilters(filters)
  }

  function reset() {
    setForm(EMPTY_FORM)
    if (Object.keys(activeFilters).length === 0) {
      void load(1, {})
      return
    }
    setActiveFilters({})
  }

  return (
    <Card className='overflow-hidden'>
      <CardHeader className='border-b bg-[linear-gradient(110deg,hsl(var(--primary)/.08),transparent_58%)]'>
        <div className='flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between'>
          <div>
            <CardTitle className='flex items-center gap-2'>
              <Trophy className='text-primary size-5' />
              用户中奖记录
            </CardTitle>
            <p className='text-muted-foreground mt-1 text-sm'>
              查看谁在什么时候使用哪张幸运卡，以及奖品的真实到账结果。
            </p>
          </div>
          <div className='flex gap-2'>
            <div className='bg-background/80 rounded-xl border px-3 py-2'>
              <div className='font-mono text-lg font-semibold tabular-nums'>
                {total}
              </div>
              <div className='text-muted-foreground text-[11px]'>匹配记录</div>
            </div>
            <div className='bg-background/80 rounded-xl border px-3 py-2'>
              <div className='font-mono text-lg font-semibold tabular-nums'>
                {awardedCount}/{items.length}
              </div>
              <div className='text-muted-foreground text-[11px]'>
                本页已发放
              </div>
            </div>
          </div>
        </div>
      </CardHeader>
      <CardContent className='space-y-4 pt-5'>
        <div className='grid gap-3 md:grid-cols-2 xl:grid-cols-[minmax(12rem,1.2fr)_minmax(10rem,.8fr)_minmax(9rem,.7fr)_minmax(11rem,1fr)_minmax(11rem,1fr)_auto]'>
          <div className='space-y-2'>
            <Label htmlFor='lucky-draw-user-search'>用户</Label>
            <Input
              id='lucky-draw-user-search'
              value={form.keyword}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  keyword: event.target.value,
                }))
              }
              onKeyDown={(event) => {
                if (event.key === 'Enter') search()
              }}
              placeholder='用户 ID、用户名或显示名'
            />
          </div>
          <div className='space-y-2'>
            <Label>奖品</Label>
            <Select
              value={form.prizeType}
              onValueChange={(value) =>
                value !== null &&
                setForm((current) => ({
                  ...current,
                  prizeType: value,
                }))
              }
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value='all'>全部奖品</SelectItem>
                {PRIZE_OPTIONS.map((prize) => (
                  <SelectItem key={prize} value={prize}>
                    {PRIZE_NAMES[prize] || prize}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className='space-y-2'>
            <Label>状态</Label>
            <Select
              value={form.status}
              onValueChange={(value) =>
                value !== null &&
                setForm((current) => ({
                  ...current,
                  status: value,
                }))
              }
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value='all'>全部状态</SelectItem>
                <SelectItem value='awarded'>已发放</SelectItem>
                <SelectItem value='review_required'>待人工审核</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div className='space-y-2'>
            <Label htmlFor='lucky-draw-start-time'>开始时间</Label>
            <Input
              id='lucky-draw-start-time'
              type='datetime-local'
              value={form.startTime}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  startTime: event.target.value,
                }))
              }
            />
          </div>
          <div className='space-y-2'>
            <Label htmlFor='lucky-draw-end-time'>结束时间</Label>
            <Input
              id='lucky-draw-end-time'
              type='datetime-local'
              value={form.endTime}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  endTime: event.target.value,
                }))
              }
            />
          </div>
          <div className='flex items-end gap-2'>
            <Button disabled={loading} onClick={search}>
              <Search />
              查询
            </Button>
            <Button
              variant='outline'
              size='icon'
              disabled={loading}
              onClick={reset}
              aria-label='重置筛选'
            >
              <RotateCcw />
            </Button>
          </div>
        </div>

        <div className='overflow-hidden rounded-2xl border'>
          <Table className='min-w-[1120px]'>
            <TableHeader>
              <TableRow>
                <TableHead>抽奖时间</TableHead>
                <TableHead>用户</TableHead>
                <TableHead>抽中奖品</TableHead>
                <TableHead>实际发放</TableHead>
                <TableHead>幸运卡来源</TableHead>
                <TableHead className='text-right'>状态</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.map((draw) => {
                const meta = statusMeta(draw.status)
                const displayName = draw.display_name || draw.username
                return (
                  <TableRow key={draw.id}>
                    <TableCell>
                      <div className='font-mono text-xs'>
                        {formatTime(draw.awarded_at || draw.created_at)}
                      </div>
                      <div className='text-muted-foreground mt-1 text-[11px]'>
                        记录 #{draw.id}
                      </div>
                    </TableCell>
                    <TableCell>
                      <div className='font-medium'>
                        {displayName || `用户 #${draw.user_id}`}
                      </div>
                      <div className='text-muted-foreground mt-1 text-xs'>
                        #{draw.user_id}
                        {draw.username && displayName !== draw.username
                          ? ` · ${draw.username}`
                          : ''}
                      </div>
                    </TableCell>
                    <TableCell>
                      <div className='font-medium'>
                        {PRIZE_NAMES[draw.prize_type] || draw.prize_type}
                      </div>
                      <div className='text-muted-foreground mt-1 text-xs'>
                        规则版本 #{draw.rule_set_id}
                      </div>
                    </TableCell>
                    <TableCell>
                      <div className='font-medium'>{actualAwardText(draw)}</div>
                      {draw.reward_subscription_id > 0 &&
                        (draw.prize_type.startsWith('quota_') ||
                          draw.prize_type.startsWith('gift_')) && (
                          <div className='text-muted-foreground mt-1 text-xs'>
                            奖励订阅 #{draw.reward_subscription_id}
                          </div>
                        )}
                    </TableCell>
                    <TableCell>
                      <div className='flex items-center gap-2'>
                        <Badge variant='outline'>
                          {draw.pool_type === 'subscription'
                            ? '套餐卡'
                            : '充值卡'}
                        </Badge>
                        <span className='font-mono text-xs'>
                          卡 #{draw.card_id}
                        </span>
                      </div>
                      <div className='text-muted-foreground mt-1 max-w-72 truncate text-xs'>
                        {sourceLabel(draw.source_type)}
                        {draw.source_subscription_id > 0
                          ? ` · 来源订阅 #${draw.source_subscription_id}`
                          : ''}
                        {draw.source_ref ? ` · ${draw.source_ref}` : ''}
                      </div>
                    </TableCell>
                    <TableCell className='text-right'>
                      <Badge variant={meta.variant}>{meta.label}</Badge>
                    </TableCell>
                  </TableRow>
                )
              })}
              {!loading && items.length === 0 && (
                <TableRow>
                  <TableCell
                    colSpan={6}
                    className='text-muted-foreground h-32 text-center'
                  >
                    没有符合当前条件的中奖记录
                  </TableCell>
                </TableRow>
              )}
              {loading && (
                <TableRow>
                  <TableCell colSpan={6} className='h-32 text-center'>
                    <span className='text-muted-foreground inline-flex items-center gap-2 text-sm'>
                      <Loader2 className='size-4 animate-spin' />
                      正在读取中奖记录
                    </span>
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </div>

        <div className='flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between'>
          <div className='text-muted-foreground text-xs'>
            共 {total} 条，每页 {PAGE_SIZE} 条；中奖与发放数据均来自服务端记录。
          </div>
          <div className='flex items-center justify-end gap-2'>
            <Button
              size='sm'
              variant='outline'
              disabled={loading || page <= 1}
              onClick={() => void load(page - 1, activeFilters)}
            >
              <ChevronLeft />
              上一页
            </Button>
            <span className='text-muted-foreground min-w-20 text-center text-xs'>
              {page} / {pageCount}
            </span>
            <Button
              size='sm'
              variant='outline'
              disabled={loading || page >= pageCount}
              onClick={() => void load(page + 1, activeFilters)}
            >
              下一页
              <ChevronRight />
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}
