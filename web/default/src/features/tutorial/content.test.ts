import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import { getSiteGuide } from './content.ts'

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

  test('labels administrator content as a supplement to the main tutorial', () => {
    const guide = getSiteGuide('本站专属说明')
    assert.equal(guide?.title, '本站使用教程（补充）')
    assert.match(guide?.subtitle || '', /补充步骤/)
  })

  test('does not create an empty site guide', () => {
    assert.equal(getSiteGuide('   '), null)
  })
})
