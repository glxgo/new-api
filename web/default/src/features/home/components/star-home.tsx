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
import { ArrowRight } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Aurora } from './reactbits/aurora'
import { ScrollVelocity } from './reactbits/scroll-velocity'

// Stellaisle 品牌主页（马卡龙多彩系）：
// 一切颜色走设计系统 token（var(--primary)/--chart-*），不硬编码紫蓝渐变。
// 多彩感由 chart-1~5（薄荷/樱花/奶油/天蓝/薰衣草）轮换点缀，主色蜜桃珊瑚统领。
interface StarHomeProps {
  isAuthenticated?: boolean
}

// 马卡龙点缀色轮换（完整类名字面量，确保 Tailwind 识别）
const FEATURES = [
  {
    ico: '🛡️',
    title: '稳定优先',
    iconBg: 'bg-chart-1/10',
    hoverBorder: 'hover:border-chart-1/40',
    desc: (
      <>
        宁留余量，<b className='text-foreground'>不超售、不滥用</b>。线路精挑细选，高峰期也不让你排队失败——该通的请求，一定通。
      </>
    ),
  },
  {
    ico: '💰',
    title: '余额可退',
    iconBg: 'bg-chart-2/10',
    hoverBorder: 'hover:border-chart-2/40',
    desc: (
      <>
        没用完的余额<b className='text-foreground'>支持退款</b>。不靠&ldquo;充进去拿不出来&rdquo;赚钱，靠你长期稳稳地用。
      </>
    ),
  },
  {
    ico: '⚖️',
    title: '公道定价',
    iconBg: 'bg-chart-3/10',
    hoverBorder: 'hover:border-chart-3/40',
    desc: (
      <>
        不卷最低价（低价的代价是超售和掉线）。稳定前提下给公道价，<b className='text-foreground'>每一分钱都买得到可用性</b>。
      </>
    ),
  },
] as const

const CAPABILITIES = [
  { ci: '💬', t: '对话补全', p: 'GPT / Claude / Gemini 全系列', bg: 'bg-chart-4/10' },
  { ci: '🖼️', t: '图像生成', p: 'DALL·E · Midjourney · SD', bg: 'bg-chart-1/10' },
  { ci: '🔢', t: '向量与重排', p: 'Embedding · Rerank · RAG', bg: 'bg-chart-2/10' },
  { ci: '⚡', t: '进阶能力', p: 'Function Call · Realtime', bg: 'bg-chart-3/10' },
  { ci: '📡', t: '全客户端兼容', p: 'Cherry / NextChat / Cursor', bg: 'bg-chart-5/10' },
  { ci: '🔌', t: '统一接口', p: '兼容 OpenAI 格式', bg: 'bg-chart-4/10' },
] as const

const STEPS = [
  { num: '01', t: '注册账号', p: '创建账号，登录控制台' },
  { num: '02', t: '充值余额', p: '按需充值，用多少花多少，支持退款' },
  { num: '03', t: '生成令牌', p: '拿到 sk-xxxx，填进任意兼容客户端' },
] as const

