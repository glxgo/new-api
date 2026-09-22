import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'

export function ArchiveArtworkView(props: { children: ReactNode }) {
  const { t } = useTranslation()
  const [zoom, setZoom] = useState('fit')
  const widths: Record<string, string> = {
    fit: 'w-full',
    '150': 'w-[150%]',
    '200': 'w-[200%]',
  }
  return (
    <section className='flex min-w-0 flex-col gap-3'>
      <div className='flex flex-wrap items-center justify-between gap-2'>
        <p className='text-muted-foreground text-xs'>{t('Original artwork')}</p>
        <NativeSelect
          size='sm'
          aria-label={t('Artwork magnification')}
          value={zoom}
          onChange={(event) => setZoom(event.target.value)}
        >
          <NativeSelectOption value='fit'>{t('Fit width')}</NativeSelectOption>
          <NativeSelectOption value='150'>150%</NativeSelectOption>
          <NativeSelectOption value='200'>200%</NativeSelectOption>
        </NativeSelect>
      </div>
      <div
        className='bg-muted/20 focus-visible:outline-ring max-h-[65dvh] overflow-auto rounded-lg border focus-visible:outline-2'
        tabIndex={0}
        role='region'
        aria-label={t('Original artwork')}
      >
        <div className={widths[zoom]}>{props.children}</div>
      </div>
      <p className='text-muted-foreground text-xs leading-6'>
        {t(
          'The drawing, text and calculations are the original model output. Magnification changes only the viewing size.'
        )}
      </p>
    </section>
  )
}
