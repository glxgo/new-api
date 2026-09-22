import { useRef, useState, type ReactNode } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import { ROLE } from '@/lib/roles'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from '@/components/ui/card'
import {
  Field,
  FieldLabel,
  FieldDescription,
  FieldGroup,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import {
  Table,
  TableHeader,
  TableHead,
  TableBody,
  TableRow,
  TableCell,
} from '@/components/ui/table'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'
import { SettingsSection } from '@/features/system-settings/components/settings-section'
import {
  capabilityGet,
  capabilityUpdate,
  formatTime,
  type Control,
  type Config,
  type Presentation,
  type Group,
  type Target,
  type Exclusion,
  type AdminRunDetail,
} from './api'
import { ArtifactPlayer } from './artifact-player'
import { CopyEditor } from './copy-editor'
import { CapabilityOverview } from './index'
import { RecoveryEvidence } from './recovery-evidence'

const copySlots = [
  ['page_title', 'Overview title', 'User page → Overview → Title'],
  ['page_intro', 'Overview description', 'User page → Overview → Description'],
  ['gallery_title', 'Gallery description', 'User page → Group → Gallery'],
  [
    'method_intro',
    'Method introduction',
    'User page → Method reader → Introduction',
  ],
  [
    'empty_text',
    'Empty state description',
    'User page → No results → Description',
  ],
  [
    'method_scope_title',
    'Scope and scoring',
    'User page → Method reader → Chapter 1',
  ],
  [
    'method_evidence_title',
    'From questions to evidence',
    'User page → Method reader → Chapter 2',
  ],
  [
    'method_selection_title',
    'Selection and sources',
    'User page → Method reader → Chapter 3',
  ],
  [
    'method_reading_title',
    'Reading results',
    'User page → Method reader → Chapter 4',
  ],
  [
    'method_trace_title',
    'Traceability and comparison',
    'User page → Method reader → Chapter 5',
  ],
  [
    'method_limits_title',
    'Interpreting the tests',
    'User page → Method reader → Chapter 6',
  ],
] as const

function Panel({
  title,
  description,
  children,
}: {
  title: string
  description?: string
  children: ReactNode
}) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>{title}</CardTitle>
        {description && <CardDescription>{description}</CardDescription>}
      </CardHeader>
      <CardContent>{children}</CardContent>
    </Card>
  )
}

