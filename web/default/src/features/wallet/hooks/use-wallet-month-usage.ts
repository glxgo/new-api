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
import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getUserQuotaDates } from '@/features/dashboard/api'
import type { QuotaDataItem } from '@/features/dashboard/types'

// 月初到现在的秒级时间区间（getUserQuotaDates 接收秒，与 lib/time 的
// computeTimeRange 保持一致：Math.floor(getTime()/1000)）。
function getMonthRangeSeconds() {
  const now = new Date()
  const start = new Date(now.getFullYear(), now.getMonth(), 1)
  return {
    startTimestamp: Math.floor(start.getTime() / 1000),
    endTimestamp: Math.floor(now.getTime() / 1000),
  }
}

/**
 * 拉取当前用户本月（自然月）的 quota 明细，供钱包页「本月消费」统计卡
 * 与「本月模型消费分布」图表共用，避免重复请求。
 *
 * 数据源与 dashboard summary-cards 的「近 24h 用量」一致：getUserQuotaDates
 * 走 /api/data/self（isAdmin=false），reduce quota 求和。
 */
export function useWalletMonthUsage() {
  const range = useMemo(getMonthRangeSeconds, [])

  const query = useQuery({
    queryKey: [
      'wallet',
      'month-usage',
      range.startTimestamp,
      range.endTimestamp,
    ],
    queryFn: () =>
      getUserQuotaDates(
        {
          start_timestamp: range.startTimestamp,
          end_timestamp: range.endTimestamp,
          default_time: 'day',
        },
        false
      ),
    staleTime: 60_000,
  })

  const data: QuotaDataItem[] = query.data?.data ?? []

  const monthUsage = useMemo(
    () => data.reduce((sum, item) => sum + (Number(item.quota) || 0), 0),
    [data]
  )

  return {
    data,
    loading: query.isLoading,
    monthUsage,
  }
}
