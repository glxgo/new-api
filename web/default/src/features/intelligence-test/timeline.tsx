import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { capabilityGet, formatTime, type Run } from './api'

type TimelineItem = {
  kind: string
  score: number | null
  maximum: number
  evaluated: number
  public_id: string
}
export type TimelineRound = {
  slot: number
  completed_at: number
  suite: string
  samples: number
  items: TimelineItem[]
}

const resultColors = {
  complete: 'bg-success/75 border-success/20',
  partial: 'bg-warning/75 border-warning/20',
  incorrect: 'bg-destructive/65 border-destructive/20',
  pending: 'bg-muted border-border border-dashed',
}

function resultColor(item: TimelineItem) {
  if (item.score === null) return resultColors.pending
  if (item.score === item.maximum) return resultColors.complete
  if (item.score === 0) return resultColors.incorrect
  return resultColors.partial
}

export function ResultTimeline(props: {
  groupUID: string
  model: string
  revision: number
  onOpen: (id: string, kind: string) => void
  fixture?: Run[]
}) {
  const { t } = useTranslation()
  const query = useQuery({
    queryKey: [
      'capability',
      'timeline',
      props.groupUID,
      props.model,
      props.revision,
    ],
    queryFn: ({ signal }) =>
      capabilityGet<{ data: TimelineRound[] }>(
        `/groups/${props.groupUID}/timeline?model=${encodeURIComponent(props.model)}`,
        signal
      ),
    enabled: !props.fixture,
    refetchInterval: 30_000,
    retry: false,
  })
  // Draft previews never read or invent execution history.
  if (props.fixture) return null
  if (query.isPending) return <Skeleton className='h-40 w-full' />
  if (query.isError)
    return (
      <div className='text-muted-foreground flex items-center gap-2 text-sm'>
        {t('Unable to load test history.')}
        <Button variant='link' onClick={() => query.refetch()}>
          {t('Retry')}
        </Button>
      </div>
    )
  const rounds = query.data?.data || []
  const labels = [t('Logic'), t('Geometry'), t('Scene drawing')]
  const stateLabel = (item: TimelineItem) => {
    if (item.score === null) return t('Awaiting evaluation')
    if (item.score === item.maximum) return t('All requirements met')
    if (item.score === 0) return t('Requirements not met')
    return t('Some requirements met')
  }
  return (
    <section className='flex min-w-0 flex-col gap-2.5'>
      <Separator />
      <div className='flex flex-wrap items-baseline justify-between gap-2'>
        <h4 className='text-xs font-semibold'>
          {t('Results and history by task')}
        </h4>
        <p className='text-muted-foreground text-xs'>
          {t('Recent completed rounds · Select a cell to view the question')}
        </p>
      </div>
      {!rounds.length && (
        <p className='text-muted-foreground text-sm'>
          {t('No completed test rounds yet.')}
        </p>
      )}
      {rounds.length > 0 && (
        <TooltipProvider delay={150}>
          <div className='min-w-0 overflow-x-auto overscroll-x-contain pb-1'>
            <div className='flex min-w-[36rem] flex-col gap-1.5 px-1 [@media(pointer:coarse)]:min-w-[78rem]'>
              {['logic', 'geometry', 'scene'].map((kind, k) => (
                <div key={kind} className='flex flex-col gap-1'>
                  <div className='bg-card sticky left-0 flex w-fit flex-wrap items-center gap-x-3 text-xs'>
                    <h5 className='font-medium'>{labels[k]}</h5>
                    <span className='text-muted-foreground tabular-nums'>
                      {t('Evaluated rounds')}:{' '}
                      {
                        rounds.filter((r) =>
                          r.items.some(
                            (i) => i.kind === kind && i.score !== null
                          )
                        ).length
                      }
                      /{rounds.length}
                    </span>
                  </div>
                  <div
                    className='grid gap-1'
                    style={{
                      gridTemplateColumns: `repeat(${rounds.length}, minmax(0, 1.5rem))`,
                    }}
                    role='group'
                    aria-label={labels[k]}
                  >
                    {rounds.map((round) => {
                      // Missing tasks keep their column so the three rows stay aligned.
                      const item = round.items.find((v) => v.kind === kind) || {
                        kind,
                        score: null,
                        maximum: [2, 5, 6][k],
                        evaluated: 0,
                        public_id: '',
                      }
                      const label = `${labels[k]} · ${formatTime(round.slot)} · ${stateLabel(item)} · ${item.score ?? '—'}/${item.maximum} · ${t('evaluated samples')} ${item.evaluated}/${round.samples}`
                      return (
                        <Tooltip key={round.slot}>
                          <TooltipTrigger
                            type='button'
                            aria-label={label}
                            aria-disabled={!item.public_id}
                            onClick={() => {
                              if (item.public_id)
                                props.onOpen(item.public_id, kind)
                            }}
                            className='group focus-visible:ring-ring ring-offset-background flex h-8 min-w-0 items-center rounded-sm outline-none focus-visible:ring-2 focus-visible:ring-offset-1 aria-disabled:cursor-default'
                          >
                            <span
                              aria-hidden
                              className={cn(
                                'h-5 w-full rounded-[3px] border transition-[filter] duration-150 group-hover:brightness-90 group-focus-visible:brightness-90 motion-reduce:transition-none',
                                resultColor(item)
                              )}
                            />
                          </TooltipTrigger>
                          <TooltipContent>
                            <div className='flex flex-col gap-1 py-1 tabular-nums'>
                              <span>
                                {labels[k]} · {formatTime(round.slot)}
                              </span>
                              <span>
                                {stateLabel(item)} · {item.score ?? '—'}/
                                {item.maximum}
                              </span>
                              <span>
                                {t('evaluated samples')} {item.evaluated}/
                                {round.samples}
                              </span>
                              <span>
                                {t('Method version')}: {round.suite || '—'}
                              </span>
                            </div>
                          </TooltipContent>
                        </Tooltip>
                      )
                    })}
                  </div>
                </div>
              ))}
              <div className='text-muted-foreground flex justify-between gap-3 text-xs tabular-nums'>
                <span>{formatTime(rounds[0].slot)}</span>
                {rounds.length > 1 && (
                  <span>{formatTime(rounds.at(-1)!.slot)}</span>
                )}
              </div>
            </div>
          </div>
        </TooltipProvider>
      )}
      {rounds.length > 0 && (
        <>
          <div className='text-muted-foreground flex flex-wrap gap-x-4 gap-y-1 text-xs'>
            {[
              [t('All requirements met'), resultColors.complete],
              [t('Some requirements met'), resultColors.partial],
              [t('Requirements not met'), resultColors.incorrect],
              [t('Awaiting evaluation'), resultColors.pending],
            ].map(([label, color]) => (
              <span key={label} className='flex items-center gap-2'>
                <span
                  aria-hidden
                  className={cn('size-2.5 rounded-[2px] border', color)}
                />
                {label}
              </span>
            ))}
          </div>
          <p className='text-muted-foreground text-xs leading-5'>
            {t(
              'Each cell opens an original sample selected for that task and round. Up to 48 completed rounds are shown; missing or pending results are not treated as incorrect answers.'
            )}
          </p>
        </>
      )}
    </section>
  )
}
