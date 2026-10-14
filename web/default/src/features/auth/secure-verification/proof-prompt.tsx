import { useState } from 'react'
import { createRoot } from 'react-dom/client'
import { t } from 'i18next'
import { useTranslation } from 'react-i18next'
import { api } from '@/lib/api'
import {
  buildAssertionResult,
  prepareCredentialRequestOptions,
} from '@/lib/passkey'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Dialog } from '@/components/dialog'
import { choosePasskeyDomain } from '../passkey/rp-picker'

export interface SecurityChallenge {
  challenge: string
  action: string
  methods: string[]
  oauth_providers: string[]
}

export async function verifyChallenge(
  info: SecurityChallenge,
  method: string,
  value: string
) {
  if (method === 'oauth') {
    const popup = window.open('about:blank', '_blank', 'width=560,height=700')
    if (!popup) throw new Error(t('Please allow popups to verify your account'))
    try {
      const begin = await api.get('/api/verify/oauth/begin', {
        params: { challenge: info.challenge, provider: value },
      })
      if (!begin.data.success) throw new Error(begin.data.message)
      popup.location.href = begin.data.data.url
      const deadline = Date.now() + 180000
      let verified = false
      while (Date.now() < deadline) {
        await new Promise((resolve) => setTimeout(resolve, 2000))
        const result = await api.get('/api/verify/oauth/status', {
          headers: { 'X-Security-Challenge': info.challenge },
        })
        if (!result.data.success) throw new Error(result.data.message)
        if (result.data.data.verified) {
          verified = true
          break
        }
        if (popup.closed) throw new Error(t('Verification cancelled'))
      }
      if (!verified) throw new Error(t('Verification expired'))
    } finally {
      popup.close()
    }
  }
  if (method === 'passkey') {
    let begin = await api.post('/api/user/passkey/verify/begin', null, {
      headers: { 'X-Security-Challenge': info.challenge },
    })
    if (!begin.data.success) throw new Error(begin.data.message)
    if (begin.data.data.rp_ids?.length > 1) {
      const domain = await choosePasskeyDomain(begin.data.data.rp_ids)
      begin = await api.post('/api/user/passkey/verify/begin', null, {
        params: { rp_id: domain },
        headers: { 'X-Security-Challenge': info.challenge },
      })
      if (!begin.data.success) throw new Error(begin.data.message)
    }
    const credential = (await navigator.credentials.get({
      publicKey: prepareCredentialRequestOptions(begin.data.data.options),
    })) as PublicKeyCredential | null
    if (!credential) throw new Error(t('Verification cancelled'))
    const finish = await api.post(
      '/api/user/passkey/verify/finish',
      buildAssertionResult(credential)
    )
    if (!finish.data.success) throw new Error(finish.data.message)
  }
  const result = await api.post(
    '/api/verify',
    {
      challenge: info.challenge,
      method,
      code: method === '2fa' ? value.trim() : undefined,
      password: method === 'password' ? value : undefined,
    },
    { skipBusinessError: true }
  )
  if (!result.data.success) throw new Error(result.data.message)
  return result.data.data.proof_token as string
}

function ProofPrompt({
  info,
  done,
}: {
  info: SecurityChallenge
  done: (token?: string) => void
}) {
  const { t } = useTranslation()
  const methods = info.methods
  const [method, setMethod] = useState(methods[0] ?? '')
  const [value, setValue] = useState(
    info.methods[0] === 'oauth' ? (info.oauth_providers[0] ?? '') : ''
  )
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const labels: Record<string, string> = {
    password: t('Password'),
    '2fa': t('Authenticator code'),
    passkey: t('Passkey'),
    oauth: t('OAuth'),
  }
  const submit = async () => {
    if (busy || !method) return
    setBusy(true)
    setError('')
    try {
      done(await verifyChallenge(info, method, value))
    } catch (err) {
      setError(err instanceof Error ? err.message : t('Verification failed'))
      setBusy(false)
    }
  }
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !busy) done()
      }}
      title={t('Security verification')}
      contentClassName='sm:max-w-md'
      showCloseButton={!busy}
      footer={
        <>
          <Button variant='outline' disabled={busy} onClick={() => done()}>
            {t('Cancel')}
          </Button>
          <Button
            disabled={busy || !method || (method !== 'passkey' && !value)}
            onClick={submit}
          >
            {t('Verify')}
          </Button>
        </>
      }
    >
      <form
        className='space-y-4'
        onSubmit={(event) => {
          event.preventDefault()
          void submit()
        }}
      >
        {methods.length > 1 && (
          <select
            aria-label={t('Verification method')}
            className='border-input bg-background w-full rounded-md border p-2'
            value={method}
            disabled={busy}
            onChange={(event) => {
              setMethod(event.target.value)
              setValue(
                event.target.value === 'oauth'
                  ? (info.oauth_providers[0] ?? '')
                  : ''
              )
            }}
          >
            {methods.map((item) => (
              <option key={item} value={item}>
                {labels[item]}
              </option>
            ))}
          </select>
        )}
        {method === 'oauth' && (
          <select
            aria-label={t('OAuth provider')}
            value={value}
            disabled={busy}
            onChange={(event) => setValue(event.target.value)}
          >
            {info.oauth_providers.map((provider) => (
              <option key={provider} value={provider}>
                {provider}
              </option>
            ))}
          </select>
        )}
        {method && method !== 'passkey' && method !== 'oauth' && (
          <Input
            aria-label={labels[method]}
            placeholder={labels[method]}
            autoFocus
            type={method === 'password' ? 'password' : 'text'}
            autoComplete={
              method === 'password' ? 'current-password' : 'one-time-code'
            }
            value={value}
            disabled={busy}
            onChange={(event) => setValue(event.target.value)}
          />
        )}
        {!method && <p>{t('Verification unavailable')}</p>}
        {error && (
          <p role='alert' className='text-destructive text-sm'>
            {error}
          </p>
        )}
      </form>
    </Dialog>
  )
}

let queue: Promise<unknown> = Promise.resolve()
export function requestSecurityProof(info: SecurityChallenge): Promise<string> {
  const next = queue.then(
    () =>
      new Promise<string>((resolve, reject) => {
        const host = document.createElement('div')
        document.body.appendChild(host)
        const root = createRoot(host)
        const done = (token?: string) => {
          root.unmount()
          host.remove()
          if (token) resolve(token)
          else reject(new Error(t('Verification cancelled')))
        }
        root.render(<ProofPrompt info={info} done={done} />)
      })
  )
  queue = next.catch(() => undefined)
  return next
}
