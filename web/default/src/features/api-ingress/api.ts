import { api } from '@/lib/api'

export interface APIIngressProfile {
  id: number
  code: string
  display_name: string
  public_base_url: string
  network_mode: string
  description: string
  billing_multiplier_ppm: number
  multiplier: number
  enabled: boolean
  visible: boolean
  default: boolean
  probe_enabled: boolean
  sort_order: number
}

export interface APIIngressResponse<T = unknown> {
  success: boolean
  message?: string
  data?: T
}

export function resolveAPIIngressBaseUrl(
  profile: Pick<APIIngressProfile, 'public_base_url'> | null | undefined,
  fallback: string
): string {
  const raw = profile?.public_base_url?.trim() || fallback.trim()
  return raw.replace(/\/v1\/?$/, '').replace(/\/+$/, '')
}

export function resolveAPIIngressEndpoint(
  profile: Pick<APIIngressProfile, 'public_base_url'> | null | undefined,
  fallback: string
): string {
  return `${resolveAPIIngressBaseUrl(profile, fallback)}/v1`
}

export async function getAPIIngressProfiles(): Promise<
  APIIngressResponse<APIIngressProfile[]>
> {
  const res = await api.get('/api/ingress/profiles')
  return res.data
}

export async function getAdminAPIIngressProfiles(): Promise<
  APIIngressResponse<APIIngressProfile[]>
> {
  const res = await api.get('/api/ingress/admin/profiles')
  return res.data
}

export async function saveAdminAPIIngressProfile(
  data: Partial<APIIngressProfile> & { id?: number }
): Promise<APIIngressResponse<APIIngressProfile>> {
  const { id, ...payload } = data
  const res = id
    ? await api.put(`/api/ingress/admin/profiles/${id}`, payload)
    : await api.post('/api/ingress/admin/profiles', payload)
  return res.data
}

export async function deleteAdminAPIIngressProfile(
  id: number
): Promise<APIIngressResponse> {
  const res = await api.delete(`/api/ingress/admin/profiles/${id}`)
  return res.data
}
