/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { Link } from '@tanstack/react-router'
import {
  ArrowRight,
  Shield,
  Undo2,
  Lock,
  Zap,
  CircleCheck,
  Terminal,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Aurora } from './reactbits/aurora'
import { DotGrid } from './reactbits/dot-grid'
import { LightRays } from './reactbits/lightrays'

// Star API 品牌主页（纯展示 Landing，参考 sui-xiang / shuaiapi）：
// 极简白底 + 衬线大标题 + 点阵网格底纹 + Aurora/LightRays 氛围层。
// 点「即刻开始」跳 /sign-in 登录页（左文案 + 右登录卡）。
interface StarHomeProps {
  isAuthenticated?: boolean
}

// 顶部强调标签（青蓝波浪线，随想的标志手法）
const TAGS = ['稳定优先', '余额可退', '公道定价'] as const

// OUR PROMISE 四张卡片
const PROMISES = [
  {
    icon: Shield,
    label: '稳定',
    title: '不超售',
    desc: '宁留余量，高峰期也不让你排队失败——该通的请求，一定通。',
  },
  {
    icon: Undo2,
    label: '退款',
    title: '余额可退',
    desc: '没用完的余额支持退款。不靠「充进去拿不出来」赚钱。',
  },
  {
    icon: Lock,
    label: '安全',
    title: '请求即焚',
    desc: '不写日志、不入数据库、不参与训练、不与任何第三方共享。',
  },
  {
    icon: Zap,
    label: '速度',
    title: '毫秒调度',
    desc: '调度链路 稳定不锻炼，首字时间与官方直连持平。',
  },
] as const

// 数据统计
const STATS = [
  { value: '99.81%', label: '近 30 天可用率' },
  { value: '<1000ms', label: '平均调度延迟' },
  { value: '7×24', label: '实时线路监控' },
] as const

// WHY 六点
const WHYS = [
  'Claude / GPT / Gemini 等主流模型一站接入，统一 OpenAI 兼容协议。',
  '多线路冗余、跨区域容灾、自动故障切换，长链路 SSE 不中断。',
  '请求体不落盘、不用于模型训练、不与第三方共享，全链路 TLS。',
  '注册即开 API，无需审批、无需绑卡，第一次充值即可开始调用。',
  '按官方 Token 倍率计价，每一次调用都有用量日志与原始响应可回溯。',
  '支持主流支付方式，余额可视化，接近阈值自动提醒。',
] as const

// 接入三步
const STEPS = [
  { num: '01', t: '注册账号', p: '创建账号，登录控制台' },
  { num: '02', t: '充值余额', p: '按需充值，用多少花多少，支持退款' },
  { num: '03', t: '生成令牌', p: '拿到 sk-xxxx，填进任意兼容客户端' },
] as const

// 透明可核验
const VERIFIED = [
  '多线路冗余 + 自动故障切换',
  '用量日志逐条可回溯',
  '余额变动透明可查',
  '请求体不留存、不训练',
] as const

// 第三方检测报告外链
const REPORT_URL =
  'https://modeloc.com/r/TyOa-WnpChjxPgpXr3jzcytXz4RSn5pnpjtVlDofZIo'

