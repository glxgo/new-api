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
import { useMemo, useState } from 'react'
import { Check, ChevronsUpDown } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { useIsMobile } from '@/hooks/use-mobile'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
} from '@/components/ui/command'
import {
  Drawer,
  DrawerContent,
  DrawerHeader,
  DrawerTitle,
  DrawerTrigger,
} from '@/components/ui/drawer'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'

export type ApiKeyGroupOption = {
  value: string
  label: string
  desc?: string
  ratio?: number | string
}

type ApiKeyGroupComboboxProps = {
  options: ApiKeyGroupOption[]
  value?: string
  onValueChange: (value: string) => void
  placeholder?: string
  disabled?: boolean
  // Groups the user has unlocked via an active subscription plan. These are
  // pinned to the top of the list and badged as a "Plan Group".
  subscribedGroups?: string[]
}

function formatGroupRatio(
  ratio: ApiKeyGroupOption['ratio'],
  ratioLabel: string
) {
  if (ratio === undefined || ratio === null || ratio === '') return null
  return `${ratio}x ${ratioLabel}`
}

function getRatioBadgeClassName(ratio: ApiKeyGroupOption['ratio']) {
  if (typeof ratio !== 'number') {
    return 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-900/60 dark:bg-emerald-950/40 dark:text-emerald-300'
  }

  if (ratio > 5) {
    return 'border-rose-200 bg-rose-50 text-rose-700 dark:border-rose-900/60 dark:bg-rose-950/40 dark:text-rose-300'
  }
  if (ratio > 3) {
    return 'border-orange-200 bg-orange-50 text-orange-700 dark:border-orange-900/60 dark:bg-orange-950/40 dark:text-orange-300'
  }
  if (ratio > 1) {
    return 'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-900/60 dark:bg-blue-950/40 dark:text-blue-300'
  }
  return 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-900/60 dark:bg-emerald-950/40 dark:text-emerald-300'
}

function GroupRatioBadge({
  ratio,
  compact = false,
}: {
  ratio: ApiKeyGroupOption['ratio']
  compact?: boolean
}) {
  const { t } = useTranslation()
  const label = compact
    ? ratio === undefined || ratio === null || ratio === ''
      ? null
      : `${ratio}x`
    : formatGroupRatio(ratio, t('Ratio'))

  if (!label) return null

  return (
    <Badge
      variant='outline'
      className={cn(
        'max-w-24 shrink-0 truncate text-[10px] sm:max-w-none sm:text-xs',
        getRatioBadgeClassName(ratio)
      )}
    >
      {label}
    </Badge>
  )
}

function PlanGroupBadge() {
  const { t } = useTranslation()
  return (
    <Badge
      variant='outline'
      className='shrink-0 border-violet-200 bg-violet-50 text-[10px] text-violet-700 sm:text-xs dark:border-violet-900/60 dark:bg-violet-950/40 dark:text-violet-300'
    >
      {t('Plan Group')}
    </Badge>
  )
}

