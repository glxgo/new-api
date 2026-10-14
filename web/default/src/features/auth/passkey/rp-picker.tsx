import { createRoot } from 'react-dom/client'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Dialog } from '@/components/dialog'

function DomainPicker(props: {
  domains: string[]
  done: (domain?: string) => void
}) {
  const { t } = useTranslation()
  return (
    <Dialog
      open
      title={t('Choose a Passkey domain')}
      onOpenChange={(open) => {
        if (!open) props.done()
      }}
    >
      <div className='grid gap-2'>
        {props.domains.map((domain) => (
          <Button
            key={domain}
            variant='outline'
            onClick={() => props.done(domain)}
          >
            {domain}
          </Button>
        ))}
      </div>
    </Dialog>
  )
}

export function choosePasskeyDomain(
  domains?: string[]
): Promise<string | undefined> {
  if (!domains || domains.length < 2) return Promise.resolve(domains?.[0])
  return new Promise((resolve, reject) => {
    const host = document.createElement('div')
    document.body.appendChild(host)
    const root = createRoot(host)
    const done = (domain?: string) => {
      root.unmount()
      host.remove()
      if (domain) resolve(domain)
      else reject(new DOMException('Cancelled', 'AbortError'))
    }
    root.render(<DomainPicker domains={domains} done={done} />)
  })
}
