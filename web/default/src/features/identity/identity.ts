export type IdentityType = 'none' | 'enterprise' | 'education'

export function normalizeIdentityType(value?: string): IdentityType {
  if (value === 'enterprise' || value === 'education') return value
  return 'none'
}
