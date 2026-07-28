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

// 关于页默认卖点内容（当后台未配置关于内容时展示）。
// 内容迁移自原 StarHome 长滚动 Landing：为什么选我们 / 能力一览 / 三步上手 / CTA。
// 局部强制 stellaisle 马卡龙配色，与首页视觉连贯。
const FEATURES = [
  {
    ico: '🛡️',
    title: '稳定优先',
    iconBg: 'bg-chart-1/10',
    hoverBorder: 'hover:border-chart-1/40',
    desc: (
      <>
        宁留余量，<b className='text-foreground'>不超售、不滥用</b>
        。线路精挑细选，高峰期也不让你排队失败——该通的请求，一定通。
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
        没用完的余额<b className='text-foreground'>支持退款</b>
        。不靠&ldquo;充进去拿不出来&rdquo;赚钱，靠你长期稳稳地用。
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
        不卷最低价（低价的代价是超售和掉线）。稳定前提下给公道价，
        <b className='text-foreground'>每一分钱都买得到可用性</b>。
      </>
    ),
  },
] as const

const CAPABILITIES = [
  {
    ci: '💬',
    t: '对话补全',
    p: 'GPT / Claude / Gemini 全系列',
    bg: 'bg-chart-4/10',
  },
  {
    ci: '🖼️',
    t: '图像生成',
    p: 'DALL·E · Midjourney · SD',
    bg: 'bg-chart-1/10',
  },
  {
    ci: '🔢',
    t: '向量与重排',
    p: 'Embedding · Rerank · RAG',
    bg: 'bg-chart-2/10',
  },
  {
    ci: '⚡',
    t: '进阶能力',
    p: 'Function Call · Realtime',
    bg: 'bg-chart-3/10',
  },
  {
    ci: '📡',
    t: '全客户端兼容',
    p: 'Cherry / NextChat / Cursor',
    bg: 'bg-chart-5/10',
  },
  { ci: '🔌', t: '统一接口', p: '兼容 OpenAI 格式', bg: 'bg-chart-4/10' },
] as const

const STEPS = [
  { num: '01', t: '注册账号', p: '创建账号，登录控制台' },
  { num: '02', t: '充值余额', p: '按需充值，用多少花多少，支持退款' },
  { num: '03', t: '生成令牌', p: '拿到 sk-xxxx，填进任意兼容客户端' },
] as const

export function AboutHighlights() {
  return (
    <div
      className='relative'
      data-theme-preset='stellaisle'
      /* 局部强制 stellaisle，保持与首页马卡龙调性一致 */
    >
      <div className='mx-auto max-w-[1120px] px-6 pt-28 pb-20'>
        {/* 标题区 */}
        <div className='mx-auto max-w-[760px] text-center'>
          <span className='border-primary/20 bg-primary/10 text-primary mb-6 inline-flex items-center gap-2 rounded-full border px-3.5 py-1.5 text-xs font-semibold tracking-[1.5px]'>
            <span className='bg-primary size-1.5 rounded-full' />
            关于 STABLE FIRST
          </span>
          <h1 className='text-[clamp(32px,5vw,52px)] leading-[1.15] font-extrabold tracking-tight'>
            稳定，是我们<span className='text-[#d97757]'>最重要的功能</span>
          </h1>
          <p className='text-muted-foreground mx-auto mt-5 max-w-[600px] text-[clamp(15px,2vw,18px)] leading-relaxed'>
            不是最便宜，也不是最花哨。我们只做一件事：当你调用 API
            的时候，它永远都在。
          </p>
        </div>

        {/* 为什么选我们 */}
        <section className='py-[50px]'>
          <div className='mb-10 text-center'>
            <h2 className='text-[clamp(25px,4vw,34px)] font-bold'>
              为什么选我们
            </h2>
            <p className='text-muted-foreground mt-2.5'>三件事，说到做到</p>
          </div>
          <div className='grid gap-[22px] sm:grid-cols-3'>
            {FEATURES.map((f) => (
              <div
                key={f.title}
                className={`border-border bg-card rounded-3xl border p-7 shadow-sm transition-all duration-300 hover:-translate-y-1.5 ${f.hoverBorder} hover:shadow-xl`}
              >
                <div
                  className={`mb-[18px] flex size-14 items-center justify-center rounded-2xl ${f.iconBg} text-[28px]`}
                >
                  {f.ico}
                </div>
                <h3 className='mb-2.5 text-xl font-bold'>{f.title}</h3>
                <p className='text-muted-foreground text-[14.5px] leading-relaxed'>
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
            <p className='text-muted-foreground mt-2.5'>
              主流模型与接口，一站接入
            </p>
          </div>
          <div className='grid gap-4 sm:grid-cols-2 lg:grid-cols-3'>
            {CAPABILITIES.map((c) => (
              <div
                key={c.t}
                className='border-border bg-card hover:border-primary/30 rounded-2xl border p-[22px] transition-all duration-200 hover:-translate-y-[3px] hover:shadow-lg'
              >
                <div
                  className={`mb-2.5 flex size-11 items-center justify-center rounded-xl ${c.bg} text-[22px]`}
                >
                  {c.ci}
                </div>
                <h4 className='mb-1.5 text-[15.5px] font-semibold'>{c.t}</h4>
                <p className='text-muted-foreground text-[13px]'>{c.p}</p>
              </div>
            ))}
          </div>
        </section>

        {/* 三步上手 */}
        <section className='py-[50px]'>
          <div className='mb-10 text-center'>
            <h2 className='text-[clamp(25px,4vw,34px)] font-bold'>三步上手</h2>
            <p className='text-muted-foreground mt-2.5'>
              从注册到调用，几分钟搞定
            </p>
          </div>
          <div className='grid gap-6 sm:grid-cols-3'>
            {STEPS.map((s) => (
              <div
                key={s.num}
                className='border-border bg-card rounded-2xl border p-7 shadow-sm'
              >
                <div className='text-primary mb-3 text-[42px] leading-none font-extrabold'>
                  {s.num}
                </div>
                <h4 className='mb-2 text-[17px] font-semibold'>{s.t}</h4>
                <p className='text-muted-foreground text-sm'>{s.p}</p>
              </div>
            ))}
          </div>
          <div className='border-border bg-muted/40 mt-7 overflow-x-auto rounded-2xl border p-5 font-mono text-[13px] leading-[1.9] shadow-sm backdrop-blur-sm'>
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
        <div className='border-primary/20 bg-primary/10 my-[50px] rounded-3xl border px-8 py-[58px] text-center'>
          <h2 className='text-[clamp(23px,4vw,32px)] font-bold'>
            选择我们，不是选最便宜的，而是选最省心的
          </h2>
          <p className='text-muted-foreground my-[14px] italic'>
            稳定是承诺，退款是底气，平衡是态度。
          </p>
          <Button
            className='star-glow bg-primary h-[51px] rounded-2xl px-8 text-[15px] font-semibold shadow-lg hover:-translate-y-0.5 hover:shadow-xl'
            render={<Link to='/dashboard' />}
          >
            进入控制台 <ArrowRight className='ml-1 size-4' />
          </Button>
        </div>
      </div>
    </div>
  )
}
