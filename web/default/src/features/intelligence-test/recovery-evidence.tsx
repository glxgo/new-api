import { useTranslation } from 'react-i18next'
import {
  Accordion,
  AccordionItem,
  AccordionTrigger,
  AccordionContent,
} from '@/components/ui/accordion'
import { Badge } from '@/components/ui/badge'
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from '@/components/ui/card'
import { formatTime, type AdminRunDetail } from './api'
import { ArtifactPlayer } from './artifact-player'

export function RecoveryEvidence({ detail }: { detail: AdminRunDetail }) {
  const { t } = useTranslation()
  if (!detail.recovery?.length && !detail.evaluations?.length) return null
  const kinds: Record<string, string> = {
    logic: t('Logic'),
    geometry: t('Geometry'),
    scene: t('Scene drawing'),
  }
  const states: Record<string, string> = {
    pending: t('Waiting for recovery'),
    running: t('Recovery in progress'),
    complete: t('Recovery complete'),
    cancelled: t('Recovery cancelled'),
    blocked: t('Recovery requires review'),
  }
  const evaluations: Record<string, string> = {
    graded: t('Evaluation complete'),
    ungraded: t('Unable to evaluate'),
    pending_render: t('Waiting for rendering'),
    pending_review: t('Awaiting evaluation'),
  }
  const checks: Record<string, string> = {
    pass: t('Met'),
    fail: t('Not met'),
    uncertain: t('Uncertain'),
  }
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('Recovery and evaluation revisions')}</CardTitle>
        <CardDescription>
          {t(
            'Recovery reuses saved responses. Revisions retain their own timestamps and do not overwrite the published round.'
          )}
        </CardDescription>
      </CardHeader>
      <CardContent className='flex flex-col gap-4'>
        {detail.recovery?.map((job) => (
          <div
            key={job.id}
            className='flex flex-wrap items-center justify-between gap-2 text-sm'
          >
            <span>{kinds[job.kind] || job.kind}</span>
            <Badge variant='outline'>{states[job.status] || job.status}</Badge>
            <span className='text-muted-foreground'>
              {t('Recovery checks: {{count}}', { count: job.attempts })}
            </span>
            {job.status === 'pending' && (
              <span className='text-muted-foreground'>
                {t('Next recovery check')}: {formatTime(job.next_at)}
              </span>
            )}
            {job.reason && (
              <code className='text-muted-foreground w-full text-xs break-all'>
                {job.reason}
              </code>
            )}
          </div>
        ))}
        <Accordion multiple>
          {detail.evaluations?.map((revision) => (
            <AccordionItem key={revision.id} value={revision.id}>
              <AccordionTrigger>
                {kinds[revision.item.kind] || revision.item.kind} ·{' '}
                {t('Evaluation revision {{number}}', {
                  number: revision.revision,
                })}{' '}
                · {formatTime(revision.created_at)}
              </AccordionTrigger>
              <AccordionContent className='flex flex-col gap-4'>
                <div className='flex flex-wrap items-center gap-2'>
                  <Badge variant='outline'>
                    {evaluations[revision.item.status] || revision.item.status}
                  </Badge>
                  {revision.item.reason && (
                    <code className='text-muted-foreground text-xs break-all'>
                      {revision.item.reason}
                    </code>
                  )}
                </div>
                <ArtifactPlayer item={revision.item} />
                <p className='text-sm leading-6 whitespace-pre-wrap'>
                  {revision.item.question.prompt}
                </p>
                {revision.item.checks && (
                  <p className='text-sm'>
                    {t('Requirements met')}:{' '}
                    {revision.item.checks.filter(Boolean).length}/
                    {revision.item.checks.length}
                  </p>
                )}
                {revision.item.judgment?.items.map((check, i) => (
                  <p key={i} className='text-sm'>
                    {i + 1}. {checks[check.status] || check.status} ·{' '}
                    {check.evidence}
                  </p>
                ))}
                <details>
                  <summary className='cursor-pointer text-sm'>
                    {t('Original response')}
                  </summary>
                  <pre className='bg-muted mt-3 max-h-72 overflow-auto rounded-md p-3 text-xs break-words whitespace-pre-wrap'>
                    {revision.item.answer}
                  </pre>
                </details>
              </AccordionContent>
            </AccordionItem>
          ))}
        </Accordion>
      </CardContent>
    </Card>
  )
}
