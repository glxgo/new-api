/*
Copyright (C) 2023-2026 QuantumNous
*/
import { HandCoins } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { SettingsSection } from '../components/settings-section'

const rules = [
  ['普通用户邀新', '直属充值 5%，二级充值 2%'],
  ['代理邀新', '直属充值 8%，二级充值 4%'],
  ['管理员分润', '直属用户充值 15%，二级用户充值 5%'],
  ['超级管理员', '全部有效充值事件固定 5%'],
] as const

// Fixed business policy: intentionally read-only. Model prices, channel costs,
// balance purchases and free membership grants never change or trigger it.
export function DividendSettingsSection() {
  const { t } = useTranslation()
  return (
    <SettingsSection title={t('固定充值分润规则')}>
      <div className='border-border/70 bg-muted/15 overflow-hidden rounded-xl border'>
        <div className='flex items-start gap-3 border-b border-dashed px-5 py-4'>
          <div className='bg-background flex size-9 shrink-0 items-center justify-center rounded-lg border shadow-xs'>
            <HandCoins className='size-4' />
          </div>
          <div>
            <h3 className='text-sm font-semibold'>{t('充值事件固定比例')}</h3>
            <p className='text-muted-foreground mt-1 text-xs leading-relaxed'>
              {t(
                '比例由业务规则固定，不再根据售价、成本、毛利或套餐到期结果计算。'
              )}
            </p>
          </div>
        </div>
        <div className='grid gap-3 p-5 sm:grid-cols-2'>
          {rules.map(([label, value]) => (
            <div key={label} className='bg-background rounded-lg border p-4'>
              <div className='text-sm font-medium'>{t(label)}</div>
              <div className='text-muted-foreground mt-1 text-xs'>
                {t(value)}
              </div>
            </div>
          ))}
        </div>
        <p className='text-muted-foreground border-t border-dashed px-5 py-4 text-xs leading-relaxed'>
          {t(
            '外部付款、人工补单和管理员充值均计入累充与分润；余额购买和免费会员发放不重复触发。历史已结算记录保持不变。'
          )}
        </p>
      </div>
    </SettingsSection>
  )
}
