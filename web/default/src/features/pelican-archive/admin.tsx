import { useState } from 'react'
import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import { api } from '@/lib/api'
import { ROLE } from '@/lib/roles'
import { Accordion } from '@/components/ui/accordion'
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
  FieldGroup,
  FieldDescription,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
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
import { CopyEditor } from '@/features/intelligence-test/copy-editor'
import { SettingsAccordion } from '@/features/system-settings/components/settings-accordion'
import { SettingsSection } from '@/features/system-settings/components/settings-section'
import {
  getArchive,
  updateArchive,
  base,
  timeLabel,
  type Admin,
  type Target,
  type Presentation,
  type MappingReport,
} from './api'
import { ArchiveEvidence } from './evidence'
import { PelicanArchivePage } from './index'
import { SavedMappingReport } from './mapping-report'

const slots = [
  ['page_title', 'Overview title', 'User page → Overview → Title'],
  ['page_intro', 'Overview description', 'User page → Overview → Description'],
  ['gallery_title', 'Gallery description', 'User page → Group → Gallery'],
  [
    'empty_text',
    'Empty state description',
    'User page → No results → Description',
  ],
  [
    'method_intro',
    'Method introduction',
    'User page → Method reader → Introduction',
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

export function PelicanArchiveAdmin() {
  const { t } = useTranslation()
  const user = useAuthStore((s) => s.auth.user)
  const client = useQueryClient()
  const query = useQuery({
    queryKey: ['pelican', user?.id, 'admin'],
    queryFn: ({ signal }) =>
      getArchive<{ data: Admin }>('/admin/control', signal),
    retry: false,
    refetchInterval: 30_000,
  })
  const [tab, setTab] = useState('settings')
  const change = useMutation({
    mutationFn: async (input: {
      revision: number
      action: string
      payload?: Record<string, unknown>
    }) => {
      await updateArchive(input.revision, input.action, input.payload)
    },
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: ['pelican'] })
      toast.success(t('Saved'))
    },
    onError: (e) => toast.error(e.message),
  })
  if (query.isPending) return <Skeleton className='h-96 w-full' />
  const data = query.data?.data
  if (query.isError || !data)
    return (
      <Alert>
        <AlertDescription>
          {t('Unable to load test settings.')}
          <Button variant='link' onClick={() => query.refetch()}>
            {t('Retry')}
          </Button>
        </AlertDescription>
      </Alert>
    )
  const disabled =
    change.isPending ||
    (user?.role ?? 0) < ROLE.SUPER_ADMIN ||
    !data.writable_node
  const act = (action: string, payload?: Record<string, unknown>) =>
    change.mutate({ revision: data.control.revision, action, payload })
  return (
    <SettingsSection title={t('Intelligence Test')}>
      <Tabs value={tab} onValueChange={(v) => setTab(String(v))}>
        <TabsList>
          <TabsTrigger value='settings'>{t('Archive settings')}</TabsTrigger>
          <TabsTrigger value='records'>{t('Source records')}</TabsTrigger>
          <TabsTrigger value='preview'>{t('User preview')}</TabsTrigger>
        </TabsList>
        <TabsContent value='settings' className='flex flex-col gap-4'>
          <Card>
            <CardHeader>
              <CardTitle>{t('Pelican archive source')}</CardTitle>
              <CardDescription>
                {t(
                  'This site synchronizes saved results only. These controls do not start or stop tests on the external service.'
                )}
              </CardDescription>
            </CardHeader>
            <CardContent className='flex flex-col gap-4'>
              <div className='flex flex-wrap gap-2'>
                <Badge variant='secondary'>
                  {data.control.sync_enabled
                    ? t('Synchronization enabled')
                    : t('Synchronization paused')}
                </Badge>
                <Badge variant='outline'>
                  {data.control.visible ? t('Visible') : t('Hidden')}
                </Badge>
              </div>
              <dl className='grid gap-2 text-xs sm:grid-cols-2'>
                <div>
                  {t('Last synchronization')} ·{' '}
                  {timeLabel(data.control.last_imported_at)}
                </div>
                <div>
                  {t('Archive captured')} ·{' '}
                  {data.control.last_captured_at
                    ? new Date(data.control.last_captured_at).toLocaleString()
                    : '—'}
                </div>
                <div>
                  {t('Source schedule')} ·{' '}
                  {data.source_config?.interval_minutes ?? '—'} {t('minutes')}
                </div>
                <div>
                  {t('Reference answer')} ·{' '}
                  {data.source_config?.expected_answer ?? '—'}
                </div>
                <div>
                  {t('Source identifier')} · {data.control.source_id || '—'}
                </div>
              </dl>
              {!data.source_configured && (
                <Alert>
                  <AlertDescription>
                    {t(
                      'A private archive source must be configured before synchronization.'
                    )}
                  </AlertDescription>
                </Alert>
              )}
              {data.control.last_error && (
                <Alert variant='destructive'>
                  <AlertDescription>{data.control.last_error}</AlertDescription>
                </Alert>
              )}
              <div className='flex flex-wrap gap-2'>
                <Button
                  disabled={disabled}
                  onClick={() =>
                    act(data.control.sync_enabled ? 'pause' : 'resume')
                  }
                >
                  {data.control.sync_enabled
                    ? t('Pause synchronization')
                    : t('Resume synchronization')}
                </Button>
                <SyncButton
                  disabled={
                    disabled ||
                    !data.control.sync_enabled ||
                    !data.source_configured
                  }
                />
                <Button
                  variant='outline'
                  disabled={disabled}
                  onClick={() => act('hide_and_pause')}
                >
                  {t('Hide and pause synchronization')}
                </Button>
                <Button
                  variant='outline'
                  disabled={disabled}
                  onClick={() => act(data.control.visible ? 'hide' : 'show')}
                >
                  {data.control.visible ? t('Hide only') : t('Show page')}
                </Button>
              </div>
              <IntervalForm
                key={data.control.interval_minutes}
                interval={data.control.interval_minutes}
                disabled={disabled}
                save={(v) => act('interval', { interval_minutes: v })}
              />
            </CardContent>
          </Card>
          <Card>
            <CardHeader>
              <CardTitle>{t('Channel associations')}</CardTitle>
              <CardDescription>
                {t(
                  'Associate each external target with a site channel and model. Names are labels; stable identifiers preserve the association. Hiding a target affects only this page.'
                )}
              </CardDescription>
            </CardHeader>
            <CardContent>
              <div className='flex flex-col gap-3'>
                {data.targets.map((target) => (
                  <MappingRow
                    key={`${target.id}:${target.channel_id}:${target.display_model}:${target.hidden}`}
                    target={target}
                    report={data.mapping_reports?.find(
                      (report) => report.target_id === target.id
                    )}
                    channels={data.channels}
                    disabled={disabled}
                    save={(payload) => act('mapping', payload)}
                  />
                ))}
              </div>
            </CardContent>
          </Card>
          <PresentationForm
            key={JSON.stringify(data.presentation)}
            data={data}
            disabled={disabled}
            save={(presentation) => act('presentation', { presentation })}
          />
          <Accordion>
            <SettingsAccordion
              value='events'
              title={t('Management and synchronization events')}
            >
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('Time')}</TableHead>
                    <TableHead>{t('Action')}</TableHead>
                    <TableHead>{t('Actor')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {data.events.map((e) => (
                    <TableRow key={e.id}>
                      <TableCell>{timeLabel(e.at)}</TableCell>
                      <TableCell>{e.action}</TableCell>
                      <TableCell>{e.actor_id || t('System')}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </SettingsAccordion>
          </Accordion>
        </TabsContent>
        <TabsContent value='records'>
          <ArchiveRecords targets={data.targets} />
        </TabsContent>
        <TabsContent value='preview'>
          <Alert>
            <AlertDescription>
              {t(
                'Administrator preview uses saved records and bypasses page visibility. It does not publish the page or call a model.'
              )}
            </AlertDescription>
          </Alert>
          <PelicanArchivePage preview />
        </TabsContent>
      </Tabs>
    </SettingsSection>
  )
}
function SyncButton(props: { disabled: boolean }) {
  const { t } = useTranslation()
  const client = useQueryClient()
  const sync = useMutation({
    mutationFn: async () => {
      const { data } = await api.post<{ success: boolean; message?: string }>(
        base + '/admin/sync'
      )
      if (!data.success) throw new Error(data.message)
    },
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: ['pelican'] })
      toast.success(t('Synchronization complete'))
    },
    onError: (e) => toast.error(e.message),
  })
  return (
    <Button
      variant='outline'
      disabled={props.disabled || sync.isPending}
      onClick={() => sync.mutate()}
    >
      {t('Synchronize now')}
    </Button>
  )
}
const intervalSchema = z.object({ interval: z.number().int().min(1).max(1440) })
function IntervalForm(props: {
  interval: number
  disabled: boolean
  save: (value: number) => void
}) {
  const { t } = useTranslation()
  const form = useForm({
    resolver: zodResolver(intervalSchema),
    defaultValues: { interval: props.interval },
  })
  return (
    <form onSubmit={form.handleSubmit((v) => props.save(v.interval))}>
      <FieldGroup>
        <Field data-invalid={!!form.formState.errors.interval}>
          <FieldLabel htmlFor='pelican-interval'>
            {t('Synchronization interval (minutes)')}
          </FieldLabel>
          <Input
            id='pelican-interval'
            type='number'
            min={1}
            max={1440}
            aria-invalid={!!form.formState.errors.interval}
            disabled={props.disabled}
            {...form.register('interval', { valueAsNumber: true })}
          />
          <FieldDescription>
            {t(
              'Custom interval: 1–1440 minutes. This does not change the external test schedule.'
            )}
          </FieldDescription>
        </Field>
        <Button
          type='submit'
          className='self-start'
          variant='outline'
          disabled={props.disabled}
        >
          {t('Save')}
        </Button>
      </FieldGroup>
    </form>
  )
}
function MappingRow(props: {
  target: Target
  report?: MappingReport
  channels: Admin['channels']
  disabled: boolean
  save: (payload: Record<string, unknown>) => void
}) {
  const { t } = useTranslation()
  const [channel, setChannel] = useState(props.target.channel_id)
  const [model, setModel] = useState(props.target.display_model)
  const [hidden, setHidden] = useState(props.target.hidden)
  const models =
    props.channels
      .find((c) => c.id === channel)
      ?.models.split(',')
      .map((m) => m.trim())
      .filter(Boolean) || []
  return (
    <section className='rounded-lg border p-3'>
      <p className='text-sm font-medium'>
        {props.target.provider_name} · {props.target.model_name}
      </p>
      <p className='text-muted-foreground mb-3 font-mono text-[11px] break-all'>
        {props.target.provider_id} ·{' '}
        {props.target.present && props.target.enabled
          ? t('Source target active')
          : t('Source target removed or paused')}
      </p>
      <FieldGroup className='gap-3 sm:grid sm:grid-cols-4'>
        <Field>
          <FieldLabel htmlFor={`channel-${props.target.id}`}>
            {t('Site channel')}
          </FieldLabel>
          <NativeSelect
            id={`channel-${props.target.id}`}
            disabled={props.disabled}
            value={channel}
            onChange={(e) => {
              setChannel(Number(e.target.value))
              setModel('')
            }}
          >
            <NativeSelectOption value={0}>
              {t('Not associated')}
            </NativeSelectOption>
            {!!channel && !props.channels.some((c) => c.id === channel) && (
              <NativeSelectOption value={channel}>
                #{channel} · {t('Associated channel no longer exists')}
              </NativeSelectOption>
            )}
            {props.channels.map((c) => (
              <NativeSelectOption key={c.id} value={c.id}>
                #{c.id} {c.name}
              </NativeSelectOption>
            ))}
          </NativeSelect>
        </Field>
        <Field>
          <FieldLabel htmlFor={`model-${props.target.id}`}>
            {t('Site model')}
          </FieldLabel>
          <NativeSelect
            id={`model-${props.target.id}`}
            disabled={props.disabled || !channel}
            value={model}
            onChange={(e) => setModel(e.target.value)}
          >
            <NativeSelectOption value=''>
              {t('Select model')}
            </NativeSelectOption>
            {models.map((m) => (
              <NativeSelectOption key={m} value={m}>
                {m}
              </NativeSelectOption>
            ))}
          </NativeSelect>
        </Field>
        <Field>
          <FieldLabel htmlFor={`hide-${props.target.id}`}>
            {t('Hide this target')}
          </FieldLabel>
          <Switch
            id={`hide-${props.target.id}`}
            disabled={props.disabled}
            checked={hidden}
            onCheckedChange={setHidden}
          />
        </Field>
        <Button
          className='self-end'
          disabled={props.disabled || (!!channel && !models.includes(model))}
          onClick={() =>
            props.save({
              target_id: props.target.id,
              channel_id: channel,
              display_model: model,
              hidden,
            })
          }
        >
          {t('Save association')}
        </Button>
      </FieldGroup>
      <SavedMappingReport
        report={props.report}
        dirty={
          channel !== props.target.channel_id ||
          model !== props.target.display_model ||
          hidden !== props.target.hidden
        }
      />
    </section>
  )
}
function PresentationForm(props: {
  data: Admin
  disabled: boolean
  save: (p: Presentation) => void
}) {
  const { t } = useTranslation()
  const [view, setView] = useState(props.data.presentation)
  const ordered = [...props.data.groups].sort((a, b) => {
    const ai = view.order.indexOf(a.group_uid),
      bi = view.order.indexOf(b.group_uid)
    return (ai < 0 ? 100000 : ai) - (bi < 0 ? 100000 : bi)
  })
  const move = (i: number, d: number) => {
    const order = ordered.map((g) => g.group_uid)
    ;[order[i], order[i + d]] = [order[i + d], order[i]]
    setView({ ...view, order })
  }
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('Presentation and copy')}</CardTitle>
        <CardDescription>
          {t(
            'Apply changes to update the user page. Display aliases never change routing names or price multipliers.'
          )}
        </CardDescription>
      </CardHeader>
      <CardContent className='flex flex-col gap-4'>
        <FieldGroup>
          {(['show_method', 'show_history'] as const).map((key) => (
            <Field key={key} orientation='horizontal'>
              <Switch
                id={key}
                disabled={props.disabled}
                checked={view[key]}
                onCheckedChange={(v) => setView({ ...view, [key]: v })}
              />
              <FieldLabel htmlFor={key}>
                {key === 'show_method'
                  ? t('Show methodology')
                  : t('Show history')}
              </FieldLabel>
            </Field>
          ))}
        </FieldGroup>
        <Accordion multiple>
          {ordered.map((g, i) => {
            const override = view.groups[g.group_uid] || {
              name: null,
              description_mode: 'inherit',
              description: '',
              hidden: false,
            }
            const update = (v: Partial<typeof override>) =>
              setView({
                ...view,
                groups: {
                  ...view.groups,
                  [g.group_uid]: { ...override, ...v },
                },
              })
            return (
              <SettingsAccordion
                key={g.group_uid}
                value={g.group_uid}
                title={`${override.name || g.display_name} · ${g.ratio ?? '—'}×`}
              >
                <FieldGroup>
                  <Field>
                    <FieldLabel htmlFor={`name-${g.group_uid}`}>
                      {t('Display name')}
                    </FieldLabel>
                    <Input
                      id={`name-${g.group_uid}`}
                      disabled={props.disabled}
                      value={override.name || ''}
                      placeholder={g.routing_key}
                      maxLength={40}
                      onChange={(e) => update({ name: e.target.value || null })}
                    />
                  </Field>
                  <Field>
                    <FieldLabel htmlFor={`desc-${g.group_uid}`}>
                      {t('Description')}
                    </FieldLabel>
                    <Input
                      id={`desc-${g.group_uid}`}
                      disabled={props.disabled}
                      value={override.description}
                      placeholder={g.description}
                      maxLength={160}
                      onChange={(e) =>
                        update({
                          description: e.target.value,
                          description_mode: e.target.value
                            ? 'custom'
                            : 'inherit',
                        })
                      }
                    />
                  </Field>
                  <Field orientation='horizontal'>
                    <Switch
                      id={`group-${g.group_uid}`}
                      disabled={props.disabled}
                      checked={override.hidden}
                      onCheckedChange={(hidden) => update({ hidden })}
                    />
                    <FieldLabel htmlFor={`group-${g.group_uid}`}>
                      {t('Hide group')}
                    </FieldLabel>
                  </Field>
                  <div className='flex gap-2'>
                    <Button
                      variant='outline'
                      disabled={props.disabled || i === 0}
                      onClick={() => move(i, -1)}
                    >
                      {t('Move up')}
                    </Button>
                    <Button
                      variant='outline'
                      disabled={props.disabled || i === ordered.length - 1}
                      onClick={() => move(i, 1)}
                    >
                      {t('Move down')}
                    </Button>
                  </div>
                </FieldGroup>
              </SettingsAccordion>
            )
          })}
        </Accordion>
        <CopyEditor
          slots={slots}
          value={view.copy}
          saved={props.data.presentation.copy}
          disabled={props.disabled}
          onChange={(copy) => setView({ ...view, copy })}
        />
        <div className='flex flex-wrap items-center gap-3'>
          <Button disabled={props.disabled} onClick={() => props.save(view)}>
            {t('Apply presentation')}
          </Button>
          {JSON.stringify(view) !== JSON.stringify(props.data.presentation) && (
            <span className='text-muted-foreground text-xs'>
              {t('Unsaved changes')}
            </span>
          )}
          <Button
            variant='ghost'
            disabled={props.disabled}
            onClick={() => setView(props.data.presentation)}
          >
            {t('Discard changes')}
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}
function ArchiveRecords(props: { targets: Target[] }) {
  const { t } = useTranslation()
  const [page, setPage] = useState(1)
  const [target, setTarget] = useState('')
  const [detail, setDetail] = useState<string | null>(null)
  const query = useQuery({
    queryKey: ['pelican', 'admin-records', page, target],
    queryFn: ({ signal }) =>
      getArchive<{
        data: {
          id: string
          external_id: number
          tested_at: number
          grade: string
          active: boolean
          digest: string
        }[]
        total: number
      }>(
        `/admin/records?p=${page}&page_size=20&target_id=${encodeURIComponent(target)}`,
        signal
      ),
    retry: false,
  })
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('Source records')}</CardTitle>
        <CardDescription>
          {t(
            'Saved calls, original verdicts and revisions. These records do not represent new calls made by this site.'
          )}
        </CardDescription>
      </CardHeader>
      <CardContent className='flex flex-col gap-4'>
        <NativeSelect
          aria-label={t('Target')}
          value={target}
          onChange={(e) => {
            setTarget(e.target.value)
            setPage(1)
          }}
        >
          <NativeSelectOption value=''>{t('All targets')}</NativeSelectOption>
          {props.targets.map((v) => (
            <NativeSelectOption key={v.id} value={v.id}>
              {v.provider_name} · {v.model_name}
            </NativeSelectOption>
          ))}
        </NativeSelect>
        {query.isError ? (
          <Alert>
            <AlertDescription>
              {t('Unable to load test results.')}
            </AlertDescription>
          </Alert>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('Record')}</TableHead>
                <TableHead>{t('Time')}</TableHead>
                <TableHead>{t('Result')}</TableHead>
                <TableHead>{t('Details')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {query.data?.data.map((r) => (
                <TableRow key={r.id}>
                  <TableCell>
                    #{r.external_id}
                    {!r.active && (
                      <Badge variant='outline'>{t('Previous revision')}</Badge>
                    )}
                  </TableCell>
                  <TableCell>{timeLabel(r.tested_at)}</TableCell>
                  <TableCell>{r.grade}</TableCell>
                  <TableCell>
                    <Button variant='link' onClick={() => setDetail(r.id)}>
                      {t('View')}
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
        <div className='flex items-center gap-3'>
          <Button
            variant='outline'
            disabled={page === 1}
            onClick={() => setPage(page - 1)}
          >
            {t('Previous')}
          </Button>
          <span className='text-xs tabular-nums'>
            {page} · {query.data?.total ?? 0}
          </span>
          <Button
            variant='outline'
            disabled={page * 20 >= (query.data?.total ?? 0)}
            onClick={() => setPage(page + 1)}
          >
            {t('Next')}
          </Button>
        </div>
        <ArchiveEvidence id={detail} admin onClose={() => setDetail(null)} />
      </CardContent>
    </Card>
  )
}
