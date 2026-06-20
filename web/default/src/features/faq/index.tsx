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
import { PublicLayout } from '@/components/layout'
import { useFAQ } from '@/features/dashboard/hooks/use-status-data'
import type { FAQItem } from '@/features/dashboard/types'

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
      <div className='mx-auto max-w-3xl px-4 py-10'>
        <div className='mb-8 text-center'>
          <div className='mb-3 flex justify-center'>
            <HelpCircle className='text-primary h-12 w-12' />
          </div>
          <h1 className='text-3xl font-bold'>{t('FAQ')}</h1>
          <p className='text-muted-foreground mt-2'>
            {t('Answers for common access and billing questions')}
          </p>
        </div>

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
              <Skeleton key={i} className='h-16 w-full' />
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
                className='bg-card rounded-lg border px-5'
              >
                <AccordionTrigger>
                  <div className='flex items-start gap-3 pr-2'>
                    <span className='bg-primary/10 text-primary mt-0.5 inline-flex h-6 w-6 flex-shrink-0 items-center justify-center rounded-full text-xs font-bold'>
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
    </PublicLayout>
  )
}
