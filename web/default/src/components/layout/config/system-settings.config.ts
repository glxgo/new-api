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
import { type TFunction } from 'i18next'
import {
  Box,
  CreditCard,
  Layout,
  Settings,
  Shield,
  ShieldAlert,
  Wrench,
} from 'lucide-react'
import {
  AUTH_SECTION_IDS,
  AUTH_SECTION_TITLES,
} from '@/features/system-settings/auth/section-meta'
import {
  BILLING_SECTION_IDS,
  BILLING_SECTION_TITLES,
} from '@/features/system-settings/billing/section-meta'
import {
  CONTENT_SECTION_IDS,
  CONTENT_SECTION_TITLES,
} from '@/features/system-settings/content/section-meta'
import {
  MODELS_SECTION_IDS,
  MODELS_SECTION_TITLES,
} from '@/features/system-settings/models/section-meta'
import {
  OPERATIONS_SECTION_IDS,
  OPERATIONS_SECTION_TITLES,
} from '@/features/system-settings/operations/section-meta'
import {
  SECURITY_SECTION_IDS,
  SECURITY_SECTION_TITLES,
} from '@/features/system-settings/security/section-meta'
import {
  SITE_SECTION_IDS,
  SITE_SECTION_TITLES,
} from '@/features/system-settings/site/section-meta'
import type { NavGroup, SidebarView } from '../types'

function getSectionNavItems<TSectionId extends string>(
  t: TFunction,
  basePath: string,
  sectionIds: readonly TSectionId[],
  sectionTitles: Record<TSectionId, string>
) {
  return sectionIds.map((sectionId) => ({
    title: t(sectionTitles[sectionId]),
    url: `${basePath}/${sectionId}`,
  }))
}

/**
 * Sidebar nav groups for the System Settings nested view.
 *
 * Kept as a single group because the workspace title in the sidebar
 * header already provides top-level context — the inner group label
 * scopes the items as "administration" actions.
 */
function getSystemSettingsNavGroups(t: TFunction): NavGroup[] {
  return [
    {
      id: 'system-administration',
      title: t('System Administration'),
      items: [
        {
          title: t('Site & Branding'),
          icon: Settings,
          items: getSectionNavItems(
            t,
            '/system-settings/site',
            SITE_SECTION_IDS,
            SITE_SECTION_TITLES
          ),
        },
        {
          title: t('Authentication'),
          icon: Shield,
          items: getSectionNavItems(
            t,
            '/system-settings/auth',
            AUTH_SECTION_IDS,
            AUTH_SECTION_TITLES
          ),
        },
        {
          title: t('Billing & Payment'),
          icon: CreditCard,
          items: getSectionNavItems(
            t,
            '/system-settings/billing',
            BILLING_SECTION_IDS,
            BILLING_SECTION_TITLES
          ),
        },
        {
          title: t('Models & Routing'),
          icon: Box,
          items: getSectionNavItems(
            t,
            '/system-settings/models',
            MODELS_SECTION_IDS,
            MODELS_SECTION_TITLES
          ),
        },
        {
          title: t('Security & Limits'),
          icon: ShieldAlert,
          items: getSectionNavItems(
            t,
            '/system-settings/security',
            SECURITY_SECTION_IDS,
            SECURITY_SECTION_TITLES
          ),
        },
        {
          title: t('Console Content'),
          icon: Layout,
          items: getSectionNavItems(
            t,
            '/system-settings/content',
            CONTENT_SECTION_IDS,
            CONTENT_SECTION_TITLES
          ),
        },
        {
          title: t('Operations'),
          icon: Wrench,
          items: getSectionNavItems(
            t,
            '/system-settings/operations',
            OPERATIONS_SECTION_IDS,
            OPERATIONS_SECTION_TITLES
          ),
        },
      ],
    },
  ]
}

/**
 * Nested sidebar view for `/system-settings/*`.
 *
 * Activates the Vercel / Cloudflare-style drill-in sidebar:
 * the root navigation is replaced by the system administration
 * groups, with a "Back to Dashboard" affordance in the header.
 */
export const SYSTEM_SETTINGS_VIEW: SidebarView = {
  id: 'system-settings',
  pathPattern: /^\/system-settings(\/|$)/,
  parent: {
    to: '/dashboard/overview',
    label: 'Back to Dashboard',
  },
  getNavGroups: getSystemSettingsNavGroups,
}
