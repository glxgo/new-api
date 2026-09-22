import { useQuery } from '@tanstack/react-query'
import { useAuthStore } from '@/stores/auth-store'
import { api } from '@/lib/api'

export type Group = {
  group_uid: string
  routing_key: string
  display_name: string
  description: string
  ratio: number | null
  icon_type: number
  models: string[]
}
export type Profile = {
  model: string
  protocol: string
  max_tokens: number
  enabled: boolean
}
export type Exclusion = { channel_id: number; group_uid: string; model: string }
export type Config = {
  interval_minutes: number
  anchor: number
  timezone: string
  weekdays: number[]
  window_start: number
  window_end: number
  all_day: boolean
  models: Profile[]
  excluded: Exclusion[]
  daily_budget_micros: number
  call_reserve_micros: number
  judge_channel_id: number
  judge_model: string
}
export type GroupOverride = {
  name: string | null
  description_mode: string
  description: string
  hidden: boolean
}
export type Presentation = {
  groups: Record<string, GroupOverride>
  order: string[]
  copy: Record<string, string>
  show_method: boolean
  show_history: boolean
  gallery_size: number
}
export type Control = {
  revision: number
  running: boolean
  visible: boolean
  updated_at: number
  config: Config
  draft: Presentation
  presentation: Presentation
  next_times: number[]
}
export type Target = {
  group_uid: string
  group: string
  model: string
  channel_id: number
  channel_name: string
  eligible: boolean
  reason: string
}
export type Item = {
  kind: string
  question: {
    prompt: string
    label?: string
    expected?: number
    requirements?: string[]
    hash: string
  }
  answer: string
  artifact?: string
  animation?: {
    artifact: string
    evidence: string
    source_sha256: string
    renderer: string
    browser: string
    frames: number
    step_ms: number
    evidence_ms: number[]
    changed_frames: number
  }
  status: string
  reason?: string
  checks?: boolean[]
  judgment?: {
    items: { status: string; evidence: string }[]
    aesthetic: number
    confidence: number
  }
}
export type Run = {
  public_id: string
  model: string
  sample: string
  time: number
  slot: number
  status: string
  score: (number | null)[]
  ranked: boolean
  items: Item[]
  suite?: string
  metrics?: Record<
    string,
    {
      attempts: number
      started_at: number | null
      duration_seconds: number | null
      input_tokens: number | null
      output_tokens: number | null
    }
  >
}
export type Method = {
  suite: string
  interval_minutes: number
  timezone: string
  all_day: boolean
  window_start: number
  window_end: number
  weekdays: number[]
}
export type AdminRunDetail = {
  run: {
    id: string
    channel_id: number
    model: string
    protocol: string
    status: string
    reason: string
    fingerprint: string
    started_at: number
    completed_at: number
  }
  items: Item[]
  recovery?: {
    id: string
    kind: string
    status: string
    reason: string
    attempts: number
    next_at: number
  }[]
  evaluations?: {
    id: string
    revision: number
    created_at: number
    item: Item
  }[]
  bindings: {
    public_id: string
    group_uid: string
    routing_key: string
    dispatched_at: number
    withdrawn: boolean
  }[]
  attempts: {
    id: string
    kind: string
    channel_id: number
    status: string
    reason: string
    endpoint: string
    key_slot: number
    request_id: string
    request_hash: string
    upstream_model: string
    reported_model: string
    prompt: string
    answer: string
    usage: string
    usage_source: string
    reserve_micros: number
    cost_source: string
    started_at: number
    completed_at: number
  }[]
}
export type Overview = {
  success: boolean
  data: Group[]
  visible: boolean
  running: boolean
  revision: number
  copy: Record<string, string>
  show_method: boolean
  show_history: boolean
  gallery_size: number
  next_times: number[]
  method?: Method
}
export async function capabilityGet<T>(
  path: string,
  signal?: AbortSignal
): Promise<T> {
  const { data } = await api.get<T & { success?: boolean; message?: string }>(
    `/api/capability${path}`,
    { signal }
  )
  if (data.success === false) throw new Error(data.message || 'Request failed')
  return data
}
export function useCapabilityOverview() {
  const userId = useAuthStore((s) => s.auth.user?.id)
  return useQuery({
    queryKey: ['capability', 'overview', userId],
    queryFn: ({ signal }) => capabilityGet<Overview>('/groups', signal),
    enabled: !!userId,
    refetchInterval: 30_000,
    retry: false,
  })
}
export async function capabilityUpdate(
  revision: number,
  action: string,
  payload: Record<string, unknown> = {}
) {
  const { data } = await api.put<{
    success: boolean
    message?: string
    data: Control
  }>('/api/capability/admin/control', { revision, action, ...payload })
  if (!data.success) throw new Error(data.message || 'Save failed')
  return data.data
}
export const formatTime = (seconds: number) =>
  seconds ? new Date(seconds * 1000).toLocaleString() : '—'
