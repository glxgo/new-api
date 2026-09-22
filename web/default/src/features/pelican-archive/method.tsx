import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardHeader,
  CardTitle,
  CardContent,
  CardDescription,
} from '@/components/ui/card'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'

const chapters = [
  [
    'Scope and scoring',
    'The pelican task combines SVG drawing with a numerical reasoning answer. The saved artwork and the source verdict provide a concrete observation of a model response, rather than a standardized IQ score.',
  ],
  [
    'From questions to evidence',
    'The external test service sends the question, extracts the SVG and checks its text against the configured reference answer. This site preserves those records without generating another response or grading them again.',
  ],
  [
    'Selection and sources',
    'Each group shows up to three recent channel results for the selected model, prioritizing matching answers among available artworks. One channel may be referenced by several groups; these references are one observation, not independent tests.',
  ],
  [
    'Reading results',
    'Answer matched means the configured answer was found in the SVG text. Other numbers in the drawing may affect the separately extracted answer. Missing artwork, an undetected answer and an interrupted test remain distinct outcomes.',
  ],
  [
    'Traceability and comparison',
    'A new completed record replaces the preceding result for that channel and model as a whole. While a test or synchronization is incomplete, the saved result keeps its original timestamp. Channels can finish at different times; the gallery is not a single synchronized round.',
  ],
  [
    'Interpreting the tests',
    'Repeated observations under the same question version help examine task behavior. Selected works do not establish the average performance of every request or verify a provider identity. Group names and multipliers follow this site; shared historical records keep their original test time.',
  ],
] as const
const keys = [
  'method_scope_title',
  'method_evidence_title',
  'method_selection_title',
  'method_reading_title',
  'method_trace_title',
  'method_limits_title',
]
export function ArchiveMethod(props: { copy: Record<string, string> }) {
  const { t } = useTranslation()
  const [chapter, setChapter] = useState<number | null>(null)
  const title = (i: number) => props.copy[keys[i]] || t(chapters[i][0])
  return (
    <>
      <Card className='border-border/70 from-card via-card to-muted/20 ring-foreground/5 overflow-hidden rounded-2xl bg-gradient-to-br shadow-xs ring-1 ring-inset'>
        <CardHeader className='border-border/60 gap-3 border-b px-4 pt-5 pb-5 sm:px-6'>
          <div className='flex flex-wrap items-center gap-2'>
            <span className='border-primary/20 bg-primary/5 text-primary rounded-full border px-2.5 py-1 text-[10px] font-semibold tracking-[0.14em] uppercase'>
              {t('Reading framework')}
            </span>
            <span className='text-muted-foreground text-[11px]'>
              {t('Evidence and interpretation')}
            </span>
          </div>
          <div className='before:bg-primary relative pl-3 before:absolute before:inset-y-0 before:left-0 before:w-0.5 before:rounded-full'>
            <CardTitle className='text-lg tracking-[-0.015em] sm:text-xl'>
              {t('Test methodology and scoring')}
            </CardTitle>
          </div>
          <CardDescription className='max-w-3xl text-[13px] leading-6'>
            {props.copy.method_intro ||
              t(
                'Explore the task, selection rules and interpretation of saved evidence.'
              )}
          </CardDescription>
        </CardHeader>
        <CardContent className='grid gap-3 px-4 py-4 sm:px-6 sm:py-5 md:grid-cols-3'>
          {[0, 2, 5].map((i, index) => (
            <section
              key={i}
              className='group border-border/70 bg-background/55 hover:bg-muted/20 hover:border-foreground/20 relative flex min-h-56 flex-col overflow-hidden rounded-xl border p-4 shadow-xs transition-[background-color,transform,box-shadow,border-color] duration-300 ease-[cubic-bezier(0.32,0.72,0,1)] hover:-translate-y-0.5 hover:shadow-sm'
            >
              <div
                aria-hidden='true'
                className='from-primary/60 absolute inset-y-4 left-0 w-px bg-gradient-to-b via-transparent to-transparent opacity-0 transition-opacity duration-300 group-hover:opacity-100'
              />
              <div
                aria-hidden='true'
                className='bg-primary/8 pointer-events-none absolute -top-10 -right-10 size-24 rounded-full blur-2xl transition-transform duration-500 ease-[cubic-bezier(0.32,0.72,0,1)] group-hover:scale-125'
              />
              <div className='relative flex items-center justify-between gap-3'>
                <span className='border-primary/20 bg-primary/5 text-primary inline-flex size-7 items-center justify-center rounded-lg border font-mono text-[11px] font-semibold tracking-[0.08em] shadow-xs'>
                  {String(index + 1).padStart(2, '0')}
                </span>
                <span className='text-muted-foreground text-[10px] tracking-wide uppercase'>
                  {t('Chapter')}
                </span>
              </div>
              <h3 className='relative mt-4 text-[13px] font-semibold tracking-[-0.005em]'>
                {title(i)}
              </h3>
              <p className='text-muted-foreground relative mt-2 text-xs leading-6'>
                {t(chapters[i][1])}
              </p>
              <Button
                variant='link'
                size='sm'
                className='relative mt-auto self-start px-0 pt-4 text-xs'
                onClick={() => setChapter(i)}
              >
                {t('Read more')}
              </Button>
            </section>
          ))}
        </CardContent>
      </Card>
      <Sheet
        open={chapter !== null}
        onOpenChange={(open) => {
          if (!open) setChapter(null)
        }}
      >
        <SheetContent className='w-full overflow-y-auto sm:max-w-2xl'>
          <SheetHeader>
            <SheetTitle>{t('Test methodology and scoring')}</SheetTitle>
          </SheetHeader>
          {chapter !== null && (
            <div className='flex flex-col gap-5 p-4'>
              <div className='border-primary/15 bg-primary/5 rounded-xl border p-4'>
                <p className='text-muted-foreground text-[10px] font-semibold tracking-[0.14em] uppercase'>
                  {t('Chapter')} {String(chapter + 1).padStart(2, '0')}
                </p>
                <h3 className='mt-2 text-base font-semibold'>
                  {title(chapter)}
                </h3>
              </div>
              <NativeSelect
                aria-label={t('Chapter')}
                value={chapter}
                onChange={(e) => setChapter(Number(e.target.value))}
              >
                {chapters.map((_, i) => (
                  <NativeSelectOption key={i} value={i}>
                    {i + 1}. {title(i)}
                  </NativeSelectOption>
                ))}
              </NativeSelect>
              <p className='text-sm leading-8'>{t(chapters[chapter][1])}</p>
              <div className='flex justify-between gap-3'>
                <Button
                  variant='outline'
                  disabled={chapter === 0}
                  onClick={() => setChapter(chapter - 1)}
                >
                  {t('Previous')}
                </Button>
                <Button
                  variant='outline'
                  disabled={chapter === 5}
                  onClick={() => setChapter(chapter + 1)}
                >
                  {t('Next')}
                </Button>
              </div>
            </div>
          )}
        </SheetContent>
      </Sheet>
    </>
  )
}
