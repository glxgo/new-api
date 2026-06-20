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
import { useEffect, useMemo, useState } from 'react'
import { Plus, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

// 分组(渠道分组维度)独立模型定价: 覆盖全局 ModelRatio/ModelCost, miss 回退全局。
// GroupRatio 倍率已废(计费直接用分组价)。详见 plan mellow-growing-waterfall.md 需求2。

interface CostJson {
  input_cost_per_m?: number
  output_cost_per_m?: number
  cache_cost_per_m?: number
  cost_per_request?: number
  cost_expr?: string
}

interface Row {
  model: string
  ratio: string // 售价 ratio(空=继承全局)
  costInput: string // 成本输入 $/1M(空=继承全局)
  costOutput: string // 成本输出 $/1M(空=继承全局)
}

interface GroupModelPricingDefaultValues {
  ratio: string // GroupModelRatio JSON {group:{model:ratio}}
  price: string // GroupModelPrice JSON
  cost: string // GroupModelCost JSON {group:{model:{input,output,...}}}
}

function parseNested<T>(s: string): Record<string, Record<string, T>> {
  if (!s || s.trim() === '') return {}
  try {
    const p = JSON.parse(s)
    return p && typeof p === 'object' ? (p as Record<string, Record<string, T>>) : {}
  } catch {
    return {}
  }
}

function rowsForGroup(
  group: string,
  ratioMap: Record<string, Record<string, number>>,
  costMap: Record<string, Record<string, CostJson>>
): Row[] {
  const r = ratioMap[group] || {}
  const c = costMap[group] || {}
  const models = Array.from(new Set([...Object.keys(r), ...Object.keys(c)]))
  return models.map((m) => ({
    model: m,
    ratio: r[m] != null ? String(r[m]) : '',
    costInput: c[m]?.input_cost_per_m != null ? String(c[m]!.input_cost_per_m) : '',
    costOutput: c[m]?.output_cost_per_m != null ? String(c[m]!.output_cost_per_m) : '',
  }))
}

export function GroupModelPricingSection({
  defaultValues,
}: {
  defaultValues: GroupModelPricingDefaultValues
}) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [group, setGroup] = useState('')
  const [rows, setRows] = useState<Row[]>([])
  const [newModel, setNewModel] = useState('')
  const [saving, setSaving] = useState(false)

  const ratioMap = useMemo(() => parseNested<number>(defaultValues.ratio), [defaultValues.ratio])
  const priceMap = useMemo(() => parseNested<number>(defaultValues.price), [defaultValues.price])
  const costMap = useMemo(() => parseNested<CostJson>(defaultValues.cost), [defaultValues.cost])

  const loadGroup = (g: string) => {
    setGroup(g)
    setRows(rowsForGroup(g, ratioMap, costMap))
  }

  // 默认选第一个已配分组
  useEffect(() => {
    if (group) return
    const allGroups = Array.from(
      new Set([...Object.keys(ratioMap), ...Object.keys(priceMap), ...Object.keys(costMap)])
    )
    if (allGroups.length > 0) loadGroup(allGroups[0])
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [ratioMap, priceMap, costMap])

  const addRow = () => {
    const name = newModel.trim()
    if (!name) return
    if (rows.some((r) => r.model === name)) {
      toast.error(t('Model already exists'))
      return
    }
    setRows([...rows, { model: name, ratio: '', costInput: '', costOutput: '' }])
    setNewModel('')
  }
  const updateRow = (idx: number, patch: Partial<Row>) =>
    setRows(rows.map((r, i) => (i === idx ? { ...r, ...patch } : r)))
  const removeRow = (idx: number) => setRows(rows.filter((_, i) => i !== idx))

  const dirty = useMemo(() => {
    if (!group) return false
    return JSON.stringify(rows) !== JSON.stringify(rowsForGroup(group, ratioMap, costMap))
  }, [rows, group, ratioMap, costMap])

  const save = async () => {
    if (!group) {
      toast.error(t('Select a group first'))
      return
    }
    setSaving(true)
    try {
      const newRatio = { ...ratioMap }
      const newCost = { ...costMap }
      const groupRatio: Record<string, number> = {}
      const groupCost: Record<string, CostJson> = {}
      for (const row of rows) {
        if (row.ratio !== '' && !Number.isNaN(Number(row.ratio))) {
          groupRatio[row.model] = Number(row.ratio)
        }
        if (row.costInput !== '' || row.costOutput !== '') {
          const prev = newCost[group]?.[row.model] || {}
          groupCost[row.model] = {
            ...prev,
            input_cost_per_m:
              row.costInput !== '' ? Number(row.costInput) : (prev.input_cost_per_m ?? 0),
            output_cost_per_m:
              row.costOutput !== '' ? Number(row.costOutput) : (prev.output_cost_per_m ?? 0),
          }
        }
      }
      newRatio[group] = groupRatio
      newCost[group] = groupCost
      await updateOption.mutateAsync({ key: 'GroupModelRatio', value: JSON.stringify(newRatio) })
      await updateOption.mutateAsync({ key: 'GroupModelCost', value: JSON.stringify(newCost) })
    } finally {
      setSaving(false)
    }
  }

  return (
    <SettingsSection title={t('Group Model Pricing')}>
      <SettingsPageFormActions
        onSave={save}
        isSaving={saving || updateOption.isPending}
        isSaveDisabled={!dirty}
        saveLabel='Save group model pricing'
      />
      <p className='text-muted-foreground text-sm'>
        {t(
          'Per-group (channel group) override of model sale ratio and cost. Empty = inherit global. GroupRatio multiplier is deprecated — billing uses the group price directly.'
        )}
      </p>
      <div className='flex items-center gap-2'>
        <Input
          placeholder={t('Group name (e.g. default, vip)')}
          value={group}
          onChange={(e) => setGroup(e.target.value)}
          className='max-w-[240px]'
        />
        <Button variant='outline' size='sm' onClick={() => loadGroup(group)}>
          {t('Load')}
        </Button>
      </div>
      {group && (
        <div className='overflow-hidden rounded-lg border'>
          <div className='grid grid-cols-[1fr_110px_120px_120px_36px] gap-2 border-b bg-muted/50 px-3 py-2 text-xs font-medium'>
            <span>{t('Model')}</span>
            <span>{t('Sale Ratio')}</span>
            <span>{t('Cost In $/M')}</span>
            <span>{t('Cost Out $/M')}</span>
            <span />
          </div>
          <div className='divide-y'>
            {rows.length === 0 ? (
              <div className='text-muted-foreground px-3 py-6 text-center text-sm'>
                {t('No models configured for this group. Add one below.')}
              </div>
            ) : (
              rows.map((row, idx) => (
                <div
                  key={row.model}
                  className='grid grid-cols-[1fr_110px_120px_120px_36px] items-center gap-2 px-3 py-2'
                >
                  <span className='truncate text-sm font-medium'>{row.model}</span>
                  <Input
                    className='h-8'
                    type='number'
                    step='0.001'
                    value={row.ratio}
                    onChange={(e) => updateRow(idx, { ratio: e.target.value })}
                    placeholder={t('global')}
                  />
                  <Input
                    className='h-8'
                    type='number'
                    step='0.01'
                    value={row.costInput}
                    onChange={(e) => updateRow(idx, { costInput: e.target.value })}
                    placeholder={t('global')}
                  />
                  <Input
                    className='h-8'
                    type='number'
                    step='0.01'
                    value={row.costOutput}
                    onChange={(e) => updateRow(idx, { costOutput: e.target.value })}
                    placeholder={t('global')}
                  />
                  <Button
                    size='icon'
                    variant='ghost'
                    className='h-8 w-8'
                    onClick={() => removeRow(idx)}
                  >
                    <Trash2 className='size-4' />
                  </Button>
                </div>
              ))
            )}
          </div>
          <div className='flex items-center gap-2 border-t px-3 py-2'>
            <Input
              className='h-8 max-w-[240px]'
              placeholder={t('Add model name')}
              value={newModel}
              onChange={(e) => setNewModel(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') {
                  e.preventDefault()
                  addRow()
                }
              }}
            />
            <Button size='sm' variant='outline' onClick={addRow}>
              <Plus className='size-4' />
              {t('Add')}
            </Button>
          </div>
        </div>
      )}
    </SettingsSection>
  )
}
