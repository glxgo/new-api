import { useEffect, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { ImageOff } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth-store'
import { api } from '@/lib/api'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Empty, EmptyDescription } from '@/components/ui/empty'
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,
} from '@/components/ui/sheet'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'
import { base, getArchive, timeLabel, type Detail } from './api'
import { ArchiveArtworkView } from './artwork-view'

export function gradeLabel(grade: string) {
  switch (grade) {
    case 'correct':
      return 'Answer matched'
    case 'wrong':
      return 'Answer not matched'
    case 'no_svg':
      return 'No artwork returned'
    case 'no_answer':
      return 'Answer not detected'
    default:
      return 'No test result'
  }
}
export function ArchiveUnavailable() {
  const { t } = useTranslation()
  return (
    <Empty className='bg-muted/20 aspect-[3/2] min-h-32 gap-3 rounded-lg'>
      <ImageOff
        aria-hidden='true'
        className='text-muted-foreground/60 size-7'
        strokeWidth={1.5}
      />
      <EmptyDescription className='text-muted-foreground m-0 text-xs'>
        {t('Artwork unavailable')}
      </EmptyDescription>
    </Empty>
  )
}
export function ArchiveImage(props: {
  id: string
  admin?: boolean
  className?: string
}) {
  const { t } = useTranslation()
  const user = useAuthStore((s) => s.auth.user?.id)
  return (
    <ImageResource
      key={`${user}:${props.id}:${props.admin}`}
      {...props}
      alt={t('Model response drawing')}
    />
  )
}
function ImageResource(props: {
  id: string
  admin?: boolean
  className?: string
  alt: string
}) {
  const [url, setUrl] = useState('')
  const [failed, setFailed] = useState(false)
  useEffect(() => {
    const controller = new AbortController()
    let local = ''
    api
      .get<Blob>(
        `${base}${props.admin ? '/admin' : ''}/records/${props.id}/artwork`,
        { signal: controller.signal, responseType: 'blob' }
      )
      .then(({ data }) => {
        if (controller.signal.aborted) return
        if (data.type !== 'image/svg+xml') {
          setFailed(true)
          return
        }
        local = URL.createObjectURL(data)
        setUrl(local)
      })
      .catch(() => {
        if (!controller.signal.aborted) setFailed(true)
      })
    return () => {
      controller.abort()
      if (local) URL.revokeObjectURL(local)
    }
  }, [props.id, props.admin])
  if (failed) return <ArchiveUnavailable />
  if (!url) return <Skeleton className='aspect-[3/2] w-full' />
  // Browser image mode: never inject the external SVG into the page DOM.
  return (
    <img
      src={url}
      alt={props.alt}
      onError={() => setFailed(true)}
      className={props.className || 'aspect-[3/2] w-full object-contain'}
    />
  )
}
export function ArchiveEvidence(props: {
  id: string | null
  admin?: boolean
  onClose: () => void
}) {
  const { t } = useTranslation()
  const user = useAuthStore((s) => s.auth.user?.id)
  const query = useQuery({
    queryKey: ['pelican', user, 'detail', props.admin, props.id],
    queryFn: ({ signal }) =>
      getArchive<{ data: Detail }>(
        `${props.admin ? '/admin' : ''}/records/${props.id}`,
        signal
      ),
    enabled: !!props.id,
    refetchInterval: 30_000,
    retry: false,
  })
  const r = query.data?.data
  return (
    <Sheet
      open={!!props.id}
      onOpenChange={(open) => {
        if (!open) props.onClose()
      }}
    >
      <SheetContent className='w-full overflow-y-auto sm:max-w-3xl'>
        <SheetHeader>
          <SheetTitle>{t('Pelican test record')}</SheetTitle>
          <SheetDescription>
            {r ? timeLabel(r.time) : t('Loading')}
          </SheetDescription>
        </SheetHeader>
        {query.isError ? (
          <Alert>
            <AlertDescription>
              {t('Unable to load test results.')}
            </AlertDescription>
          </Alert>
        ) : !r ? (
          <Skeleton className='h-80 w-full' />
        ) : (
          <div className='flex flex-col gap-5 px-4 pb-6'>
            <div className='border-border/70 from-card to-muted/30 rounded-2xl border bg-gradient-to-br p-4 shadow-xs'>
              <div className='flex flex-wrap items-start justify-between gap-3'>
                <div className='min-w-0'>
                  <p className='text-muted-foreground mb-1 text-[10px] font-semibold tracking-[0.14em] uppercase'>
                    {t('Source assessment')}
                  </p>
                  <p className='font-medium break-all'>{r.model}</p>
                </div>
                <div className='flex flex-wrap items-center gap-2'>
                  <Badge variant='secondary'>{t(gradeLabel(r.grade))}</Badge>
                  {r.truncated && (
                    <Badge variant='outline'>{t('Partial artwork')}</Badge>
                  )}
                </div>
              </div>
              <p className='text-muted-foreground mt-3 text-xs tabular-nums'>
                {timeLabel(r.time)}
              </p>
            </div>
            <Tabs
              key={r.id}
              defaultValue={r.has_artwork ? 'artwork' : 'question'}
            >
              <TabsList className='max-w-full overflow-x-auto'>
                {r.has_artwork && (
                  <TabsTrigger value='artwork'>
                    {t('Original artwork')}
                  </TabsTrigger>
                )}
                <TabsTrigger value='question'>
                  {t('Question and answer')}
                </TabsTrigger>
                <TabsTrigger value='data'>{t('Record data')}</TabsTrigger>
                {props.admin && (
                  <TabsTrigger value='source'>{t('Source record')}</TabsTrigger>
                )}
              </TabsList>
              {r.has_artwork && (
                <TabsContent value='artwork'>
                  <ArchiveArtworkView key={r.id}>
                    <ArchiveImage
                      id={r.id}
                      admin={props.admin}
                      className='block h-auto w-full max-w-none'
                    />
                  </ArchiveArtworkView>
                </TabsContent>
              )}
              <TabsContent value='question' className='flex flex-col gap-4'>
                <dl className='grid grid-cols-2 gap-3'>
                  <div className='border-primary/20 bg-primary/5 rounded-xl border p-4'>
                    <dt className='text-muted-foreground text-xs'>
                      {t('Reference answer')}
                    </dt>
                    <dd className='mt-2 text-3xl font-semibold tracking-tight tabular-nums'>
                      {r.expected_answer}
                    </dd>
                  </div>
                  <div className='border-border/70 bg-muted/30 rounded-xl border p-4'>
                    <dt className='text-muted-foreground text-xs'>
                      {t('Detected answer')}
                    </dt>
                    <dd className='mt-2 text-3xl font-semibold tracking-tight tabular-nums'>
                      {r.reported_answer ?? '—'}
                    </dd>
                  </div>
                </dl>
                <p className='text-muted-foreground text-xs leading-6'>
                  {t(
                    'The source checks whether the configured answer appears in the SVG text. The detected number is a supplementary extraction and can differ from the match result.'
                  )}
                </p>
                <h3 className='text-sm font-medium'>
                  {t('Question for this record')}
                </h3>
                <p className='bg-muted/20 border-border/70 rounded-xl border p-4 text-[13px] leading-7 break-words whitespace-pre-wrap'>
                  {r.prompt ||
                    t('The original question is unavailable for this record.')}
                </p>
                {r.prompt && (
                  <p className='text-muted-foreground text-xs'>
                    {t(
                      'Question matched to the source configuration by its version marker.'
                    )}
                  </p>
                )}
              </TabsContent>
              <TabsContent value='data'>
                <dl className='border-border/70 bg-muted/20 grid grid-cols-2 gap-x-4 gap-y-3 rounded-xl border p-4 text-xs'>
                  {[
                    [t('Model'), r.model],
                    [t('Test time'), timeLabel(r.time)],
                    [t('Response duration'), `${r.latency_ms} ms`],
                    [t('First token'), `${r.ttft_ms} ms`],
                    [t('Input tokens'), r.input_tokens],
                    [t('Output tokens'), r.output_tokens],
                    [t('Attempts'), r.attempts],
                    [t('Question version'), r.prompt_hash],
                  ].map(([k, v]) => (
                    <div key={k} className='contents'>
                      <dt className='text-muted-foreground'>{k}</dt>
                      <dd className='font-mono break-all tabular-nums'>{v}</dd>
                    </div>
                  ))}
                </dl>
                <p className='text-muted-foreground mt-4 text-xs leading-6'>
                  {t(
                    'Timing and usage are reported by the test source. A zero may mean that the source did not receive usage information.'
                  )}
                </p>
                <p className='text-muted-foreground mt-4 font-mono text-[11px] break-all'>
                  {r.id}
                </p>
              </TabsContent>
              {props.admin && (
                <TabsContent value='source'>
                  <pre className='bg-muted max-h-[60dvh] overflow-auto rounded-lg p-4 text-xs break-all whitespace-pre-wrap'>
                    {JSON.stringify(
                      { record: r.record, source: r.source_record },
                      null,
                      2
                    )}
                  </pre>
                </TabsContent>
              )}
            </Tabs>
          </div>
        )}
      </SheetContent>
    </Sheet>
  )
}
