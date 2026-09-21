import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Activity, RefreshCw, Search } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Dialog } from '@/components/dialog'
import {
  getChannelMetrics,
  shanghaiDate,
  type ChannelMetricsParams,
} from '../../metrics-api'
import { ChannelMetricsResults } from './channel-metrics-results'

export function ChannelMetricsDialog({
  open,
  onOpenChange,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { t, i18n } = useTranslation()
  const [params, setParams] = useState<ChannelMetricsParams>({
    period: 'hour',
    date: shanghaiDate(),
    keyword: '',
    p: 1,
    page_size: 20,
  })
  const [search, setSearch] = useState('')
  const query = useQuery({
    queryKey: ['channel-metrics', params],
    queryFn: ({ signal }) => getChannelMetrics(params, signal),
    enabled: open,
    refetchInterval: 60_000,
    staleTime: 15_000,
  })
  const data = query.data
  const formatTime = (ts: number) =>
    new Intl.DateTimeFormat(i18n.language, {
      timeZone: 'Asia/Shanghai',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      hour12: false,
    }).format(new Date(ts * 1000))

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={
        <span className='flex items-center gap-2'>
          <Activity className='size-5' aria-hidden='true' />
          {t('Channel metrics')}
        </span>
      }
      description={t('Review channel health by day or over the last hour.')}
      contentClassName='sm:max-w-5xl'
      bodyClassName='space-y-4'
    >
      <div className='flex flex-wrap items-end justify-between gap-3'>
        <Tabs
          value={params.period}
          onValueChange={(v) =>
            setParams((p) => ({
              ...p,
              period: v === 'day' ? 'day' : 'hour',
              p: 1,
            }))
          }
        >
          <TabsList aria-label={t('Metrics period')}>
            <TabsTrigger value='hour'>{t('Last hour')}</TabsTrigger>
            <TabsTrigger value='day'>{t('By day')}</TabsTrigger>
          </TabsList>
        </Tabs>
        {params.period === 'day' && (
          <div className='space-y-1.5'>
            <Label htmlFor='channel-metrics-date'>
              {t('Date (Beijing time)')}
            </Label>
            <Input
              id='channel-metrics-date'
              className='w-auto'
              type='date'
              value={params.date}
              max={shanghaiDate()}
              onChange={(e) => {
                if (e.target.value)
                  setParams((p) => ({ ...p, date: e.target.value, p: 1 }))
              }}
            />
          </div>
        )}
        <Button
          variant='outline'
          size='sm'
          onClick={() => void query.refetch()}
          disabled={query.isFetching}
          aria-label={t('Refresh')}
        >
          <RefreshCw className='size-4' aria-hidden='true' />
          {t('Refresh')}
        </Button>
      </div>
      <form
        className='flex gap-2'
        onSubmit={(e) => {
          e.preventDefault()
          setParams((p) => ({ ...p, keyword: search.trim(), p: 1 }))
        }}
      >
        <Input
          aria-label={t('Search channel name or ID')}
          placeholder={t('Search channel name or ID')}
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
        <Button type='submit' variant='outline' aria-label={t('Search')}>
          <Search className='size-4' aria-hidden='true' />
          <span className='hidden sm:inline'>{t('Search')}</span>
        </Button>
      </form>
      <div
        className='text-muted-foreground flex flex-wrap justify-between gap-1 text-xs'
        aria-live='polite'
      >
        <span>
          {data
            ? `${formatTime(data.start_ts)} – ${formatTime(data.end_ts)}`
            : t('Loading...')}{' '}
          · {t('Beijing time (UTC+8)')}
        </span>
        <span>{t('Updated every minute; complete minutes only.')}</span>
      </div>
      {query.isPending && (
        <div className='space-y-3' role='status' aria-label={t('Loading...')}>
          {[0, 1, 2, 3].map((n) => (
            <Skeleton key={n} className='h-14 w-full' />
          ))}
        </div>
      )}
      {query.isError && (
        <div
          role='alert'
          className='text-destructive rounded-lg border p-6 text-center'
        >
          {t('Failed to load channel metrics')}
          <Button
            variant='outline'
            className='ml-3'
            onClick={() => void query.refetch()}
          >
            {t('Retry')}
          </Button>
        </div>
      )}
      {!query.isPending && !query.isError && data && (
        <ChannelMetricsResults
          data={data}
          params={params}
          onPageChange={(page) => setParams((p) => ({ ...p, p: page }))}
        />
      )}
      <div className='text-muted-foreground space-y-1 border-t pt-3 text-xs leading-relaxed'>
        <p>
          {t(
            'Success rate counts final requests, excluding internal retries and client-caused failures. Cache rate follows the existing input-token metric. First token averages streaming samples only; — means no samples.'
          )}
        </p>
        <p>
          {t(
            'Recent data includes this instance. Other instances may take up to {{minutes}} minutes to persist their metrics.',
            { minutes: data?.flush_interval_minutes ?? 5 }
          )}
        </p>
      </div>
    </Dialog>
  )
}
