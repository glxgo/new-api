/*
Copyright (C) 2023-2026 QuantumNous
*/
import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { HandCoins, Save } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import { ROLE } from '@/lib/roles'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  getCommissionSettings,
  updateCommissionSettings,
  type CommissionSettings,
} from '../api'
import { SettingsSection } from '../components/settings-section'

const rules = [
  ['普通用户邀新', 'ordinary_direct_bp', 'ordinary_indirect_bp'],
  ['代理邀新', 'agent_direct_bp', 'agent_indirect_bp'],
  ['管理员分润', 'admin_direct_bp', 'admin_indirect_bp'],
  ['超级管理员', 'root_bp', null],
] as const

type RateKey = keyof CommissionSettings

function percentValue(value: number) {
  return (Number(value || 0) / 100).toFixed(2).replace(/\.00$/, '')
}

export function DividendSettingsSection() {
  const { t } = useTranslation()
  const isRoot =
    (useAuthStore((state) => state.auth.user?.role) ?? 0) >= ROLE.SUPER_ADMIN
  const query = useQuery({
    queryKey: ['commission-settings'],
    queryFn: getCommissionSettings,
    enabled: isRoot,
  })
  const [values, setValues] = useState<Partial<CommissionSettings>>({})
  const [saving, setSaving] = useState(false)

  const changed = useMemo(() => Object.keys(values).length > 0, [values])
  const save = async () => {
    if (!changed || saving) return
    const payload: Partial<CommissionSettings> = {}
    for (const [key, value] of Object.entries(values)) {
      const parsed = Number(value)
      // `values` stores basis points (10000 = 100%). Validate in the same
      // unit before persisting so values such as 5%, 8%, and 15% remain
      // editable instead of being rejected as >100.
      if (!Number.isFinite(parsed) || parsed < 0 || parsed > 10000) {
        toast.error(t('Commission rates must be between 0% and 100%'))
        return
      }
      // `values` stores basis points (the input handler converts the visible
      // percentage to `percent * 100`).  Do not multiply a second time here:
      // doing so turns a visible 5% into 50000 bp and makes every normal save
      // fail the server-side 0..10000 validation.
      payload[key as RateKey] = Math.round(parsed) as never
    }
    setSaving(true)
    try {
      const result = await updateCommissionSettings(payload)
      if (!result.success || !result.data)
        throw new Error(result.message || 'save failed')
      setValues({})
      await query.refetch()
      toast.success(t('Commission settings saved'))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('Save failed'))
    } finally {
      setSaving(false)
    }
  }

  return (
    <SettingsSection title={t('Dividend & Rebate Settings')}>
      <div className='border-border/70 bg-muted/15 overflow-hidden rounded-xl border'>
        <div className='flex items-start gap-3 border-b border-dashed px-5 py-4'>
          <div className='bg-background flex size-9 shrink-0 items-center justify-center rounded-lg border shadow-xs'>
            <HandCoins className='size-4' />
          </div>
          <div>
            <h3 className='text-sm font-semibold'>
              {t('Recharge commission rates')}
            </h3>
            <p className='text-muted-foreground mt-1 text-xs leading-relaxed'>
              {t(
                'Only future recharge settlements use the new rates; historical records stay unchanged.'
              )}
            </p>
          </div>
        </div>
        <div className='grid gap-3 p-5 sm:grid-cols-2'>
          {rules.map(([label, direct, indirect]) => {
            const directKey = direct as RateKey
            const indirectKey = indirect as RateKey | null
            return (
              <div key={label} className='bg-background rounded-lg border p-4'>
                <div className='mb-3 text-sm font-medium'>{t(label)}</div>
                <div className='grid gap-3 sm:grid-cols-2'>
                  <div className='space-y-1.5'>
                    <Label className='text-muted-foreground text-xs'>
                      {t(indirectKey ? 'Direct' : 'Rate')} (%)
                    </Label>
                    <Input
                      type='number'
                      min={0}
                      max={100}
                      step={0.01}
                      disabled={!isRoot || query.isLoading || saving}
                      value={percentValue(
                        values[directKey] ?? query.data?.data?.[directKey] ?? 0
                      )}
                      onChange={(event) =>
                        setValues((prev) => ({
                          ...prev,
                          [directKey]: Number(event.target.value) * 100,
                        }))
                      }
                    />
                  </div>
                  {indirectKey ? (
                    <div className='space-y-1.5'>
                      <Label className='text-muted-foreground text-xs'>
                        {t('Indirect')} (%)
                      </Label>
                      <Input
                        type='number'
                        min={0}
                        max={100}
                        step={0.01}
                        disabled={!isRoot || query.isLoading || saving}
                        value={percentValue(
                          values[indirectKey] ??
                            query.data?.data?.[indirectKey] ??
                            0
                        )}
                        onChange={(event) =>
                          setValues((prev) => ({
                            ...prev,
                            [indirectKey]: Number(event.target.value) * 100,
                          }))
                        }
                      />
                    </div>
                  ) : null}
                </div>
              </div>
            )
          })}
        </div>
        {isRoot ? (
          <div className='flex justify-end border-t border-dashed px-5 py-4'>
            <Button onClick={() => void save()} disabled={!changed || saving}>
              <Save className='size-4' />
              {saving ? t('Saving...') : t('Save commission settings')}
            </Button>
          </div>
        ) : (
          <p className='text-muted-foreground border-t border-dashed px-5 py-4 text-xs'>
            {t('Only the super administrator can edit commission rates.')}
          </p>
        )}
      </div>
    </SettingsSection>
  )
}
