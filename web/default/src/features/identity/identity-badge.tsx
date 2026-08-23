import { cn } from '@/lib/utils'
import { normalizeIdentityType } from './identity'

export function IdentityBadge(props: {
  identityType?: string
  className?: string
}) {
  const identity = normalizeIdentityType(props.identityType)
  if (identity === 'none') return null

  if (identity === 'enterprise') {
    return (
      <span
        aria-label='ENTERPRISE'
        className={cn(
          'inline-flex items-center rounded-full border border-[#c9a45b] bg-[#151515] px-2.5 py-0.5 text-[10px] font-bold tracking-[0.18em] text-[#e5c477] shadow-xs',
          props.className
        )}
      >
        ENTERPRISE
      </span>
    )
  }

  return (
    <span
      aria-label='student'
      className={cn(
        'inline-flex items-center rounded-full border border-black bg-white px-2.5 py-0.5 text-[10px] font-semibold tracking-[0.12em] text-blue-700 shadow-xs dark:border-white dark:bg-white',
        props.className
      )}
    >
      student
    </span>
  )
}
