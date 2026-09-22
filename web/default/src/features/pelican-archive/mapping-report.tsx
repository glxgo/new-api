import { useTranslation } from 'react-i18next'
import { Badge } from '@/components/ui/badge'
import type { MappingReport } from './api'

const reasons: Record<string, string> = {
  source_removed: 'Source target no longer available',
  source_disabled: 'Source target disabled',
  target_hidden: 'Test display hidden',
  unmapped: 'Not associated',
  channel_missing: 'Associated channel no longer exists',
  channel_disabled: 'Associated channel disabled',
  model_unavailable: 'Associated model no longer supported',
  group_unavailable: 'Group not registered',
  group_hidden: 'Group display hidden',
  ability_unavailable: 'Channel model unavailable in this group',
  no_eligible_group: 'No eligible display group',
}

export function SavedMappingReport(props: {
  report?: MappingReport
  dirty: boolean
}) {
  const { t } = useTranslation()
  if (!props.report) return null
  return (
    <div className='bg-muted/20 mt-3 flex flex-col gap-2 rounded-md border p-3 text-xs'>
      <div className='flex flex-wrap items-center gap-2'>
        <span className='font-medium'>{t('Saved association')}</span>
        <Badge variant='outline'>
          {props.report.reason
            ? t(reasons[props.report.reason] || 'Association needs review')
            : t('Association valid')}
        </Badge>
        {props.dirty && (
          <span className='text-muted-foreground'>
            {t('Unsaved changes do not affect the report below.')}
          </span>
        )}
      </div>
      {props.report.channel_disabled && (
        <p className='text-muted-foreground leading-5'>
          {t(
            'Business channel disabled. External test records remain eligible under its current group membership.'
          )}
        </p>
      )}
      {props.report.groups.map((group) => (
        <div
          key={group.routing_key}
          className='flex flex-wrap items-center justify-between gap-2'
        >
          <span className='break-all'>
            {group.display_name}{' '}
            <span className='text-muted-foreground tabular-nums'>
              · {group.ratio ?? '—'}×
            </span>
          </span>
          <span className='text-muted-foreground'>
            {group.eligible
              ? t('Eligible for selection')
              : t(reasons[group.reason] || 'Association needs review')}
          </span>
        </div>
      ))}
      <p className='text-muted-foreground leading-5'>
        {t(
          'Groups follow the current channel membership. Visibility also depends on page settings, user access and result selection.'
        )}
      </p>
    </div>
  )
}
