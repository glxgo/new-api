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
import { useMemo, useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { CalendarClock, Pencil, Plus, Trash2 } from 'lucide-react'
import { toast } from 'sonner'
import dayjs from '@/lib/dayjs'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import {
  getVirtualMembershipResetCalendar,
  deleteAdminVirtualMembershipResetCalendarEntry,
  saveAdminVirtualMembershipResetCalendarEntry,
} from '@/features/virtual-membership/api'
import type { VirtualMembershipResetCalendarEntry } from '@/features/virtual-membership/types'

function currentMonth() {
  const parts = new Intl.DateTimeFormat('en-US', {
    timeZone: 'Asia/Shanghai',
    year: 'numeric',
    month: 'numeric',
  }).formatToParts(new Date())
  return {
    year: Number(parts.find((part) => part.type === 'year')?.value),
    month: Number(parts.find((part) => part.type === 'month')?.value),
  }
}

function shanghaiInputValue(timestamp: number) {
  const parts = new Intl.DateTimeFormat('en-US', {
    timeZone: 'Asia/Shanghai',
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hourCycle: 'h23',
  }).formatToParts(new Date(timestamp * 1000))
  const value = (type: string) =>
    parts.find((part) => part.type === type)?.value ?? ''
  return `${value('year')}-${value('month')}-${value('day')}T${value('hour')}:${value('minute')}`
}

function parseShanghaiInput(value: string) {
  if (!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/.test(value)) return null
  const timestamp = Date.parse(`${value}:00+08:00`)
  if (!Number.isFinite(timestamp)) return null
  const normalized = shanghaiInputValue(Math.floor(timestamp / 1000))
  return normalized === value ? Math.floor(timestamp / 1000) : null
}

function displayTime(timestamp: number) {
  return new Intl.DateTimeFormat('zh-CN', {
    timeZone: 'Asia/Shanghai',
    dateStyle: 'medium',
    timeStyle: 'short',
    hourCycle: 'h23',
  }).format(new Date(timestamp * 1000))
}

export function ResetCalendarAdminPanel() {
  const initial = useMemo(() => currentMonth(), [])
  const [year, setYear] = useState(initial.year)
  const [month, setMonth] = useState(initial.month)
  const [editing, setEditing] =
    useState<VirtualMembershipResetCalendarEntry | null>(null)
  const [resetAt, setResetAt] = useState('')
  const [count, setCount] = useState('1')
  const [reason, setReason] = useState('')
  const [saving, setSaving] = useState(false)
  const queryClient = useQueryClient()
  const query = useQuery({
    queryKey: ['admin-virtual-membership-reset-calendar', year, month],
    queryFn: () => getVirtualMembershipResetCalendar(year, month, true),
  })
  const entries = [...(query.data?.data?.entries ?? [])].sort(
    (left, right) => right.reset_at - left.reset_at
  )
  const resetForm = () => {
    setEditing(null)
    setResetAt('')
    setCount('1')
    setReason('')
  }
  const editEntry = (entry: VirtualMembershipResetCalendarEntry) => {
    setEditing(entry)
    setResetAt(shanghaiInputValue(entry.reset_at))
    setCount(String(entry.count))
    setReason(entry.reason)
  }
  const save = async () => {
    const parsedTime = parseShanghaiInput(resetAt)
    const parsedCount = Number(count)
    if (
      !resetAt ||
      parsedTime === null ||
      !Number.isSafeInteger(parsedCount) ||
      parsedCount <= 0 ||
      !reason.trim()
    ) {
      toast.error('请填写有效的时间、次数和理由')
      return
    }
    setSaving(true)
    try {
      const result = await saveAdminVirtualMembershipResetCalendarEntry({
        id: editing?.id,
        reset_at: parsedTime,
        count: parsedCount,
        reason: reason.trim(),
      })
      if (!result.success) {
        toast.error(result.message || '保存重置记录失败')
        return
      }
      toast.success(editing ? '重置记录已更新' : '重置记录已添加')
      resetForm()
      await queryClient.invalidateQueries({
        queryKey: ['admin-virtual-membership-reset-calendar'],
      })
      await queryClient.invalidateQueries({
        queryKey: ['virtual-membership-reset-calendar'],
      })
    } finally {
      setSaving(false)
    }
  }
  const remove = async (entry: VirtualMembershipResetCalendarEntry) => {
    if (
      !window.confirm(`确认删除 ${displayTime(entry.reset_at)} 的重置记录吗？`)
    )
      return
    const result = await deleteAdminVirtualMembershipResetCalendarEntry(
      entry.id
    )
    if (!result.success) {
      toast.error(result.message || '删除重置记录失败')
      return
    }
    toast.success('重置记录已删除')
    await queryClient.invalidateQueries({
      queryKey: ['admin-virtual-membership-reset-calendar'],
    })
    await queryClient.invalidateQueries({
      queryKey: ['virtual-membership-reset-calendar'],
    })
  }
  const chooseMonth = (value: string) => {
    const parsed = dayjs(`${value}-01`)
    if (parsed.isValid()) {
      setYear(parsed.year())
      setMonth(parsed.month() + 1)
      resetForm()
    }
  }
  return (
    <section className='bg-card rounded-2xl border p-5'>
      <div className='flex flex-wrap items-start justify-between gap-3'>
        <div className='flex items-center gap-2'>
          <CalendarClock className='size-5 text-blue-600' />
          <div>
            <h2 className='font-semibold'>重置日历维护</h2>
            <p className='text-muted-foreground mt-1 text-xs'>
              这里填写的是对外展示的重置次数，不等同于点击重置按钮次数。
            </p>
          </div>
        </div>
        <Input
          type='month'
          className='w-44'
          value={`${year}-${String(month).padStart(2, '0')}`}
          onChange={(event) => chooseMonth(event.target.value)}
        />
      </div>
      <div className='mt-5 grid gap-5 xl:grid-cols-[minmax(0,1fr)_22rem]'>
        <div className='space-y-2'>
          {query.isLoading ? (
            <p className='text-muted-foreground py-6 text-center text-sm'>
              加载中…
            </p>
          ) : entries.length === 0 ? (
            <p className='text-muted-foreground rounded-xl border border-dashed p-8 text-center text-sm'>
              本月还没有录入重置记录
            </p>
          ) : (
            entries.map((entry) => (
              <div
                key={entry.id}
                className='flex items-start gap-3 rounded-xl border px-3 py-3'
              >
                <div className='min-w-0 flex-1'>
                  <p className='text-sm font-medium'>
                    {displayTime(entry.reset_at)} · {entry.count} 次
                  </p>
                  <p className='text-muted-foreground mt-1 text-xs leading-5'>
                    {entry.reason}
                  </p>
                </div>
                <Button
                  variant='ghost'
                  size='icon-sm'
                  onClick={() => editEntry(entry)}
                  aria-label='编辑'
                >
                  <Pencil className='size-3.5' />
                </Button>
                <Button
                  variant='ghost'
                  size='icon-sm'
                  className='text-destructive'
                  onClick={() => void remove(entry)}
                  aria-label='删除'
                >
                  <Trash2 className='size-3.5' />
                </Button>
              </div>
            ))
          )}
        </div>
        <div className='bg-muted/25 rounded-xl border p-4'>
          <h3 className='flex items-center gap-2 text-sm font-semibold'>
            {editing ? (
              <Pencil className='size-4 text-blue-600' />
            ) : (
              <Plus className='size-4 text-blue-600' />
            )}
            {editing ? '编辑记录' : '新增记录'}
          </h3>
          <div className='mt-3 space-y-3'>
            <label className='block text-xs'>
              <span className='text-muted-foreground mb-1 block'>
                重置时间（北京时间）
              </span>
              <Input
                type='datetime-local'
                value={resetAt}
                onChange={(event) => setResetAt(event.target.value)}
              />
            </label>
            <label className='block text-xs'>
              <span className='text-muted-foreground mb-1 block'>重置次数</span>
              <Input
                type='number'
                min={1}
                step={1}
                value={count}
                onChange={(event) => setCount(event.target.value)}
              />
            </label>
            <label className='block text-xs'>
              <span className='text-muted-foreground mb-1 block'>重置理由</span>
              <Textarea
                className='min-h-24'
                value={reason}
                onChange={(event) => setReason(event.target.value)}
                placeholder='例如：官方额度周期调整'
              />
            </label>
            <div className='flex gap-2'>
              <Button
                className='flex-1'
                onClick={() => void save()}
                disabled={saving}
              >
                {editing ? '保存修改' : '添加记录'}
              </Button>
              {editing && (
                <Button variant='outline' onClick={resetForm}>
                  取消
                </Button>
              )}
            </div>
          </div>
        </div>
      </div>
    </section>
  )
}
