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
import { ChevronLeft, ChevronRight, Receipt } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatCurrencyFromUSD } from '@/lib/currency'
import { formatQuota } from '@/lib/format'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { TitledCard } from '@/components/ui/titled-card'
import {
  StaticDataTable,
  type StaticDataTableColumn,
} from '@/components/data-table/static/static-data-table'
import { StatusBadge } from '@/components/status-badge'
import {
  useFinancialFlow,
  type FinancialFlowItem,
  type FinancialFlowType,
} from '../hooks/use-financial-flow'
import { formatTimestamp } from '../lib/billing'

interface RechargeHistoryCardProps {
  onOpenBilling?: () => void
  pageSize?: number
}

type FlowFilter = 'all' | FinancialFlowType

const FLOW_FILTERS: { value: FlowFilter; labelKey: string }[] = [
  { value: 'all', labelKey: 'All' },
  { value: 'recharge', labelKey: 'Recharge' },
  { value: 'consume', labelKey: 'Consume' },
]

// 财务流水表：合并「充值记录」与「消费记录(按天聚合)」，按时间倒序展示，
// 每条显示操作后余额快照(balance_after，历史数据为空显示「—」)。
// 支持按类型筛选(全部/充值/消费) + 分页(超过 pageSize 条时显示分页器)。
// 「查看全部」仍打开完整 BillingHistoryDialog（仅充值明细）。
export function RechargeHistoryCard({
  onOpenBilling,
  pageSize = 10,
}: RechargeHistoryCardProps) {
  const { t } = useTranslation()
  const { items, loading } = useFinancialFlow()
  const [filter, setFilter] = useState<FlowFilter>('all')
  const [page, setPage] = useState(1)

  // 类型筛选
  const filtered = useMemo(
    () => (filter === 'all' ? items : items.filter((i) => i.type === filter)),
    [items, filter]
  )

  // 分页
  const totalPages = Math.max(1, Math.ceil(filtered.length / pageSize))
  const currentPage = Math.min(page, totalPages)
  const startIdx = (currentPage - 1) * pageSize
  const rows = filtered.slice(startIdx, startIdx + pageSize)

  const handleFilterChange = (value: FlowFilter) => {
    setFilter(value)
    setPage(1)
  }

  const columns: StaticDataTableColumn<FinancialFlowItem>[] = [
    {
      id: 'date',
      header: t('Date'),
      cell: (row) => (
        <span className='text-muted-foreground text-xs whitespace-nowrap'>
          {formatTimestamp(row.time)}
        </span>
      ),
    },
    {
      id: 'type',
      header: t('Type'),
      cell: (row) =>
        row.type === 'recharge' ? (
          <StatusBadge
            label={t('Recharge')}
            variant='success'
            showDot
            copyable={false}
          />
        ) : (
          <StatusBadge
            label={t('Consume')}
            variant='info'
            showDot
            copyable={false}
          />
        ),
    },
    {
      id: 'amount',
      header: t('Amount'),
      cell: (row) => (
        <span className='font-semibold tabular-nums'>
          {row.type === 'recharge'
            ? formatCurrencyFromUSD(row.amountQuota, {
                digitsLarge: 2,
                digitsSmall: 2,
                abbreviate: false,
              })
            : formatQuota(row.amountQuota)}
        </span>
      ),
    },
    {
      id: 'balance_after',
      header: t('Balance After'),
      cell: (row) =>
        row.balanceAfter !== undefined ? (
          <span className='text-muted-foreground tabular-nums'>
            {formatQuota(row.balanceAfter)}
          </span>
        ) : (
          <span className='text-muted-foreground/60'>—</span>
        ),
    },
  ]

  return (
    <TitledCard
      title={t('Financial Flow')}
      description={t('Recent financial transactions')}
      icon={<Receipt className='h-4 w-4' />}
      disableHoverEffect
      action={
        onOpenBilling ? (
          <Button variant='outline' size='sm' onClick={onOpenBilling}>
            {t('View All')}
          </Button>
        ) : null
      }
      contentClassName='space-y-3'
    >
      {/* 类型筛选：全部 / 充值 / 消费 */}
      <div className='flex flex-wrap gap-1.5'>
        {FLOW_FILTERS.map((f) => (
          <Button
            key={f.value}
            size='sm'
            variant={filter === f.value ? 'default' : 'outline'}
            onClick={() => handleFilterChange(f.value)}
          >
            {t(f.labelKey)}
          </Button>
        ))}
      </div>

      {loading ? (
        <div className='space-y-2'>
          {Array.from({ length: 3 }).map((_, i) => (
            <Skeleton key={i} className='h-9 w-full' />
          ))}
        </div>
      ) : (
        <>
          <StaticDataTable
            columns={columns}
            data={rows}
            getRowKey={(row) => row.key}
            empty={rows.length === 0}
            emptyContent={t('No billing records found')}
          />

          {/* 分页器：超过 1 页时显示 */}
          {totalPages > 1 && (
            <div className='flex items-center justify-between gap-2 pt-1'>
              <span className='text-muted-foreground text-xs tabular-nums'>
                {t('{{from}}-{{to}} of {{total}}', {
                  from: startIdx + 1,
                  to: Math.min(startIdx + pageSize, filtered.length),
                  total: filtered.length,
                })}
              </span>
              <div className='flex items-center gap-2'>
                <span className='text-muted-foreground text-xs tabular-nums'>
                  {currentPage} / {totalPages}
                </span>
                <Button
                  size='icon-sm'
                  variant='outline'
                  disabled={currentPage <= 1}
                  onClick={() => setPage((p) => Math.max(1, p - 1))}
                  aria-label={t('Previous')}
                >
                  <ChevronLeft className='size-4' />
                </Button>
                <Button
                  size='icon-sm'
                  variant='outline'
                  disabled={currentPage >= totalPages}
                  onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                  aria-label={t('Next')}
                >
                  <ChevronRight className='size-4' />
                </Button>
              </div>
            </div>
          )}
        </>
      )}
    </TitledCard>
  )
}
