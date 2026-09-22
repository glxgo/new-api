import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,
} from '@/components/ui/sheet'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'
import { capabilityGet, formatTime, type Item, type Run } from './api'
import { ArtifactPlayer } from './artifact-player'

export function RunEvidence(props: {
  id: string | null
  onClose: () => void
  fixture?: Run
  initialKind: string
  groupName: string
}) {
  const { t } = useTranslation()
  const query = useQuery({
    queryKey: ['capability', 'evidence', props.id],
    queryFn: ({ signal }) =>
      capabilityGet<{ data: Run }>(`/runs/${props.id}`, signal),
    enabled: !!props.id && !props.fixture,
    retry: false,
  })
  const run = props.fixture || query.data?.data
  const kinds: Record<string, string> = {
    logic: t('Logic'),
    geometry: t('Geometry'),
    scene: t('Scene drawing'),
  }
  return (
    <Sheet
      open={!!props.id}
      onOpenChange={(open) => {
        if (!open) props.onClose()
      }}
    >
      <SheetContent className='w-full gap-0 sm:max-w-5xl'>
        <SheetHeader className='gap-1.5 border-b px-4 py-4 pr-12 sm:px-6'>
          <SheetTitle className='flex flex-wrap items-baseline gap-x-3 gap-y-1 text-base'>
            <span className='break-words'>{props.groupName}</span>
            <span className='text-muted-foreground text-sm font-medium'>
              {run?.model}
            </span>
          </SheetTitle>
          <SheetDescription className='text-xs leading-5'>
            {t('Questions and evaluation evidence')}
            {run && ` · ${formatTime(run.time)} · ${t('Sample')} ${run.sample}`}
          </SheetDescription>
        </SheetHeader>
        <div className='min-h-0 flex-1 overflow-y-auto px-4 py-4 sm:px-6'>
          {query.isError ? (
            <Alert>
              <AlertDescription>
                {t('Unable to load test results.')}{' '}
                <Button variant='link' onClick={() => query.refetch()}>
                  {t('Retry')}
                </Button>
              </AlertDescription>
            </Alert>
          ) : !run ? (
            <Skeleton className='h-72 w-full' />
          ) : (
            <Tabs
              key={`${props.id}:${props.initialKind}`}
              defaultValue={props.initialKind}
              className='gap-4'
            >
              <TabsList
                aria-label={t('Drawing task')}
                className='w-full sm:w-fit'
              >
                {run.items.map((item) => (
                  <TabsTrigger
                    key={item.kind}
                    value={item.kind}
                    className='px-3 text-xs'
                  >
                    {kinds[item.kind] || item.kind}
                  </TabsTrigger>
                ))}
              </TabsList>
              {run.items.map((item) => (
                <TabsContent key={item.kind} value={item.kind}>
                  <TaskEvidence item={item} run={run} />
                </TabsContent>
              ))}
            </Tabs>
          )}
        </div>
      </SheetContent>
    </Sheet>
  )
}

