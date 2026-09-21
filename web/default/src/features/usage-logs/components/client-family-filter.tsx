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
import { useTranslation } from 'react-i18next'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

const families = [
  ['all', 'All clients'],
  ['codex', 'Codex'],
  ['claude_code', 'Claude Code'],
  ['pi', 'Pi'],
  ['opencode', 'OpenCode'],
  ['newapi', 'NewAPI'],
  ['zcode', 'ZCode'],
  ['deepseek_harness', 'DeepSeek Harness (DSH)'],
  ['openclaw', 'OpenClaw'],
  ['cherry_studio', 'Cherry Studio'],
  ['openai_sdk', 'OpenAI SDK'],
  ['node', 'Node'],
  ['bun', 'Bun'],
  ['python', 'Python'],
  ['browser', 'Browser'],
  ['curl', 'curl'],
  ['unknown', 'Unknown client'],
  ['unrecorded', 'Not recorded'],
]
export function ClientFamilyFilter(props: {
  value?: string
  onChange: (value: string) => void
}) {
  const { t } = useTranslation()
  const items = families.map(([value, label]) => ({ value, label: t(label) }))
  return (
    <Select
      items={items}
      value={props.value || 'all'}
      onValueChange={(value) =>
        props.onChange(value === 'all' ? '' : value || '')
      }
    >
      <SelectTrigger aria-label={t('Client category')}>
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {items.map((item) => (
          <SelectItem key={item.value} value={item.value}>
            {item.label}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}
