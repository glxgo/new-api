import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { type Item } from './api'
import { ArtifactImage } from './artifact-image'

// Explicit playback only: no HTML execution and no automatic motion, including
// when a user prefers reduced motion. Stopping restores the recorded poster.
export function ArtifactPlayer(props: { item: Item }) {
  return <Artwork key={props.item.artifact} item={props.item} />
}

function Artwork(props: { item: Item }) {
  const { t } = useTranslation()
  const [playing, setPlaying] = useState(false)
  const [evidenceOpen, setEvidenceOpen] = useState(false)
  const item = props.item
  if (!item.artifact) return null
  const animation = item.animation
  const src = playing && animation ? animation.artifact : item.artifact
  return (
    <figure className='space-y-2'>
      <ArtifactImage
        src={src}
        className='aspect-[3/2] max-h-[26rem] w-full rounded-lg border object-contain'
      />
      {animation && (
        <>
          <figcaption className='flex flex-wrap items-center justify-between gap-2 text-xs'>
            <Button
              size='sm'
              variant='outline'
              aria-pressed={playing}
              onClick={() => setPlaying(!playing)}
            >
              {playing ? t('Stop') : t('Play recorded animation')}
            </Button>
            <span className='text-muted-foreground tabular-nums'>
              {t('Recorded window: {{seconds}} s · {{frames}} frames', {
                seconds: ((animation.frames - 1) * animation.step_ms) / 1000,
                frames: animation.frames,
              })}
            </span>
          </figcaption>
          <details
            className='rounded-lg border'
            onToggle={(event) => setEvidenceOpen(event.currentTarget.open)}
          >
            <summary className='focus-visible:ring-ring cursor-pointer rounded-lg px-3 py-2.5 text-xs font-medium outline-none focus-visible:ring-2'>
              {t('Sequential frame evidence')}
            </summary>
            <div className='space-y-2 border-t p-3'>
              {evidenceOpen && (
                <ArtifactImage
                  src={animation.evidence}
                  className='w-full rounded-md border object-contain'
                />
              )}
              <p className='text-muted-foreground text-xs leading-5'>
                {t('Frames are ordered left to right, then top to bottom.')}{' '}
                {animation.evidence_ms.map((ms) => `${ms / 1000}s`).join(' · ')}
              </p>
            </div>
          </details>
        </>
      )}
    </figure>
  )
}
