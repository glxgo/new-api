import { useState } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { i18nReady } from '@/i18n/config'
import '@/styles/index.css'
import i18next from 'i18next'
import { Toaster } from 'sonner'
import animationPoster from './animation-fixture/poster.png'
import animationRecording from './animation-fixture/animation.png'
import animationEvidence from './animation-fixture/evidence.png'
import animationMetadata from './animation-fixture/metadata.json'
import { useAuthStore } from '@/stores/auth-store'
import { api } from '@/lib/api'
import { FontProvider } from '@/context/font-provider'
import { ThemeProvider, useTheme } from '@/context/theme-provider'
import { Alert, AlertTitle, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'
import { CapabilityOverview } from '@/features/intelligence-test'
import { CapabilityAdmin } from '@/features/intelligence-test/admin'
import {
  useCapabilityOverview,
  type AdminRunDetail,
  type Control,
  type Group,
  type Run,
  type Target,
} from '@/features/intelligence-test/api'

const client = new QueryClient({
  defaultOptions: { queries: { retry: false } },
})
const started = Math.floor(Date.now() / 1000) - 48 * 1800
let round = 48
let pending = false
let disabledChannel = false
let moved = false
const groups: Group[] = [
  {
    group_uid: 'a',
    routing_key: '优选线路',
    display_name: '优选线路',
    description: '适合日常创作与稳定的推理任务。',
    ratio: 1,
    icon_type: 0,
    models: ['演示模型'],
  },
  {
    group_uid: 'b',
    routing_key: '专业线路',
    display_name: '专业线路',
    description: '面向复杂任务的精选模型线路。',
    ratio: 2.5,
    icon_type: 0,
    models: ['演示模型'],
  },
]
let control: Control = {
  revision: 1,
  running: true,
  visible: true,
  updated_at: started,
  config: {
    interval_minutes: 30,
    anchor: started,
    timezone: 'Asia/Shanghai',
    weekdays: [0, 1, 2, 3, 4, 5, 6],
    window_start: 0,
    window_end: 0,
    all_day: true,
    models: [
      { model: '演示模型', protocol: 'chat', max_tokens: 8192, enabled: true },
    ],
    excluded: [],
    daily_budget_micros: 1000000,
    call_reserve_micros: 1000,
    judge_channel_id: 9,
    judge_model: '演示评审模型',
  },
  draft: {
    groups: {},
    order: [],
    copy: {},
    show_method: true,
    show_history: true,
    gallery_size: 3,
  },
  presentation: {
    groups: {},
    order: [],
    copy: {},
    show_method: true,
    show_history: true,
    gallery_size: 3,
  },
  next_times: [started + 49 * 1800],
}

function recoveryDemo(): AdminRunDetail {
  const source = sample(1, 'a', 47)
  const geometry = { ...source.items[1], status: 'pending_render', artifact: undefined, checks: undefined }
  const scene = { ...source.items[2], status: 'pending_review', judgment: undefined }
  return {
    run: { id: 'recovery-demo', channel_id: 1, model: '演示模型', protocol: 'chat', status: 'complete', reason: '', fingerprint: 'local-demo-only', started_at: source.time, completed_at: source.time + 120 },
    items: [source.items[0], geometry, scene],
    attempts: [],
    bindings: [{ public_id: source.public_id, group_uid: 'a', routing_key: '优选线路', dispatched_at: source.time, withdrawn: false }],
    recovery: [
      { id: 'geometry-recovery', kind: 'geometry', status: 'complete', reason: '', attempts: 3, next_at: 0 },
      { id: 'scene-recovery', kind: 'scene', status: 'blocked', reason: 'judge_outcome_unknown', attempts: 2, next_at: 0 },
    ],
    evaluations: [
      { id: 'geometry-revision', revision: 2, created_at: source.time + 400, item: { ...geometry, status: 'graded', checks: [true, true, true, true, true], artifact: '/api/capability/admin/runs/recovery-demo/evaluations/geometry-revision/artifact' } },
      { id: 'geometry-pending', revision: 1, created_at: source.time + 180, item: { ...geometry, reason: 'renderer_unavailable' } },
    ],
  }
}
function targetList(): Target[] {
  return [1, 2, 3].flatMap((channel) =>
    (channel === 1 ? (moved ? ['b'] : ['a', 'b']) : ['a']).map((group) => {
      const excluded = control.config.excluded.some(
        (e) =>
          (!e.channel_id || e.channel_id === channel) &&
          (!e.group_uid || e.group_uid === group) &&
          (!e.model || e.model === '演示模型')
      )
      const reason =
        channel === 1 && disabledChannel
          ? 'channel_disabled'
          : excluded
            ? 'test_excluded'
            : ''
      return {
        group_uid: group,
        group: groups.find((g) => g.group_uid === group)!.routing_key,
        model: '演示模型',
        channel_id: channel,
        channel_name: `模拟渠道 ${channel}`,
        eligible: !reason,
        reason,
      }
    })
  )
}
function visibleGroups(draft = false) {
  const view = draft ? control.draft : control.presentation
  return groups
    .filter((g) => draft || !view.groups[g.group_uid]?.hidden)
    .map((g) => {
      const v = view.groups[g.group_uid]
      return {
        ...g,
        display_name: v?.name || g.routing_key,
        description:
          v?.description_mode === 'hidden'
            ? ''
            : v?.description_mode === 'custom'
              ? v.description
              : g.description,
      }
    })
    .sort((a, b) => {
      const rank = (id: string) =>
        view.order.includes(id) ? view.order.indexOf(id) : 100
      return rank(a.group_uid) - rank(b.group_uid)
    })
}
function demoScore(
  version: number,
  task: number,
  channel: number
): number | null {
  const maximum = [2, 5, 6][task]
  const variant = version + task * 3
  if (variant % 13 === 0) return null
  if (variant % 11 === 0) return 0
  if (variant % 7 === 0) return maximum - 1
  return Math.max(0, maximum - channel + 1)
}
function sample(channel: number, group: string, version: number): Run {
  const id = `${group}-${channel}-${version}`
  const prompts: Record<string, string> = {
    logic:
      '袋中有圆形苹果糖7颗、圆形桃子糖9颗、星形苹果糖7颗、星形桃子糖6颗，以及西瓜糖12颗。不看糖取出，最少取几颗，才能保证得到一对口味相同、形状不同的非西瓜糖？',
    geometry: `在600×400白色画布上，左侧绘制3个完整可见、互不重叠的红圆，右侧绘制2个蓝色正方形；底部标注 JX-${version}。`,
    scene:
      '生成 html，内容是 svg 绘制鹈鹕骑自行车 2D 动画，不进行测试，不使用 skill，不参考本地文件。',
  }
  return {
    public_id: id,
    model: '演示模型',
    suite: 'local-demo-only',
    sample: `D${channel}`,
    time: started + (version - 1) * 1800,
    slot: started + (version - 1) * 1800,
    status: 'complete',
    ranked: true,
    // Explicitly simulated metrics for the public evidence interaction.
    metrics: Object.fromEntries(['logic', 'geometry', 'scene'].map((kind, k) => [kind, {
      attempts: 1,
      started_at: started + (version - 1) * 1800 - 42 - k * 12,
      duration_seconds: 42 + k * 12,
      input_tokens: 1455 + k * 100,
      output_tokens: k === 0 ? 37 : null,
    }])),
    score: [0, 1, 2].map((k) => demoScore(version, k, channel)).concat(4),
    items: ['logic', 'geometry', 'scene'].map((kind, k) => {
      const score = demoScore(version, k, channel)
      return {
        kind,
        status: score === null ? 'pending_review' : 'graded',
        question: {
          prompt: `第 ${version} 轮 · 模拟题目\n${prompts[kind]}\n此内容用于界面验收，不表示真实模型表现。`,
          hash: `demo-${version}`,
        },
        answer: '本地模拟回答，仅用于交互验收；动画为开发样例（移动圆形），并非模型生成的鹈鹕或实测成绩。评分状态为模拟数据。',
        animation: kind === 'scene' ? {
          ...animationMetadata,
          artifact: `/api/capability/runs/${id}/artifacts/scene?view=animation`,
          evidence: `/api/capability/runs/${id}/artifacts/scene?view=evidence`,
        } : undefined,
        artifact:
          kind === 'logic'
            ? undefined
            : `/api/capability/runs/${id}/artifacts/${kind}`,
        checks:
          score !== null && kind !== 'scene'
            ? Array.from({ length: [2, 5][k] }, (_, i) => i < score)
            : undefined,
        judgment:
          kind === 'scene' && score !== null
            ? {
                items: Array.from({ length: 6 }, (_, i) => ({
                  status: i < score ? 'pass' : 'fail',
                  evidence: '模拟评审证据，仅用于交互验收。',
                })),
                aesthetic: 4,
                confidence: 1,
              }
            : undefined,
      }
    }),
  }
}
function demoImage(id: string, kind: string) {
  const canvas = document.createElement('canvas')
  canvas.width = 600
  canvas.height = 400
  const ctx = canvas.getContext('2d')!
  const channel = Number(id.split('-')[1]) || 1
  const circle = (x: number, y: number, r: number, color: string) => {
    ctx.fillStyle = color
    ctx.beginPath()
    ctx.arc(x, y, r, 0, Math.PI * 2)
    ctx.fill()
  }
  const line = (points: number[][], color: string, width = 7) => {
    ctx.strokeStyle = color
    ctx.lineWidth = width
    ctx.beginPath()
    points.forEach(([x, y], i) => (i ? ctx.lineTo(x, y) : ctx.moveTo(x, y)))
    ctx.stroke()
  }
  ctx.fillStyle = ['#d3e8e4', '#e5ead6', '#dbe7ef'][channel - 1]
  ctx.fillRect(0, 0, 600, 400)
  if (kind === 'geometry') {
    ctx.fillStyle = '#ffffff'
    ctx.fillRect(0, 0, 600, 400)
    ;[60, 150, 240].forEach((x) => circle(x, 130, 26, '#ef4444'))
    ctx.fillStyle = '#3b82f6'
    ctx.fillRect(350, 104, 52, 52)
    ctx.fillRect(450, 104, 52, 52)
    ctx.fillStyle = '#171717'
    ctx.font = '20px sans-serif'
    ctx.fillText('JX-' + id.split('-')[2], 24, 330)
  } else {
    ctx.fillStyle = ['#88b4aa', '#9eaf7d', '#89adbc'][channel - 1]
    ctx.fillRect(0, 275, 600, 125)
    circle(503, 64, 34, '#ffe6a0')
    ctx.strokeStyle = '#365d55'
    ctx.lineWidth = 8
    ;[190, 400].forEach((x) => {
      ctx.beginPath()
      ctx.arc(x, 288, 47, 0, Math.PI * 2)
      ctx.stroke()
    })
    line(
      [
        [190, 288],
        [255, 195],
        [321, 288],
        [190, 288],
      ],
      '#365d55'
    )
    line(
      [
        [321, 288],
        [370, 190],
        [400, 288],
      ],
      '#365d55'
    )
    line(
      [
        [255, 195],
        [370, 190],
        [371, 174],
        [393, 174],
      ],
      '#365d55'
    )
    ctx.fillStyle = '#fffcef'
    ctx.beginPath()
    ctx.ellipse(291, 159, 59, 41, -0.2, 0, Math.PI * 2)
    ctx.fill()
    circle(344, 112, 29, '#fffcef')
    circle(356, 107, 4, '#365d55')
    ctx.fillStyle = '#dda857'
    ctx.beginPath()
    ctx.moveTo(367, 115)
    ctx.lineTo(431, 136)
    ctx.lineTo(367, 134)
    ctx.closePath()
    ctx.fill()
    line(
      [
        [279, 188],
        [298, 221],
        [263, 235],
      ],
      '#c38950',
      8
    )
    line(
      [
        [328, 137],
        [352, 151],
        [333, 166],
      ],
      '#b75b4c',
      8
    )
    ctx.fillStyle = '#9c7954'
    ctx.fillRect(393, 162, 47, 32)
    circle(405, 161, 10, '#ebac52')
    circle(425, 161, 10, '#ebac52')
  }
  ctx.fillStyle = '#283f3c'
  ctx.font = '17px sans-serif'
  ctx.fillText('模拟作品 · 非模型实测', 22, 380)
  return new Promise<Blob>((resolve) =>
    canvas.toBlob((blob) => resolve(blob!), 'image/png')
  )
}

// This entry intercepts every API call in memory and cannot send to a server.
api.defaults.adapter = async (request) => {
  const url = new URL(request.url || '/', 'http://local.invalid')
  const path = url.pathname.replace('/api/capability', '')
  let data: unknown
  if (path.endsWith('/artifacts/scene')) {
    const view = url.searchParams.get('view')
    let asset = animationPoster
    if (view === 'animation') asset = animationRecording
    if (view === 'evidence') asset = animationEvidence
    data = await fetch(asset).then((response) => response.blob())
  } else if (path.includes('/artifacts/'))
    data = await demoImage(path.split('/')[2], path.split('/').at(-1)!)
  else if (path === '/admin/runs/recovery-demo/evaluations/geometry-revision/artifact')
    data = await demoImage('a-1-47', 'geometry')
  else if (request.method === 'put') {
    const input = JSON.parse(request.data)
    if (input.revision !== control.revision)
      throw new Error('模拟并发版本冲突，请刷新。')
    const next = structuredClone(control)
    switch (input.action) {
      case 'hide_and_stop':
        next.visible = false
        next.running = false
        break
      case 'hide_only':
        next.visible = false
        break
      case 'show_only':
        next.visible = true
        break
      case 'stop':
        next.running = false
        break
      case 'resume':
        next.running = true
        break
      case 'save_config':
        next.config = input.config
        break
      case 'save_draft':
        next.draft = input.presentation
        break
      case 'publish':
        next.presentation = input.presentation || next.draft
        next.draft = next.presentation
        break
      case 'set_order':
        next.draft.order = input.order
        next.presentation.order = input.order
        break
      default:
        throw new Error('Unknown demo action')
    }
    next.revision++
    control = next
    data = { success: true, data: next }
  } else if (path === '/admin/control') data = { success: true, data: control }
  else if (path === '/admin/groups')
    data = { success: true, data: visibleGroups(true) }
  else if (path === '/admin/targets')
    data = { success: true, data: targetList() }
  else if (path === '/admin/runtime')
    data = {
      success: true,
      data: {
        readiness: [],
        active_calls: pending ? 1 : 0,
        worker: {
          state: pending ? 'running' : 'idle',
          heartbeat: Date.now() / 1000,
        },
        budget: { reserved_micros: 0 },
      },
    }
  else if (path === '/admin/runs')
    data = { success: true, data: [recoveryDemo().run], has_more: false }
  else if (path === '/admin/runs/recovery-demo')
    data = { success: true, data: recoveryDemo() }
  else if (path === '/groups')
    data = {
      success: true,
      data: visibleGroups(),
      ...control.presentation,
      visible: control.visible,
      running: control.running,
      revision: control.revision,
      next_times: control.next_times,
      method: {
        suite: 'local-demo-only',
        interval_minutes: control.config.interval_minutes,
        timezone: control.config.timezone,
        all_day: control.config.all_day,
        window_start: control.config.window_start,
        window_end: control.config.window_end,
        weekdays: control.config.weekdays,
      },
    }
  else if (/\/groups\/[^/]+\/timeline/.test(path)) {
    const group = path.split('/')[2]
    data = {
      success: true,
      data: Array.from({ length: 48 }, (_, i) => {
        const version = round - 47 + i
        const samples = group === 'a' ? 3 : 1
        return {
          slot: started + (version - 1) * 1800,
          completed_at: started + (version - 1) * 1800 + 120,
          suite: 'local-demo-only',
          samples,
          items: ['logic', 'geometry', 'scene'].map((kind, k) => {
            const maximum = [2, 5, 6][k]
            const score = demoScore(version, k, 1)
            return {
              kind,
              maximum,
              evaluated: score === null ? 0 : samples,
              score,
              public_id: `${group}-1-${version}`,
            }
          }),
        }
      }),
    }
  } else if (/\/groups\/[^/]+\/results/.test(path)) {
    const group = path.split('/')[2]
    data = {
      success: true,
      data: targetList()
        .filter((t) => t.group_uid === group && t.eligible)
        .map((t) => sample(t.channel_id, group, round)),
      has_more: false,
    }
  } else if (path.startsWith('/runs/')) {
    const [group, channel, version] = path.split('/')[2].split('-')
    data = {
      success: true,
      data: sample(Number(channel), group, Number(version)),
    }
  } else throw new Error(`Unmapped demo request: ${path}`)
  return {
    data: structuredClone(data),
    status: 200,
    statusText: 'OK',
    headers: {},
    config: request,
  }
}
useAuthStore
  .getState()
  .auth.setUser({ id: 1, username: 'local-demo', role: 100 })

function Preview() {
  const overview = useCapabilityOverview()
  const { resolvedTheme, setTheme } = useTheme()
  const [, refresh] = useState(0)
  const change = (action: () => void) => {
    action()
    refresh((v) => v + 1)
    void client.invalidateQueries({ queryKey: ['capability'] })
  }
  return (
    <main className='mx-auto flex max-w-7xl flex-col gap-6 px-4 py-6 sm:px-8'>
      <Alert>
        <AlertTitle>本地交互验收</AlertTitle>
        <AlertDescription>
          全部为模拟数据，不连接生产、不调用模型。移动圆形是动画链路样例，不代表鹈鹕作品或实测成绩。下方使用真实产品页面与原生组件。
        </AlertDescription>
      </Alert>
      <div className='flex flex-wrap gap-2'>
        <Button
          variant='outline'
          onClick={() => setTheme(resolvedTheme === 'dark' ? 'light' : 'dark')}
        >
          切换明暗主题
        </Button>
        <Button
          variant='outline'
          onClick={() =>
            change(() => {
              disabledChannel = !disabledChannel
            })
          }
        >
          {disabledChannel ? '启用模拟渠道 1' : '关闭模拟渠道 1'}
        </Button>
        <Button
          variant='outline'
          onClick={() =>
            change(() => {
              moved = !moved
            })
          }
        >
          {moved ? '恢复 A/B 共享' : '将渠道 1 从 A 移到 B'}
        </Button>
        <Button
          variant='outline'
          disabled={pending || !control.running}
          onClick={() =>
            change(() => {
              pending = true
            })
          }
        >
          开始下一轮（保留当前展示）
        </Button>
        <Button
          disabled={!pending}
          onClick={() =>
            change(() => {
              pending = false
              round++
            })
          }
        >
          完成本轮并切换
        </Button>
      </div>
      <p className='text-muted-foreground text-sm'>
        当前展示第 {round} 轮
        {pending ? ' · 下一轮测试中，题目与作品保持不变' : ''}
      </p>
      <Tabs defaultValue='public'>
        <TabsList>
          <TabsTrigger value='public'>用户页面</TabsTrigger>
          <TabsTrigger value='admin'>后台设置</TabsTrigger>
        </TabsList>
        <TabsContent value='public'>
          {overview.data && <CapabilityOverview data={overview.data} />}
        </TabsContent>
        <TabsContent value='admin'>
          <CapabilityAdmin />
        </TabsContent>
      </Tabs>
      <Toaster />
    </main>
  )
}
await i18nReady
await i18next.changeLanguage('zh')
createRoot(document.getElementById('root')!).render(
  <QueryClientProvider client={client}>
    <ThemeProvider storageKey='iq-demo-theme'>
      <FontProvider>
        <Preview />
      </FontProvider>
    </ThemeProvider>
  </QueryClientProvider>
)
