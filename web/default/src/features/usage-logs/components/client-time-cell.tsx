/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import type { ReactNode } from 'react'
import CherryStudio from '@lobehub/icons/es/CherryStudio'
import ClaudeCode from '@lobehub/icons/es/ClaudeCode'
import Codex from '@lobehub/icons/es/Codex'
import OpenAI from '@lobehub/icons/es/OpenAI'
import OpenClaw from '@lobehub/icons/es/OpenClaw'
import OpenCode from '@lobehub/icons/es/OpenCode'
import { Copy, Monitor } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { copyToClipboard } from '@/lib/copy-to-clipboard'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { Dialog } from '@/components/dialog'
import type { ClientSnapshot } from '../types'

const tint: Record<string, string> = {
  codex: 'from-zinc-500/15 dark:from-zinc-400/15',
  claude_code: 'from-orange-500/15 dark:from-orange-400/15',
  pi: 'from-blue-500/15 dark:from-blue-400/15',
  opencode: 'from-purple-500/15 dark:from-purple-400/15',
  newapi: 'from-cyan-600/15 dark:from-cyan-400/15',
  zcode: 'from-gray-500/15 dark:from-gray-400/15',
  deepseek_harness: 'from-blue-500/15 dark:from-blue-400/15',
}

// Product icons are shipped by the existing @lobehub/icons dependency. No
// remote UA-dependent images, invented logos, or supplier-logo substitutions.
function ClientIcon(props: { family: string }) {
  const icons = {
    codex: Codex,
    claude_code: ClaudeCode,
    opencode: OpenCode,
    openclaw: OpenClaw,
    cherry_studio: CherryStudio,
    openai_sdk: OpenAI,
  }
  const Icon = icons[props.family as keyof typeof icons]
  return (
    <span
      aria-hidden='true'
      className='flex size-3.5 shrink-0 items-center justify-center'
    >
      {Icon ? <Icon size={14} /> : <Monitor className='size-3.5' />}
    </span>
  )
}

export function ClientDetails(props: { client?: ClientSnapshot | null }) {
  const { t } = useTranslation()
  const client = props.client
  if (!client)
    return (
      <span className='text-muted-foreground text-xs'>{t('Not recorded')}</span>
    )
  const name = t(client.display_name)
  const source =
    client.source === 'generic_go_ua'
      ? t('Generic Go UA inference')
      : t('Explicit UA prefix')
  const sourceLabel =
    client.confidence === 'unknown' ? t('Unrecognized UA') : source
  return (
    <Dialog
      title={t('Client details')}
      description={t(
        'UA can be spoofed. Classification and admission do not verify official identity or guarantee cache hit rates.'
      )}
      trigger={
        <button
          type='button'
          className='focus-visible:ring-ring inline-flex min-w-0 items-center gap-1.5 rounded text-left text-xs outline-none hover:underline focus-visible:ring-2'
          aria-label={`${t('Client details')}: ${name}`}
        >
          <ClientIcon family={client.family} />
          <span className='truncate'>{name}</span>
        </button>
      }
    >
      <dl className='grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-3 text-sm'>
        <dt className='text-muted-foreground'>{t('Client name')}</dt>
        <dd className='break-words'>{name}</dd>
        <dt className='text-muted-foreground'>{t('Client variant')}</dt>
        <dd>
          {t(`client.variant.${client.variant}`, {
            defaultValue: client.variant,
          })}
        </dd>
        <dt className='text-muted-foreground'>{t('Version')}</dt>
        <dd>{client.version || t('Not recorded')}</dd>
        <dt className='text-muted-foreground'>{t('Recognition source')}</dt>
        <dd>{sourceLabel}</dd>
      </dl>
      <div className='mt-4 space-y-2'>
        <div className='flex items-center justify-between gap-2'>
          <span className='text-sm'>{t('Original User-Agent')}</span>
          <Tooltip>
            <TooltipTrigger
              render={
                <Button
                  aria-label={t('Copy User-Agent')}
                  size='icon'
                  variant='ghost'
                  onClick={async () => {
                    const ok = await copyToClipboard(client.user_agent)
                    if (ok) toast.success(t('Copied'))
                    else toast.error(t('Copy failed'))
                  }}
                />
              }
            >
              <Copy />
            </TooltipTrigger>
            <TooltipContent>{t('Copy User-Agent')}</TooltipContent>
          </Tooltip>
        </div>
        <pre
          className='bg-muted max-h-48 overflow-auto rounded-md p-3 font-mono text-xs break-all whitespace-pre-wrap'
          dir='ltr'
        >
          {client.user_agent || t('Empty User-Agent')}
        </pre>
        {client.truncated && (
          <p className='text-muted-foreground text-xs'>
            {t(
              'User-Agent truncated to 2048 bytes; control characters removed.'
            )}
          </p>
        )}
        {client.source === 'generic_go_ua' && (
          <p className='text-muted-foreground text-xs'>
            {t(
              'NewAPI is a site convention for generic Go UA; the actual sender may be another Go client.'
            )}
          </p>
        )}
      </div>
    </Dialog>
  )
}

export function ClientTimeCell(props: {
  time: ReactNode
  status: ReactNode
  client?: ClientSnapshot | null
  coding?: boolean
}) {
  return (
    <div className='@container/client-time relative isolate min-w-0 overflow-hidden rounded-md px-1 py-1'>
      {props.client && tint[props.client.family] && (
        <div
          aria-hidden='true'
          className={cn(
            'pointer-events-none absolute inset-x-0 bottom-0 -z-10 h-5 bg-gradient-to-t to-transparent',
            tint[props.client.family]
          )}
        />
      )}
      <div className='flex items-center gap-x-1.5 whitespace-nowrap'>
        <span
          className='min-w-0 truncate font-mono text-xs tabular-nums'
          title={typeof props.time === 'string' ? props.time : undefined}
        >
          {typeof props.time === 'string' ? (
            <>
              <span className='hidden @[220px]/client-time:inline'>
                {props.time}
              </span>
              <span
                className='@[220px]/client-time:hidden'
                aria-label={props.time}
              >
                {props.time.replace(/^\d{4}-\d{2}-\d{2} /, '')}
              </span>
            </>
          ) : (
            props.time
          )}
        </span>
        {props.status}
      </div>
      <div className='mt-1 flex min-h-5 min-w-0 items-center gap-1.5'>
        <ClientDetails client={props.client} />
        {props.coding && (
          <span className='border-border text-muted-foreground shrink-0 rounded border px-1 py-0.5 text-[9px] leading-none'>
            Coding
          </span>
        )}
      </div>
    </div>
  )
}
