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
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth-store'
import { parseHeaderNavModulesFromStatus } from '@/lib/nav-modules'
import { useStatus } from '@/hooks/use-status'

export type TopNavLink = {
  title: string
  href: string
  disabled?: boolean
  requiresAuth?: boolean
  external?: boolean
  // internal: 用于后台排序配置(TopNavOrder option), 不参与渲染
  navKey?: string
}

/**
 * Generate top navigation links based on HeaderNavModules configuration from backend /api/status,
 * then reorder them by the super-admin-configured TopNavOrder option (JSON array of navKey).
 */
export function useTopNavLinks(): TopNavLink[] {
  const { t } = useTranslation()
  const { status } = useStatus()
  const { auth } = useAuthStore()

  // Parse HeaderNavModules
  const modules = useMemo(() => {
    return parseHeaderNavModulesFromStatus(
      status as Record<string, unknown> | null
    )
  }, [status])

  // Documentation link (may be external)
  const docsLink: string | undefined = status?.docs_link as string | undefined

  const isAuthed = !!auth?.user

  // Build links in default order, each carrying a stable navKey for ordering.
  const links = useMemo<TopNavLink[]>(() => {
    const arr: TopNavLink[] = []

    if (modules?.home !== false) {
      arr.push({ navKey: 'home', title: t('Home'), href: '/' })
    }

    if (modules?.console !== false) {
      arr.push({ navKey: 'console', title: t('Console'), href: '/dashboard' })
    }

    const pricing = modules?.pricing
    if (pricing && typeof pricing === 'object' && pricing.enabled) {
      const requiresAuth = pricing.requireAuth && !isAuthed
      arr.push({
        navKey: 'pricing',
        title: t('Model Square'),
        href: '/pricing',
        requiresAuth,
      })
    }

    const rankings = modules?.rankings
    if (rankings && typeof rankings === 'object' && rankings.enabled) {
      const requiresAuth = rankings.requireAuth && !isAuthed
      arr.push({
        navKey: 'rankings',
        title: t('Rankings'),
        href: '/rankings',
        requiresAuth,
      })
    }

    if (modules?.docs !== false) {
      if (docsLink) {
        arr.push({
          navKey: 'docs',
          title: t('Docs'),
          href: docsLink,
          external: true,
        })
      } else {
        arr.push({ navKey: 'docs', title: t('Docs'), href: '/docs' })
      }
    }

    if (modules?.about !== false) {
      arr.push({ navKey: 'about', title: t('About'), href: '/about' })
    }

    // Tutorial (使用教程, 后台 Markdown 编辑)
    arr.push({ navKey: 'tutorial', title: t('Tutorial'), href: '/tutorial' })

    // FAQ (常见问题, 从概览迁出)
    arr.push({ navKey: 'faq', title: t('FAQ'), href: '/faq' })

    return arr
  }, [t, modules, docsLink, isAuthed])

  // Reorder by super-admin-configured TopNavOrder (JSON array of navKey from /api/status).
  return useMemo(() => {
    const orderRaw = (status as Record<string, unknown> | null)?.top_nav_order
    let order: string[] = []
    if (typeof orderRaw === 'string' && orderRaw.trim()) {
      try {
        const parsed = JSON.parse(orderRaw)
        if (Array.isArray(parsed)) {
          order = parsed.filter((x) => typeof x === 'string')
        }
      } catch {
        // empty / invalid config → keep default order
      }
    }
    if (order.length === 0) return links

    const idx = new Map<string, number>()
    order.forEach((k, i) => idx.set(k, i))
    return [...links].sort((a, b) => {
      const ia =
        a.navKey && idx.has(a.navKey) ? (idx.get(a.navKey) as number) : 9999
      const ib =
        b.navKey && idx.has(b.navKey) ? (idx.get(b.navKey) as number) : 9999
      return ia - ib
    })
  }, [links, status])
}
