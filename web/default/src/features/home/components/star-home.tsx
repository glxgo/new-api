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

// 复刻自 home.html（Star API 品牌主页）：稳定优先 + 三大卖点 + 能力 + 三步 + CTA
interface StarHomeProps {
  isAuthenticated?: boolean
}

const FEATURES = [
  {
    ico: '🛡️',
    title: '稳定优先',
    grad: 'from-blue-500/14 to-blue-500/4',
    desc: (
      <>
        宁留余量，<b className='text-slate-900'>不超售、不滥用</b>。线路精挑细选，高峰期也不让你排队失败——该通的请求，一定通。
      </>
    ),
  },
  {
    ico: '💰',
    title: '余额可退',
    grad: 'from-emerald-500/14 to-emerald-500/4',
    desc: (
      <>
        没用完的余额<b className='text-slate-900'>支持退款</b>。不靠&ldquo;充进去拿不出来&rdquo;赚钱，靠你长期稳稳地用。
      </>
    ),
  },
  {
    ico: '⚖️',
    title: '公道定价',
    grad: 'from-purple-500/14 to-purple-500/4',
    desc: (
      <>
        不卷最低价（低价的代价是超售和掉线）。稳定前提下给公道价，<b className='text-slate-900'>每一分钱都买得到可用性</b>。
      </>
    ),
  },
] as const

const CAPABILITIES = [
  { ci: '💬', t: '对话补全', p: 'GPT / Claude / Gemini 全系列' },
  { ci: '🖼️', t: '图像生成', p: 'DALL·E · Midjourney · SD' },
  { ci: '🔢', t: '向量与重排', p: 'Embedding · Rerank · RAG' },
  { ci: '⚡', t: '进阶能力', p: 'Function Call · Realtime' },
  { ci: '📡', t: '全客户端兼容', p: 'Cherry / NextChat / Cursor' },
  { ci: '🔌', t: '统一接口', p: '兼容 OpenAI 格式' },
] as const

const STEPS = [
  { num: '01', t: '注册账号', p: '创建账号，登录控制台' },
  { num: '02', t: '充值余额', p: '按需充值，用多少花多少，支持退款' },
  { num: '03', t: '生成令牌', p: '拿到 sk-xxxx，填进任意兼容客户端' },
] as const

