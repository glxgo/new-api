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
import { useMemo } from 'react'
import { ExternalLink, ShieldCheck } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import { inferApiInfo } from '../lib/model-metadata'
import type { PricingModel } from '../types'

export function ModelDetailsProviderInfo(props: { model: PricingModel }) {
  const { t } = useTranslation()
  const info = useMemo(() => inferApiInfo(props.model), [props.model])

  return (
    <section>
      <h3 className='text-foreground mb-3 flex items-center gap-1.5 text-sm font-semibold'>
        <ShieldCheck className='text-muted-foreground/70 size-3.5' />
        {t('Provider & data privacy')}
      </h3>

      <div className='border-border/60 bg-border/60 grid grid-cols-1 gap-px overflow-hidden rounded-lg border sm:grid-cols-2'>
        <InfoCell label={t('Provider')}>
          <div className='flex items-center gap-1.5'>
            <span className='text-sm font-medium'>{info.vendor_label}</span>
            {info.homepage && (
              <a
                href={info.homepage}
                target='_blank'
                rel='noopener noreferrer'
                className='text-muted-foreground hover:text-foreground inline-flex items-center gap-0.5 text-[11px]'
              >
                {t('Docs')}
                <ExternalLink className='size-3' />
              </a>
            )}
          </div>
        </InfoCell>

        <InfoCell label={t('Tokenizer')}>
          <div className='flex flex-col gap-0.5'>
            <code className='font-mono text-xs'>{info.tokenizer}</code>
            {info.tokenizer_note && (
              <span className='text-muted-foreground text-[10px]'>
                {info.tokenizer_note}
              </span>
            )}
          </div>
        </InfoCell>

        <InfoCell label={t('License')}>
          <div className='flex flex-col gap-1'>
            <span className='text-sm'>{info.license}</span>
            <Badge
              variant='outline'
              className={cn(
                'h-4 w-fit px-1.5 text-[9px] font-medium',
                info.license_kind === 'open' &&
                  'border-success/30 text-success',
                info.license_kind === 'open-weight' &&
                  'border-info/30 text-info',
                info.license_kind === 'proprietary' &&
                  'border-warning/30 text-warning'
              )}
            >
              {info.license_kind === 'open'
                ? t('Open source')
                : info.license_kind === 'open-weight'
                  ? t('Open weights')
                  : info.license_kind === 'proprietary'
                    ? t('Proprietary')
                    : t('Unknown')}
            </Badge>
          </div>
        </InfoCell>

        <InfoCell label={t('Data retention')}>
          <span className='text-sm'>
            {info.data_retention_days === 0
              ? t('Zero retention')
              : `${info.data_retention_days} ${t('days')}`}
          </span>
          <span className='text-muted-foreground text-[10px]'>
            {info.training_opt_out
              ? t('Not used for upstream training by default')
              : t('May be used for training by upstream provider')}
          </span>
        </InfoCell>
      </div>
    </section>
  )
}

function InfoCell(props: { label: string; children: React.ReactNode }) {
  return (
    <div className='bg-card flex flex-col gap-1 px-3 py-2.5'>
      <span className='text-muted-foreground text-[10px] font-medium tracking-wider uppercase'>
        {props.label}
      </span>
      {props.children}
    </div>
  )
}
