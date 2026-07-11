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
import { createFileRoute, redirect } from '@tanstack/react-router'
import { AuthenticatedLayout } from '@/components/layout'
import { getSelf } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

const VERIFIED_USER_KEY = 'new-api:verified-user-id'

function getVerifiedUserId() {
  if (typeof window === 'undefined') return null
  return window.sessionStorage.getItem(VERIFIED_USER_KEY)
}

function setVerifiedUserId(userId: number) {
  if (typeof window === 'undefined') return
  window.sessionStorage.setItem(VERIFIED_USER_KEY, String(userId))
}

function clearVerifiedUserId() {
  if (typeof window === 'undefined') return
  window.sessionStorage.removeItem(VERIFIED_USER_KEY)
}

export const Route = createFileRoute('/_authenticated')({
  beforeLoad: async ({ location }) => {
    const { auth } = useAuthStore.getState()

    // 如果本地没有用户信息，直接跳转登录页
    if (!auth.user) {
      throw redirect({
        to: '/sign-in',
        search: { redirect: location.href },
      })
    }

    // 本地有用户信息，但需要按当前用户验证 session 是否有效。
    // 不使用模块级变量，避免退出/切号/热更新后复用旧验证状态。
    if (getVerifiedUserId() !== String(auth.user.id)) {
      const res = await getSelf().catch(() => null)
      if (res?.success && res.data) {
        // 验证成功，更新用户信息（可能有变化）
        auth.setUser(res.data)
        setVerifiedUserId(res.data.id)
      } else {
        // 验证失败或 API 调用失败，清除本地缓存并跳转登录页
        clearVerifiedUserId()
        auth.reset()
        throw redirect({
          to: '/sign-in',
          search: { redirect: location.href },
        })
      }
    }
  },
  component: AuthenticatedLayout,
})
