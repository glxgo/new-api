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
import { describe, test } from 'node:test'
import {
  getSiteGuide,
  getSiteGuideSections,
  groupSiteGuideSections,
  resolveTutorialApiBaseUrl,
} from './content.ts'

describe('resolveTutorialApiBaseUrl', () => {
  test('uses the public server address instead of the local preview origin', () => {
    assert.equal(
      resolveTutorialApiBaseUrl(
        'https://token.stellaisle.com/',
        'http://localhost:3001'
      ),
      'https://token.stellaisle.com'
    )
  })

  test('falls back to the real public API address during local preview', () => {
    assert.equal(
      resolveTutorialApiBaseUrl('', 'http://localhost:3001'),
      'https://token.stellaisle.com'
    )
  })
})

describe('getSiteGuide', () => {
  test('keeps tutorial markdown that has no level-two headings', () => {
    const content =
      '先创建 API Key，然后复制到客户端。\n\n![示例](/upload/demo.png)'
    assert.equal(getSiteGuide(content)?.content, content)
  })

  test('keeps the complete original markdown instead of splitting it into cards', () => {
    const content = '# 我的教程\n\n开场说明\n\n## 第一步\n\n正文'
    assert.equal(getSiteGuide(content)?.content, content)
  })

  test('does not create an empty site guide', () => {
    assert.equal(getSiteGuide('   '), null)
  })
})

describe('getSiteGuideSections', () => {
  test('places the seven site sections into related documentation pages', () => {
    const content = [
      '## 1. 登录控制台\n登录正文',
      '## 2. 准备余额或套餐\n余额正文',
      '## 3. 创建 API Key\n密钥正文',
      '## 4. 选择可用模型\n模型正文',
      '## 5. 手动配置到客户端\n手动正文',
      '## 6自动配置到客户端\n自动正文',
      '## 7. 查看用量和消耗\n用量正文',
    ].join('\n\n')
    const grouped = groupSiteGuideSections(getSiteGuideSections(content))

    assert.deepEqual(
      Object.fromEntries(
        Object.entries(grouped).map(([placement, sections]) => [
          placement,
          sections.map((section) => section.title),
        ])
      ),
      {
        'quick-start': ['1. 登录控制台'],
        'api-key': ['3. 创建 API Key'],
        'model-selection': ['4. 选择可用模型'],
        'client-import': ['5. 手动配置到客户端', '6自动配置到客户端'],
        balance: ['2. 准备余额或套餐'],
        usage: ['7. 查看用量和消耗'],
        other: [],
      }
    )
  })

  test('keeps unsectioned content in a fallback supplement module', () => {
    const content = '先创建 API Key。\n\n然后配置客户端。'
    const sections = getSiteGuideSections(content)
    assert.equal(sections.length, 1)
    assert.equal(sections[0].placement, 'other')
    assert.equal(sections[0].content, content)
  })

  test('keeps preamble and every section body', () => {
    const content =
      '开场说明\n\n## 登录账号\n登录正文\n\n## 未分类提醒\n提醒正文'
    const sections = getSiteGuideSections(content)
    assert.deepEqual(
      sections.map((section) => section.content),
      ['开场说明', '登录正文', '提醒正文']
    )
  })
})
