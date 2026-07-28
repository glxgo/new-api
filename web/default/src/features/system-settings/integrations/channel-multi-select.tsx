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
import * as React from 'react'
import { Check, ChevronsUpDown } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from '@/components/ui/command'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'

export type ChannelOption = { id: number; name: string }

type Props = {
  channels: ChannelOption[]
  selectedIds: number[]
  onChange: (ids: number[]) => void
  disabled?: boolean
  className?: string
}

// 渠道多选（用于"监控与警报"选择性探测）。留空 = 探测所有渠道。
export function ChannelMultiSelect({
  channels,
  selectedIds,
  onChange,
  disabled,
  className,
}: Props) {
  const { t } = useTranslation()
  const [open, setOpen] = React.useState(false)
  const [search, setSearch] = React.useState('')

  const selectedSet = new Set(selectedIds)

  const toggle = (id: number) => {
    onChange(
      selectedSet.has(id)
        ? selectedIds.filter((x) => x !== id)
        : [...selectedIds, id]
    )
  }

  const filtered = React.useMemo(() => {
    const q = search.trim().toLowerCase()
    if (!q) return channels
    return channels.filter(
      (c) => c.name.toLowerCase().includes(q) || String(c.id).includes(q)
    )
  }, [channels, search])

  const selectedChannels = channels.filter((c) => selectedSet.has(c.id))

  return (
    <div className={cn('space-y-2', className)}>
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger
          render={
            <Button
              type='button'
              variant='outline'
              role='combobox'
              aria-expanded={open}
              disabled={disabled}
              className='w-full justify-between font-normal'
            />
          }
        >
          <span className='truncate'>
            {selectedIds.length === 0
              ? t('All channels (probe all)')
              : t('{{count}} channels selected', { count: selectedIds.length })}
          </span>
          <ChevronsUpDown className='ml-2 size-4 shrink-0 opacity-50' />
        </PopoverTrigger>
        <PopoverContent className='w-(--anchor-width) p-0' align='start'>
          <Command shouldFilter={false}>
            <CommandInput
              placeholder={t('Search channels...')}
              value={search}
              onValueChange={setSearch}
            />
            <CommandList>
              <CommandEmpty>{t('No channels found')}</CommandEmpty>
              <CommandGroup>
                {filtered.map((ch) => (
                  <CommandItem
                    key={ch.id}
                    value={`${ch.name}-${ch.id}`}
                    onSelect={() => toggle(ch.id)}
                  >
                    <Check
                      className={cn(
                        'mr-1 size-4',
                        selectedSet.has(ch.id) ? 'opacity-100' : 'opacity-0'
                      )}
                    />
                    <span className='flex-1 truncate'>{ch.name}</span>
                    <span className='text-muted-foreground text-xs'>
                      #{ch.id}
                    </span>
                  </CommandItem>
                ))}
              </CommandGroup>
            </CommandList>
          </Command>
        </PopoverContent>
      </Popover>
      {selectedChannels.length > 0 && (
        <div className='flex flex-wrap gap-1'>
          {selectedChannels.map((ch) => (
            <Badge key={ch.id} variant='secondary' className='gap-1 pr-1'>
              {ch.name}
              <button
                type='button'
                onClick={() => toggle(ch.id)}
                className='hover:bg-muted-foreground/20 flex size-4 items-center justify-center rounded-full text-xs leading-none'
                aria-label={t('Remove')}
              >
                ×
              </button>
            </Badge>
          ))}
        </div>
      )}
    </div>
  )
}
