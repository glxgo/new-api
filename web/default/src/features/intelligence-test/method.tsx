import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,
} from '@/components/ui/sheet'
import type { Method } from './api'

const chapters = [
  [
    'Scope and scoring',
    'These tests observe task performance for each group and model. Text models produce SVG drawings; this is not an image-generation benchmark. Logic has two checks, geometry has five, and scene drawing has six requirements. Uncertain results remain ungraded.',
  ],
  [
    'From questions to evidence',
    'A versioned question set is frozen for each round. The selected channel answers each task, responses are retained, SVG is checked and rendered, and the results are evaluated. Reading this page never starts a model request.',
  ],
  [
    'Selection and sources',
    'The gallery prioritizes stronger task performance within the group. Logical answers, geometric requirements and scene requirements are compared in order; visual presentation resolves ties. Selected works do not represent an average across all channels.',
  ],
  [
    'Reading results',
    'A failed requirement describes this particular task. Missing responses, interrupted calls and pending reviews are not counted as correct or incorrect answers. Actual test times and sample sources accompany each work.',
  ],
  [
    'Traceability and comparison',
    'Open a work to inspect its question, answer and evaluation evidence. Compare results within the same model, profile and test version. Historical works keep their original times and group membership; shared samples do not become additional independent calls.',
  ],
  [
    'Interpreting the tests',
    'Intelligence Test is the name of this task-based evaluation, not a standardized human IQ score. A single result cannot establish overall model capability or verify the upstream model identity. Visual judgments include uncertainty and require calibration.',
  ],
] as const

const titleSlots = [
  'method_scope_title',
  'method_evidence_title',
  'method_selection_title',
  'method_reading_title',
  'method_trace_title',
  'method_limits_title',
] as const
const explanations = [
  [
    'Each sample is one set of tasks completed through a configured channel for the selected model. Group names and price multipliers follow the service configuration; a display alias does not change the underlying route.',
    'Logic answers are compared with a deterministic answer key. Geometry is checked against explicit shape, position, color and visibility requirements. Scene drawings are assessed from the rendered image, with evidence recorded for each requirement.',
    'A dash indicates that no reliable score is available. Zero is a completed assessment in which none of the checked requirements were met. Scores from different task types are not added into an IQ score.',
  ],
  [
    'The reference question stays fixed within a test version. The second logic question and drawing parameters vary with the round. All comparable samples in the round receive the same frozen questions, so selection is based on a common task.',
    'Animation is recorded in a fixed viewport over a defined observation window. Playback and sequential frames come from the same response. Motion alone does not establish that the requested action was completed.',
    'While a new round is running, the page retains the preceding published questions, works and scores. A completed round replaces that selection together. If it produces no displayable work, the preceding selection keeps its original time.',
  ],
  [
    'Complete assessments are compared first by logic, then geometry, then scene requirements. Visual presentation is a supplementary comparison when these results match. Pending assessments remain distinguishable from scored samples.',
    'A channel used by several groups may share one test only when the effective requests are equivalent. The work can appear in each eligible group, while its underlying call remains a single observation.',
    'The gallery shows available original works. An empty position does not establish poor model performance, and displaying the same source more than once would not create an additional independent sample.',
  ],
  [
    'Use the task requirements alongside the image. A visually polished drawing can omit an object, while a simple drawing can satisfy the requested layout. The recorded checks explain which requirements were established.',
    'An interruption or an unavailable evaluator is a missing observation, not an incorrect answer. An uncertain visual judgment is kept as pending rather than converted into a passing score.',
    'The test time belongs to the saved sample. Refreshing the page does not make it newer. A paused schedule retains completed results; it does not imply that another round is in progress.',
  ],
  [
    'Work details link the original question, visible response, image and evaluation to one sample. The method version identifies the rules used for that round. Display-name changes do not rewrite the sample identity.',
    'A channel moved into a group contributes new observations after joining. Its earlier work does not become historical evidence for the new group. Shared samples should not be counted as separate observations in cross-group comparisons.',
    'Compare repeated observations under the same question version and model configuration. A change in task difficulty, model mapping or request parameters needs a new comparison baseline.',
  ],
  [
    'These tasks provide inspectable evidence of reasoning, instruction following and structured drawing within the tested scope. They complement the separate Model Status page, which describes request availability.',
    'One sample describes one observed response. Repeated results under consistent conditions are more informative about changes over time; selected gallery works alone do not establish the average experience of every request.',
    'A reported model name is an upstream declaration. The test records behavior and evidence, rather than certifying the provider identity. Visual assessments retain uncertainty where the image does not support a clear conclusion.',
  ],
] as const

export function MethodSummary(props: {
  copy?: Record<string, string>
  onOpen: (index: number) => void
}) {
  const { t } = useTranslation()
  return (
    <Card className='gap-3'>
      <CardHeader>
        <CardTitle className='text-sm font-semibold'>
          {t('Test methodology and scoring')}
        </CardTitle>
      </CardHeader>
      <CardContent className='grid gap-4 md:grid-cols-3 md:gap-5'>
        {[0, 2, 5].map((index) => (
          <section key={index} className='flex min-w-0 flex-col gap-2'>
            <h3 className='text-xs font-semibold'>
              {props.copy?.[titleSlots[index]] || t(chapters[index][0])}
            </h3>
            <p className='text-muted-foreground text-xs leading-6'>
              {t(chapters[index][1])}
            </p>
            <Button
              size='sm'
              variant='link'
              className='mt-auto h-auto justify-start self-start px-0 text-xs'
              onClick={() => props.onOpen(index)}
            >
              {t('View details')}
            </Button>
          </section>
        ))}
      </CardContent>
    </Card>
  )
}