export function ApiKeyGroupCombobox({
  options,
  value,
  onValueChange,
  placeholder,
  disabled,
  subscribedGroups,
}: ApiKeyGroupComboboxProps) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [searchValue, setSearchValue] = useState('')
  const isMobile = useIsMobile()
  const selectedOption = options.find((option) => option.value === value)
  const subscribedSet = useMemo(
    () => new Set((subscribedGroups ?? []).filter(Boolean)),
    [subscribedGroups]
  )
  const isSubscribed = (val: string) => subscribedSet.has(val)
  const selectedIsSubscribed = !!value && isSubscribed(value)

  const filteredOptions = useMemo(() => {
    const search = searchValue.trim().toLowerCase()
    const matched = search
      ? options.filter((option) => {
          const ratioText = String(option.ratio ?? '').toLowerCase()
          return (
            option.value.toLowerCase().includes(search) ||
            option.label.toLowerCase().includes(search) ||
            option.desc?.toLowerCase().includes(search) ||
            ratioText.includes(search)
          )
        })
      : options
    // Stable sort: subscribed plan groups first (preserving original order),
    // then the rest. Keeps the highlighting deterministic across re-renders.
    return [...matched].sort((a, b) => {
      const aSub = isSubscribed(a.value) ? 0 : 1
      const bSub = isSubscribed(b.value) ? 0 : 1
      return aSub - bSub
    })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [options, searchValue, subscribedSet])

  const handleSelect = (selectedValue: string) => {
    onValueChange(selectedValue)
    setOpen(false)
    setSearchValue('')
  }

  const subscribedOptions = filteredOptions.filter((option) =>
    isSubscribed(option.value)
  )
  const otherOptions = filteredOptions.filter(
    (option) => !isSubscribed(option.value)
  )

  const renderOption = (option: ApiKeyGroupOption) => (
    <CommandItem
      key={option.value}
      value={option.value}
      onSelect={() => handleSelect(option.value)}
      className={cn(
        'data-[selected=true]:bg-muted grid min-h-16 grid-cols-[1rem_minmax(0,1fr)_auto] items-start gap-x-3 rounded-lg px-3 py-3 transition-colors',
        '[&>svg:last-child]:hidden',
        value === option.value && 'bg-muted/60'
      )}
    >
      <Check
        className={cn(
          'mt-0.5 size-4',
          value === option.value ? 'opacity-100' : 'opacity-0'
        )}
      />
      <span className='min-w-0'>
        <span
          className='text-foreground block text-sm leading-5 font-medium break-words'
          title={option.label}
        >
          {option.label}
        </span>
        {option.desc && (
          <span
            className='text-muted-foreground mt-0.5 line-clamp-2 text-xs leading-[1.125rem] break-words'
            title={option.desc}
          >
            {option.desc}
          </span>
        )}
      </span>
      <span className='mt-0.5 flex shrink-0 items-center'>
        <GroupRatioBadge ratio={option.ratio} compact />
      </span>
    </CommandItem>
  )

  const renderCommandContent = (mobile = false) => (
    <Command
      shouldFilter={false}
      className={cn(
        mobile &&
          'min-h-0 flex-1 *:data-[slot=command-input]:text-base *:data-[slot=input-group]:h-11!'
      )}
    >
      <CommandInput
        placeholder={t('Search...')}
        value={searchValue}
        onValueChange={setSearchValue}
      />
      <CommandList className={mobile ? 'max-h-[55dvh] pb-2' : 'max-h-[360px]'}>
        <CommandEmpty>{t('No group found.')}</CommandEmpty>
        {subscribedOptions.length > 0 && (
          <CommandGroup heading={t('Plan Group')}>
            {subscribedOptions.map(renderOption)}
          </CommandGroup>
        )}
        {subscribedOptions.length > 0 && otherOptions.length > 0 && (
          <CommandSeparator />
        )}
        {otherOptions.length > 0 && (
          <CommandGroup
            heading={subscribedOptions.length > 0 ? t('Other') : undefined}
          >
            {otherOptions.map(renderOption)}
          </CommandGroup>
        )}
      </CommandList>
    </Command>
  )

  const triggerContent = (
    <>
      <span className='flex min-w-0 flex-1 items-center justify-between gap-2 sm:gap-3'>
        <span className='min-w-0'>
          <span className='block truncate font-medium'>
            {selectedOption?.label || placeholder || t('Select a group')}
          </span>
          {selectedOption?.desc && (
            <span className='text-muted-foreground block truncate text-[11px] sm:text-xs'>
              {selectedOption.desc}
            </span>
          )}
        </span>
        <span className='hidden items-center gap-1.5 sm:flex'>
          {selectedIsSubscribed && <PlanGroupBadge />}
          <GroupRatioBadge ratio={selectedOption?.ratio} />
        </span>
      </span>
      <ChevronsUpDown className='h-4 w-4 shrink-0 opacity-50' />
    </>
  )

  const triggerClassName =
    'border-input bg-muted/40 hover:bg-muted/55 hover:text-foreground active:bg-background data-popup-open:border-ring data-popup-open:bg-background data-popup-open:ring-ring/20 h-auto min-h-14 w-full justify-between gap-2 rounded-lg px-3 py-2 text-start shadow-none transition-[background-color,border-color,box-shadow] duration-150 data-popup-open:ring-[3px] sm:min-h-20 sm:gap-3 sm:px-4 sm:py-3'

  if (isMobile) {
    return (
      <Drawer open={open} onOpenChange={setOpen}>
        <DrawerTrigger asChild>
          <Button
            type='button'
            variant='outline'
            role='combobox'
            aria-expanded={open}
            disabled={disabled}
            className={triggerClassName}
          >
            {triggerContent}
          </Button>
        </DrawerTrigger>
        <DrawerContent className='max-h-[78dvh]'>
          <DrawerHeader className='pb-2 text-left'>
            <DrawerTitle>{t('Select a group')}</DrawerTitle>
          </DrawerHeader>
          <div className='flex min-h-0 flex-1 px-3 pb-[max(1rem,env(safe-area-inset-bottom))]'>
            {renderCommandContent(true)}
          </div>
        </DrawerContent>
      </Drawer>
    )
  }

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger
        render={
          <Button
            type='button'
            variant='outline'
            role='combobox'
            aria-expanded={open}
            disabled={disabled}
            className={triggerClassName}
          />
        }
      >
        {triggerContent}
      </PopoverTrigger>
      <PopoverContent
        className='data-closed:zoom-out-95 data-open:zoom-in-95 data-[side=bottom]:slide-in-from-top-1 data-[side=left]:slide-in-from-right-1 data-[side=right]:slide-in-from-left-1 data-[side=top]:slide-in-from-bottom-1 w-[26rem] max-w-[calc(100vw-1.5rem)] origin-(--transform-origin) overflow-hidden rounded-xl p-0 shadow-lg data-closed:duration-100 data-open:duration-150 motion-reduce:duration-0'
        align='start'
        collisionPadding={12}
        onWheel={(event) => event.stopPropagation()}
        onTouchMove={(event) => event.stopPropagation()}
        onPointerDown={(event) => event.stopPropagation()}
      >
        {renderCommandContent()}
      </PopoverContent>
    </Popover>
  )
}
