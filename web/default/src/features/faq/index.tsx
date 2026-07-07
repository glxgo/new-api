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
import { HelpCircle, Search } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from '@/components/ui/accordion'
import { Input } from '@/components/ui/input'
import { Markdown } from '@/components/ui/markdown'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'
import { PublicLayout } from '@/components/layout'
import { useFAQ } from '@/features/dashboard/hooks/use-status-data'
import type { FAQItem } from '@/features/dashboard/types'

// 编号马卡龙轮换色（default 主题 chart 色）
const NUM_TINTS = [
  'bg-chart-1/15 text-chart-1',
  'bg-chart-2/15 text-chart-2',
  'bg-chart-3/15 text-chart-3',
  'bg-chart-4/15 text-chart-4',
  'bg-chart-5/15 text-chart-5',
]

// FAQ 常见问题独立页(从概览迁出): 搜索框 + 默认折叠的手风琴 + Markdown 渲染。
export function FAQ() {
  const { t } = useTranslation()
  const { items: list, loading } = useFAQ()
  const [query, setQuery] = useState('')

  const filtered = useMemo(() => {
    if (!query.trim()) return list
    const q = query.toLowerCase()
    return list.filter(
      (item: FAQItem) =>
        item.question?.toLowerCase().includes(q) ||
        item.answer?.toLowerCase().includes(q)
    )
  }, [list, query])

  return (
    <PublicLayout>
      <div className='mx-auto max-w-5xl px-4 py-10'>
        <div className='mb-10 text-center'>
          <div className='mb-4 flex justify-center'>
            <div className='bg-primary/10 text-primary flex h-16 w-16 items-center justify-center rounded-2xl'>
              <HelpCircle className='h-8 w-8' />
            </div>
          </div>
          <h1 className='text-[clamp(2rem,5vw,3rem)] font-bold tracking-tight'>
            {t('FAQ')}
          </h1>
          <p className='text-muted-foreground mt-3'>
            {t('Answers for common access and billing questions')}
          </p>
        </div>

        <div className='grid gap-6 md:grid-cols-[280px_1fr]'>
          <div className='md:sticky md:top-6 md:self-start'>
            <div className='bg-card flex flex-col items-center gap-3 rounded-2xl border p-5 shadow-sm'>
              <img
                src='/uploads/after-sales-qrcode.jpg'
                alt={t('After-sales group QR code')}
                className='w-full max-w-[240px] rounded-lg object-contain'
              />
              <p className='text-muted-foreground text-center text-sm'>
                {t('Scan to join the after-sales group')}
              </p>
            </div>
          </div>

          <div className='min-w-0'>
            <div className='relative mb-6'>
              <Search className='text-muted-foreground absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2' />
              <Input
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder={t('Search FAQs...')}
                className='pl-9'
              />
            </div>

            {loading ? (
              <div className='space-y-3'>
                {Array.from({ length: 4 }).map((_, i) => (
                  <Skeleton key={i} className='h-16 w-full rounded-xl' />
                ))}
              </div>
            ) : filtered.length === 0 ? (
              <div className='text-muted-foreground py-16 text-center'>
                {query ? t('No matching FAQs') : t('No FAQ entries available')}
              </div>
            ) : (
              <Accordion multiple className='space-y-3'>
                {filtered.map((item: FAQItem, idx: number) => (
                  <AccordionItem
                    key={item.id ?? `faq-${idx}`}
                    value={String(item.id ?? idx)}
                    className='group bg-card rounded-xl border px-5 shadow-sm transition-all duration-300 hover:scale-[1.01] hover:shadow-md'
                  >
                    <AccordionTrigger>
                      <div className='flex items-start gap-3 pr-2'>
                        <span
                          className={cn(
                            'mt-0.5 inline-flex h-6 w-6 flex-shrink-0 items-center justify-center rounded-full text-xs font-bold',
                            NUM_TINTS[idx % NUM_TINTS.length]
                          )}
                        >
                          {idx + 1}
                        </span>
                        <Markdown className='text-base leading-relaxed font-semibold'>
                          {item.question}
                        </Markdown>
                      </div>
                    </AccordionTrigger>
                    <AccordionContent>
                      <Markdown className='text-muted-foreground ml-9 text-sm leading-relaxed'>
                        {item.answer}
                      </Markdown>
                    </AccordionContent>
                  </AccordionItem>
                ))}
              </Accordion>
            )}
          </div>
        </div>
      </div>
    </PublicLayout>
  )
}
