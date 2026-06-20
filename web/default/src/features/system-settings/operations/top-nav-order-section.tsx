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
import { useState } from 'react'
import { ChevronDown, ChevronUp, GripVertical } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

// 可排序的顶部导航项(key 对应 use-top-nav-links 的 navKey)
const NAV_ITEMS: ReadonlyArray<{ key: string; labelKey: string }> = [
  { key: 'home', labelKey: 'Home' },
  { key: 'console', labelKey: 'Console' },
  { key: 'pricing', labelKey: 'Model Square' },
  { key: 'rankings', labelKey: 'Rankings' },
  { key: 'docs', labelKey: 'Docs' },
  { key: 'about', labelKey: 'About' },
  { key: 'tutorial', labelKey: 'Tutorial' },
  { key: 'faq', labelKey: 'FAQ' },
]

function parseOrder(raw: string): string[] {
  const defaults = NAV_ITEMS.map((i) => i.key)
  if (!raw || !raw.trim()) return defaults
  try {
    const parsed = JSON.parse(raw)
    if (Array.isArray(parsed)) {
      const valid = parsed.filter(
        (x): x is string =>
          typeof x === 'string' && NAV_ITEMS.some((i) => i.key === x)
      )
      // 补上缺失项(默认追加到末尾)
      NAV_ITEMS.forEach((i) => {
        if (!valid.includes(i.key)) valid.push(i.key)
      })
      return valid
    }
  } catch {
    // 非法配置 → 默认顺序
  }
  return defaults
}

export function TopNavOrderSection({
  defaultValues,
}: {
  defaultValues: { topNavOrder: string }
}) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const initial = parseOrder(defaultValues.topNavOrder)
  const [order, setOrder] = useState<string[]>(initial)
  const [saving, setSaving] = useState(false)

  const labelOf = (key: string) => {
    const item = NAV_ITEMS.find((i) => i.key === key)
    return item ? t(item.labelKey) : key
  }

  const move = (idx: number, dir: -1 | 1) => {
    const target = idx + dir
    if (target < 0 || target >= order.length) return
    const next = [...order]
    ;[next[idx], next[target]] = [next[target], next[idx]]
    setOrder(next)
  }

  const dirty = JSON.stringify(order) !== JSON.stringify(initial)

  const save = async () => {
    setSaving(true)
    try {
      await updateOption.mutateAsync({
        key: 'TopNavOrder',
        value: JSON.stringify(order),
      })
    } finally {
      setSaving(false)
    }
  }

  return (
    <SettingsSection title={t('Top Navigation Order')}>
      <SettingsPageFormActions
        onSave={save}
        isSaving={saving || updateOption.isPending}
        isSaveDisabled={!dirty}
        saveLabel='Save top nav order'
      />
      <p className='text-muted-foreground text-sm'>
        {t(
          'Adjust the display order of the top navigation links. Links disabled in header modules remain hidden.'
        )}
      </p>
      <div className='space-y-2'>
        {order.map((key, idx) => (
          <div
            key={key}
            className='bg-card flex items-center justify-between rounded-lg border px-4 py-2.5'
          >
            <div className='flex items-center gap-2'>
              <GripVertical className='text-muted-foreground/40 size-4' />
              <span className='text-sm font-medium'>{labelOf(key)}</span>
            </div>
            <div className='flex gap-1'>
              <Button
                size='icon'
                variant='outline'
                className='size-7'
                disabled={idx === 0 || saving}
                onClick={() => move(idx, -1)}
              >
                <ChevronUp className='size-4' />
              </Button>
              <Button
                size='icon'
                variant='outline'
                className='size-7'
                disabled={idx === order.length - 1 || saving}
                onClick={() => move(idx, 1)}
              >
                <ChevronDown className='size-4' />
              </Button>
            </div>
          </div>
        ))}
      </div>
    </SettingsSection>
  )
}