export function StarHome({ isAuthenticated }: StarHomeProps) {
  return (
    <div className='relative'>
      {/* 背景光晕 */}
      <div
        aria-hidden
        className='pointer-events-none fixed inset-0 z-0'
        style={{
          background: [
            'radial-gradient(ellipse 60% 50% at 20% 18%, oklch(0.72 0.18 250 / .45) 0%, transparent 70%)',
            'radial-gradient(ellipse 50% 40% at 82% 12%, oklch(0.65 0.15 200 / .38) 0%, transparent 70%)',
            'radial-gradient(ellipse 42% 38% at 45% 88%, oklch(0.70 0.12 280 / .32) 0%, transparent 70%)',
          ].join(', '),
        }}
      />
      {/* 网格背景 */}
      <div
        aria-hidden
        className='pointer-events-none fixed inset-0 z-0 opacity-55 dark:opacity-25'
        style={{
          backgroundImage:
            'linear-gradient(to right, #e2e8f0 1px, transparent 1px), linear-gradient(to bottom, #e2e8f0 1px, transparent 1px)',
          backgroundSize: '56px 56px',
          maskImage:
            'radial-gradient(ellipse 65% 55% at 50% 22%, #000 12%, transparent 100%)',
          WebkitMaskImage:
            'radial-gradient(ellipse 65% 55% at 50% 22%, #000 12%, transparent 100%)',
        }}
      />

      <div className='relative z-10 mx-auto max-w-[1120px] px-6 pt-[84px]'>
        {/* Hero */}
        <div className='py-9 text-center sm:py-11'>
          <span className='mb-6 inline-flex items-center gap-2 rounded-full border border-blue-500/22 bg-blue-500/[0.08] px-3.5 py-1.5 text-xs font-semibold tracking-[1.5px] text-blue-600 dark:text-blue-400'>
            <span className='size-1.5 rounded-full bg-blue-500 shadow-[0_0_0_3px_rgba(59,130,246,.18)]' />
            STABLE FIRST · 稳定优先
          </span>
          <h1 className='text-[clamp(34px,6vw,58px)] font-extrabold leading-[1.15] tracking-tight'>
            稳到让你忘了它存在的
            <br />
            <span className='bg-gradient-to-r from-blue-500 via-violet-500 to-purple-500 bg-clip-text text-transparent'>
              AI API 中转站
            </span>
          </h1>
          <p className='mx-auto mt-5 max-w-[620px] text-[clamp(16px,2.5vw,19px)] leading-relaxed text-slate-500 dark:text-slate-400'>
            聚合 OpenAI、Claude、Gemini 等主流模型，一个 Key 通吃。
            <br />
            不盲目追求最低价——只做<strong className='text-slate-700 dark:text-slate-300'>稳定</strong>与<strong className='text-slate-700 dark:text-slate-300'>低价</strong>之间最舒服的平衡。
          </p>
          <div className='mt-8 flex flex-wrap justify-center gap-3.5'>
            {isAuthenticated ? (
              <Button
                className='h-[51px] rounded-[10px] bg-gradient-to-br from-indigo-500 to-violet-500 px-8 text-[15px] font-semibold shadow-[0_6px_18px_rgba(99,102,241,.28)] hover:shadow-[0_10px_24px_rgba(99,102,241,.4)]'
                render={<Link to='/dashboard' />}
              >
                进入控制台 <ArrowRight className='ml-1 size-4' />
              </Button>
            ) : (
              <>
                <Button
                  className='h-[51px] rounded-[10px] bg-gradient-to-br from-indigo-500 to-violet-500 px-7 text-[15px] font-semibold shadow-[0_6px_18px_rgba(99,102,241,.28)] hover:-translate-y-0.5 hover:shadow-[0_10px_24px_rgba(99,102,241,.4)]'
                  render={<Link to='/sign-up' />}
                >
                  🚀 立即注册
                </Button>
                <Button
                  variant='outline'
                  className='h-[51px] rounded-[10px] px-7 text-[15px] font-semibold'
                  render={<Link to='/pricing' />}
                >
                  查看定价
                </Button>
              </>
            )}
          </div>
        </div>

        {/* 为什么选我们 */}
        <section className='py-[50px]'>
          <div className='mb-10 text-center'>
            <h2 className='text-[clamp(25px,4vw,34px)] font-bold'>为什么选我们</h2>
            <p className='mt-2.5 text-slate-500 dark:text-slate-400'>三件事，说到做到</p>
          </div>
          <div className='grid gap-[22px] sm:grid-cols-3'>
            {FEATURES.map((f) => (
              <div
                key={f.title}
                className='rounded-[18px] border border-[#e8ecf3] bg-white p-7 shadow-[0_1px_3px_rgba(15,23,42,.04),0_10px_30px_rgba(15,23,42,.04)] transition-all duration-300 hover:-translate-y-1.5 hover:border-indigo-500/30 hover:shadow-[0_4px_8px_rgba(15,23,42,.05),0_20px_40px_rgba(99,102,241,.1)] dark:bg-card'
              >
                <div
                  className={`mb-[18px] flex size-14 items-center justify-center rounded-[14px] bg-gradient-to-br ${f.grad} text-[28px]`}
                >
                  {f.ico}
                </div>
                <h3 className='mb-2.5 text-xl font-bold'>{f.title}</h3>
                <p className='text-[14.5px] leading-relaxed text-slate-500 dark:text-slate-400'>
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
            <p className='mt-2.5 text-slate-500 dark:text-slate-400'>主流模型与接口，一站接入</p>
          </div>
          <div className='grid gap-4 sm:grid-cols-2 lg:grid-cols-3'>
            {CAPABILITIES.map((c) => (
              <div
                key={c.t}
                className='rounded-[14px] border border-[#e8ecf3] bg-white p-[22px] transition-all duration-200 hover:-translate-y-[3px] hover:border-violet-500/35 hover:shadow-[0_12px_24px_rgba(15,23,42,.06)] dark:bg-card'
              >
                <div className='mb-2.5 text-[26px]'>{c.ci}</div>
                <h4 className='mb-1.5 text-[15.5px] font-semibold'>{c.t}</h4>
                <p className='text-[13px] text-slate-500 dark:text-slate-400'>{c.p}</p>
              </div>
            ))}
          </div>
        </section>

        {/* 三步上手 */}
        <section className='py-[50px]'>
          <div className='mb-10 text-center'>
            <h2 className='text-[clamp(25px,4vw,34px)] font-bold'>三步上手</h2>
            <p className='mt-2.5 text-slate-500 dark:text-slate-400'>从注册到调用，几分钟搞定</p>
          </div>
          <div className='grid gap-6 sm:grid-cols-3'>
            {STEPS.map((s) => (
              <div
                key={s.num}
                className='rounded-[16px] border border-[#e8ecf3] bg-white p-7 shadow-[0_1px_3px_rgba(15,23,42,.04)] dark:bg-card'
              >
                <div className='mb-3 bg-gradient-to-br from-indigo-500 to-violet-500 bg-clip-text text-[42px] leading-none font-extrabold text-transparent'>
                  {s.num}
                </div>
                <h4 className='mb-2 text-[17px] font-semibold'>{s.t}</h4>
                <p className='text-sm text-slate-500 dark:text-slate-400'>{s.p}</p>
              </div>
            ))}
          </div>
          <div className='mt-7 overflow-x-auto rounded-[14px] border border-[#1e293b] bg-[#0b1020] p-5 font-mono text-[13px] leading-[1.9] text-[#93c5fd] shadow-[0_12px_30px_rgba(15,23,42,.12)]'>
            <span className='text-slate-500'># 发起你的第一个请求</span>
            <br />
            curl https://token.stellaisle.com/v1/chat/completions \
            <br />
            &nbsp;&nbsp;-H &quot;Authorization: Bearer sk-你的令牌&quot; \
            <br />
            &nbsp;&nbsp;-H &quot;Content-Type: application/json&quot; \
            <br />
            &nbsp;&nbsp;-d &apos;&#123;&quot;model&quot;:&quot;gpt-4o-mini&quot;,&quot;messages&quot;:[&#123;&quot;role&quot;:&quot;user&quot;,&quot;content&quot;:&quot;你好&quot;&#125;]&#125;&apos;
          </div>
        </section>

        {/* Final CTA */}
        <div className='my-[50px] rounded-3xl border border-indigo-500/18 bg-gradient-to-br from-indigo-500/[0.08] to-purple-500/[0.06] px-8 py-[58px] text-center'>
          <h2 className='text-[clamp(23px,4vw,32px)] font-bold'>
            选择我们，不是选最便宜的，而是选最省心的
          </h2>
          <p className='my-[14px] italic text-slate-500 dark:text-slate-400'>
            稳定是承诺，退款是底气，平衡是态度。
          </p>
          {isAuthenticated ? (
            <Button
              className='h-[51px] rounded-[10px] bg-gradient-to-br from-indigo-500 to-violet-500 px-8 text-[15px] font-semibold shadow-[0_6px_18px_rgba(99,102,241,.28)] hover:shadow-[0_10px_24px_rgba(99,102,241,.4)]'
              render={<Link to='/dashboard' />}
            >
              进入控制台 <ArrowRight className='ml-1 size-4' />
            </Button>
          ) : (
            <Button
              className='h-[51px] rounded-[10px] bg-gradient-to-br from-indigo-500 to-violet-500 px-8 text-[15px] font-semibold shadow-[0_6px_18px_rgba(99,102,241,.28)] hover:shadow-[0_10px_24px_rgba(99,102,241,.4)]'
              render={<Link to='/sign-up' />}
            >
              立即开始使用 <ArrowRight className='ml-1 size-4' />
            </Button>
          )}
        </div>
      </div>
    </div>
  )
}
