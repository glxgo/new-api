import { useTranslation } from 'react-i18next'
import { useIsMobile } from '@/hooks/use-mobile'
import { Badge } from '@/components/ui/badge'
import { CountUp } from '@/components/ui/count-up'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import type { ChannelMetric } from '../../metrics-api'

export function ChannelMetricsTable({ items }: { items: ChannelMetric[] }) {
  const { t } = useTranslation()
  const isMobile = useIsMobile()
  if (isMobile) {
    return (
      <div className='space-y-3'>
        {items.map((item) => (
          <ChannelMetricCard key={item.id} item={item} />
        ))}
      </div>
    )
  }
  return (
    <div className='overflow-x-auto rounded-lg border'>
      <Table className='min-w-[660px]'>
        <TableHeader>
          <TableRow>
            <TableHead>{t('Channel')}</TableHead>
            <TableHead className='text-right'>{t('Requests')}</TableHead>
            <TableHead className='text-right'>{t('Success rate')}</TableHead>
            <TableHead className='text-right'>{t('Cache rate')}</TableHead>
            <TableHead className='text-right'>
              {t('Average first token')}
            </TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {items.map((item) => (
            <TableRow key={item.id}>
              <TableCell className='max-w-64 whitespace-normal'>
                <div className='font-medium break-words'>
                  {item.name || `#${item.id}`}
                </div>
                <div className='text-muted-foreground mt-1 flex flex-wrap items-center gap-2 text-xs'>
                  <span>#{item.id}</span>
                  {item.status !== 1 && (
                    <Badge variant='secondary'>{t('Disabled')}</Badge>
                  )}
                </div>
              </TableCell>
              <TableCell className='text-right tabular-nums'>
                {item.request_count.toLocaleString()}
              </TableCell>
              <TableCell className='text-right tabular-nums'>
                <MetricValue value={item.success_rate} percent />
                <div className='text-muted-foreground mt-1 text-xs'>
                  {t('{{count}} successful requests', {
                    count: item.success_count,
                  })}
                </div>
              </TableCell>
              <TableCell className='text-right tabular-nums'>
                <MetricValue value={item.cache_rate} percent />
              </TableCell>
              <TableCell className='text-right tabular-nums'>
                <MetricValue value={item.avg_ttft_ms} />
                <div className='text-muted-foreground mt-1 text-xs'>
                  {t('{{count}} streaming samples', { count: item.ttft_count })}
                </div>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}

function ChannelMetricCard({ item }: { item: ChannelMetric }) {
  const { t } = useTranslation()
  return (
    <article className='space-y-3 rounded-lg border p-3'>
      <div className='flex items-start justify-between gap-2'>
        <div className='min-w-0'>
          <div className='font-medium break-words'>
            {item.name || `#${item.id}`}
          </div>
          <div className='text-muted-foreground mt-1 text-xs'>
            #{item.id} · {t('Requests')} {item.request_count.toLocaleString()}
          </div>
        </div>
        {item.status !== 1 && (
          <Badge variant='secondary'>{t('Disabled')}</Badge>
        )}
      </div>
      <dl className='grid grid-cols-3 gap-2 border-t pt-3'>
        <div>
          <dt className='text-muted-foreground text-xs'>{t('Success rate')}</dt>
          <dd className='mt-1 font-medium tabular-nums'>
            <MetricValue value={item.success_rate} percent />
          </dd>
        </div>
        <div>
          <dt className='text-muted-foreground text-xs'>{t('Cache rate')}</dt>
          <dd className='mt-1 font-medium tabular-nums'>
            <MetricValue value={item.cache_rate} percent />
          </dd>
        </div>
        <div>
          <dt className='text-muted-foreground text-xs'>
            {t('Average first token')}
          </dt>
          <dd className='mt-1 font-medium tabular-nums'>
            <MetricValue value={item.avg_ttft_ms} />
          </dd>
        </div>
      </dl>
      <p className='text-muted-foreground text-xs'>
        {t('{{count}} successful requests', { count: item.success_count })} ·{' '}
        {t('{{count}} streaming samples', { count: item.ttft_count })}
      </p>
    </article>
  )
}

function MetricValue({
  value,
  percent = false,
}: {
  value: number | null
  percent?: boolean
}) {
  const { t } = useTranslation()
  if (value === null)
    return (
      <span className='text-muted-foreground' aria-label={t('No samples')}>
        —
      </span>
    )
  return (
    <CountUp
      value={percent ? value : value / 1000}
      duration={600}
      format={(n) =>
        percent
          ? `${n.toFixed(2)}%`
          : t('{{seconds}} s', { seconds: n.toFixed(2) })
      }
    />
  )
}
