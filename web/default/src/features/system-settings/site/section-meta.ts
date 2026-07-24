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
export const SITE_SECTION_IDS = [
  'system-info',
  'notice',
  'header-navigation',
  'sidebar-modules',
] as const

export type SiteSectionId = (typeof SITE_SECTION_IDS)[number]
export const SITE_DEFAULT_SECTION: SiteSectionId = 'system-info'
export const SITE_SECTION_TITLES = {
  'system-info': 'System Information',
  notice: 'System Notice',
  'header-navigation': 'Header navigation',
  'sidebar-modules': 'Sidebar modules',
} satisfies Record<SiteSectionId, string>