export function StarHome({ isAuthenticated }: StarHomeProps) {
  return (
    <div
      className='relative isolate font-sans'
      data-theme-preset='stellaisle'
      /* 局部强制 stellaisle 调性，强调色青蓝 #008AB7 */
    >
      {/* ── 背景层：Aurora 极光 + LightRays 光束 + DotGrid 点矩阵（整体淡化，保证文字可读）── */}
      <div
        aria-hidden
        className='pointer-events-none fixed inset-0 z-0 overflow-hidden'
      >
        {/* Aurora 马卡龙极光（极淡氛围）*/}
        <div className='absolute inset-0 opacity-[0.16] dark:opacity-[0.12]'>
          <Aurora
            colorStops={['#E8916C', '#7AD9B8', '#FFE5A0']}
            amplitude={0.8}
            blend={0.6}
          />
        </div>
        {/* LightRays 暖色光束（极淡）*/}
        <div className='absolute inset-0 opacity-[0.16] dark:opacity-[0.12]'>
          <LightRays
            raysOrigin='top-center'
            raysColor='#ffedb6'
            raysSpeed={1.1}
            lightSpread={0.85}
            rayLength={1.25}
            pulsating
            followMouse
            mouseInfluence={0.07}
            noiseAmount={0.08}
            distortion={0.03}
          />
        </div>
        {/* 交互式点矩阵（淡纹理，鼠标靠近时青蓝高亮 / 点击冲击波）*/}
        <DotGrid
          dotSize={4}
          gap={30}
          baseColor='#e6e1f0'
          activeColor='#008AB7'
          proximity={140}
          shockRadius={250}
          shockStrength={5}
          resistance={750}
          returnDuration={1.5}
        />
        {/* 上下渐隐 */}
        <div className='from-background absolute inset-x-0 top-0 h-32 bg-gradient-to-b to-transparent' />
        <div className='from-background absolute inset-x-0 bottom-0 h-40 bg-gradient-to-t to-transparent' />
      </div>

      {/* ════════ Hero ════════ */}
      <section className='relative z-10 mx-auto max-w-[1080px] px-6 pt-36 pb-20 text-center sm:pt-44'>
        {/* 顶部波浪线强调标签 */}
        <div className='text-muted-foreground mb-8 text-sm font-medium tracking-wide'>
          {TAGS.map((tag, i) => (
            <span key={tag}>
              <span className='underline decoration-[#008AB7] decoration-wavy decoration-2 underline-offset-4'>
                {tag}
              </span>
              {i < TAGS.length - 1 && (
                <span className='mx-2.5 opacity-50'>·</span>
              )}
            </span>
          ))}
        </div>

        {/* 衬线大标题 */}
        <h1 className='font-serif text-[clamp(46px,8vw,88px)] leading-[1.04] font-bold tracking-tight'>
          Star <span className='text-[#008AB7]'>API</span>
        </h1>

        <p className='text-muted-foreground mx-auto mt-6 max-w-[640px] text-[clamp(17px,2.4vw,22px)] leading-relaxed'>
          稳到让你忘了它存在的 AI 中转平台。
          <br className='hidden sm:block' />
          聚合 GPT、Claude、Gemini，一个 Key 通吃。
        </p>

        {/* CTA */}
        <div className='mt-10 flex flex-wrap items-center justify-center gap-3'>
          <Button
            className='star-glow bg-foreground text-background hover:bg-foreground/90 h-[54px] rounded-full px-8 text-[15px] font-medium shadow-lg transition-all hover:-translate-y-0.5 hover:shadow-xl'
            render={<Link to={isAuthenticated ? '/dashboard' : '/sign-in'} />}
          >
            {isAuthenticated ? '进入控制台' : '即刻开始'}
            <ArrowRight className='ml-1.5 size-4' />
          </Button>
          <Button
            variant='outline'
            className='h-[54px] rounded-full px-8 text-[15px] font-medium'
            render={
              <a href={REPORT_URL} target='_blank' rel='noopener noreferrer' />
            }
          >
            第三方报告
          </Button>
        </div>

        {/* WORKS ON */}
        <div className='text-muted-foreground/70 mt-16 text-[11px] font-medium tracking-[0.22em] uppercase'>
          Works On · Cherry Studio · NextChat · Cursor · Cline · LobeChat
        </div>
      </section>

      {/* ════════ OUR PROMISE ════════ */}
      <section className='relative z-10 mx-auto max-w-[1040px] px-6 py-24'>
        <div className='text-center'>
          <div className='text-muted-foreground mb-4 text-xs font-semibold tracking-[0.25em] uppercase'>
            Our Promise · 我们的承诺
          </div>
          <h2 className='font-serif text-[clamp(30px,5vw,46px)] leading-[1.15] font-bold tracking-tight'>
            稳到的每一笔调用，
            <span className='text-[#008AB7]'>都经得起核对</span>
          </h2>
          <p className='text-muted-foreground mx-auto mt-6 max-w-[680px] text-[15px] leading-relaxed'>
            Star API
            网关只做一件事——把你的请求按官方协议稳稳送到上游模型，响应原样回到你手里。
            不超售、不滥用、不静默切换替代模型，你收到的每一个 token
            都来自你声明的那个模型。
          </p>
        </div>

        {/* 四卡片 */}
        <div className='mt-14 grid gap-5 sm:grid-cols-2 lg:grid-cols-4'>
          {PROMISES.map((p) => (
            <div
              key={p.title}
              className='border-border/70 bg-card/80 rounded-2xl border p-6 text-left shadow-sm backdrop-blur-sm transition-all duration-300 hover:-translate-y-1 hover:border-[#008AB7]/40 hover:shadow-md'
            >
              <div className='flex size-11 items-center justify-center rounded-xl bg-[#008AB7]/10'>
                <p.icon className='size-5 text-[#008AB7]' />
              </div>
              <div className='text-muted-foreground mt-4 text-[11px] font-semibold tracking-[0.2em] uppercase'>
                {p.label}
              </div>
              <div className='mt-0.5 text-lg font-bold'>{p.title}</div>
              <p className='text-muted-foreground mt-2 text-[13px] leading-relaxed'>
                {p.desc}
              </p>
            </div>
          ))}
        </div>
      </section>

      {/* ════════ 数据统计 ════════ */}
      <section className='relative z-10 mx-auto max-w-[920px] px-6 py-10'>
        <div className='border-border/60 bg-card/60 grid grid-cols-1 gap-6 rounded-3xl border px-8 py-10 backdrop-blur-sm sm:grid-cols-3'>
          {STATS.map((s) => (
            <div key={s.label} className='text-center'>
              <div className='font-serif text-[clamp(32px,5vw,46px)] font-bold tracking-tight text-[#008AB7]'>
                {s.value}
              </div>
              <div className='text-muted-foreground mt-1.5 text-[13px]'>
                {s.label}
              </div>
            </div>
          ))}
        </div>
      </section>

      {/* ════════ WHY ════════ */}
      <section className='relative z-10 mx-auto max-w-[1040px] px-6 py-24'>
        <div className='text-center'>
          <div className='text-muted-foreground mb-4 text-xs font-semibold tracking-[0.25em] uppercase'>
            Why · 为什么选我们
          </div>
          <h2 className='font-serif text-[clamp(30px,5vw,46px)] leading-[1.15] font-bold tracking-tight'>
            少一些妨碍，<span className='text-[#008AB7]'>多一份安稳</span>
          </h2>
        </div>

        <div className='mt-14 grid gap-x-10 gap-y-7 sm:grid-cols-2'>
          {WHYS.map((text, i) => (
            <div key={i} className='flex gap-4'>
              <div className='font-serif text-2xl font-bold text-[#008AB7]/40'>
                {String(i + 1).padStart(2, '0')}
              </div>
              <p className='text-foreground/80 pt-1 text-[14.5px] leading-relaxed'>
                {text}
              </p>
            </div>
          ))}
        </div>
      </section>

      {/* ════════ 透明可核验（参考帅 API）════════ */}
      <section className='relative z-10 mx-auto max-w-[1040px] px-6 py-24'>
        <div className='text-center'>
          <div className='text-muted-foreground mb-4 text-xs font-semibold tracking-[0.25em] uppercase'>
            Transparent · 透明可核验
          </div>
          <h2 className='font-serif text-[clamp(30px,5vw,46px)] leading-[1.15] font-bold tracking-tight'>
            直连线路、透明价格、<span className='text-[#008AB7]'>稳定调度</span>
          </h2>
          <p className='text-muted-foreground mx-auto mt-6 max-w-[640px] text-[15px] leading-relaxed'>
            不搞营销倍率、不混用超售线路——这里的每一条承诺都可核验。
          </p>
        </div>

        <div className='mx-auto mt-14 max-w-xl'>
          {/* 可核验清单 */}
          <div className='border-border/70 bg-card/80 rounded-2xl border p-7 backdrop-blur-sm'>
            <h3 className='text-lg font-bold'>可核验的稳定</h3>
            <p className='text-muted-foreground mt-2 text-[13.5px] leading-relaxed'>
              透明、可追溯，让你用得放心。
            </p>
            <ul className='mt-5 space-y-3'>
              {VERIFIED.map((v) => (
                <li key={v} className='flex items-start gap-2.5'>
                  <CircleCheck className='mt-0.5 size-4 shrink-0 text-[#008AB7]' />
                  <span className='text-[14px] leading-relaxed'>{v}</span>
                </li>
              ))}
            </ul>
          </div>
        </div>
      </section>

      {/* ════════ ONBOARD 零代码接入 ════════ */}
      <section className='relative z-10 mx-auto max-w-[1040px] px-6 py-24'>
        <div className='text-center'>
          <div className='text-muted-foreground mb-4 text-xs font-semibold tracking-[0.25em] uppercase'>
            Onboard · 零代码接入
          </div>
          <h2 className='font-serif text-[clamp(30px,5vw,46px)] leading-[1.15] font-bold tracking-tight'>
            一行代码都<span className='text-[#008AB7]'>不用写</span>
          </h2>
        </div>

        <div className='mt-14 grid gap-6 sm:grid-cols-3'>
          {STEPS.map((s) => (
            <div
              key={s.num}
              className='border-border/70 bg-card/80 rounded-2xl border p-6 backdrop-blur-sm'
            >
              <div className='font-serif text-[34px] leading-none font-bold text-[#008AB7]'>
                {s.num}
              </div>
              <h4 className='mt-3 text-[16px] font-semibold'>{s.t}</h4>
              <p className='text-muted-foreground mt-1.5 text-[13px]'>{s.p}</p>
            </div>
          ))}
        </div>

        {/* 代码块 */}
        <div className='border-border/70 mt-8 overflow-x-auto rounded-2xl border bg-[#1a1a1a] p-5 font-mono text-[13px] leading-[1.9] shadow-sm'>
          <div className='mb-2 flex items-center gap-2 text-white/40'>
            <Terminal className='size-4' />
            <span className='text-xs'>curl</span>
          </div>
          <span className='text-[#7AD9B8]'>curl</span>{' '}
          <span className='text-white/90'>
            https://token.stellaisle.com/v1/chat/completions
          </span>{' '}
          \
          <br />
          &nbsp;&nbsp;<span className='text-[#E8916C]'>-H</span>{' '}
          <span className='text-[#FFE5A0]'>
            &quot;Authorization: Bearer sk-你的令牌&quot;
          </span>{' '}
          \
          <br />
          &nbsp;&nbsp;<span className='text-[#E8916C]'>-d</span>{' '}
          <span className='text-[#FFE5A0]'>
            &apos;&#123;&quot;model&quot;:&quot;gpt-4o-mini&quot;,&quot;messages&quot;:[&#123;&quot;role&quot;:&quot;user&quot;,&quot;content&quot;:&quot;你好&quot;&#125;]&#125;&apos;
          </span>
        </div>
      </section>

      {/* ════════ Final CTA ════════ */}
      <section className='relative z-10 mx-auto max-w-[920px] px-6 py-28 text-center'>
        <div className='text-muted-foreground mb-4 text-xs font-semibold tracking-[0.3em] uppercase'>
          Global · Multi-Model · API Gateway
        </div>
        <h2 className='font-serif text-[clamp(36px,6vw,64px)] leading-[1.1] font-bold tracking-tight'>
          一念既起，<span className='text-[#008AB7]'>模型即至</span>
        </h2>
        <p className='text-muted-foreground mx-auto mt-6 max-w-[520px] text-[15px] leading-relaxed'>
          稳定是承诺，退款是底气，平衡是态度。
        </p>
        <div className='mt-9 flex flex-wrap items-center justify-center gap-3'>
          <Button
            className='star-glow bg-foreground text-background hover:bg-foreground/90 h-[54px] rounded-full px-8 text-[15px] font-medium shadow-lg transition-all hover:-translate-y-0.5 hover:shadow-xl'
            render={<Link to={isAuthenticated ? '/dashboard' : '/sign-in'} />}
          >
            {isAuthenticated ? '进入控制台' : '即刻开始'}
            <ArrowRight className='ml-1.5 size-4' />
          </Button>
        </div>
      </section>
    </div>
  )
}
