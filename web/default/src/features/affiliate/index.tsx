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
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Copy, Gift, Users } from 'lucide-react'
import { toast } from 'sonner'
import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Badge } from '@/components/ui/badge'
import { formatQuota } from '@/lib/format'
import dayjs from '@/lib/dayjs'
import { getAffiliateDownline, getAffiliateRebates, getAffiliateSummary } from './api'

const PAGE_SIZE = 10

// 邀新计划页(普通用户): 邀请规则 + 邀请链接 + 直邀/间邀人数 + 累计返利
// + 下级两层列表(直接/间接切换) + 返利明细。
export function Affiliate() {
  const { t } = useTranslation()
  const [layer, setLayer] = useState<1 | 2>(1)
  const [dlPage, setDlPage] = useState(0) // 0-based
  const [rbPage, setRbPage] = useState(0)

  const { data: summary, isLoading: sumLoading } = useQuery({
    queryKey: ['affiliate-summary'],
    queryFn: getAffiliateSummary,
  })
  const s = summary?.data

  const { data: dl, isLoading: dlLoading } = useQuery({
    queryKey: ['affiliate-downline', layer, dlPage],
    queryFn: () => getAffiliateDownline(layer, dlPage + 1, PAGE_SIZE),
  })
  const { data: rb, isLoading: rbLoading } = useQuery({
    queryKey: ['affiliate-rebates', rbPage],
    queryFn: () => getAffiliateRebates(rbPage + 1, PAGE_SIZE),
  })

  const copyLink = async () => {
    if (!s?.aff_link) return
    try {
      await navigator.clipboard.writeText(s.aff_link)
      toast.success(t('Copied'))
    } catch {
      toast.error(t('Copy failed'))
    }
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Affiliate Program')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='mx-auto flex w-full max-w-4xl flex-col gap-4'>
          {/* 邀请规则 + 链接 */}
          <div className='rounded-lg border p-5'>
            <div className='flex items-center gap-2'>
              <Gift className='text-primary size-5' />
              <h2 className='text-lg font-semibold'>{t('Invitation Rules')}</h2>
            </div>
            <p className='text-muted-foreground mt-2 text-sm leading-relaxed'>
              {t(
                'You earn rebates from invitees consumption. Direct invitee: {{rate}}% of gross profit; indirect (their invitee): {{irate}}%.',
                {
                  rate: Math.round((s?.direct_rate ?? 0.1) * 100),
                  irate: Math.round((s?.indirect_rate ?? 0.05) * 100),
                }
              )}
            </p>
            <p className='text-muted-foreground mt-1 text-sm'>
              {t(
                'Rebates go to your gift balance (usable, not withdrawable). Settled T+1 daily.'
              )}
            </p>
            {s?.aff_link && (
              <div className='mt-3 flex items-center gap-2'>
                <code className='bg-muted flex-1 truncate rounded px-3 py-2 text-sm'>
                  {s.aff_link}
                </code>
                <Button size='sm' variant='outline' onClick={copyLink}>
                  <Copy className='size-4' />
                  {t('Copy')}
                </Button>
              </div>
            )}
          </div>

          {/* 汇总卡 */}
          <div className='grid grid-cols-3 gap-4'>
            <StatCard
              label={t('Direct Invitees')}
              value={sumLoading ? null : String(s?.direct_count ?? 0)}
            />
            <StatCard
              label={t('Indirect Invitees')}
              value={sumLoading ? null : String(s?.indirect_count ?? 0)}
            />
            <StatCard
              label={t('Total Rebate')}
              value={sumLoading ? null : formatQuota(s?.total_rebate ?? 0)}
            />
          </div>

          {/* 下级列表(直接/间接切换) */}
          <div className='overflow-hidden rounded-lg border'>
            <div className='flex items-center justify-between border-b px-4 py-3'>
              <div className='flex items-center gap-2'>
                <Users className='size-4' />
                <span className='text-sm font-semibold'>{t('My Downline')}</span>
              </div>
              <div className='bg-muted inline-flex rounded-lg p-0.5'>
                <button
                  type='button'
                  onClick={() => {
                    setLayer(1)
                    setDlPage(0)
                  }}
                  className={`rounded-md px-3 py-1 text-xs font-medium transition-colors ${
                    layer === 1 ? 'bg-background text-foreground shadow-sm' : 'text-muted-foreground'
                  }`}
                >
                  {t('Direct')}
                </button>
                <button
                  type='button'
                  onClick={() => {
                    setLayer(2)
                    setDlPage(0)
                  }}
                  className={`rounded-md px-3 py-1 text-xs font-medium transition-colors ${
                    layer === 2 ? 'bg-background text-foreground shadow-sm' : 'text-muted-foreground'
                  }`}
                >
                  {t('Indirect')}
                </button>
              </div>
            </div>
            <div className='divide-y'>
              {dlLoading ? (
                <Skeleton className='m-4 h-12 w-full' />
              ) : (dl?.data?.data ?? []).length === 0 ? (
                <div className='text-muted-foreground px-4 py-8 text-center text-sm'>
                  {t('No downline yet')}
                </div>
              ) : (
                (dl?.data?.data ?? []).map((u) => (
                  <div
                    key={u.id}
                    className='flex items-center justify-between px-4 py-3 text-sm'
                  >
                    <div>
                      <div className='font-medium'>{u.username}</div>
                      <div className='text-muted-foreground text-xs'>
                        {dayjs(u.created_at * 1000).format('YYYY-MM-DD HH:mm')}
                      </div>
                    </div>
                    <div className='text-muted-foreground text-xs'>
                      {t('Rebate')}:{' '}
                      <span className='text-foreground font-mono'>
                        {formatQuota(u.rebate)}
                      </span>
                    </div>
                  </div>
                ))
              )}
            </div>
            <Pager
              page={dlPage}
              total={dl?.data?.total ?? 0}
              onChange={setDlPage}
              t={t}
            />
          </div>

          {/* 返利明细 */}
          <div className='overflow-hidden rounded-lg border'>
            <div className='border-b px-4 py-3 text-sm font-semibold'>
              {t('Rebate Records')}
            </div>
            <div className='divide-y'>
              {rbLoading ? (
                <Skeleton className='m-4 h-12 w-full' />
              ) : (rb?.data?.data ?? []).length === 0 ? (
                <div className='text-muted-foreground px-4 py-8 text-center text-sm'>
                  {t('No rebate records')}
                </div>
              ) : (
                (rb?.data?.data ?? []).map((r) => (
                  <div
                    key={r.id}
                    className='flex items-center justify-between px-4 py-3 text-sm'
                  >
                    <div className='flex items-center gap-2'>
                      <Badge variant='secondary' className='text-xs'>
                        {r.type === 1 ? t('Direct') : t('Indirect')}
                      </Badge>
                      <span className='text-muted-foreground text-xs'>
                        {t('from user')} #{r.source_user_id}
                      </span>
                    </div>
                    <div className='text-right'>
                      <div className='font-mono font-semibold text-emerald-600'>
                        +{formatQuota(r.amount)}
                      </div>
                      <div className='text-muted-foreground text-xs'>
                        {dayjs(r.created_at * 1000).format('YYYY-MM-DD HH:mm')}
                      </div>
                    </div>
                  </div>
                ))
              )}
            </div>
            <Pager
              page={rbPage}
              total={rb?.data?.total ?? 0}
              onChange={setRbPage}
              t={t}
            />
          </div>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}

function StatCard({ label, value }: { label: string; value: string | null }) {
  return (
    <div className='rounded-lg border p-4'>
      <div className='text-muted-foreground text-xs font-medium tracking-wider uppercase'>
        {label}
      </div>
      <div className='mt-1.5 font-mono text-xl font-bold tabular-nums sm:text-2xl'>
        {value === null ? <Skeleton className='h-7 w-16' /> : value}
      </div>
    </div>
  )
}

function Pager({
  page,
  total,
  onChange,
  t,
}: {
  page: number
  total: number
  onChange: (p: number) => void
  t: (k: string) => string
}) {
  const pages = Math.max(1, Math.ceil(total / PAGE_SIZE))
  if (pages <= 1) return null
  return (
    <div className='flex items-center justify-end gap-2 border-t px-4 py-2 text-sm'>
      <Button
        size='sm'
        variant='outline'
        disabled={page === 0}
        onClick={() => onChange(page - 1)}
      >
        {t('Prev')}
      </Button>
      <span className='text-muted-foreground text-xs'>
        {page + 1} / {pages}
      </span>
      <Button
        size='sm'
        variant='outline'
        disabled={page >= pages - 1}
        onClick={() => onChange(page + 1)}
      >
        {t('Next')}
      </Button>
    </div>
  )
}
