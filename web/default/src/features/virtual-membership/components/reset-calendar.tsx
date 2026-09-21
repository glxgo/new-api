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
import { useQuery } from '@tanstack/react-query'
import { CalendarDays, ChevronLeft, ChevronRight, Clock3 } from 'lucide-react'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { getVirtualMembershipResetCalendar } from '../api'
import type { VirtualMembershipResetCalendarEntry } from '../types'

const weekdays = ['日', '一', '二', '三', '四', '五', '六']

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

function shanghaiDate(timestamp: number, withTime = false) {
  return new Intl.DateTimeFormat('zh-CN', {
    timeZone: 'Asia/Shanghai',
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    ...(withTime
      ? { hour: '2-digit', minute: '2-digit', hourCycle: 'h23' as const }
      : {}),
  }).format(new Date(timestamp * 1000))
}

export function ResetCalendar() {
  const initial = useMemo(() => currentMonth(), [])
  const [year, setYear] = useState(initial.year)
  const [month, setMonth] = useState(initial.month)
  const [selectedDay, setSelectedDay] = useState<number | null>(null)
  const query = useQuery({
    queryKey: ['virtual-membership-reset-calendar', year, month],
    queryFn: () => getVirtualMembershipResetCalendar(year, month),
  })
  const data = query.data?.data
  const entries = data?.entries ?? []
  const entriesByDay = useMemo(() => {
    const map = new Map<number, VirtualMembershipResetCalendarEntry[]>()
    entries.forEach((entry) => {
      const parts = new Intl.DateTimeFormat('en-CA', {
        timeZone: 'Asia/Shanghai',
        year: 'numeric',
        month: 'numeric',
        day: 'numeric',
      }).formatToParts(new Date(entry.reset_at * 1000))
      const entryYear = Number(
        parts.find((part) => part.type === 'year')?.value
      )
      const entryMonth = Number(
        parts.find((part) => part.type === 'month')?.value
      )
      const entryDay = Number(parts.find((part) => part.type === 'day')?.value)
      if (entryYear === year && entryMonth === month)
        map.set(entryDay, [...(map.get(entryDay) ?? []), entry])
    })
    return map
  }, [entries, month, year])
  const firstWeekday = new Date(Date.UTC(year, month - 1, 1)).getUTCDay()
  const daysInMonth = new Date(Date.UTC(year, month, 0)).getUTCDate()
  const selectedEntries = selectedDay
    ? (entriesByDay.get(selectedDay) ?? [])
    : []
  const moveMonth = (delta: number) => {
    const absoluteMonth = year * 12 + month - 1 + delta
    setYear(Math.floor(absoluteMonth / 12))
    setMonth((absoluteMonth % 12) + 1)
    setSelectedDay(null)
  }
  return (
    <section className='bg-card overflow-hidden rounded-2xl border'>
      <header className='flex flex-wrap items-center justify-between gap-3 border-b px-4 py-4 sm:px-5'>
        <div className='flex items-center gap-2'>
          <CalendarDays className='size-4 text-blue-600' />
          <div>
            <h2 className='font-semibold'>重置日历</h2>
            <p className='text-muted-foreground mt-1 text-xs'>
              蓝色日期表示管理员记录了额度重置
            </p>
          </div>
        </div>
        <div className='flex items-center gap-2'>
          <Button
            variant='outline'
            size='icon-sm'
            onClick={() => moveMonth(-1)}
            aria-label='上个月'
          >
            <ChevronLeft className='size-4' />
          </Button>
          <span className='min-w-24 text-center text-sm font-semibold tabular-nums'>
            {year} 年 {month} 月
          </span>
          <Button
            variant='outline'
            size='icon-sm'
            onClick={() => moveMonth(1)}
            aria-label='下个月'
          >
            <ChevronRight className='size-4' />
          </Button>
        </div>
      </header>
      <div className='p-4 sm:p-5'>
        <div className='mb-4 flex items-baseline justify-between gap-3'>
          <span className='text-muted-foreground text-xs'>本月共重置</span>
          <strong className='text-2xl font-semibold text-blue-600 tabular-nums'>
            {data?.total_count ?? 0}
            <span className='ml-1 text-sm font-normal'>次</span>
          </strong>
        </div>
        {query.isLoading ? (
          <Skeleton className='h-64 w-full rounded-xl' />
        ) : (
          <div className='overflow-hidden rounded-xl border'>
            <div className='bg-muted/35 grid grid-cols-7 border-b'>
              {weekdays.map((day) => (
                <div
                  key={day}
                  className='text-muted-foreground py-2 text-center text-xs font-medium'
                >
                  {day}
                </div>
              ))}
            </div>
            <div className='grid grid-cols-7'>
              {Array.from(
                { length: firstWeekday + daysInMonth },
                (_, index) => {
                  const day = index - firstWeekday + 1
                  if (day < 1)
                    return (
                      <div
                        key={`empty-${index}`}
                        className='bg-muted/10 min-h-16 border-r border-b sm:min-h-20'
                      />
                    )
                  const dayEntries = entriesByDay.get(day) ?? []
                  const active = dayEntries.length > 0
                  return (
                    <button
                      key={day}
                      type='button'
                      onClick={() => active && setSelectedDay(day)}
                      className={cn(
                        'relative min-h-16 border-r border-b p-2 text-left transition-colors last:border-r-0 sm:min-h-20',
                        active
                          ? 'cursor-pointer bg-blue-500/8 hover:bg-blue-500/15'
                          : 'text-muted-foreground hover:bg-muted/30'
                      )}
                    >
                      <span
                        className={cn(
                          'inline-flex size-7 items-center justify-center rounded-full text-sm font-medium',
                          active && 'bg-blue-600 text-white'
                        )}
                      >
                        {day}
                      </span>
                      {active && (
                        <span className='absolute right-2 bottom-2 text-[10px] font-semibold text-blue-600'>
                          {dayEntries.reduce(
                            (sum, entry) => sum + entry.count,
                            0
                          )}{' '}
                          次
                        </span>
                      )}
                    </button>
                  )
                }
              )}
            </div>
          </div>
        )}
        {selectedDay !== null && (
          <div className='mt-4 rounded-xl border border-blue-500/20 bg-blue-500/5 p-4'>
            <div className='flex items-center justify-between gap-3'>
              <h3 className='font-medium'>
                {month} 月 {selectedDay} 日重置详情
              </h3>
              <Button
                variant='ghost'
                size='sm'
                onClick={() => setSelectedDay(null)}
              >
                收起
              </Button>
            </div>
            <div className='mt-3 space-y-2'>
              {selectedEntries.map((entry) => (
                <div
                  key={entry.id}
                  className='bg-background flex flex-wrap items-start gap-3 rounded-lg border px-3 py-2.5 text-sm'
                >
                  <Clock3 className='mt-0.5 size-4 shrink-0 text-blue-600' />
                  <div className='min-w-0 flex-1'>
                    <p className='font-medium'>
                      {shanghaiDate(entry.reset_at, true)} · {entry.count} 次
                    </p>
                    <p className='text-muted-foreground mt-1 text-xs leading-5'>
                      {entry.reason}
                    </p>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
    </section>
  )
}
