import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
  CardAction,
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
  capabilityGet,
  useCapabilityOverview,
  formatTime,
  type Group,
  type Run,
  type Overview,
} from './api'
import { ArtifactImage } from './artifact-image'
import { MethodReader, MethodSummary } from './method'
import { RunEvidence } from './run-evidence'
import { ResultTimeline } from './timeline'

export function IntelligenceTest() {
  const { t } = useTranslation()
  const query = useCapabilityOverview()
  return (
    <SectionPageLayout variant='editorial'>
      <SectionPageLayout.Title>
        {t('Intelligence Test')}
      </SectionPageLayout.Title>
      <SectionPageLayout.Description>
        {t('Task performance, supported by original evidence.')}
      </SectionPageLayout.Description>
      <SectionPageLayout.Content>
        {query.isPending ? (
          <Skeleton className='h-72 w-full' />
        ) : query.isError ? (
          <Alert>
            <AlertDescription>
              {t('Unable to load test results.')}{' '}
              <Button variant='link' onClick={() => query.refetch()}>
                {t('Retry')}
              </Button>
            </AlertDescription>
          </Alert>
        ) : !query.data?.visible ? (
          <Empty>
            <EmptyHeader>
              <EmptyTitle>
                {t('This page is currently unavailable.')}
              </EmptyTitle>
            </EmptyHeader>
          </Empty>
        ) : (
          <CapabilityOverview data={query.data} />
        )}
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}

export function CapabilityOverview({
  data,
  fixture,
}: {
  data: Overview
  fixture?: Record<string, Run[]>
}) {
  const { t } = useTranslation()
  const [method, setMethod] = useState(false)
  const [chapter, setChapter] = useState(0)
  const [model, setModel] = useState('')
  if (!data.visible)
    return (
      <Empty>
        <EmptyHeader>
          <EmptyTitle>{t('This page is currently unavailable.')}</EmptyTitle>
        </EmptyHeader>
      </Empty>
    )
  const models = [...new Set(data.data.flatMap((g) => g.models))]
  return (
    <div className='mx-auto flex max-w-7xl flex-col gap-4'>
      <Card className='gap-3'>
        <CardHeader>
          <CardTitle className='text-sm font-semibold'>
            {data.copy?.page_title ||
              t('Model performance, backed by evidence')}
          </CardTitle>
          <CardDescription className='col-span-full max-w-5xl pt-1 text-[13px] leading-6'>
            {data.running
              ? data.copy?.page_intro ||
                (data.method
                  ? t(
                      'On a {{minutes}}-minute schedule, models in each group complete logic, geometry and scene drawing tasks. Selected works highlight stronger results. Open a work or history cell to inspect the original question, response and evaluation.',
                      { minutes: data.method.interval_minutes }
                    )
                  : t(
                      'Explore selected model responses, with original questions and evaluation evidence.'
                    ))
              : t(
                  'The test schedule is paused. Completed results remain available.'
                )}
          </CardDescription>
          <CardAction>
            <Badge variant='outline'>
              {data.running ? t('Test schedule active') : t('Paused')}
            </Badge>
          </CardAction>
        </CardHeader>
        <CardContent>
          <div className='flex flex-wrap items-center justify-between gap-2'>
            <p className='text-muted-foreground text-xs tabular-nums'>
              {t('Next scheduled test')}:{' '}
              {data.running ? formatTime(data.next_times?.[0]) : '—'}
            </p>
            {data.show_method && (
              <Button
                size='sm'
                variant='outline'
                onClick={() => {
                  setChapter(0)
                  setMethod(true)
                }}
              >
                {t('Test methodology and scoring')}
              </Button>
            )}
          </div>
        </CardContent>
      </Card>
      <div className='flex flex-wrap items-center justify-between gap-2'>
        <NativeSelect
          aria-label={t('Model')}
          value={model}
          onChange={(e) => setModel(e.target.value)}
        >
          <NativeSelectOption value=''>{t('All Models')}</NativeSelectOption>
          {models.map((m) => (
            <NativeSelectOption key={m} value={m}>
              {m}
            </NativeSelectOption>
          ))}
        </NativeSelect>
        <p className='text-muted-foreground text-xs'>
          {data.copy?.gallery_title || t('Selected works')}
        </p>
      </div>
      {data.data
        .filter((g) => !model || g.models.includes(model))
        .map((g) => (
          <CapabilityGroupCard
            key={g.group_uid}
            group={g}
            model={model}
            count={data.gallery_size}
            showHistory={data.show_history}
            fixture={fixture ? fixture[g.group_uid] || [] : undefined}
            revision={data.revision}
          />
        ))}
      {data.data.length === 0 && (
        <Empty>
          <EmptyHeader>
            <EmptyTitle>{t('No test results yet')}</EmptyTitle>
            <EmptyDescription>
              {data.copy?.empty_text ||
                t('Results will appear after an eligible test completes.')}
            </EmptyDescription>
          </EmptyHeader>
        </Empty>
      )}
      {data.show_method && (
        <MethodSummary
          copy={data.copy}
          onOpen={(index) => {
            setChapter(index)
            setMethod(true)
          }}
        />
      )}
      {data.show_method && method && (
        <MethodReader
          initialIndex={chapter}
          open={method}
          onOpenChange={setMethod}
          introduction={data.copy?.method_intro}
          copy={data.copy}
          method={data.method}
        />
      )}
    </div>
  )
}

export function CapabilityGroupCard({
  group,
  model,
  count,
  showHistory,
  fixture,
  revision,
}: {
  group: Group
  model: string
  count: number
  showHistory: boolean
  fixture?: Run[]
  revision: number
}) {
  const { t } = useTranslation()
  const [kind, setKind] = useState('scene')
  const [selected, setSelected] = useState<string | null>(null)
  const [selectedKind, setSelectedKind] = useState('scene')
  const [history, setHistory] = useState(false)
  const [page, setPage] = useState(1)
  const query = useQuery({
    queryKey: [
      'capability',
      'results',
      group.group_uid,
      model,
      revision,
      history,
      showHistory,
      page,
    ],
    queryFn: ({ signal }) =>
      capabilityGet<{ data: Run[]; has_more: boolean }>(
        `/groups/${group.group_uid}/results?model=${encodeURIComponent(model)}&history=${history && showHistory}&page=${page}`,
        signal
      ),
    enabled: !fixture,
    refetchInterval: 30_000,
    retry: false,
  })
  const runs = fixture || query.data?.data || []
  // Never mix distinct models into one ranking or fill repeated gallery slots.
  const models = model ? [model] : group.models
  return (
    <Card className='@container gap-3'>
      <CardHeader>
        <CardTitle className='flex min-w-0 flex-wrap items-baseline gap-x-3 gap-y-1 text-sm font-semibold'>
          <span className='break-words'>{group.display_name}</span>
          {models.length === 1 && (
            <span className='text-muted-foreground text-xs font-medium break-all'>
              {models[0]}
            </span>
          )}
        </CardTitle>
        {group.description && (
          <CardDescription className='col-span-full text-xs leading-5'>
            {group.description}
          </CardDescription>
        )}
        <CardAction>
          <Badge variant='secondary'>
            {group.ratio === null
              ? t('Multiplier not configured')
              : `×${group.ratio}`}
          </Badge>
        </CardAction>
      </CardHeader>
      <CardContent className='flex flex-col gap-3'>
        <div className='flex flex-wrap items-center justify-between gap-3'>
          <NativeSelect
            aria-label={t('Drawing task')}
            value={kind}
            onChange={(e) => setKind(e.target.value)}
          >
            <NativeSelectOption value='scene'>
              {t('Scene drawing')}
            </NativeSelectOption>
            <NativeSelectOption value='geometry'>
              {t('Geometry')}
            </NativeSelectOption>
          </NativeSelect>
          {showHistory && (
            <Button
              size='sm'
              variant='outline'
              onClick={() => {
                setHistory(!history)
                setPage(1)
              }}
            >
              {history ? t('Current selection') : t('View all samples')}
            </Button>
          )}
        </div>
        {!fixture && query.isPending ? (
          <Skeleton className='h-52 w-full' />
        ) : query.isError ? (
          <Alert>
            <AlertDescription>
              {t('Unable to load test results.')}{' '}
              <Button variant='link' onClick={() => query.refetch()}>
                {t('Retry')}
              </Button>
            </AlertDescription>
          </Alert>
        ) : (
          models.map((m) => {
            const all = runs.filter((r) => r.model === m)
            const latest = Math.max(0, ...all.map((r) => r.slot))
            const current = all.filter((r) => r.slot === latest)
            const works = (history ? all : current)
              .filter((r) => r.items.some((i) => i.kind === kind && i.artifact))
              .slice(0, history ? 100 : count)
            return (
              <section key={m} className='flex flex-col gap-3'>
                <div className='flex flex-wrap items-center justify-between gap-2 text-xs'>
                  <h3 className='font-medium break-all'>
                    {models.length > 1 ? m : t('Selected works')}
                  </h3>
                  <span className='text-muted-foreground tabular-nums'>
                    {t('Test round')}: {formatTime(latest)}
                  </span>
                </div>
                {works.length ? (
                  <div className='grid grid-cols-1 gap-3 @xl:grid-cols-3'>
                    {works.map((run) => {
                      const item = run.items.find((i) => i.kind === kind)!
                      return (
                        <button
                          type='button'
                          key={run.public_id}
                          onClick={() => {
                            setSelectedKind(kind)
                            setSelected(run.public_id)
                          }}
                          className='focus-visible:ring-ring bg-card flex min-w-0 flex-col overflow-hidden rounded-lg border text-left transition-shadow hover:shadow-sm focus-visible:ring-2'
                        >
                          <ArtifactImage
                            src={item.artifact!}
                            className='aspect-[3/2] w-full border-b object-contain'
                          />
                          <div className='flex w-full flex-col gap-1 p-2.5'>
                            <span className='flex items-center justify-between gap-2 text-xs font-medium'>
                              <span>
                                {item.question.label ||
                                  (kind === 'scene'
                                    ? t('Scene drawing')
                                    : t('Geometry'))}
                              </span>
                              <span className='text-muted-foreground font-normal'>
                                {item.status === 'graded'
                                  ? t('Selected work')
                                  : t('Awaiting evaluation')}
                              </span>
                            </span>
                            <span className='text-muted-foreground text-xs'>
                              {t('Sample')} {run.sample} ·{' '}
                              {formatTime(run.time)}
                            </span>
                          </div>
                        </button>
                      )
                    })}
                  </div>
                ) : (
                  <Empty className='border'>
                    <EmptyHeader>
                      <EmptyTitle>{t('No test results yet')}</EmptyTitle>
                      <EmptyDescription>
                        {t(
                          'Results will appear after an eligible test completes.'
                        )}
                      </EmptyDescription>
                    </EmptyHeader>
                  </Empty>
                )}
                <div className='text-muted-foreground flex flex-wrap gap-x-5 gap-y-1 text-xs'>
                  <span>
                    {t('Samples in displayed round')}:{' '}
                    <strong className='tabular-nums'>{current.length}</strong>
                  </span>
                  <span>
                    {t('Logic')}:{' '}
                    {
                      current.filter(
                        (r) =>
                          r.items.find((i) => i.kind === 'logic')?.status ===
                          'graded'
                      ).length
                    }{' '}
                    {t('evaluated samples')}
                  </span>
                  <span>
                    {t('Geometry')}:{' '}
                    {
                      current.filter(
                        (r) =>
                          r.items.find((i) => i.kind === 'geometry')?.status ===
                          'graded'
                      ).length
                    }{' '}
                    {t('evaluated samples')}
                  </span>
                </div>
                {showHistory && !history && (
                  <ResultTimeline
                    groupUID={group.group_uid}
                    model={m}
                    revision={revision}
                    fixture={fixture}
                    onOpen={(id, task) => {
                      setSelectedKind(task)
                      setSelected(id)
                    }}
                  />
                )}
              </section>
            )
          })
        )}
      </CardContent>
      {history && showHistory && (
        <div className='flex justify-end gap-3 px-6 pb-6'>
          <Button
            variant='outline'
            disabled={page === 1}
            onClick={() => setPage(page - 1)}
          >
            {t('Previous')}
          </Button>
          <Button
            variant='outline'
            disabled={!query.data?.has_more}
            onClick={() => setPage(page + 1)}
          >
            {t('Next')}
          </Button>
        </div>
      )}
      <RunEvidence
        groupName={group.display_name}
        id={selected}
        onClose={() => setSelected(null)}
        fixture={fixture?.find((r) => r.public_id === selected)}
        initialKind={selectedKind}
      />
    </Card>
  )
}
