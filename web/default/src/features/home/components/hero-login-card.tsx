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
import { useState } from 'react'
import { Link } from '@tanstack/react-router'
import { ArrowRight, Sparkles } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { useStatus } from '@/hooks/use-status'
import { Button } from '@/components/ui/button'
import { UserAuthForm } from '@/features/auth/sign-in/components/user-auth-form'
import { SignUpForm } from '@/features/auth/sign-up/components/sign-up-form'

// 登录卡（浅色 + 深色边框，参考 apito.ai/login）：
// 顶部「登录 / 注册」Tab 原地切换，不跳转新页面。
// 登录态渲染 UserAuthForm，注册态渲染 SignUpForm —— 两者均为既有组件，直接复用。
type AuthMode = 'sign-in' | 'sign-up'

interface HeroLoginCardProps {
  isAuthenticated?: boolean
  redirectTo?: string
}

// 浅色卡片样式：半透明白底 + 模糊 + 深色边框勾勒轮廓 + 柔和投影
const cardClass =
  'w-full max-w-[420px] rounded-[20px] border border-foreground/15 bg-card/95 p-7 shadow-[0_24px_70px_-20px_rgba(0,0,0,0.18)] backdrop-blur-xl sm:p-8'

export function HeroLoginCard({
  isAuthenticated,
  redirectTo,
}: HeroLoginCardProps) {
  const { t } = useTranslation()
  const [mode, setMode] = useState<AuthMode>('sign-in')
  const { status } = useStatus()

  // 注册开关：self_use 模式或后台关闭注册时，不显示注册 Tab
  const registerEnabled =
    status?.register_enabled !== false && !status?.self_use_mode_enabled

  // 已登录：显示「进入控制台」引导卡
  if (isAuthenticated) {
    return (
      <div className={cardClass}>
        <div className='space-y-5 text-center'>
          <div className='bg-primary/15 ring-primary/30 mx-auto flex size-12 items-center justify-center rounded-full ring-1'>
            <Sparkles className='text-primary size-5' />
          </div>
          <div className='space-y-1.5'>
            <h2 className='text-[22px] font-semibold tracking-tight'>
              {t('Welcome back')}
            </h2>
            <p className='text-muted-foreground text-sm'>
              {t('Continue to your dashboard')}
            </p>
          </div>
          <Button
            className='h-11 w-full justify-center gap-2 rounded-lg'
            render={<Link to='/dashboard' />}
          >
            {t('Go to Dashboard')}
            <ArrowRight className='size-4' />
          </Button>
        </div>
      </div>
    )
  }

  return (
    <div className={cardClass}>
      {/* 标题（随模式切换）*/}
      <div className='mb-5'>
        <h2 className='text-[20px] font-semibold tracking-tight'>
          {mode === 'sign-in' ? '登录' : '注册'}
        </h2>
        <p className='text-muted-foreground mt-0.5 text-[13px]'>
          {mode === 'sign-in'
            ? '最稳定的大模型中转站 · 一个 Key 通吃'
            : '几分钟创建账户，开始调用'}
        </p>
      </div>

      {/* 登录 / 注册 Tab（注册关闭时不显示）*/}
      {registerEnabled && (
        <div className='bg-muted mb-5 flex gap-1 rounded-xl p-1'>
          <button
            type='button'
            onClick={() => setMode('sign-in')}
            aria-pressed={mode === 'sign-in'}
            className={cn(
              'flex-1 cursor-pointer rounded-lg py-2 text-sm font-medium transition-all duration-200',
              mode === 'sign-in'
                ? 'bg-card text-foreground shadow-sm'
                : 'text-muted-foreground hover:text-foreground'
            )}
          >
            {t('Sign in')}
          </button>
          <button
            type='button'
            onClick={() => setMode('sign-up')}
            aria-pressed={mode === 'sign-up'}
            className={cn(
              'flex-1 cursor-pointer rounded-lg py-2 text-sm font-medium transition-all duration-200',
              mode === 'sign-up'
                ? 'bg-card text-foreground shadow-sm'
                : 'text-muted-foreground hover:text-foreground'
            )}
          >
            {t('Sign up')}
          </button>
        </div>
      )}

      {/* 表单（key 触发切换时重新挂载，触发淡入）*/}
      <div
        key={mode}
        className='landing-animate-fade-up opacity-0'
        style={{ animationDelay: '0ms' }}
      >
        {mode === 'sign-in' ? (
          <UserAuthForm redirectTo={redirectTo} />
        ) : (
          <SignUpForm />
        )}
      </div>
    </div>
  )
}
