import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth-store'
import { cn } from '@/lib/utils'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from '@/components/ui/card'
import {
  Empty,
  EmptyHeader,
  EmptyTitle,
  EmptyDescription,
} from '@/components/ui/empty'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { Skeleton } from '@/components/ui/skeleton'
import { SectionPageLayout } from '@/components/layout/components/section-page-layout'
import {
  getArchive,
  usePelicanOverview,
  timeLabel,
  type Results,
  type Group,
} from './api'
import {
  ArchiveEvidence,
  ArchiveImage,
  ArchiveUnavailable,
  gradeLabel,
} from './evidence'
import { ArchiveMethod } from './method'

function gradeColor(grade: string) {
  if (grade === 'correct') return 'bg-success'
  if (grade === 'wrong') return 'bg-warning'
  return 'bg-muted-foreground/25'
}

export function PelicanArchivePage(props: { preview?: boolean }) {
  const { t } = useTranslation()
  const query = usePelicanOverview(props.preview)
  const [selected, setSelected] = useState('')
  const data = query.data
  const models = [...new Set(data?.data.flatMap((g) => g.models) || [])]
  const model = models.includes(selected) ? selected : models[0] || ''
  return (
    <SectionPageLayout variant='editorial'>
      <SectionPageLayout.Title>
        {t('Intelligence Test')}
      </SectionPageLayout.Title>
      <SectionPageLayout.Description>
        {t('Pelican test records, supported by original evidence.')}
      </SectionPageLayout.Description>
      <SectionPageLayout.Content>
        {query.isPending ? (
          <Skeleton className='h-72 w-full' />
        ) : query.isError ? (
          <Alert>
            <AlertDescription>
              {t('Unable to load test results.')}
              <Button variant='link' onClick={() => query.refetch()}>
                {t('Retry')}
              </Button>
            </AlertDescription>
          </Alert>
        ) : !data?.visible ? (
          <Empty>
            <EmptyHeader>
              <EmptyTitle>
                {t('This page is currently unavailable.')}
              </EmptyTitle>
            </EmptyHeader>
          </Empty>
        ) : (
          <div className='mx-auto flex max-w-7xl flex-col gap-4'>
            <Card
              size='sm'
              className='border-border/70 from-card via-card to-muted/30 ring-foreground/5 relative overflow-hidden rounded-2xl bg-gradient-to-br shadow-xs ring-1 ring-inset'
            >
              <div
                aria-hidden='true'
                className='bg-primary/12 pointer-events-none absolute -top-28 right-[-7%] size-80 rounded-full blur-3xl'
              />
              <div
                aria-hidden='true'
                className='from-primary/80 via-primary/25 absolute inset-x-0 top-0 h-px bg-gradient-to-r to-transparent'
              />
              <CardHeader className='border-border/50 relative gap-3 border-b px-4 pt-5 pb-5 sm:px-6'>
                <div className='flex flex-wrap items-center gap-2'>
                  <span className='border-primary/20 bg-primary/8 text-primary relative rounded-full border py-1 pr-2.5 pl-5 text-[10px] font-semibold tracking-[0.14em] uppercase before:absolute before:top-1/2 before:left-2 before:size-1.5 before:-translate-y-1/2 before:rounded-full before:bg-current before:shadow-[0_0_0_3px_color-mix(in_srgb,currentColor_18%,transparent)]'>
                    {t('Verified source archive')}
                  </span>
                  <span className='text-muted-foreground text-[11px]'>
                    {t('Original records preserved')}
                  </span>
                </div>
                <div className='before:bg-primary relative pl-3 before:absolute before:inset-y-0 before:left-0 before:w-0.5 before:rounded-full'>
                  <CardTitle className='text-xl tracking-[-0.02em] text-balance sm:text-2xl'>
                    {data.presentation.copy.page_title ||
                      t('Pelican test records')}
                  </CardTitle>
                </div>
                <CardDescription className='max-w-3xl text-[13px] leading-6'>
                  {data.presentation.copy.page_intro ||
                    t(
                      'Inspect saved SVG works and numerical answers from channel tests. Open an artwork or a history cell to review the evidence.'
                    )}
                </CardDescription>
              </CardHeader>
              <CardContent className='relative flex flex-wrap items-end justify-between gap-4 px-4 py-4 sm:px-6 sm:py-5'>
                <div className='flex min-w-0 flex-1 flex-wrap gap-2'>
                  <div className='border-border/60 bg-background/45 rounded-xl border px-3 py-2.5 shadow-xs'>
                    <p className='text-muted-foreground text-[10px] tracking-wide uppercase'>
                      {t('Archive captured')}
                    </p>
                    <p className='mt-0.5 text-xs font-medium tabular-nums'>
                      {data.source_captured_at
                        ? new Date(data.source_captured_at).toLocaleString()
                        : '—'}
                    </p>
                  </div>
                  <div className='border-border/60 bg-background/45 rounded-xl border px-3 py-2.5 shadow-xs'>
                    <p className='text-muted-foreground text-[10px] tracking-wide uppercase'>
                      {t('Source schedule')}
                    </p>
                    <p className='mt-0.5 text-xs font-medium tabular-nums'>
                      {data.source_interval_minutes || '—'} {t('minutes')}
                    </p>
                  </div>
                </div>
                <div className='border-border/60 bg-background/45 rounded-xl border p-2 shadow-xs'>
                  <label className='text-muted-foreground flex min-w-[180px] flex-col gap-1.5 px-1 text-[10px] font-semibold tracking-wide uppercase'>
                    {t('Model')}
                    <NativeSelect
                      aria-label={t('Model')}
                      value={model}
                      onChange={(e) => setSelected(e.target.value)}
                      className='text-xs font-medium tracking-normal normal-case'
                    >
                      {models.map((m) => (
                        <NativeSelectOption key={m} value={m}>
                          {m}
                        </NativeSelectOption>
                      ))}
                    </NativeSelect>
                  </label>
                </div>
              </CardContent>
            </Card>
            {data.data
              .filter((g) => g.models.includes(model))
              .map((g) => (
                <GroupResults
                  key={`${g.group_uid}:${model}`}
                  group={g}
                  model={model}
                  preview={props.preview}
                  showHistory={data.presentation.show_history}
                  galleryText={data.presentation.copy.gallery_title}
                  emptyText={data.presentation.copy.empty_text}
                />
              ))}
            {!models.length && (
              <Empty>
                <EmptyHeader>
                  <EmptyTitle>{t('No test records available')}</EmptyTitle>
                  <EmptyDescription>
                    {data.presentation.copy.empty_text ||
                      t(
                        'Results will appear when verified channel records are available.'
                      )}
                  </EmptyDescription>
                </EmptyHeader>
              </Empty>
            )}
            {data.presentation.show_method && (
              <ArchiveMethod copy={data.presentation.copy} />
            )}
          </div>
        )}
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
function GroupResults(props: {
  group: Group
  model: string
  preview?: boolean
  showHistory: boolean
  galleryText?: string
  emptyText?: string
}) {
  const { t } = useTranslation()
  const user = useAuthStore((s) => s.auth.user?.id)
  const [detail, setDetail] = useState<string | null>(null)
  const query = useQuery({
    queryKey: [
      'pelican',
      user,
      'results',
      props.preview,
      props.group.group_uid,
      props.model,
    ],
    queryFn: ({ signal }) =>
      getArchive<{ data: Results }>(
        `${props.preview ? '/admin' : ''}/groups/${props.group.group_uid}/results?model=${encodeURIComponent(props.model)}`,
        signal
      ),
    refetchInterval: 30_000,
    retry: false,
  })
  const result = query.data?.data
  return (
    <Card
      size='sm'
      className='border-border/70 bg-card hover:border-foreground/20 overflow-hidden rounded-2xl shadow-xs transition-[transform,box-shadow,border-color] duration-300 ease-[cubic-bezier(0.32,0.72,0,1)] hover:-translate-y-0.5 hover:shadow-sm'
    >
      <CardHeader className='border-border/50 gap-3 border-b px-4 pt-4 pb-4 sm:px-5'>
        <div className='flex items-start justify-between gap-3'>
          <div className='min-w-0'>
            <p className='text-muted-foreground mb-1 text-[10px] font-semibold tracking-[0.14em] uppercase'>
              {t('Group result')}
            </p>
            <CardTitle className='truncate text-base sm:text-lg'>
              {props.group.display_name}
            </CardTitle>
          </div>
          {props.group.ratio !== null && (
            <Badge
              variant='outline'
              className='border-primary/20 bg-primary/5 text-primary'
            >
              {props.group.ratio}×
            </Badge>
          )}
        </div>
        <div className='flex flex-wrap items-center gap-x-3 gap-y-1'>
          <span className='text-muted-foreground max-w-full truncate font-mono text-[11px]'>
            {props.model}
          </span>
          {props.group.description && (
            <CardDescription className='text-[12px] leading-5'>
              {props.group.description}
            </CardDescription>
          )}
        </div>
      </CardHeader>
      <CardContent className='flex flex-col gap-4 px-4 py-4 sm:px-5'>
        {query.isPending ? (
          <Skeleton className='h-56 w-full' />
        ) : query.isError ? (
          <Alert>
            <AlertDescription>
              {t('Unable to load test results.')}
              <Button variant='link' onClick={() => query.refetch()}>
                {t('Retry')}
              </Button>
            </AlertDescription>
          </Alert>
        ) : !result?.gallery.length ? (
          <Empty>
            <EmptyHeader>
              <EmptyTitle>{t('No test records available')}</EmptyTitle>
              <EmptyDescription>
                {props.emptyText ||
                  t(
                    'Results will appear when verified channel records are available.'
                  )}
              </EmptyDescription>
            </EmptyHeader>
          </Empty>
        ) : (
          <>
            <div className='flex flex-wrap items-center justify-between gap-2'>
              <p className='text-muted-foreground text-xs'>
                {props.galleryText || t('Selected channel records')}
              </p>
              <span className='text-muted-foreground border-border/60 bg-muted/30 rounded-full border px-2.5 py-1 text-[11px] tabular-nums'>
                {t('{{count}} linked targets', { count: result.targets })}
              </span>
            </div>
            <div className='grid items-start gap-3 md:grid-cols-3'>
              {result.gallery.map((r) => (
                <button
                  key={r.id}
                  type='button'
                  onClick={() => setDetail(r.id)}
                  className='group hover:bg-muted/40 focus-visible:outline-ring border-border/70 bg-background/65 hover:border-foreground/20 flex min-w-0 flex-col overflow-hidden rounded-xl border p-1 text-left shadow-xs transition-[transform,box-shadow,border-color] duration-300 ease-[cubic-bezier(0.32,0.72,0,1)] hover:-translate-y-0.5 hover:shadow-sm focus-visible:outline-2'
                  aria-label={`${t('Open test record')} ${timeLabel(r.time)}`}
                >
                  {r.has_artwork ? (
                    <ArchiveImage
                      id={r.id}
                      admin={props.preview}
                      className='block h-auto w-full rounded-lg transition-transform duration-500 ease-[cubic-bezier(0.32,0.72,0,1)] group-hover:scale-[1.01]'
                    />
                  ) : (
                    <ArchiveUnavailable />
                  )}
                </button>
              ))}
            </div>
            {props.showHistory && (
              <section className='flex flex-col gap-2'>
                <div className='flex items-center justify-between gap-3'>
                  <h3 className='text-xs font-semibold tracking-wide uppercase'>
                    {t('Test history')}
                  </h3>
                  <span className='text-muted-foreground text-[10px]'>
                    {t('Select a record to inspect')}
                  </span>
                </div>
                <div className='border-border/60 bg-muted/20 flex gap-1.5 overflow-x-auto rounded-xl border p-2 pb-3'>
                  {[...result.history].reverse().map((r) => (
                    <button
                      key={r.id}
                      onClick={() => setDetail(r.id)}
                      title={`${timeLabel(r.time)} · ${t(gradeLabel(r.grade))}`}
                      aria-label={`${t('Open test record')} ${timeLabel(r.time)} ${t(gradeLabel(r.grade))}`}
                      className='focus-visible:outline-ring flex h-9 w-6 shrink-0 items-center rounded focus-visible:outline-2'
                    >
                      <span
                        className={cn(
                          'h-5 w-full rounded-sm ring-1 ring-black/5 transition-transform duration-200 ring-inset hover:scale-y-110 dark:ring-white/10',
                          gradeColor(r.grade)
                        )}
                      />
                    </button>
                  ))}
                </div>
                <div className='text-muted-foreground flex flex-wrap gap-3 text-[11px]'>
                  {['correct', 'wrong', 'error'].map((grade) => (
                    <span
                      key={grade}
                      className='inline-flex items-center gap-1.5'
                    >
                      <span
                        aria-hidden='true'
                        className={cn('size-2 rounded-sm', gradeColor(grade))}
                      />
                      {t(gradeLabel(grade))}
                    </span>
                  ))}
                </div>
                <p className='text-muted-foreground text-[11px] leading-5'>
                  {t(
                    'History contains actual saved records; channels may finish at different times.'
                  )}
                </p>
              </section>
            )}
          </>
        )}
      </CardContent>
      <ArchiveEvidence
        id={detail}
        admin={props.preview}
        onClose={() => setDetail(null)}
      />
    </Card>
  )
}
