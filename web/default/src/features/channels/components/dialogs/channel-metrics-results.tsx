import { ChevronLeft, ChevronRight } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import type {
  ChannelMetricsPage,
  ChannelMetricsParams,
} from '../../metrics-api'
import { ChannelMetricsTable } from './channel-metrics-table'

export function ChannelMetricsResults({
  data,
  params,
  onPageChange,
}: {
  data: ChannelMetricsPage
  params: ChannelMetricsParams
  onPageChange: (page: number) => void
}) {
  const { t } = useTranslation()
  const pages = Math.max(1, Math.ceil(data.total / params.page_size))
  return (
    <>
      {!data.enabled && (
        <p role='status' className='rounded-md border p-3 text-sm'>
          {t(
            'Performance collection is disabled. Only recorded history is shown.'
          )}
        </p>
      )}
      {data.items.some((item) => item.legacy_resolution) && (
        <p role='status' className='rounded-md border p-3 text-sm'>
          {t(
            'This range includes older coarse buckets. Hourly boundaries are approximate; daily totals retain the recorded data.'
          )}
        </p>
      )}
      {data.items.length ? (
        <ChannelMetricsTable items={data.items} />
      ) : (
        <div className='text-muted-foreground rounded-lg border p-10 text-center'>
          {t('No matching channels')}
        </div>
      )}
      <div className='flex flex-wrap items-center justify-between gap-2 text-sm'>
        <span className='text-muted-foreground'>
          {t('{{count}} channels', { count: data.total })}
        </span>
        <div className='flex items-center gap-2'>
          <Button
            variant='outline'
            size='icon'
            aria-label={t('Previous page')}
            disabled={params.p <= 1}
            onClick={() => onPageChange(params.p - 1)}
          >
            <ChevronLeft className='size-4' />
          </Button>
          <span className='tabular-nums'>
            {params.p} / {pages}
          </span>
          <Button
            variant='outline'
            size='icon'
            aria-label={t('Next page')}
            disabled={params.p >= pages}
            onClick={() => onPageChange(params.p + 1)}
          >
            <ChevronRight className='size-4' />
          </Button>
        </div>
      </div>
    </>
  )
}
