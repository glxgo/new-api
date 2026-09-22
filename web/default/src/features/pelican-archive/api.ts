import { useQuery } from '@tanstack/react-query'
import { useAuthStore } from '@/stores/auth-store'
import { api } from '@/lib/api'
import type { Group, Presentation } from '@/features/intelligence-test/api'

export type { Group, Presentation }
export type Summary = {
  id: string
  time: number
  grade: string
  has_artwork: boolean
  model: string
}
export type Results = {
  group: Group
  model: string
  gallery: Summary[]
  history: Summary[]
  targets: number
}
export type Overview = {
  visible: boolean
  data: Group[]
  presentation: Presentation
  last_imported_at: number
  source_captured_at: string
  source_interval_minutes: number
  source_auto_run: boolean
}
export type Detail = Summary & {
  expected_answer: number
  reported_answer: number | null
  answer_in_svg: boolean
  answer_in_text: boolean
  truncated: boolean
  prompt: string
  prompt_hash: string
  prompt_source: string
  latency_ms: number
  ttft_ms: number
  input_tokens: number
  output_tokens: number
  attempts: number
  source_record?: Record<string, unknown>
  record?: Record<string, unknown>
}
export type Target = {
  id: string
  provider_id: string
  provider_name: string
  model_name: string
  present: boolean
  enabled: boolean
  channel_id: number
  display_model: string
  hidden: boolean
  interval_minutes: number
}
export type Channel = {
  id: number
  name: string
  status: number
  group: string
  models: string
}
export type Control = {
  revision: number
  visible: boolean
  sync_enabled: boolean
  interval_minutes: number
  last_imported_at: number
  last_captured_at: string
  last_error: string
  source_id: string
}
export type Admin = {
  mapping_reports: MappingReport[]
  control: Control
  presentation: Presentation
  source_config: {
    interval_minutes: number
    expected_answer: number
    auto_run: boolean
  } | null
  source_configured: boolean
  writable_node: boolean
  targets: Target[]
  channels: Channel[]
  groups: Group[]
  events: {
    id: number
    action: string
    at: number
    actor_id: number
    detail: string
  }[]
}
export type MappingReport = {
  target_id: string
  reason: string
  channel_disabled: boolean
  groups: {
    group_uid: string
    routing_key: string
    display_name: string
    ratio: number | null
    eligible: boolean
    reason: string
  }[]
}
export const base = '/api/pelican-archive'
export async function getArchive<T>(
  path: string,
  signal?: AbortSignal
): Promise<T> {
  const { data } = await api.get<T & { success?: boolean; message?: string }>(
    base + path,
    { signal }
  )
  if (data.success === false) throw new Error(data.message || 'Request failed')
  return data
}
export function usePelicanOverview(preview = false) {
  const user = useAuthStore((s) => s.auth.user?.id)
  return useQuery({
    queryKey: ['pelican', user, 'overview', preview],
    queryFn: ({ signal }) =>
      getArchive<Overview>(preview ? '/admin/preview' : '/groups', signal),
    enabled: !!user,
    refetchInterval: 30_000,
    retry: false,
  })
}
export async function updateArchive(
  revision: number,
  action: string,
  payload: Record<string, unknown> = {}
) {
  const { data } = await api.put<{ success: boolean; message?: string }>(
    base + '/admin/control',
    { revision, action, ...payload }
  )
  if (!data.success) throw new Error(data.message || 'Save failed')
}
export function timeLabel(seconds: number) {
  return seconds ? new Date(seconds * 1000).toLocaleString() : '—'
}