function TaskEvidence(props: { item: Item; run: Run }) {
  const { t } = useTranslation()
  const item = props.item
  const checks =
    item.judgment?.items ||
    item.checks?.map((pass) => ({
      status: pass ? 'pass' : 'fail',
      evidence: '',
    })) ||
    []
  const complete =
    item.status === 'graded' &&
    checks.length > 0 &&
    checks.every((check) => check.status === 'pass' || check.status === 'fail')
  const met = checks.filter((check) => check.status === 'pass').length
  let status = t('Awaiting evaluation')
  if (complete)
    status =
      met === checks.length
        ? t('All requirements met')
        : t('Some requirements met')
  if (complete && met === 0) status = t('Requirements not met')
  const metrics = props.run.metrics?.[item.kind]
  const number = (value: number | null | undefined) =>
    value == null ? '—' : value.toLocaleString()
  const duration = metrics?.duration_seconds
  const entries = [
    [t('Generation started'), formatTime(metrics?.started_at || 0)],
    [
      t('Generation duration'),
      duration == null ? '—' : t('{{count}} seconds', { count: duration }),
    ],
    [t('Input tokens'), number(metrics?.input_tokens)],
    [t('Output tokens'), number(metrics?.output_tokens)],
    [t('Generation calls'), number(metrics?.attempts)],
    [t('Method version'), props.run.suite || '—'],
  ]
  const checkLabels: Record<string, string> = {
    pass: t('Met'),
    fail: t('Not met'),
    uncertain: t('Uncertain'),
  }
  return (
    <article className='flex flex-col gap-4'>
      <div className='bg-muted/30 flex flex-wrap items-center justify-between gap-2 rounded-lg border px-3 py-2.5'>
        <Badge
          variant='outline'
          className={cn(
            'gap-1.5',
            complete &&
              met === checks.length &&
              'border-success/30 bg-success/10'
          )}
        >
          <span
            aria-hidden
            className={cn(
              'bg-muted-foreground size-1.5 rounded-full',
              complete && met === checks.length && 'bg-success',
              complete && met < checks.length && 'bg-warning'
            )}
          />
          {status}
        </Badge>
        <span className='text-xs tabular-nums'>
          {t('Requirements met')}:{' '}
          <strong>{complete ? `${met} / ${checks.length}` : '—'}</strong>
        </span>
      </div>
      <ArtifactPlayer item={item} />
      <section className='space-y-2'>
        <h3 className='text-xs font-semibold'>{t('Original question')}</h3>
        <div
          className='bg-muted/30 max-h-64 overflow-y-auto rounded-lg border p-3 text-[13px] leading-6 break-words whitespace-pre-wrap'
          tabIndex={0}
          role='region'
          aria-label={t('Original question')}
        >
          {item.question.prompt}
        </div>
      </section>
      <dl className='grid grid-cols-2 gap-x-4 gap-y-3 rounded-lg border p-3 text-xs sm:grid-cols-3'>
        {entries.map(([label, value]) => (
          <div key={label} className='min-w-0 space-y-1'>
            <dt className='text-muted-foreground'>{label}</dt>
            <dd className='font-medium break-words tabular-nums'>{value}</dd>
          </div>
        ))}
      </dl>
      <p className='text-muted-foreground text-xs leading-5'>
        {t(
          'Generation metrics exclude rendering and evaluation. Token counts are reported by the provider; unrecorded values remain blank.'
        )}
      </p>
      {checks.length > 0 && (
        <section className='space-y-2'>
          <h3 className='text-xs font-semibold'>{t('Evaluation details')}</h3>
          <ol className='divide-y rounded-lg border px-3'>
            {checks.map((check, index) => (
              <li
                key={index}
                className='flex items-start gap-3 py-2.5 text-[13px] leading-5'
              >
                <span className='text-muted-foreground pt-0.5 text-xs tabular-nums'>
                  {String(index + 1).padStart(2, '0')}
                </span>
                <div className='min-w-0 flex-1'>
                  <p className='font-medium'>
                    {item.question.requirements?.[index] ||
                      t('Requirement {{number}}', { number: index + 1 })}
                  </p>
                  {check.evidence && (
                    <p className='text-muted-foreground mt-1 break-words'>
                      {check.evidence}
                    </p>
                  )}
                </div>
                <Badge variant='outline'>
                  {checkLabels[check.status] || t('Uncertain')}
                </Badge>
              </li>
            ))}
          </ol>
        </section>
      )}
      <details open={item.kind === 'logic'} className='group rounded-lg border'>
        <summary className='focus-visible:ring-ring cursor-pointer rounded-lg p-3 text-xs font-semibold outline-none focus-visible:ring-2'>
          {t('Original response')}
        </summary>
        <pre
          className='bg-muted/30 max-h-96 overflow-auto border-t p-3 text-xs leading-6 break-words whitespace-pre-wrap'
          tabIndex={0}
        >
          {item.answer || t('No response recorded.')}
        </pre>
      </details>
      <dl className='text-muted-foreground grid gap-1 border-t pt-3 text-xs leading-5'>
        <div className='flex flex-wrap gap-x-2'>
          <dt>{t('Record ID')}</dt>
          <dd className='font-mono break-all'>{props.run.public_id}</dd>
        </div>
        <div className='flex flex-wrap gap-x-2'>
          <dt>{t('Question fingerprint')}</dt>
          <dd className='min-w-0 font-mono break-all'>{item.question.hash}</dd>
        </div>
      </dl>
    </article>
  )
}
