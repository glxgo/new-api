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
import { getFinancialConsumeDaily } from '../api'
import { useBillingHistory } from './use-billing-history'

// ============================================================================
// Financial Flow — 合并充值记录 + 消费记录的财务流水
// ============================================================================

export type FinancialFlowType = 'recharge' | 'consume'

export interface FinancialFlowItem {
  /** React key (unique across recharge + consume sources) */
  key: string
  /** 秒级时间戳，用于排序与展示（充值=完成时间，消费=当日时间） */
  time: number
  /** 流水类型 */
  type: FinancialFlowType
  /** 金额(quota 单位)：充值=到账额度(amount)，消费=当日消费配额求和 */
  amountQuota: number
  /** 操作后余额快照(quota 单位，本金+赠金)；undefined 表示无快照 */
  balanceAfter?: number
}

interface UseFinancialFlowOptions {
  /** 拉取消费记录的天数窗口（默认 30 天，与后端 TopUp 查询窗口一致） */
  consumeDays?: number
  /** 拉取充值记录的条数（用于合并，取最近 N 条） */
  rechargeLimit?: number
}

/**
 * 财务流水：合并「充值记录」(useBillingHistory → TopupRecord) 与
 * 「消费记录」(专用个人财务流水接口，按天聚合)，按时间倒序。
 *
 * - 充值：每条 TopupRecord(complete_time) 一行，amount = 到账额度。
 * - 消费：后端按本地日历日聚合，quota 求和，并保留当日末笔消费后的余额。
 * - balance_after 由后端(best-effort 快照)填充；0 是有效余额。
 */
export function useFinancialFlow(options: UseFinancialFlowOptions = {}) {
  const { consumeDays = 30, rechargeLimit = 50 } = options

  const { records: rechargeRecords, loading: rechargeLoading } =
    useBillingHistory({
      initialPage: 1,
      initialPageSize: rechargeLimit,
      scope: 'self',
    })

  const consumeQuery = useQuery({
    queryKey: ['wallet', 'financial-flow-consume', consumeDays],
    queryFn: () => {
      const endTimestamp = Math.floor(Date.now() / 1000)
      return getFinancialConsumeDaily(
        endTimestamp - consumeDays * 86400,
        endTimestamp
      )
    },
    staleTime: 60_000,
  })

  const items = useMemo<FinancialFlowItem[]>(() => {
    const consumeData = consumeQuery.data?.data ?? []
    // 充值流水：仅取成功订单，时间用完成时间(回退到创建时间)
    const rechargeItems: FinancialFlowItem[] = rechargeRecords
      .filter((r) => r.status === 'success')
      .map((r) => ({
        key: `recharge-${r.id}`,
        time: r.complete_time || r.create_time,
        type: 'recharge' as const,
        amountQuota: r.amount,
        balanceAfter: r.balance_after,
      }))

    const consumeItems: FinancialFlowItem[] = consumeData.map((d) => ({
      key: `consume-${d.day_start}`,
      time: d.day_start,
      type: 'consume' as const,
      amountQuota: d.quota,
      balanceAfter: d.balance_after,
    }))

    // 合并 + 按时间倒序
    return [...rechargeItems, ...consumeItems].sort((a, b) => b.time - a.time)
  }, [rechargeRecords, consumeQuery.data])

  return {
    items,
    loading: rechargeLoading || consumeQuery.isLoading,
  }
}
