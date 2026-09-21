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
import { useAuthStore } from '@/stores/auth-store'
import { getValidatedSelf } from '@/lib/auth-query'
import { AuthenticatedLayout } from '@/components/layout'

export const Route = createFileRoute('/_authenticated')({
  beforeLoad: async ({ location, context, cause }) => {
    const { auth } = useAuthStore.getState()

    // 如果本地没有用户信息，直接跳转登录页
    if (!auth.user) {
      throw redirect({
        to: '/sign-in',
        search: { redirect: location.href },
      })
    }

    // 首次进入/重新登录强制验证；同一应用内翻页和筛选短暂复用，
    // 避免每次查询都先等待一次身份接口。缓存不持久化到浏览器存储。
    const res = await getValidatedSelf(
      context.queryClient,
      auth.user.id,
      cause === 'enter'
    ).catch(() => null)
    if (!res?.success || !res.data || !useAuthStore.getState().auth.user) {
      // 验证失败或 API 调用失败，清除本地缓存并跳转登录页
      auth.reset()
      throw redirect({
        to: '/sign-in',
        search: { redirect: location.href },
      })
    }
  },
  component: AuthenticatedLayout,
})