export function MethodReader({
  open,
  onOpenChange,
  introduction,
  copy,
  method,
  initialIndex = 0,
}: {
  open: boolean
  onOpenChange: (value: boolean) => void
  introduction?: string
  copy?: Record<string, string>
  method?: Method
  initialIndex?: number
}) {
  const { t } = useTranslation()
  const [index, setIndex] = useState(initialIndex)
  const title = (i: number) => copy?.[titleSlots[i]] || t(chapters[i][0])
  const clock = (minutes: number) =>
    `${Math.floor(minutes / 60)
      .toString()
      .padStart(2, '0')}:${(minutes % 60).toString().padStart(2, '0')}`
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className='w-full sm:max-w-[920px]'>
        <SheetHeader>
          <SheetTitle>{t('Test methodology and scoring')}</SheetTitle>
          <SheetDescription>
            {introduction || t('Scope, evidence and interpretation')}
          </SheetDescription>
        </SheetHeader>
        <div className='grid min-h-0 flex-1 grid-rows-[auto_1fr] gap-5 px-6 pb-6 md:grid-cols-[180px_1fr] md:grid-rows-1'>
          <nav
            aria-label={t('Chapter')}
            className='hidden flex-col gap-2 border-r pr-4 md:flex'
          >
            {chapters.map(([key], i) => (
              <Button
                key={key}
                variant={index === i ? 'secondary' : 'ghost'}
                className='h-auto justify-start py-3 text-left whitespace-normal'
                aria-current={index === i ? 'step' : undefined}
                onClick={() => setIndex(i)}
              >
                <span className='text-muted-foreground mr-2 tabular-nums'>
                  {String(i + 1).padStart(2, '0')}
                </span>
                {title(i)}
              </Button>
            ))}
          </nav>
          <div className='md:hidden'>
            <NativeSelect
              aria-label={t('Chapter')}
              value={index}
              onChange={(e) => setIndex(Number(e.target.value))}
            >
              {chapters.map(([key], i) => (
                <NativeSelectOption value={i} key={key}>
                  {i + 1}. {title(i)}
                </NativeSelectOption>
              ))}
            </NativeSelect>
          </div>
          <div className='min-h-0 overflow-y-auto pr-2'>
            <article
              key={index}
              className='flex max-w-prose flex-col gap-5 text-sm leading-7'
            >
              <h3 className='text-lg font-semibold text-balance'>
                {title(index)}
              </h3>
              <p className='text-foreground font-medium'>
                {t(chapters[index][1])}
              </p>
              {explanations[index].map((text, i) => (
                <p
                  key={text}
                  className={cn(
                    'text-muted-foreground',
                    i === 0 && 'border-t pt-5'
                  )}
                >
                  {t(text)}
                </p>
              ))}
            </article>
            {index === 0 && (
              <dl className='bg-muted/40 mt-6 grid grid-cols-2 gap-3 rounded-lg border p-4 text-sm tabular-nums'>
                <dt>{t('Logic')}</dt>
                <dd>0–2</dd>
                <dt>{t('Geometry')}</dt>
                <dd>0–5</dd>
                <dt>{t('Scene drawing')}</dt>
                <dd>0–6</dd>
                <dt>{t('Visual presentation')}</dt>
                <dd>1–5</dd>
              </dl>
            )}
            {index === 1 && method && (
              <dl className='bg-muted/40 mt-6 grid grid-cols-2 gap-3 rounded-lg border p-4 text-sm'>
                <dt className='text-muted-foreground'>{t('Method version')}</dt>
                <dd>{method.suite}</dd>
                <dt className='text-muted-foreground'>{t('Test interval')}</dt>
                <dd>
                  {method.interval_minutes} {t('minutes')}
                </dd>
                <dt className='text-muted-foreground'>{t('Run window')}</dt>
                <dd>
                  {method.all_day
                    ? t('All day')
                    : `${clock(method.window_start)}–${clock(method.window_end)}`}
                </dd>
                <dt className='text-muted-foreground'>{t('Timezone')}</dt>
                <dd>{method.timezone}</dd>
              </dl>
            )}
          </div>
        </div>
        <div className='flex items-center justify-between gap-3 border-t p-4'>
          <Button
            variant='outline'
            disabled={index === 0}
            onClick={() => setIndex(index - 1)}
          >
            {t('Previous')}
          </Button>
          <span className='text-muted-foreground text-sm tabular-nums'>
            {index + 1} / {chapters.length}
          </span>
          <Button
            variant='outline'
            disabled={index === chapters.length - 1}
            onClick={() => setIndex(index + 1)}
          >
            {t('Next')}
          </Button>
        </div>
      </SheetContent>
    </Sheet>
  )
}