export function StarHome({ isAuthenticated }: StarHomeProps) {
  return (
    <div
      className='relative'
      data-theme-preset='stellaisle'
      /* 局部强制 stellaisle：主页保留马卡龙特殊配色，
         其他页面走全局 default（原系统配色）。*/
    >
      {/* 马卡龙极光背景（Aurora 改马卡龙色：珊瑚/薄荷/奶油，替代旧蓝青紫 AI 极光）*/}
      <div
        aria-hidden
        className='pointer-events-none fixed inset-0 -z-10 overflow-hidden opacity-[0.28] dark:opacity-[0.18]'
      >
        <Aurora
          colorStops={['#E8916C', '#7AD9B8', '#FFE5A0']}
          amplitude={0.8}
          blend={0.6}
        />
      </div>

      <div className='relative z-10 mx-auto max-w-[1120px] px-6 pt-[84px]'>
        {/* Hero */}
        <div className='py-9 text-center sm:py-11'>
          <span
            className='landing-animate-fade-up mb-6 inline-flex items-center gap-2 rounded-full border border-primary/20 bg-primary/10 px-3.5 py-1.5 text-xs font-semibold tracking-[1.5px] text-primary opacity-0'
            style={{ animationDelay: '0ms' }}
          >
            <span className='size-1.5 rounded-full bg-primary' />
            STABLE FIRST · 稳定优先
          </span>
          <h1
            className='landing-animate-fade-up text-[clamp(34px,6vw,58px)] font-extrabold leading-[1.15] tracking-tight opacity-0'
            style={{ animationDelay: '80ms' }}
          >
            稳到让你忘了它存在的
            <br />
            <span className='text-[#d97757]'>AI API 中转站</span>
          </h1>
          <p
            className='landing-animate-fade-up mx-auto mt-5 max-w-[620px] text-[clamp(16px,2.5vw,19px)] leading-relaxed text-muted-foreground opacity-0'
            style={{ animationDelay: '160ms' }}
          >
            聚合 OpenAI、Claude、Gemini 等主流模型，一个 Key 通吃。
            <br />
            不盲目追求最低价——只做
            <strong className='text-foreground'>稳定</strong>与
            <strong className='text-foreground'>低价</strong>之间最舒服的平衡。
          </p>
          <div
            className='landing-animate-fade-up mt-8 flex flex-wrap justify-center gap-3.5 opacity-0'
            style={{ animationDelay: '240ms' }}
          >
            {isAuthenticated ? (
              <Button
                className='star-glow h-[51px] rounded-2xl bg-primary px-8 text-[15px] font-semibold shadow-lg hover:-translate-y-0.5 hover:shadow-xl'
                render={<Link to='/dashboard' />}
              >
                进入控制台 <ArrowRight className='ml-1 size-4' />
              </Button>
            ) : (
              <>
                <Button
                  className='star-glow h-[51px] rounded-2xl bg-primary px-7 text-[15px] font-semibold shadow-lg hover:-translate-y-0.5 hover:shadow-xl'
                  render={<Link to='/dashboard' />}
                >
                  进入控制台 <ArrowRight className='ml-1 size-4' />
                </Button>
                <Button
                  variant='outline'
                  className='h-[51px] rounded-2xl px-7 text-[15px] font-semibold'
                  render={<Link to='/pricing' />}
                >
                  查看定价
                </Button>
              </>
            )}
          </div>
        </div>

        {/* ScrollVelocity 横幅：特性词 + 模型名双向滚动，活泼动效 */}
        <div
          className='landing-animate-fade-in overflow-hidden py-2 opacity-0'
          style={{ animationDelay: '320ms' }}
        >
          <ScrollVelocity
            texts={[
              <span key='a' className='text-foreground/80'>
                稳定优先 ✦ 余额可退 ✦ 公道定价 ✦ 全客户端兼容
              </span>,
              <span key='b' className='text-primary/70'>
                GPT ✦ Claude ✦ Gemini ✦ Midjourney ✦ DALL·E
              </span>,
            ]}
            velocity={40}
            numCopies={3}
          />
        </div>

        {/* 为什么选我们 */}
        <section className='py-[50px]'>
          <div
            className='landing-animate-fade-up mb-10 text-center opacity-0'
            style={{ animationDelay: '100ms' }}
          >
            <h2 className='text-[clamp(25px,4vw,34px)] font-bold'>
              为什么选我们
            </h2>
            <p className='mt-2.5 text-muted-foreground'>三件事，说到做到</p>
          </div>
          <div className='grid gap-[22px] sm:grid-cols-3'>
            {FEATURES.map((f, i) => (
              <div
                key={f.title}
                className={`landing-animate-fade-up rounded-3xl border border-border bg-card p-7 shadow-sm transition-all duration-300 hover:-translate-y-1.5 ${f.hoverBorder} hover:shadow-xl opacity-0`}
                style={{ animationDelay: `${120 + i * 80}ms` }}
              >
                <div
                  className={`mb-[18px] flex size-14 items-center justify-center rounded-2xl ${f.iconBg} text-[28px]`}
                >
                  {f.ico}
                </div>
                <h3 className='mb-2.5 text-xl font-bold'>{f.title}</h3>
                <p className='text-[14.5px] leading-relaxed text-muted-foreground'>
                  {f.desc}
                </p>
              </div>
            ))}
          </div>
        </section>

        {/* 能力一览 */}
        <section className='py-[50px]'>
          <div className='mb-10 text-center'>
            <h2 className='text-[clamp(25px,4vw,34px)] font-bold'>能力一览</h2>
            <p className='mt-2.5 text-muted-foreground'>
              主流模型与接口，一站接入
            </p>
          </div>
          <div className='grid gap-4 sm:grid-cols-2 lg:grid-cols-3'>
            {CAPABILITIES.map((c) => (
              <div
                key={c.t}
                className='rounded-2xl border border-border bg-card p-[22px] transition-all duration-200 hover:-translate-y-[3px] hover:border-primary/30 hover:shadow-lg'
              >
                <div
                  className={`mb-2.5 flex size-11 items-center justify-center rounded-xl ${c.bg} text-[22px]`}
                >
                  {c.ci}
                </div>
                <h4 className='mb-1.5 text-[15.5px] font-semibold'>{c.t}</h4>
                <p className='text-[13px] text-muted-foreground'>{c.p}</p>
              </div>
            ))}
          </div>
        </section>

        {/* 三步上手 */}
        <section className='py-[50px]'>
          <div className='mb-10 text-center'>
            <h2 className='text-[clamp(25px,4vw,34px)] font-bold'>三步上手</h2>
            <p className='mt-2.5 text-muted-foreground'>
              从注册到调用，几分钟搞定
            </p>
          </div>
          <div className='grid gap-6 sm:grid-cols-3'>
            {STEPS.map((s) => (
              <div
                key={s.num}
                className='rounded-2xl border border-border bg-card p-7 shadow-sm'
              >
                <div className='mb-3 text-[42px] leading-none font-extrabold text-primary'>
                  {s.num}
                </div>
                <h4 className='mb-2 text-[17px] font-semibold'>{s.t}</h4>
                <p className='text-sm text-muted-foreground'>{s.p}</p>
              </div>
            ))}
          </div>
          {/* 浅色马卡龙代码卡（替代旧深蓝黑终端块）：去 AI 味，语法色用 chart */}
          <div className='mt-7 overflow-x-auto rounded-2xl border border-border bg-muted/40 p-5 font-mono text-[13px] leading-[1.9] shadow-sm backdrop-blur-sm'>
            <span className='text-muted-foreground/70'>
              # 发起你的第一个请求
            </span>
            <br />
            <span className='text-chart-4'>curl</span>{' '}
            <span className='text-foreground/90'>
              https://token.stellaisle.com/v1/chat/completions
            </span>{' '}
            \
            <br />
            &nbsp;&nbsp;
            <span className='text-chart-1'>-H</span>{' '}
            <span className='text-chart-2'>
              &quot;Authorization: Bearer sk-你的令牌&quot;
            </span>{' '}
            \
            <br />
            &nbsp;&nbsp;
            <span className='text-chart-1'>-H</span>{' '}
            <span className='text-chart-2'>
              &quot;Content-Type: application/json&quot;
            </span>{' '}
            \
            <br />
            &nbsp;&nbsp;
            <span className='text-chart-1'>-d</span>{' '}
            <span className='text-chart-2'>
              &apos;&#123;&quot;model&quot;:&quot;gpt-4o-mini&quot;,&quot;messages&quot;:[&#123;&quot;role&quot;:&quot;user&quot;,&quot;content&quot;:&quot;你好&quot;&#125;]&#125;&apos;
            </span>
          </div>
        </section>

        {/* Final CTA */}
        <div className='my-[50px] rounded-3xl border border-primary/20 bg-primary/10 px-8 py-[58px] text-center'>
          <h2 className='text-[clamp(23px,4vw,32px)] font-bold'>
            选择我们，不是选最便宜的，而是选最省心的
          </h2>
          <p className='my-[14px] italic text-muted-foreground'>
            稳定是承诺，退款是底气，平衡是态度。
          </p>
          {isAuthenticated ? (
            <Button
              className='star-glow h-[51px] rounded-2xl bg-primary px-8 text-[15px] font-semibold shadow-lg hover:-translate-y-0.5 hover:shadow-xl'
              render={<Link to='/dashboard' />}
            >
              进入控制台 <ArrowRight className='ml-1 size-4' />
            </Button>
          ) : (
            <Button
              className='star-glow h-[51px] rounded-2xl bg-primary px-8 text-[15px] font-semibold shadow-lg hover:-translate-y-0.5 hover:shadow-xl'
              render={<Link to='/dashboard' />}
            >
              进入控制台 <ArrowRight className='ml-1 size-4' />
            </Button>
          )}
        </div>
      </div>
    </div>
  )
}
