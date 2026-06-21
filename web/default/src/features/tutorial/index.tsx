import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { BookOpen } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Markdown } from '@/components/ui/markdown'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { PublicLayout } from '@/components/layout'
import { getTutorialContent } from './api'

type TutorialSection = { num: number; title: string; body: string }

// parseTutorialSections 按 Markdown 二级标题(## )把教程切成步骤段:
// 每段首行作为步骤标题, 其余作为正文(仍走 Markdown 渲染)。无 ## 的零散内容返回空数组(由调用方兜底渲染)。
function parseTutorialSections(content: string): TutorialSection[] {
  const parts = content.split(/^## /m)
  const sections: TutorialSection[] = []
  for (let i = 1; i < parts.length; i++) {
    const block = parts[i].trim()
    if (!block) continue
    const nl = block.indexOf('\n')
    const title = (nl >= 0 ? block.slice(0, nl) : block).trim()
    const body = (nl >= 0 ? block.slice(nl + 1) : '').trim()
    sections.push({ num: sections.length + 1, title, body })
  }
  return sections
}

export function Tutorial() {
  const { t } = useTranslation()
  const { data, isLoading } = useQuery({
    queryKey: ['tutorial-content'],
    queryFn: getTutorialContent,
  })

  const rawContent = data?.data?.trim() ?? ''
  const hasContent = rawContent.length > 0
  const sections = useMemo(
    () => parseTutorialSections(rawContent),
    [rawContent]
  )

  if (isLoading) {
    return (
      <PublicLayout>
        <div className='mx-auto grid max-w-5xl gap-4 py-12 md:grid-cols-2'>
          <Skeleton className='h-32 w-full' />
          <Skeleton className='h-32 w-full' />
          <Skeleton className='h-32 w-full' />
          <Skeleton className='h-32 w-full' />
        </div>
      </PublicLayout>
    )
  }

  if (!hasContent) {
    return (
      <PublicLayout>
        <div className='flex min-h-[60vh] items-center justify-center p-8'>
          <div className='max-w-2xl space-y-4 text-center'>
            <div className='flex justify-center'>
              <BookOpen className='text-muted-foreground h-20 w-20' />
            </div>
            <h2 className='text-2xl font-bold'>
              {t('No Tutorial Content Set')}
            </h2>
            <p className='text-muted-foreground'>
              {t('The administrator has not configured the tutorial yet.')}
            </p>
          </div>
        </div>
      </PublicLayout>
    )
  }

  return (
    <PublicLayout>
      <div className='mx-auto max-w-5xl px-4 py-8 sm:py-12'>
        <div className='mb-8 flex items-center gap-3'>
          <div className='bg-primary/10 text-primary flex h-11 w-11 items-center justify-center rounded-xl'>
            <BookOpen className='h-6 w-6' />
          </div>
          <div>
            <h1 className='text-2xl font-bold'>{t('Usage Tutorial')}</h1>
            <p className='text-muted-foreground text-sm'>
              {t('Follow these steps to get started')}
            </p>
          </div>
        </div>

        {sections.length > 0 ? (
          <div className='grid gap-4 md:grid-cols-2'>
            {sections.map((s) => (
              <Card key={s.num} className='overflow-hidden'>
                <CardHeader className='flex flex-row items-center gap-3 space-y-0'>
                  <span className='bg-foreground text-background flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-sm font-bold'>
                    {s.num}
                  </span>
                  <CardTitle className='text-base'>{s.title}</CardTitle>
                </CardHeader>
                {s.body && (
                  <CardContent>
                    <Markdown className='prose-sm max-w-none'>
                      {s.body}
                    </Markdown>
                  </CardContent>
                )}
              </Card>
            ))}
          </div>
        ) : (
          // 兜底: 内容无 ## 标题时, 退回整段 Markdown 渲染
          <Markdown className='prose-neutral dark:prose-invert max-w-none'>
            {rawContent}
          </Markdown>
        )}
      </div>
    </PublicLayout>
  )
}
