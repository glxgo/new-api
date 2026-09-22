import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth-store'
import { api } from '@/lib/api'
import { Skeleton } from '@/components/ui/skeleton'

// Authenticated images require the same New-Api-User header as JSON APIs.
// Blob URLs are local to this mounted view and revoked on hide/logout.
export function ArtifactImage({
  src,
  className,
}: {
  src: string
  className?: string
}) {
  const userId = useAuthStore((s) => s.auth.user?.id)
  return (
    <ArtifactImageResource
      key={`${userId}:${src}`}
      src={src}
      className={className}
    />
  )
}

function ArtifactImageResource({
  src,
  className,
}: {
  src: string
  className?: string
}) {
  const { t } = useTranslation()
  const [url, setUrl] = useState('')
  const [failed, setFailed] = useState(false)
  useEffect(() => {
    const abort = new AbortController()
    let local = ''
    if (!src.startsWith('/api/capability/')) return
    api
      .get<Blob>(src, { responseType: 'blob', signal: abort.signal })
      .then(({ data }) => {
        if (abort.signal.aborted) return
        if (data.type !== 'image/png') {
          setFailed(true)
          return
        }
        local = URL.createObjectURL(data)
        setUrl(local)
      })
      .catch(() => {
        if (!abort.signal.aborted) setFailed(true)
      })
    return () => {
      abort.abort()
      if (local) URL.revokeObjectURL(local)
    }
  }, [src])
  if (failed || !src.startsWith('/api/capability/'))
    return (
      <span className='text-muted-foreground flex aspect-[3/2] items-center justify-center text-sm'>
        {t('Artwork unavailable')}
      </span>
    )
  return url ? (
    <img src={url} alt={t('Model response drawing')} className={className} />
  ) : (
    <Skeleton className='aspect-[3/2] w-full' />
  )
}
