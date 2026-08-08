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
import {
  BarChart3,
  Box,
  CreditCard,
  Crown,
  FileText,
  FlaskConical,
  Gift,
  HandCoins,
  ChartLine,
  Key,
  LayoutDashboard,
  Link2,
  ListTodo,
  MessageSquare,
  Activity,
  Radio,
  Settings,
  Ticket,
  User,
  Users,
  Wallet,
  Dices,
  Sparkles,
  Bell,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth-store'
import { ROLE } from '@/lib/roles'
import { type NavItem, type SidebarData } from '@/components/layout/types'

/**
 * Root navigation groups for the application sidebar.
 *
 * These are shown when the URL does not match any nested sidebar view
 * registered in `layout/lib/sidebar-view-registry.ts`.
 */
export function useSidebarData(): SidebarData {
  const { t } = useTranslation()
  const { auth } = useAuthStore()
  const isAdmin = (auth.user?.role ?? 0) >= ROLE.ADMIN
  const isAgent = (auth.user?.role ?? 0) === ROLE.AGENT
  const isRoot = (auth.user?.role ?? 0) >= ROLE.SUPER_ADMIN

  // Withdrawable commission/dividend account (agent and admin+).
  const personalItems: NavItem[] = [
    {
      title: '公告',
      url: '/announcements',
      icon: Bell,
    },
    {
      title: t('Wallet'),
      url: '/wallet',
      icon: Wallet,
    },
    {
      title: '虚拟会员',
      url: '/virtual-membership',
      icon: Sparkles,
    },
    {
      title: t('Subscription Plans'),
      url: '/subscription-plans',
      icon: CreditCard,
    },
    {
      title: t('Affiliate Program'),
      url: '/affiliate',
      icon: Gift,
    },
    {
      title: t('Profile'),
      url: '/profile',
      icon: User,
    },
  ]
  if (isAgent || isAdmin) {
    personalItems.push({
      title: isAgent ? t('Commission Account') : t('Dividend Account'),
      url: '/dividend',
      icon: Crown,
    })
  }

  // Profit dashboard + withdrawal review (super-admin only).
  const adminItems: NavItem[] = [
    {
      title: t('Channels'),
      url: '/channels',
      icon: Radio,
    },
    {
      title: t('Models'),
      url: '/models/metadata',
      icon: Box,
    },
    {
      title: t('Users'),
      url: '/users',
      icon: Users,
    },
    {
      title: t('Redemption Codes'),
      url: '/redemption-codes',
      icon: Ticket,
    },
    {
      title: t('Subscription Management'),
      url: '/subscriptions',
      icon: CreditCard,
    },
    {
      title: '虚拟会员管理',
      url: '/virtual-memberships',
      icon: Sparkles,
    },
    {
      title: 'API 入口与倍率',
      url: '/api-ingress',
      icon: Link2,
    },
    {
      title: '充值优惠码',
      url: '/topup-coupons',
      icon: Ticket,
    },
    {
      title: t('System Settings'),
      url: '/system-settings/site',
      activeUrls: ['/system-settings'],
      icon: Settings,
    },
  ]
  if (isRoot) {
    adminItems.push(
      {
        title: t('Withdrawal Review'),
        url: '/withdraw-review',
        icon: HandCoins,
      },
      {
        title: t('Profit Dashboard'),
        url: '/profit',
        icon: BarChart3,
      },
      {
        title: '幸运大转盘管理',
        url: '/lucky-wheel-admin',
        icon: Dices,
      }
    )
  }

  return {
    navGroups: [
      {
        id: 'chat',
        title: t('Chat'),
        items: [
          {
            title: t('Playground'),
            url: '/playground',
            icon: FlaskConical,
          },
          {
            title: t('Chat'),
            icon: MessageSquare,
            type: 'chat-presets',
          },
        ],
      },
      {
        id: 'general',
        title: t('General'),
        items: [
          {
            title: t('Dashboard'),
            url: '/dashboard/models',
            icon: LayoutDashboard,
          },
          {
            title: t('API Keys'),
            url: '/keys',
            icon: Key,
          },
          {
            title: t('Usage Statistics'),
            url: '/usage-statistics',
            icon: ChartLine,
          },
          {
            title: t('Usage Logs'),
            url: '/usage-logs/common',
            icon: FileText,
          },
          {
            title: t('Model Status'),
            url: '/model-status',
            icon: Activity,
          },
          {
            title: t('Task Logs'),
            url: '/usage-logs/task',
            activeUrls: ['/usage-logs/drawing'],
            configUrls: ['/usage-logs/drawing', '/usage-logs/task'],
            icon: ListTodo,
          },
        ],
      },
      {
        id: 'activities',
        title: t('Activities'),
        items: [
          {
            title: t('Lucky Wheel'),
            url: '/lucky-wheel',
            icon: Dices,
          },
        ],
      },
      {
        id: 'personal',
        title: t('Personal'),
        items: personalItems,
      },
      {
        id: 'admin',
        title: t('Admin'),
        items: adminItems,
      },
    ],
  }
}
