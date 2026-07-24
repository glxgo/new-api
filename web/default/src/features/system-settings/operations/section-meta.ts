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
export const OPERATIONS_SECTION_IDS = [
  'behavior',
  'monitoring',
  'email',
  'worker',
  'logs',
  'performance',
  'update-checker',
  'tutorial',
  'top-nav-order',
] as const

export type OperationsSectionId = (typeof OPERATIONS_SECTION_IDS)[number]
export const OPERATIONS_DEFAULT_SECTION: OperationsSectionId = 'behavior'
export const OPERATIONS_SECTION_TITLES = {
  behavior: 'System Behavior',
  monitoring: 'Monitoring & Alerts',
  email: 'SMTP Email',
  worker: 'Worker Proxy',
  logs: 'Log Maintenance',
  performance: 'Performance',
  'update-checker': 'System maintenance',
  tutorial: 'Usage Tutorial',
  'top-nav-order': 'Top Navigation Order',
} satisfies Record<OperationsSectionId, string>
