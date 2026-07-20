/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { ArrowLeftRight, LockKeyhole, Scale, Zap } from 'lucide-react'
import { useTranslation } from 'react-i18next'

export function EditorialFeatureStrip() {
  const { t } = useTranslation()
  const items = [
    {
      title: t('Ephemeral Requests'),
      detail: t('Request bodies are not retained'),
      icon: LockKeyhole,
    },
    {
      title: t('Unmodified Requests'),
      detail: t('No distillation or substitution'),
      icon: ArrowLeftRight,
    },
    {
      title: t('Transparent Pricing'),
      detail: t('No hidden multipliers or fees'),
      icon: Scale,
    },
    {
      title: t('Stable Connection'),
      detail: t('On par with direct official access'),
      icon: Zap,
    },
  ]

  return (
    <div className='border-border/65 mt-5 grid grid-cols-2 border-y lg:grid-cols-4'>
      {items.map((item, index) => {
        const Icon = item.icon
        return (
          <div
            key={item.title}
            className='border-border/65 flex min-w-0 items-center gap-2.5 px-2 py-3.5 even:border-l lg:px-4 lg:even:border-l lg:[&:not(:first-child)]:border-l'
          >
            <span className='border-border/70 bg-background flex size-8 shrink-0 items-center justify-center rounded-lg border shadow-xs'>
              <Icon className='size-3.5' />
            </span>
            <div className='min-w-0'>
              <p className='truncate text-xs font-semibold'>{item.title}</p>
              <p className='text-muted-foreground mt-0.5 truncate text-[10px]'>
                {item.detail}
              </p>
            </div>
            {index === 0 && <span className='sr-only'>{t('API Keys')}</span>}
          </div>
        )
      })}
    </div>
  )
}
