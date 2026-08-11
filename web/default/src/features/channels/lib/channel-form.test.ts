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
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { describe, test } from 'node:test'

const read = (relativePath: string) =>
  readFileSync(new URL(relativePath, import.meta.url), 'utf8')

describe('channel form financial boundaries', () => {
  test('does not expose or write the retired channel cost control', () => {
    const defaultForm = read('./channel-form.ts')
    const defaultDrawer = read(
      '../components/drawers/channel-mutate-drawer.tsx'
    )
    const defaultTagDialog = read('../components/dialogs/edit-tag-dialog.tsx')
    const classicForm = read(
      '../../../../../classic/src/components/table/channels/modals/EditChannelModal.jsx'
    )

    for (const source of [defaultForm, defaultDrawer, defaultTagDialog]) {
      assert.doesNotMatch(source, /cost_ratio|渠道成本倍率|成本倍率/)
    }
    assert.doesNotMatch(
      classicForm,
      /cost_ratio\b|渠道成本倍率|localInputs\.cost_ratio_ppm\s*=/
    )
  })
})