export function CapabilityAdmin() {
  const { t } = useTranslation()
  const client = useQueryClient()
  const root = useAuthStore((s) => (s.auth.user?.role ?? 0) >= ROLE.SUPER_ADMIN)
  const query = useQuery({
    queryKey: ['capability', 'control'],
    queryFn: ({ signal }) =>
      capabilityGet<{ data: Control }>('/admin/control', signal),
    retry: false,
  })
  const groups = useQuery({
    queryKey: ['capability', 'admin-groups'],
    queryFn: ({ signal }) =>
      capabilityGet<{ data: Group[] }>('/admin/groups', signal),
    refetchInterval: 30_000,
  })
  const targets = useQuery({
    queryKey: ['capability', 'targets'],
    queryFn: ({ signal }) =>
      capabilityGet<{ data: Target[] }>('/admin/targets', signal),
    refetchInterval: 30_000,
  })
  const runtime = useQuery({
    queryKey: ['capability', 'runtime'],
    queryFn: ({ signal }) =>
      capabilityGet<{
        data: {
          readiness: string[]
          active_calls: number
          worker: { heartbeat: number; state: string }
          budget: { reserved_micros: number }
        }
      }>('/admin/runtime', signal),
    refetchInterval: 5000,
  })
  const [configDraft, setConfigDraft] = useState<Config | null>(null)
  const [presentationDraft, setPresentationDraft] =
    useState<Presentation | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [preview, setPreview] = useState(false)
  const [filter, setFilter] = useState('')
  const saving = useRef(false)
  const [targetPage, setTargetPage] = useState(1)
  const row = query.data?.data
  if (query.isPending) return <Skeleton className='h-96 w-full' />
  if (!row || query.isError)
    return (
      <Alert>
        <AlertDescription>
          {t('Unable to load test settings.')}{' '}
          <Button variant='link' onClick={() => query.refetch()}>
            {t('Retry')}
          </Button>
        </AlertDescription>
      </Alert>
    )
  const config = configDraft || row.config
  const draft = presentationDraft || row.draft
  const setConfig = (change: Partial<Config>) =>
    setConfigDraft({ ...config, ...change })
  const setDraft = (change: Partial<Presentation>) =>
    setPresentationDraft({ ...draft, ...change })
  const save = async (
    action: string,
    payload: Record<string, unknown> = {},
    revision = row.revision
  ) => {
    if (saving.current) throw new Error(t('A save is already in progress.'))
    saving.current = true
    setBusy(true)
    setError('')
    try {
      const next = await capabilityUpdate(revision, action, payload)
      client.setQueryData(['capability', 'control'], { data: next })
      await client.invalidateQueries({ queryKey: ['capability'] })
      toast.success(t('Saved'))
      return next
    } catch (e) {
      const message = e instanceof Error ? e.message : t('Save failed')
      setError(message)
      throw e
    } finally {
      saving.current = false
      setBusy(false)
    }
  }
  const act = (action: string) => {
    void save(action).catch(() => {})
  }
  const persistConfig = () => {
    void save('save_config', { config })
      .then(() => setConfigDraft(null))
      .catch(() => {})
  }
  const publish = () => {
    void save('publish', { presentation: draft })
      .then(() => setPresentationDraft(null))
      .catch(() => {})
  }
  const exclude = (rule: Exclusion) => {
    const key = (e: Exclusion) => `${e.channel_id}:${e.group_uid}:${e.model}`
    const exists = row.config.excluded.some((e) => key(e) === key(rule))
    const updated = {
      ...row.config,
      excluded: exists
        ? row.config.excluded.filter((e) => key(e) !== key(rule))
        : [...row.config.excluded, rule],
    }
    void save('save_config', { config: updated })
      .then(() =>
        setConfigDraft((previous) =>
          previous ? { ...previous, excluded: updated.excluded } : null
        )
      )
      .catch(() => {})
  }
  const list = groups.data?.data || []
  const ordered = [...list].sort((a, b) => {
    const ai = draft.order.indexOf(a.group_uid),
      bi = draft.order.indexOf(b.group_uid)
    return (ai < 0 ? 100000 : ai) - (bi < 0 ? 100000 : bi)
  })
  const persistOrder = (order: string[]) => {
    void save('set_order', { order })
      .then(() =>
        setPresentationDraft((previous) =>
          previous ? { ...previous, order } : null
        )
      )
      .catch(() => {})
  }
  const move = (index: number, direction: number) => {
    const order = ordered.map((g) => g.group_uid)
    ;[order[index], order[index + direction]] = [
      order[index + direction],
      order[index],
    ]
    persistOrder(order)
  }
  const filteredTargets = (targets.data?.data || []).filter((v) =>
    `${v.group} ${v.channel_name} ${v.model}`
      .toLowerCase()
      .includes(filter.toLowerCase())
  )
  const disabled = busy || !root
  return (
    <SettingsSection title={t('Intelligence Test')}>
      {error && (
        <Alert variant='destructive'>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}
      <Panel
        title={t('Test control')}
        description={t(
          'Test controls do not disable business channels or change routing.'
        )}
      >
        <div className='flex flex-col gap-4'>
          <div className='flex flex-wrap gap-2'>
            <Badge variant='outline'>
              {row.visible ? t('Visible') : t('Hidden')}
            </Badge>
            <Badge variant='secondary'>
              {row.running
                ? t('Test schedule active')
                : runtime.data?.data.active_calls
                  ? t('Stopping')
                  : t('Stopped')}
            </Badge>
            <span className='text-muted-foreground text-sm'>
              {t('Configuration revision')}: {row.revision}
            </span>
          </div>
          <div className='flex flex-wrap gap-2'>
            <Button
              variant='destructive'
              disabled={disabled}
              onClick={() => act('hide_and_stop')}
            >
              {t('Hide and stop tests')}
            </Button>
            <Button
              variant='outline'
              disabled={disabled}
              onClick={() => act('stop')}
            >
              {t('Stop all tests')}
            </Button>
            <Button
              variant='outline'
              disabled={disabled}
              onClick={() => act('hide_only')}
            >
              {t('Hide only, keep current schedule')}
            </Button>
            <Button
              variant='outline'
              disabled={disabled}
              onClick={() => act('show_only')}
            >
              {t('Show tests')}
            </Button>
            <Button
              disabled={disabled || !!runtime.data?.data.readiness.length}
              onClick={() => act('resume')}
            >
              {t('Resume tests')}
            </Button>
          </div>
          <p className='text-muted-foreground text-sm'>
            {t(
              'Showing the page does not resume tests. Hidden tests may still incur costs if the schedule remains active.'
            )}
          </p>
          {!!runtime.data?.data.readiness.length && (
            <details>
              <summary className='cursor-pointer text-sm'>
                {t('Activation requirements')} (
                {runtime.data.data.readiness.length})
              </summary>
              <ul className='mt-3 flex list-inside list-disc flex-col gap-2 text-sm'>
                {runtime.data.data.readiness.map((reason) => (
                  <li key={reason}>{t(reason)}</li>
                ))}
              </ul>
            </details>
          )}
        </div>
      </Panel>
      <Tabs defaultValue='settings'>
        <TabsList>
          <TabsTrigger value='settings'>{t('Test settings')}</TabsTrigger>
          <TabsTrigger value='runs'>{t('Run records')}</TabsTrigger>
        </TabsList>
        <TabsContent value='settings' className='flex flex-col gap-6'>
          <Panel
            title={t('Test schedule')}
            description={t(
              'Changes are saved explicitly. Page refresh never triggers a test.'
            )}
          >
            <FieldGroup>
              <Field>
                <FieldLabel htmlFor='iq-interval'>
                  {t('Interval in minutes')}
                </FieldLabel>
                <Input
                  id='iq-interval'
                  type='number'
                  min={1}
                  max={1440}
                  value={config.interval_minutes}
                  onChange={(e) =>
                    setConfig({ interval_minutes: Number(e.target.value) })
                  }
                  disabled={disabled}
                />
              </Field>
              <Field>
                <FieldLabel htmlFor='iq-zone'>{t('Time zone')}</FieldLabel>
                <Input
                  id='iq-zone'
                  value={config.timezone}
                  onChange={(e) => setConfig({ timezone: e.target.value })}
                  disabled={disabled}
                />
              </Field>
              <Field>
                <FieldLabel htmlFor='iq-anchor'>
                  {t('Schedule anchor (UTC)')}
                </FieldLabel>
                <Input
                  id='iq-anchor'
                  type='datetime-local'
                  value={new Date(config.anchor * 1000)
                    .toISOString()
                    .slice(0, 16)}
                  onChange={(e) => {
                    const value = Date.parse(`${e.target.value}Z`)
                    if (Number.isFinite(value))
                      setConfig({ anchor: value / 1000 })
                  }}
                  disabled={disabled}
                />
              </Field>
              <Field orientation='horizontal'>
                <FieldLabel htmlFor='iq-allday'>{t('Run all day')}</FieldLabel>
                <Switch
                  id='iq-allday'
                  checked={config.all_day}
                  onCheckedChange={(all_day) => setConfig({ all_day })}
                  disabled={disabled}
                />
              </Field>
              {!config.all_day && (
                <div className='grid gap-4 sm:grid-cols-2'>
                  {(['window_start', 'window_end'] as const).map((key) => (
                    <Field key={key}>
                      <FieldLabel htmlFor={`iq-${key}`}>
                        {key === 'window_start'
                          ? t('Start time')
                          : t('End time')}
                      </FieldLabel>
                      <Input
                        id={`iq-${key}`}
                        type='time'
                        value={`${String(Math.floor(config[key] / 60)).padStart(2, '0')}:${String(config[key] % 60).padStart(2, '0')}`}
                        onChange={(e) => {
                          const [h, m] = e.target.value.split(':').map(Number)
                          setConfig({ [key]: h * 60 + m })
                        }}
                        disabled={disabled}
                      />
                    </Field>
                  ))}
                </div>
              )}
              <Field>
                <FieldLabel>{t('Run days')}</FieldLabel>
                <div className='flex flex-wrap gap-3'>
                  {[
                    'Sunday',
                    'Monday',
                    'Tuesday',
                    'Wednesday',
                    'Thursday',
                    'Friday',
                    'Saturday',
                  ].map((day, i) => (
                    <label
                      key={day}
                      className='flex items-center gap-2 text-sm'
                    >
                      <input
                        type='checkbox'
                        checked={config.weekdays.includes(i)}
                        disabled={disabled}
                        onChange={(e) =>
                          setConfig({
                            weekdays: e.target.checked
                              ? [...config.weekdays, i]
                              : config.weekdays.filter((d) => d !== i),
                          })
                        }
                      />
                      {t(day)}
                    </label>
                  ))}
                </div>
              </Field>
              <FieldDescription>
                {t('Next scheduled times')}:{' '}
                {row.next_times.map(formatTime).join(' · ')}
              </FieldDescription>
              <Button
                disabled={disabled || !configDraft}
                onClick={persistConfig}
              >
                {t('Save test settings')}
              </Button>
            </FieldGroup>
          </Panel>
          <Panel
            title={t('Models and test budget')}
            description={t(
              'Only approved model profiles can run. Budget amounts are conservative upper bounds in USD, not customer multipliers.'
            )}
          >
            <FieldGroup>
              {[...new Set(targets.data?.data.map((v) => v.model) || [])].map(
                (name) => {
                  const p = config.models.find((v) => v.model === name)
                  const update = (change: Partial<NonNullable<typeof p>>) =>
                    setConfig({
                      models: [
                        ...config.models.filter((v) => v.model !== name),
                        {
                          model: name,
                          protocol: 'chat',
                          max_tokens: 8192,
                          enabled: false,
                          ...p,
                          ...change,
                        },
                      ],
                    })
                  return (
                    <div
                      key={name}
                      className='flex flex-wrap items-center gap-3'
                    >
                      <span className='min-w-40 flex-1 text-sm break-all'>
                        {name}
                      </span>
                      <Switch
                        aria-label={`${t('Include model')} ${name}`}
                        checked={p?.enabled || false}
                        disabled={disabled}
                        onCheckedChange={(enabled) => update({ enabled })}
                      />
                      <NativeSelect
                        aria-label={`${name} ${t('Protocol')}`}
                        value={p?.protocol || 'chat'}
                        disabled={disabled}
                        onChange={(e) => update({ protocol: e.target.value })}
                      >
                        <NativeSelectOption value='chat'>
                          Chat Completions
                        </NativeSelectOption>
                        <NativeSelectOption value='responses'>
                          Responses
                        </NativeSelectOption>
                      </NativeSelect>
                      <Input
                        aria-label={`${name} ${t('Output token limit')}`}
                        className='w-28'
                        type='number'
                        min={128}
                        max={16384}
                        value={p?.max_tokens || 8192}
                        disabled={disabled}
                        onChange={(e) =>
                          update({ max_tokens: Number(e.target.value) })
                        }
                      />
                    </div>
                  )
                }
              )}
              {(['daily_budget_micros', 'call_reserve_micros'] as const).map(
                (key) => (
                  <Field key={key}>
                    <FieldLabel htmlFor={`iq-${key}`}>
                      {key === 'daily_budget_micros'
                        ? t('Daily budget (USD)')
                        : t('Approved maximum cost per call (USD)')}
                    </FieldLabel>
                    <Input
                      id={`iq-${key}`}
                      type='number'
                      min={0}
                      step='0.000001'
                      value={config[key] / 1e6}
                      onChange={(e) =>
                        setConfig({
                          [key]: Math.round(Number(e.target.value) * 1e6),
                        })
                      }
                      disabled={disabled}
                    />
                  </Field>
                )
              )}
              <Field>
                <FieldLabel htmlFor='iq-judge-channel'>
                  {t('Independent vision judge channel')}
                </FieldLabel>
                <NativeSelect
                  id='iq-judge-channel'
                  value={config.judge_channel_id}
                  disabled={disabled}
                  onChange={(e) =>
                    setConfig({ judge_channel_id: Number(e.target.value) })
                  }
                >
                  <NativeSelectOption value={0}>
                    {t('Select channel')}
                  </NativeSelectOption>
                  {[
                    ...new Map(
                      (targets.data?.data || []).map((v) => [v.channel_id, v])
                    ).values(),
                  ].map((v) => (
                    <NativeSelectOption key={v.channel_id} value={v.channel_id}>
                      {v.channel_name} #{v.channel_id}
                    </NativeSelectOption>
                  ))}
                </NativeSelect>
              </Field>
              <Field>
                <FieldLabel htmlFor='iq-judge-model'>
                  {t('Vision judge model')}
                </FieldLabel>
                <Input
                  id='iq-judge-model'
                  value={config.judge_model}
                  onChange={(e) => setConfig({ judge_model: e.target.value })}
                  disabled={disabled}
                />
              </Field>
              <Button
                disabled={disabled || !configDraft}
                onClick={persistConfig}
              >
                {t('Save test settings')}
              </Button>
            </FieldGroup>
          </Panel>
          <Panel
            title={t('Groups and channel testing')}
            description={t(
              'Existing membership is synchronized automatically. Exclusions affect tests only.'
            )}
          >
            <Input
              aria-label={t('Filter targets')}
              placeholder={t('Filter targets')}
              value={filter}
              onChange={(e) => {
                setFilter(e.target.value)
                setTargetPage(1)
              }}
            />
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('Group')}</TableHead>
                  <TableHead>{t('Channel')}</TableHead>
                  <TableHead>{t('Model')}</TableHead>
                  <TableHead>{t('Test eligibility')}</TableHead>
                  <TableHead>{t('Actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {filteredTargets
                  .slice((targetPage - 1) * 50, targetPage * 50)
                  .map((v) => (
                    <TableRow key={`${v.group_uid}-${v.channel_id}-${v.model}`}>
                      <TableCell>{v.group}</TableCell>
                      <TableCell>
                        {v.channel_name} #{v.channel_id}
                      </TableCell>
                      <TableCell>{v.model}</TableCell>
                      <TableCell>
                        <Badge variant='outline'>
                          {v.eligible ? t('Eligible') : t(v.reason)}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        <div className='flex gap-2'>
                          <Button
                            size='sm'
                            variant='outline'
                            disabled={disabled}
                            onClick={() =>
                              exclude({
                                channel_id: v.channel_id,
                                group_uid: '',
                                model: '',
                              })
                            }
                          >
                            {config.excluded.some(
                              (e) =>
                                e.channel_id === v.channel_id &&
                                !e.group_uid &&
                                !e.model
                            )
                              ? t('Remove global exclusion')
                              : t('Exclude channel globally')}
                          </Button>
                          <Button
                            size='sm'
                            variant='outline'
                            disabled={disabled}
                            onClick={() =>
                              exclude({
                                channel_id: v.channel_id,
                                group_uid: v.group_uid,
                                model: v.model,
                              })
                            }
                          >
                            {config.excluded.some(
                              (e) =>
                                e.channel_id === v.channel_id &&
                                e.group_uid === v.group_uid &&
                                e.model === v.model
                            )
                              ? t('Remove group exclusion')
                              : t('Exclude in this group')}
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
              </TableBody>
            </Table>
            <div className='flex items-center justify-end gap-3'>
              <span className='text-muted-foreground text-sm tabular-nums'>
                {targetPage} /{' '}
                {Math.max(1, Math.ceil(filteredTargets.length / 50))}
              </span>
              <Button
                variant='outline'
                disabled={targetPage === 1}
                onClick={() => setTargetPage(targetPage - 1)}
              >
                {t('Previous')}
              </Button>
              <Button
                variant='outline'
                disabled={targetPage * 50 >= filteredTargets.length}
                onClick={() => setTargetPage(targetPage + 1)}
              >
                {t('Next')}
              </Button>
            </div>
          </Panel>
          <Panel
            title={t('Presentation and wording')}
            description={t(
              'Save a draft, preview it, then publish. Names and descriptions do not modify business groups.'
            )}
          >
            <FieldGroup>
              {ordered.map((g, index) => {
                const value = draft.groups[g.group_uid] || {
                  name: null,
                  description_mode: 'inherit',
                  description: '',
                  hidden: false,
                }
                const edit = (change: Partial<typeof value>) =>
                  setDraft({
                    groups: {
                      ...draft.groups,
                      [g.group_uid]: { ...value, ...change },
                    },
                  })
                return (
                  <fieldset
                    key={g.group_uid}
                    className='flex flex-col gap-3 rounded-lg border p-4'
                  >
                    <legend className='px-2 text-sm font-medium'>
                      {g.routing_key} ·{' '}
                      {g.ratio === null
                        ? t('Multiplier not configured')
                        : `×${g.ratio}`}
                    </legend>
                    <div className='flex flex-wrap justify-between gap-2'>
                      <Badge variant='outline'>
                        {value.name === null
                          ? t('Inherited name')
                          : t('Custom display name')}
                      </Badge>
                      <div className='flex gap-2'>
                        <Button
                          variant='outline'
                          size='sm'
                          disabled={disabled || index === 0}
                          onClick={() => move(index, -1)}
                        >
                          {t('Move up')}
                        </Button>
                        <Button
                          variant='outline'
                          size='sm'
                          disabled={disabled || index === ordered.length - 1}
                          onClick={() => move(index, 1)}
                        >
                          {t('Move down')}
                        </Button>
                      </div>
                    </div>
                    <Field>
                      <FieldLabel htmlFor={`iq-name-${g.group_uid}`}>
                        {t('Display name')}
                      </FieldLabel>
                      <Input
                        id={`iq-name-${g.group_uid}`}
                        value={value.name ?? ''}
                        placeholder={g.routing_key}
                        maxLength={40}
                        disabled={disabled}
                        onChange={(e) => edit({ name: e.target.value || null })}
                      />
                      <FieldDescription>
                        {t(
                          'User page → Group card → Title. Leave empty to inherit.'
                        )}
                      </FieldDescription>
                    </Field>
                    <Field>
                      <FieldLabel htmlFor={`iq-description-${g.group_uid}`}>
                        {t('Description')}
                      </FieldLabel>
                      <NativeSelect
                        aria-label={t('Description source')}
                        value={value.description_mode || 'inherit'}
                        disabled={disabled}
                        onChange={(e) =>
                          edit({ description_mode: e.target.value })
                        }
                      >
                        <NativeSelectOption value='inherit'>
                          {t('Inherit')}
                        </NativeSelectOption>
                        <NativeSelectOption value='custom'>
                          {t('Custom')}
                        </NativeSelectOption>
                        <NativeSelectOption value='hidden'>
                          {t('Hidden')}
                        </NativeSelectOption>
                      </NativeSelect>
                      {value.description_mode === 'custom' && (
                        <Textarea
                          id={`iq-description-${g.group_uid}`}
                          value={value.description}
                          maxLength={160}
                          disabled={disabled}
                          onChange={(e) =>
                            edit({ description: e.target.value })
                          }
                        />
                      )}
                    </Field>
                    <Field orientation='horizontal'>
                      <FieldLabel htmlFor={`iq-visible-${g.group_uid}`}>
                        {t('Visible to authorized users')}
                      </FieldLabel>
                      <Switch
                        id={`iq-visible-${g.group_uid}`}
                        checked={!value.hidden}
                        disabled={disabled}
                        onCheckedChange={(v) => edit({ hidden: !v })}
                      />
                    </Field>
                  </fieldset>
                )
              })}
              <Button
                variant='outline'
                disabled={disabled}
                onClick={() => persistOrder([])}
              >
                {t('Restore inherited order')}
              </Button>
              <CopyEditor
                slots={copySlots}
                value={draft.copy}
                saved={row.draft.copy}
                disabled={disabled}
                onChange={(copy) => setDraft({ copy })}
              />
              {(['show_method', 'show_history'] as const).map((key) => (
                <Field orientation='horizontal' key={key}>
                  <FieldLabel htmlFor={`iq-${key}`}>
                    {key === 'show_method'
                      ? t('Show methodology')
                      : t('Show history')}
                  </FieldLabel>
                  <Switch
                    id={`iq-${key}`}
                    checked={draft[key]}
                    disabled={disabled}
                    onCheckedChange={(v) => setDraft({ [key]: v })}
                  />
                </Field>
              ))}
              <Field>
                <FieldLabel htmlFor='iq-gallery'>
                  {t('Gallery size')}
                </FieldLabel>
                <NativeSelect
                  id='iq-gallery'
                  value={draft.gallery_size}
                  disabled={disabled}
                  onChange={(e) =>
                    setDraft({ gallery_size: Number(e.target.value) })
                  }
                >
                  {[1, 2, 3].map((n) => (
                    <NativeSelectOption value={n} key={n}>
                      {n}
                    </NativeSelectOption>
                  ))}
                </NativeSelect>
              </Field>
              <div className='flex flex-wrap gap-2'>
                <Button
                  variant='outline'
                  disabled={disabled}
                  onClick={() => {
                    void save('save_draft', { presentation: draft })
                      .then(() => setPresentationDraft(null))
                      .catch(() => {})
                  }}
                >
                  {t('Save draft')}
                </Button>
                <Button variant='outline' onClick={() => setPreview(true)}>
                  {t('Preview')}
                </Button>
                <Button disabled={disabled} onClick={publish}>
                  {t('Publish presentation')}
                </Button>
                <Button
                  variant='ghost'
                  disabled={disabled}
                  onClick={() => setPresentationDraft(null)}
                >
                  {t('Discard unsaved changes')}
                </Button>
              </div>
            </FieldGroup>
          </Panel>
        </TabsContent>
        <TabsContent value='runs'>
          <CapabilityRunRecords />
        </TabsContent>
      </Tabs>
      <Sheet open={preview} onOpenChange={setPreview}>
        <SheetContent className='w-full sm:max-w-5xl'>
          <SheetHeader>
            <SheetTitle>
              {t('Presentation preview — no model calls')}
            </SheetTitle>
          </SheetHeader>
          <div className='min-h-0 flex-1 overflow-auto px-4 pb-4'>
            <CapabilityOverview
              fixture={{}}
              data={{
                success: true,
                visible: true,
                running: false,
                revision: row.revision,
                data: ordered
                  .filter((g) => !draft.groups[g.group_uid]?.hidden)
                  .map((g) => {
                    const v = draft.groups[g.group_uid]
                    return {
                      ...g,
                      display_name: v?.name || g.routing_key,
                      description:
                        v?.description_mode === 'custom'
                          ? v.description
                          : v?.description_mode === 'hidden'
                            ? ''
                            : g.description,
                    }
                  }),
                copy: draft.copy,
                show_method: draft.show_method,
                show_history: draft.show_history,
                gallery_size: draft.gallery_size,
                next_times: row.next_times,
              }}
            />
          </div>
        </SheetContent>
      </Sheet>
    </SettingsSection>
  )
}

function CapabilityRunRecords() {
  const { t } = useTranslation()
  const [page, setPage] = useState(1)
  const [id, setId] = useState<string | null>(null)
  const query = useQuery({
    queryKey: ['capability', 'runs', page],
    queryFn: ({ signal }) =>
      capabilityGet<{
        data: {
          id: string
          model: string
          channel_id: number
          status: string
          reason: string
          started_at: number
        }[]
        has_more: boolean
      }>(`/admin/runs?page=${page}`, signal),
    refetchInterval: 10_000,
  })
  const detail = useQuery({
    queryKey: ['capability', 'admin-run', id],
    queryFn: ({ signal }) =>
      capabilityGet<{ data: AdminRunDetail }>(`/admin/runs/${id}`, signal),
    enabled: !!id,
    refetchInterval: 10_000,
  })
  return (
    <Panel
      title={t('Run records')}
      description={t(
        'Actual test calls and evaluation evidence are stored separately from user usage logs.'
      )}
    >
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('Time')}</TableHead>
            <TableHead>{t('Channel')}</TableHead>
            <TableHead>{t('Model')}</TableHead>
            <TableHead>{t('Status')}</TableHead>
            <TableHead>{t('Details')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {query.data?.data.map((run) => (
            <TableRow key={run.id}>
              <TableCell>{formatTime(run.started_at)}</TableCell>
              <TableCell>#{run.channel_id}</TableCell>
              <TableCell>{run.model}</TableCell>
              <TableCell>
                {t(run.status)}
                {run.reason && ` · ${t(run.reason)}`}
              </TableCell>
              <TableCell>
                <Button
                  size='sm'
                  variant='outline'
                  onClick={() => setId(run.id)}
                >
                  {t('View evidence')}
                </Button>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
      {!query.data?.data.length && (
        <p className='text-muted-foreground py-6 text-center text-sm'>
          {query.isError
            ? t('Unable to load test results.')
            : t('No test records yet')}
        </p>
      )}
      <div className='flex justify-end gap-2'>
        <Button
          variant='outline'
          disabled={page === 1}
          onClick={() => setPage(page - 1)}
        >
          {t('Previous')}
        </Button>
        <Button
          variant='outline'
          disabled={!query.data?.has_more}
          onClick={() => setPage(page + 1)}
        >
          {t('Next')}
        </Button>
      </div>
      <Sheet
        open={!!id}
        onOpenChange={(open) => {
          if (!open) setId(null)
        }}
      >
        <SheetContent className='w-full sm:max-w-3xl'>
          <SheetHeader>
            <SheetTitle>{t('Call evidence')}</SheetTitle>
          </SheetHeader>
          <div className='flex min-h-0 flex-1 flex-col gap-6 overflow-auto px-6 pb-6 *:shrink-0'>
            {detail.isError ? (
              <Alert>
                <AlertDescription>
                  {t('Unable to load test results.')}
                </AlertDescription>
              </Alert>
            ) : detail.data ? (
              <>
                <Panel title={t('Run records')}>
                  <dl className='grid grid-cols-[auto_1fr] gap-x-6 gap-y-3 text-sm'>
                    <dt>{t('Channel')}</dt>
                    <dd>#{detail.data.data.run.channel_id}</dd>
                    <dt>{t('Model')}</dt>
                    <dd className='break-all'>{detail.data.data.run.model}</dd>
                    <dt>{t('Protocol')}</dt>
                    <dd>{detail.data.data.run.protocol}</dd>
                    <dt>{t('Time')}</dt>
                    <dd>
                      {formatTime(detail.data.data.run.started_at)} —{' '}
                      {formatTime(detail.data.data.run.completed_at)}
                    </dd>
                    <dt>{t('Status')}</dt>
                    <dd>
                      {t(detail.data.data.run.status)}{' '}
                      {t(detail.data.data.run.reason)}
                    </dd>
                  </dl>
                </Panel>
                <Panel title={t('Groups and channel testing')}>
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>{t('Group')}</TableHead>
                        <TableHead>{t('Time')}</TableHead>
                        <TableHead>{t('Status')}</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {detail.data.data.bindings.map((binding) => (
                        <TableRow key={binding.public_id}>
                          <TableCell>{binding.routing_key}</TableCell>
                          <TableCell>
                            {formatTime(binding.dispatched_at)}
                          </TableCell>
                          <TableCell>
                            {binding.withdrawn
                              ? t('Stopped')
                              : binding.dispatched_at
                                ? t('Eligible')
                                : '—'}
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </Panel>
                <Panel title={t('Call evidence')}>
                  <div className='flex flex-col gap-4'>
                    {detail.data.data.attempts.map((attempt) => (
                      <details
                        key={attempt.id}
                        className='rounded-md border p-3'
                      >
                        <summary className='cursor-pointer text-sm'>
                          {attempt.kind} · #{attempt.channel_id} ·{' '}
                          {t(attempt.status)} · {formatTime(attempt.started_at)}
                        </summary>
                        <dl className='mt-4 grid grid-cols-[auto_1fr] gap-x-4 gap-y-2 text-sm'>
                          <dt>{t('Endpoint')}</dt>
                          <dd className='break-all'>{attempt.endpoint}</dd>
                          <dt>{t('Model')}</dt>
                          <dd>{attempt.upstream_model}</dd>
                          <dt>{t('Request ID')}</dt>
                          <dd className='break-all'>
                            {attempt.request_id || '—'}
                          </dd>
                          <dt>{t('Approved maximum cost per call (USD)')}</dt>
                          <dd className='tabular-nums'>
                            {attempt.reserve_micros / 1e6}
                          </dd>
                          <dt>{t('Status')}</dt>
                          <dd>{t(attempt.reason)}</dd>
                        </dl>
                        <p className='text-muted-foreground mt-3 text-xs'>
                          {attempt.usage_source}
                        </p>
                        <pre className='bg-muted mt-2 overflow-auto rounded p-3 text-xs whitespace-pre-wrap'>
                          {attempt.usage || '—'}
                        </pre>
                        <p className='mt-4 text-sm font-medium'>
                          {t('Questions and evaluation evidence')}
                        </p>
                        <pre className='mt-2 max-h-48 overflow-auto text-xs whitespace-pre-wrap'>
                          {attempt.prompt}
                        </pre>
                        <p className='mt-4 text-sm font-medium'>
                          {t('Original response')}
                        </p>
                        <pre className='bg-muted mt-2 max-h-72 overflow-auto rounded p-3 text-xs break-words whitespace-pre-wrap'>
                          {attempt.answer || '—'}
                        </pre>
                      </details>
                    ))}
                  </div>
                </Panel>
                <Panel title={t('Questions and evaluation evidence')}>
                  <div className='flex flex-col gap-6'>
                    {detail.data.data.items.map((item) => (
                      <article key={item.kind} className='flex flex-col gap-3'>
                        <ArtifactPlayer item={item} />
                        <Badge variant='outline'>
                          {item.kind} · {t(item.status)}
                        </Badge>
                        {item.checks && (
                          <p className='text-sm'>
                            {t('Requirements met')}:{' '}
                            {item.checks.filter(Boolean).length} /{' '}
                            {item.checks.length}
                          </p>
                        )}
                        {item.judgment?.items.map((check, index) => (
                          <p key={index} className='text-sm'>
                            {index + 1}. {check.status} — {check.evidence}
                          </p>
                        ))}
                      </article>
                    ))}
                  </div>
                </Panel>
                <RecoveryEvidence detail={detail.data.data} />
                <details>
                  <summary className='cursor-pointer text-sm'>
                    {t('Details')}
                  </summary>
                  <pre className='mt-3 max-h-72 overflow-auto text-xs break-words whitespace-pre-wrap'>
                    {JSON.stringify(detail.data.data, null, 2)}
                  </pre>
                </details>
              </>
            ) : (
              <Skeleton className='h-72 w-full' />
            )}
          </div>
        </SheetContent>
      </Sheet>
    </Panel>
  )
}
