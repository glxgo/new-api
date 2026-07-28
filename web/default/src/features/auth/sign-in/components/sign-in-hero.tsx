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
import { ArrowLeft } from 'lucide-react'
import { HeroLoginCard } from '@/features/home/components/hero-login-card'
import { Aurora } from '@/features/home/components/reactbits/aurora'
import { LightRays } from '@/features/home/components/reactbits/lightrays'

// 登录页主视觉：左栏品牌文案 + 右栏登录卡（HeroLoginCard）。
// 背景叠 Aurora 极光 + LightRays 暖色光束，与首页氛围一致。
interface SignInHeroProps {
  redirectTo?: string
  isAuthenticated?: boolean
}

const HIGHLIGHTS = [
  { ico: '🛡️', title: '稳定优先', desc: '不超售不滥用，高峰不掉线' },
  { ico: '💰', title: '余额可退', desc: '没用完的余额支持退款' },
  { ico: '⚖️', title: '公道定价', desc: '稳定前提下给公道价' },
] as const

export function SignInHero({ redirectTo, isAuthenticated }: SignInHeroProps) {
  return (
    <div
      className='relative isolate'
      data-theme-preset='stellaisle'
      /* 局部强制 stellaisle 调性 */
    >
      {/* ── 背景层：Aurora 极光 + LightRays 暖色光束 ── */}
      <div
        aria-hidden
        className='pointer-events-none fixed inset-0 z-0 overflow-hidden'
      >
        <div className='absolute inset-0 opacity-[0.18] dark:opacity-[0.13]'>
          <Aurora
            colorStops={['#E8916C', '#7AD9B8', '#FFE5A0']}
            amplitude={0.8}
            blend={0.6}
          />
        </div>
        <div className='absolute inset-0 opacity-[0.2] dark:opacity-[0.14]'>
          <LightRays
            raysOrigin='top-center'
            raysColor='#ffedb6'
            raysSpeed={1.2}
            lightSpread={0.85}
            rayLength={1.3}
            pulsating
            followMouse
            mouseInfluence={0.08}
            noiseAmount={0.1}
            distortion={0.04}
          />
        </div>
      </div>

      {/* ── 主内容：左文案 + 右登录卡 ── */}
      <main className='relative z-10 mx-auto flex min-h-svh max-w-[1180px] flex-col items-center gap-12 px-6 pt-28 pb-20 lg:flex-row lg:gap-10 lg:pt-32'>
        {/* 左栏：品牌文案 */}
        <div className='flex w-full flex-col items-start text-left lg:w-[56%]'>
          <Link
            to='/'
            className='landing-animate-fade-up text-muted-foreground hover:text-foreground mb-6 inline-flex items-center gap-1.5 text-sm opacity-0 transition-colors'
            style={{ animationDelay: '0ms' }}
          >
            <ArrowLeft className='size-4' />
            返回首页
          </Link>

          <span
            className='landing-animate-fade-up mb-6 inline-flex items-center gap-2 rounded-full border border-[#008AB7]/20 bg-[#008AB7]/10 px-3.5 py-1.5 text-xs font-semibold tracking-[1.5px] text-[#008AB7] opacity-0'
            style={{ animationDelay: '80ms' }}
          >
            <span className='size-1.5 rounded-full bg-[#008AB7]' />
            STABLE FIRST · 稳定优先
          </span>

          <h1
            className='landing-animate-fade-up text-[clamp(36px,5.5vw,58px)] leading-[1.12] font-extrabold tracking-tight opacity-0'
            style={{ animationDelay: '160ms' }}
          >
            稳到让你忘了它存在的
            <br />
            <span className='text-[#008AB7]'>Star API 中转站</span>
          </h1>

          <p
            className='landing-animate-fade-up text-muted-foreground mt-5 max-w-[500px] text-[clamp(15px,2vw,18px)] leading-relaxed opacity-0'
            style={{ animationDelay: '240ms' }}
          >
            登录控制台，管理你的令牌、用量与余额。
            <br />
            一个 Key，通吃 GPT / Claude / Gemini 全系列。
          </p>

          {/* 卖点引子 */}
          <div
            className='landing-animate-fade-up mt-10 grid w-full max-w-[500px] gap-3 opacity-0 sm:grid-cols-3 sm:gap-4'
            style={{ animationDelay: '320ms' }}
          >
            {HIGHLIGHTS.map((h) => (
              <div
                key={h.title}
                className='border-border/60 bg-card/50 rounded-xl border p-3 backdrop-blur-sm'
              >
                <div className='mb-1 text-base'>{h.ico}</div>
                <div className='text-[13px] font-semibold'>{h.title}</div>
                <div className='text-muted-foreground text-[11.5px] leading-snug'>
                  {h.desc}
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* 右栏：登录卡 */}
        <div
          className='landing-animate-fade-up flex w-full justify-center opacity-0 lg:w-[44%]'
          style={{ animationDelay: '400ms' }}
        >
          <HeroLoginCard
            isAuthenticated={isAuthenticated}
            redirectTo={redirectTo}
          />
        </div>
      </main>
    </div>
  )
}
