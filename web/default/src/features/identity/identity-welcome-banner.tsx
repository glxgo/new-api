import { Building2, GraduationCap } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth-store'
import { cn } from '@/lib/utils'
import { normalizeIdentityType } from './identity'
import { IdentityBadge } from './identity-badge'

export function IdentityWelcomeBanner(props: { className?: string }) {
  const { t } = useTranslation()
  const user = useAuthStore((state) => state.auth.user)
  const identity = normalizeIdentityType(user?.identity_type)

  // The banner is intentionally not dismissible: the product requirement is
  // to show the verified enterprise/education notice on every site entry.
  if (!user || identity === 'none') return null
  const isEnterprise = identity === 'enterprise'
  const Icon = isEnterprise ? Building2 : GraduationCap

  return (
    <div
      role='status'
      className={cn(
        'relative flex items-center gap-3 overflow-hidden rounded-2xl border border-[#e6cf9b] bg-linear-to-r from-[#fffdf8] via-[#fffaf0] to-[#f6ecd5] px-4 py-3 text-[#5b461d] shadow-[0_12px_32px_rgba(145,103,28,0.16)] dark:from-[#221c10] dark:via-[#2a2111] dark:to-[#1b1710] dark:text-[#f0d28a]',
        props.className
      )}
    >
      <span className='flex size-9 shrink-0 items-center justify-center rounded-xl border border-[#e6cf9b] bg-[#fff7df] text-[#9b762a] dark:bg-[#3a2d17]'>
        <Icon className='size-5' aria-hidden='true' />
      </span>
      <div className='min-w-0 flex-1'>
        <div className='flex flex-wrap items-center gap-2 text-sm font-semibold'>
          <span>
            {t('Welcome back')}, {user.username}
          </span>
          <IdentityBadge identityType={identity} />
        </div>
        <p className='mt-0.5 text-xs text-[#806a3b] dark:text-[#d6bb7b]'>
          {isEnterprise
            ? t('企业身份已识别，祝你今天使用顺利。')
            : t('教育身份已识别，祝你今天使用顺利。')}
        </p>
      </div>
    </div>
  )
}
