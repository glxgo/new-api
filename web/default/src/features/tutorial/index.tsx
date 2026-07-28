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
  useEffect,
  useMemo,
  useState,
  type ElementType,
  type ReactNode,
} from 'react'
import {
  BadgeCheck,
  BookOpen,
  CheckCircle2,
  ChevronDown,
  Copy,
  Grid2X2,
  LifeBuoy,
  LockKeyhole,
  Menu,
  ReceiptText,
  SearchCheck,
  ShieldCheck,
  TerminalSquare,
  WalletCards,
  WifiOff,
} from 'lucide-react'
import { toast } from 'sonner'
import { useStatus } from '@/hooks/use-status'
import { PublicLayout } from '@/components/layout'
import { resolveTutorialApiBaseUrl } from './content'

type DocPage = {
  id: string
  title: string
  subtitle: string
  description?: string
  content: ReactNode
}

type DocCategory = {
  id: string
  title: string
  icon: ElementType
  pages: DocPage[]
}

type Selection = { categoryId: string; pageId: string }

function readSelection(): Selection | null {
  if (typeof window === 'undefined') return null
  const params = new URLSearchParams(window.location.search)
  const categoryId = params.get('cat')
  const pageId = params.get('page')
  if (pageId === 'site-guide') {
    return { categoryId: 'tutorial', pageId: 'quick-start' }
  }
  return categoryId && pageId ? { categoryId, pageId } : null
}

function InlineCode({ children }: { children: ReactNode }) {
  return (
    <code className='border-border bg-muted text-foreground rounded border px-1.5 py-0.5 text-[0.9em]'>
      {children}
    </code>
  )
}

function CodeBlock({ value }: { value: string }) {
  return (
    <div className='bg-foreground text-background group relative overflow-hidden rounded-xl shadow-sm'>
      <button
        type='button'
        onClick={async () => {
          await navigator.clipboard.writeText(value)
          toast.success('命令已复制')
        }}
        className='border-background/20 bg-background/10 text-background/70 hover:bg-background/20 hover:text-background absolute top-3 right-3 flex items-center gap-1.5 rounded-md border px-2 py-1 text-[10px] tracking-[0.16em] transition'
        aria-label='复制命令'
      >
        <Copy className='h-3 w-3' /> COPY
      </button>
      <pre className='overflow-x-auto px-5 py-5 pr-24 font-mono text-xs leading-6 sm:text-[13px]'>
        <code>{value}</code>
      </pre>
    </div>
  )
}

function ArticleSection({
  title,
  children,
}: {
  title: string
  children: ReactNode
}) {
  return (
    <section className='mt-9'>
      <h2 className='border-border text-foreground border-b pb-3 font-serif text-[22px] font-medium tracking-wide'>
        {title}
      </h2>
      <div className='text-muted-foreground mt-4 text-[15px] leading-8'>
        {children}
      </div>
    </section>
  )
}

function Callout({
  children,
  tone = 'neutral',
}: {
  children: ReactNode
  tone?: 'neutral' | 'warm'
}) {
  return (
    <div
      className={
        tone === 'warm'
          ? 'border-foreground bg-muted/70 text-foreground border-l-[3px] px-5 py-4 text-sm leading-7'
          : 'border-foreground bg-muted/40 text-foreground border-l-[3px] px-5 py-4 text-sm leading-7 font-medium'
      }
    >
      {children}
    </div>
  )
}

function GuideSteps({ items }: { items: ReactNode[] }) {
  return (
    <ol className='border-border divide-border mt-2 divide-y overflow-hidden rounded-xl border'>
      {items.map((item, index) => (
        <li key={index} className='flex gap-4 px-4 py-3.5 sm:px-5'>
          <span className='bg-foreground text-background mt-0.5 flex h-6 min-w-6 items-center justify-center rounded-full font-mono text-[10px] font-semibold'>
            {index + 1}
          </span>
          <span className='min-w-0 leading-7'>{item}</span>
        </li>
      ))}
    </ol>
  )
}

function VerificationGrid({
  items,
}: {
  items: { title: string; text: string }[]
}) {
  return (
    <div className='grid gap-3 sm:grid-cols-2'>
      {items.map((item, index) => (
        <div
          key={item.title}
          className='border-border bg-muted/20 rounded-xl border p-4'
        >
          <div className='text-foreground flex items-center gap-2 text-sm font-semibold'>
            <span className='border-border bg-background flex h-7 w-7 items-center justify-center rounded-lg border font-mono text-[10px]'>
              {String(index + 1).padStart(2, '0')}
            </span>
            {item.title}
          </div>
          <p className='mt-2 text-sm leading-6'>{item.text}</p>
        </div>
      ))}
    </div>
  )
}

function PrincipleGrid({
  items,
}: {
  items: { icon: ElementType; title: string; text: string }[]
}) {
  return (
    <div className='grid gap-3 sm:grid-cols-2'>
      {items.map((item) => {
        const Icon = item.icon
        return (
          <div
            key={item.title}
            className='border-border bg-muted/20 rounded-xl border p-4'
          >
            <div className='text-foreground flex items-center gap-2 text-sm font-semibold'>
              <span className='border-border bg-background flex h-8 w-8 items-center justify-center rounded-lg border'>
                <Icon className='h-4 w-4' aria-hidden='true' />
              </span>
              {item.title}
            </div>
            <p className='mt-2 text-sm leading-6'>{item.text}</p>
          </div>
        )
      })}
    </div>
  )
}

function buildCategories(endpoint: string): DocCategory[] {
  const quickStartCommand = [
    `curl ${endpoint}/v1/chat/completions \\`,
    '  -H "Authorization: Bearer YOUR_API_KEY" \\',
    '  -H "Content-Type: application/json" \\',
    '  -d \'{"model":"gpt-5.6-terra","messages":[{"role":"user","content":"你好"}],"stream":true}\'',
  ].join('\n')

  return [
    {
      id: 'tutorial',
      title: '使用教程',
      icon: BookOpen,
      pages: [
        {
          id: 'quick-start',
          title: '快速开始',
          subtitle: '5 分钟创建第一个 API Key 并发起请求',
          description:
            '本教程帮助你完成账号准备、API Key 创建和第一次模型调用。',
          content: (
            <>
              <Callout>
                本教程帮助你在 5 分钟内完成账号准备、API Key
                创建并发起第一次模型调用。
              </Callout>
              <ArticleSection title='第 1 步 · 登录账号'>
                <GuideSteps
                  items={[
                    <span key='open'>
                      打开首页右上角的 <InlineCode>登录 / 注册</InlineCode>。
                    </span>,
                    '输入账号信息并完成登录，随后进入控制台。',
                    '如果访问密钥、用量等页面时被送回登录页，说明登录状态已过期，重新登录即可。',
                  ]}
                />
              </ArticleSection>
              <ArticleSection title='第 2 步 · 准备额度或套餐'>
                在控制台 <InlineCode>钱包</InlineCode>{' '}
                查看本金、赠金、套餐有效期和支持分组。需要充值或购买套餐时按页面提示操作，并在
                <InlineCode>订单记录</InlineCode>
                中核对支付状态；已有可用额度时可以直接继续。
              </ArticleSection>
              <ArticleSection title='第 3 步 · 创建 API Key'>
                <GuideSteps
                  items={[
                    <span key='create'>
                      进入 <InlineCode>API 密钥</InlineCode>，点击{' '}
                      <InlineCode>创建 API 密钥</InlineCode>。
                    </span>,
                    '填写便于识别的名称，并选择与你的余额或套餐匹配的分组。',
                    '创建后立即复制并安全保存密钥；不要把完整密钥放进截图、聊天记录或公开仓库。',
                  ]}
                />
              </ArticleSection>
              <ArticleSection title='第 4 步 · 发起请求'>
                <CodeBlock value={quickStartCommand} />
              </ArticleSection>
              <div className='mt-4'>
                <Callout tone='warm'>
                  调用记录会在 <strong>使用日志</strong>{' '}
                  中展示。排查问题时请保留请求
                  ID、时间和模型名称，不要发送完整密钥。
                </Callout>
              </div>
            </>
          ),
        },
        {
          id: 'api-key',
          title: 'API Key 管理',
          subtitle: '为不同设备和成员拆分密钥，降低泄露影响',
          content: (
            <>
              <Callout>
                API Key
                相当于账户密码。建议一个客户端或一个成员使用一把独立密钥。
              </Callout>
              <ArticleSection title='创建与命名'>
                <GuideSteps
                  items={[
                    <span key='open'>
                      打开 <InlineCode>API 密钥</InlineCode> 页面并创建新密钥。
                    </span>,
                    <span key='name'>
                      按设备或用途命名，例如{' '}
                      <InlineCode>MacBook-Codex</InlineCode>、
                      <InlineCode>团队-设计组</InlineCode>。
                    </span>,
                    '选择允许使用的分组和模型，然后复制密钥并保存到密码管理器或客户端安全存储中。',
                  ]}
                />
              </ArticleSection>
              <ArticleSection title='限制权限'>
                可按需限制模型、分组、额度和 IP。不要把完整密钥提交到
                Git、公开日志或截图中。
              </ArticleSection>
              <ArticleSection title='泄露后的处理'>
                立即禁用或删除受影响密钥，再为对应客户端创建新密钥；不需要更换其他独立密钥。
              </ArticleSection>
            </>
          ),
        },
        {
          id: 'model-selection',
          title: '选择可用模型',
          subtitle: '根据任务、分组与实时状态选择正确模型',
          content: (
            <>
              <Callout>
                模型名称、可用分组和倍率可能不同，配置客户端前应先以本站模型列表为准。
              </Callout>
              <ArticleSection title='先查看模型状态'>
                <GuideSteps
                  items={[
                    <span key='market'>
                      打开 <InlineCode>模型广场</InlineCode>
                      ，搜索准备使用的模型。
                    </span>,
                    <span key='status'>
                      到 <InlineCode>模型状态</InlineCode>{' '}
                      查看近期成功率、延迟和缓存表现。
                    </span>,
                    '确认 API Key 所在分组支持该模型，再把模型名称完整填写到客户端；不要自行缩写或改名。',
                  ]}
                />
              </ArticleSection>
              <ArticleSection title='模型暂时不可用时'>
                先检查账号额度与 API Key
                分组；如果模型或渠道正在维护，可选择同分组内其他可用模型。
              </ArticleSection>
            </>
          ),
        },
        {
          id: 'concurrency',
          title: '并发与限流',
          subtitle: '理解账号并发、渠道并发和 429 响应',
          content: (
            <>
              <Callout>
                并发表示同一时刻尚未完成的请求数，不等同于每分钟请求数。
              </Callout>
              <ArticleSection title='账号并发'>
                新账号默认并发为 <strong>8</strong>
                。可在 <InlineCode>数据看板</InlineCode>
                查看当前占用和上限，并填写使用场景与联系方式申请提升。
              </ArticleSection>
              <ArticleSection title='渠道并发'>
                某个渠道满载时，系统会在同组内寻找其他可用渠道。满载不会被记为渠道故障，也不会触发渠道禁用。
              </ArticleSection>
              <ArticleSection title='收到 429 时'>
                等待正在执行的请求完成后再试。不要立即并发重放大量请求，以免持续触发限流。
              </ArticleSection>
            </>
          ),
        },
      ],
    },
    {
      id: 'clients',
      title: '工具与客户端',
      icon: TerminalSquare,
      pages: [
        {
          id: 'client-import',
          title: 'CC Switch 与 Cherry Studio',
          subtitle: '手动填写端点，或从 API Key 页面快速导入',
          content: (
            <>
              <Callout>
                CC Switch 是跨平台的供应商配置管理工具。本站支持通过
                <InlineCode>ccswitch://</InlineCode> 深链把 Claude Code、Codex
                CLI 或 Gemini CLI 配置写入 CC Switch，无需手改配置文件。
              </Callout>
              <ArticleSection title='第 1 步 · 安装 CC Switch'>
                前往{' '}
                <a
                  href='https://github.com/farion1231/cc-switch/releases/latest'
                  target='_blank'
                  rel='noreferrer'
                  className='text-foreground underline underline-offset-4'
                >
                  CC Switch 官方发布页
                </a>{' '}
                下载 Windows、macOS 或 Linux
                安装包。安装后至少启动一次，让系统注册
                <InlineCode>ccswitch://</InlineCode>{' '}
                协议；旧版本无法唤起时，先升级到最新版。
              </ArticleSection>
              <ArticleSection title='第 2 步 · 从 API Key 页面一键导入'>
                <GuideSteps
                  items={[
                    <span key='menu'>
                      在 <InlineCode>API 密钥</InlineCode>{' '}
                      列表找到目标密钥，点击{' '}
                      <InlineCode>一键导入 CC Switch</InlineCode>。
                    </span>,
                    '选择 Claude、Codex 或 Gemini（分别对应 Claude Code、Codex CLI、Gemini CLI），修改供应商名称，并从当前账号可用模型中选择主模型；Claude 还可以分别填写 Haiku、Sonnet、Opus 槽位。',
                    <span key='open'>
                      点击 <InlineCode>打开 CC Switch</InlineCode>
                      ，允许浏览器唤起应用，在 CC Switch
                      确认导入并启用该供应商。
                    </span>,
                    '导入器会自动处理地址：Codex CLI 使用带 /v1 的地址，Claude Code 与 Gemini CLI 使用站点根地址，不需要手动增删路径。',
                  ]}
                />
              </ArticleSection>
              <ArticleSection title='手动配置 Cherry Studio 或其他客户端'>
                <GuideSteps
                  items={[
                    <span key='key'>
                      在 <InlineCode>API 密钥</InlineCode> 页面复制需要使用的
                      Key。
                    </span>,
                    '在客户端新增 OpenAI 兼容供应商。',
                    <span key='endpoint'>
                      Base URL 填写 <InlineCode>{endpoint}</InlineCode>
                      ；客户端明确要求 OpenAI API 根路径时填写{' '}
                      <InlineCode>{endpoint}/v1</InlineCode>。
                    </span>,
                    '填入 API Key，并从模型广场复制当前可用的完整模型名称。保存后发送一条简短消息验证。',
                  ]}
                />
              </ArticleSection>
              <ArticleSection title='常见问题'>
                <VerificationGrid
                  items={[
                    {
                      title: '点击后没有唤起',
                      text: '确认 CC Switch 已安装且启动过一次，并允许浏览器打开外部应用；仍无反应时升级 CC Switch。',
                    },
                    {
                      title: '导入后仍然连不上',
                      text: '检查供应商地址、API Key、模型名，并确认密钥分组包含该模型；再重启目标 CLI。',
                    },
                    {
                      title: '模型列表不正确',
                      text: '以本站模型广场和当前 API Key 分组为准，不要直接照搬其他站点文档中的模型名。',
                    },
                    {
                      title: '安全提醒',
                      text: '导入深链包含 API Key，不要复制、截图或公开转发完整链接，只允许可信本站页面唤起。',
                    },
                  ]}
                />
              </ArticleSection>
            </>
          ),
        },
        {
          id: 'codex-claude',
          title: 'Codex 与 Claude Code',
          subtitle: '配置常用命令行客户端的接口地址和密钥',
          content: (
            <>
              <Callout>修改环境变量后，请重新打开终端或重启客户端。</Callout>
              <ArticleSection title='Codex / OpenAI 兼容客户端'>
                <CodeBlock
                  value={`export OPENAI_BASE_URL="${endpoint}/v1"\nexport OPENAI_API_KEY="YOUR_API_KEY"`}
                />
              </ArticleSection>
              <ArticleSection title='Claude Code'>
                <CodeBlock
                  value={`export ANTHROPIC_BASE_URL="${endpoint}"\nexport ANTHROPIC_AUTH_TOKEN="YOUR_API_KEY"`}
                />
              </ArticleSection>
            </>
          ),
        },
        {
          id: 'sdk',
          title: 'SDK 通用配置',
          subtitle: '在 OpenAI 兼容 SDK 中替换 baseURL',
          content: (
            <>
              <Callout>
                大多数 OpenAI 兼容 SDK 只需要替换 API Key 与 baseURL。
              </Callout>
              <ArticleSection title='JavaScript 示例'>
                <CodeBlock
                  value={`import OpenAI from "openai";\n\nconst client = new OpenAI({\n  apiKey: process.env.OPENAI_API_KEY,\n  baseURL: "${endpoint}/v1",\n});`}
                />
              </ArticleSection>
            </>
          ),
        },
      ],
    },
    {
      id: 'billing',
      title: '额度与套餐',
      icon: WalletCards,
      pages: [
        {
          id: 'balance',
          title: '余额、赠金与套餐',
          subtitle: '理解不同资金池与套餐分组的使用方式',
          content: (
            <>
              <Callout>
                控制台可用余额会合计展示本金和赠金；赠金可以消费，但不能提现。
              </Callout>
              <ArticleSection title='本金与赠金'>
                本金与赠金都可以用于 API
                消费。提现及其他仅支持本金的操作不会把赠金当成本金处理。
              </ArticleSection>
              <ArticleSection title='订阅套餐'>
                套餐在有效期、额度和允许分组范围内生效。创建 API Key
                时应选择套餐支持的分组。
              </ArticleSection>
              <ArticleSection title='充值与订单核对'>
                <GuideSteps
                  items={[
                    <span key='wallet'>
                      进入 <InlineCode>钱包</InlineCode>
                      ，选择充值金额、套餐或当前可用的支付方式。
                    </span>,
                    <span key='order'>
                      支付完成后打开 <InlineCode>订单记录</InlineCode>
                      ，确认订单状态。
                    </span>,
                    '余额、赠金、套餐额度和有效期以控制台实时显示为准；发现异常时提供订单号联系管理员。',
                  ]}
                />
              </ArticleSection>
            </>
          ),
        },
        {
          id: 'usage',
          title: '用量与订单',
          subtitle: '核对请求消耗、套餐使用和支付状态',
          content: (
            <>
              <Callout>
                API
                消耗以控制台的使用日志和数据看板为准，支付结果以订单记录为准。
              </Callout>
              <ArticleSection title='查看 API 消耗'>
                <GuideSteps
                  items={[
                    <span key='dashboard'>
                      在 <InlineCode>数据看板</InlineCode>{' '}
                      查看请求量、Token、消费趋势和当前并发。
                    </span>,
                    <span key='logs'>
                      在 <InlineCode>使用日志</InlineCode> 按时间、模型和 API
                      Key 定位单次请求及其扣费。
                    </span>,
                    <span key='orders'>
                      需要核对充值时，到{' '}
                      <InlineCode>钱包 → 订单记录</InlineCode> 查看支付状态。
                    </span>,
                    '定期检查异常重复请求、错误配置或意外泄露的密钥，发现异常后立即禁用对应 API Key。',
                  ]}
                />
              </ArticleSection>
              <ArticleSection title='核对充值与套餐'>
                在 <InlineCode>钱包</InlineCode> 的{' '}
                <InlineCode>订单记录</InlineCode>{' '}
                中查看支付状态；套餐余额和有效期以控制台实时数据为准。
              </ArticleSection>
            </>
          ),
        },
      ],
    },
    {
      id: 'troubleshooting',
      title: '排错指南',
      icon: WifiOff,
      pages: [
        {
          id: 'streaming',
          title: '流式连接中断',
          subtitle: '区分本地网络、代理超时和上游中断',
          content: (
            <>
              <Callout>
                先保留错误原文和请求 ID，再判断是否重试，避免掩盖真正原因。
              </Callout>
              <ArticleSection title='客户端检查'>
                确认客户端已开启流式输出，网络稳定，本地代理没有短超时或响应体解码错误。
              </ArticleSection>
              <ArticleSection title='提交排查信息'>
                请提供发生时间、请求
                ID、模型名称、客户端名称和完整错误文字，密钥只保留首尾几位。
              </ArticleSection>
            </>
          ),
        },
        {
          id: 'status-codes',
          title: '常见状态码',
          subtitle: '先自检配置，再根据状态码定位权限、额度、上下文或上游问题',
          content: (
            <>
              <Callout>
                遇到报错时，先在 <strong>使用日志</strong>{' '}
                找到对应请求的完整错误、时间和模型，再按下面的状态码处理。多数配置问题可以自行定位。
              </Callout>
              <ArticleSection title='三步快速自检'>
                <GuideSteps
                  items={[
                    <span key='playground'>
                      用同一个模型在 <InlineCode>游乐场</InlineCode>{' '}
                      发送一句短消息；站内能通通常说明服务正常，应重点检查客户端。
                    </span>,
                    <span key='three'>
                      核对三项：Base URL 是否正确带或不带{' '}
                      <InlineCode>/v1</InlineCode>、API Key
                      是否完整、模型名是否与模型广场完全一致。
                    </span>,
                    <span key='logs'>
                      到 <InlineCode>使用日志</InlineCode>{' '}
                      查看失败请求的状态码、请求 ID 和错误详情。
                    </span>,
                  ]}
                />
              </ArticleSection>
              <ArticleSection title='错误码对照表'>
                <div className='border-border divide-border divide-y overflow-hidden rounded-xl border'>
                  {[
                    {
                      code: '401',
                      error: 'invalid_api_key',
                      action:
                        'API Key 错误、已删除、缺少 sk- 前缀，或 Base URL 指向了其他站点。重新复制密钥，并核对本站地址。',
                    },
                    {
                      code: '403',
                      error: 'Forbidden',
                      action:
                        'API Key 设置了模型或 IP 限制，账号/分组无权使用该模型。检查密钥权限与分组。',
                    },
                    {
                      code: '404',
                      error: 'model_not_found',
                      action:
                        '模型名拼写错误、当前分组没有该模型，或 OpenAI 兼容路径缺少 /v1。到模型广场复制准确模型名。',
                    },
                    {
                      code: '408',
                      error: 'request_timeout',
                      action:
                        '客户端等待时间过短。增大 timeout，启用流式响应，或选择响应更快的模型。',
                    },
                    {
                      code: '413',
                      error: 'context_length_exceeded',
                      action:
                        '输入、附件或历史对话超过上下文限制。裁剪历史、减小输出上限，或改用长上下文模型。',
                    },
                    {
                      code: '429',
                      error: 'rate_limit_exceeded',
                      action:
                        '账号并发或上游频率达到限制。降低并发，并使用指数退避后重试，不要立即批量重放。',
                    },
                    {
                      code: '429',
                      error: 'insufficient_quota',
                      action:
                        '余额、套餐或 API Key 额度不足。到钱包和套餐页面检查，必要时充值或调整 Key 额度。',
                    },
                    {
                      code: '500 / 502',
                      error: 'upstream_error',
                      action:
                        '本站或上游临时错误。稍后重试；若持续出现，保留请求 ID、时间和模型反馈。',
                    },
                    {
                      code: '503',
                      error: 'no_available_channel',
                      action:
                        '当前模型或分组暂时没有可用渠道。稍后重试、换模型或换分组；反复出现时提交模型名与时间。',
                    },
                  ].map((item) => (
                    <div
                      key={`${item.code}-${item.error}`}
                      className='grid gap-2 px-4 py-4 text-sm sm:grid-cols-[88px_180px_1fr] sm:gap-4 sm:px-5'
                    >
                      <code className='text-foreground font-semibold'>
                        {item.code}
                      </code>
                      <code className='text-muted-foreground break-all'>
                        {item.error}
                      </code>
                      <span className='text-muted-foreground leading-6'>
                        {item.action}
                      </span>
                    </div>
                  ))}
                </div>
              </ArticleSection>
              <ArticleSection title='高频客户端场景'>
                <VerificationGrid
                  items={[
                    {
                      title: 'Claude Code 连不上',
                      text: `ANTHROPIC_BASE_URL 使用 ${endpoint}，结尾不带 /v1；修改环境变量或配置后重启终端。`,
                    },
                    {
                      title: 'Codex 返回 401 / 404',
                      text: `base_url 使用 ${endpoint}/v1，wire_api 设置为 responses；API Key 放在 auth.json，而不是 config.toml。`,
                    },
                    {
                      title: '流式响应中断',
                      text: '增大客户端超时；curl 使用 -N 关闭输出缓冲。自建 Nginx 反代时关闭 proxy_buffering 并设置足够长的读写超时。',
                    },
                    {
                      title: '内容被拒绝',
                      text: '输入可能触发上游安全策略。检查请求是否合法并调整表达，不要用高风险内容反复试探。',
                    },
                  ]}
                />
              </ArticleSection>
              <ArticleSection title='仍然无法解决'>
                提交使用日志中的完整错误、请求
                ID、发生时间、模型名和客户端名称；API Key
                只保留首尾几位。管理员可以根据这些信息定位最终渠道及中间重试记录。
              </ArticleSection>
            </>
          ),
        },
      ],
    },
    {
      id: 'about-us',
      title: '了解我们',
      icon: BadgeCheck,
      pages: [
        {
          id: 'about-us-overview',
          title: '为什么我们是真的中转',
          subtitle: '用存储字段、计费数据和可重复测试说明服务如何工作',
          description:
            'AI 中转站很容易陷入只讲“稳定、低价、官方”的营销话术。真正有意义的是把做了什么、不做什么、记录了什么，以及用户如何自己验证说清楚。',
          content: (
            <>
              <Callout>
                “营销话术不能保真，代码细节可以。”下面只写可以从本站页面、日志字段和实际请求中核对的事实。
              </Callout>
              <ArticleSection title='1. 请求正文不进入使用日志'>
                API 请求中的 prompt、messages、上下文、图片内容和 tool call
                参数需要在转发过程中经过服务内存，但本站的消费日志表没有这些请求正文字段。普通使用日志主要记录：
                <PrincipleGrid
                  items={[
                    {
                      icon: ReceiptText,
                      title: '请求身份',
                      text: '请求 ID、发生时间、API Key 名称、账号与分组，用于把一次调用定位到具体记录。',
                    },
                    {
                      icon: SearchCheck,
                      title: '调用结果',
                      text: '模型名、HTTP 状态、错误摘要、是否流式和端到端耗时，用于判断失败发生在哪一层。',
                    },
                    {
                      icon: BadgeCheck,
                      title: 'Token 计量',
                      text: '输入 Token、输出 Token、缓存 Token 和本次消费，是用户核对扣费的核心字段。',
                    },
                    {
                      icon: ShieldCheck,
                      title: '管理员诊断',
                      text: '管理员额外看到最终渠道、中间重试和流结束状态；普通用户日志会隐藏这些内部路由字段。',
                    },
                  ]}
                />
                <p className='mt-4'>
                  自行核对的方法很直接：在使用日志打开任意一笔调用，能看到计费和排障元数据，但看不到你的完整对话数组、prompt
                  字符串或图片 base64。敏感业务仍应先脱敏，因为任何上游 API
                  都必须处理实际请求内容。
                </p>
              </ArticleSection>
              <ArticleSection title='2. 路由失败与切换可以追踪'>
                同一分组可能包含多个渠道。新对话按配置顺序选择渠道；当前请求遇到可重试错误时，会排除已失败渠道并尝试同组其他渠道。成功后按对话保持渠道亲和，减少不同上游缓存不共享造成的体验波动。
                <GuideSteps
                  items={[
                    '中间渠道失败、最终切换成功时，用户只看到最终成功记录，不会收到中间报错。',
                    '管理员日志会保留每次尝试的渠道、状态码和错误摘要，用于追查为什么发生切换。',
                    '渠道达到并发上限只会在当前请求内被跳过，不会被当成故障自动禁用。',
                    '只有所有可用尝试都失败时，用户才会收到最终错误和一条失败日志。',
                  ]}
                />
              </ArticleSection>
              <ArticleSection title='3. 计费可以逐笔对账'>
                本站从上游响应或协议计量中取得 usage，并分别记录输入、输出和缓存
                Token。实际扣费还会结合模型价格、分组倍率，以及缓存读写等模型专属倍率；结果落在同一笔使用日志中。
                <div className='mt-4'>
                  <CodeBlock
                    value={`{
  "model": "your-model",
  "usage": {
    "input_tokens": 1234,
    "output_tokens": 567,
    "cached_tokens": 800
  }
}`}
                  />
                </div>
                <p className='mt-4'>
                  核对顺序是：先看使用日志的 Token
                  与消费，再到模型广场核对价格和分组倍率；如果协议返回上游请求
                  ID，也应一并保留，方便进一步对账。
                </p>
              </ArticleSection>
              <ArticleSection title='4. 鉴别中转真伪的 10 个测试'>
                <VerificationGrid
                  items={[
                    {
                      title: '无效参数',
                      text: '提交一个明确会触发 400 的参数，检查错误类型是否指出具体字段，而不是统一返回“服务繁忙”。',
                    },
                    {
                      title: '首个 Token 延迟',
                      text: '用同一网络和同一模型对比官方与中转的首包时间，判断是否存在明显的额外排队或假流式。',
                    },
                    {
                      title: 'Prompt Cache',
                      text: '用相同长前缀连续请求，确认第二次出现缓存 Token，并检查缓存量与费用是否合理。',
                    },
                    {
                      title: '图片理解',
                      text: '上传一张包含可识别内容的图片，验证模型确实能读取并描述，而不是忽略图片输入。',
                    },
                    {
                      title: 'Tool Use',
                      text: '提供包含复杂 JSON Schema 的工具，确认参数结构、类型和调用顺序能够正确返回。',
                    },
                    {
                      title: '流式 Usage',
                      text: '检查流结束事件是否完整，并确认输入、输出和缓存用量没有在流末尾丢失。',
                    },
                    {
                      title: '固定参数对比',
                      text: '把 temperature 设为 0，用同一提示词在官方或不同服务重复测试，比较输出能力是否接近。',
                    },
                    {
                      title: '请求标识与响应头',
                      text: '保留上游 request ID 及协议相关响应头；完全缺失时要警惕协议被二次包装或能力被裁剪。',
                    },
                    {
                      title: '长上下文',
                      text: '逐步增加上下文并验证早期信息仍能被引用，避免仅凭模型名称相信标称上下文。',
                    },
                    {
                      title: '复杂推理',
                      text: '使用数学、代码排错和多步逻辑题对比模型层级，这是识别模型降级最直接的办法之一。',
                    },
                  ]}
                />
              </ArticleSection>
              <ArticleSection title='5. 行业常见的 6 类掺水方式'>
                <VerificationGrid
                  items={[
                    {
                      title: 'IDE 集成冒充官方 API',
                      text: '把 IDE 内部接口包装成“官方 API”，容易随 IDE 升级、协议变化或账号风控突然失效；应通过缓存、响应头和完整流式用量验证。',
                    },
                    {
                      title: 'Cookie 或订阅账号包装',
                      text: '登录态过期、改密码或原账号受限会让大量用户同时断流。不同上游类型的稳定性不应被混为一谈。',
                    },
                    {
                      title: '模型降级',
                      text: '对外标高阶模型，实际转发到低阶或开源模型。固定高难度提示词并跨来源对比更容易识别。',
                    },
                    {
                      title: 'Token 虚标',
                      text: '在上游 usage 基础上人为放大 Token。检查原始 usage、缓存 Token、分组倍率和最终扣费是否能逐笔解释。',
                    },
                    {
                      title: '假流式',
                      text: '等完整结果生成后再切片输出，通常首包明显偏慢，且流末端没有规范的完成事件和 usage。',
                    },
                    {
                      title: '错误码统一改写',
                      text: '把限流、额度不足和上游过载全部改成“服务繁忙”，会让客户端无法退避，也让用户无法定位。',
                    },
                  ]}
                />
              </ArticleSection>
              <ArticleSection title='6. 本站可以怎样自证'>
                <GuideSteps
                  items={[
                    '用户可以在使用日志逐笔看到请求 ID、模型、Token、缓存、消费、状态和耗时。',
                    '模型状态页公开近期成功率、延迟、生成速度和缓存率，不能只靠静态“可用”标签。',
                    '最终成功前发生过渠道切换时，管理员可按请求 ID 查看中间尝试，而用户不会被中间错误干扰。',
                    '价格和分组倍率以模型广场为准；无法由 usage、价格与倍率解释的消费应提交核对。',
                  ]}
                />
              </ArticleSection>
              <ArticleSection title='隐私边界'>
                请求正文不是本站消费日志的持久化字段，但中转与上游仍需在内存中处理内容。不要把“日志不保存正文”理解成可以发送密码、私钥或无需脱敏的个人数据；最稳妥的做法始终是只提交完成任务所需的最少信息。
              </ArticleSection>
            </>
          ),
        },
      ],
    },
    {
      id: 'support',
      title: '安全与支持',
      icon: LifeBuoy,
      pages: [
        {
          id: 'security',
          title: '安全与问题反馈',
          subtitle: '保护密钥，并提供可以快速定位的上下文',
          content: (
            <div className='space-y-4'>
              <p className='border-border text-muted-foreground flex gap-3 border-b pb-4 text-[15px] leading-7'>
                <CheckCircle2 className='text-foreground mt-1 h-5 w-5 shrink-0' />
                提供发生时间、请求 ID、模型、客户端和错误原文。
              </p>
              <p className='text-muted-foreground flex gap-3 text-[15px] leading-7'>
                <LockKeyhole className='text-foreground mt-1 h-5 w-5 shrink-0' />
                不要发送完整 API Key；截图前清理密钥、邮箱和其他个人信息。
              </p>
            </div>
          ),
        },
      ],
    },
  ]
}

export function Tutorial() {
  const [selection, setSelection] = useState<Selection | null>(() =>
    readSelection()
  )
  const { status } = useStatus()
  const serverAddress =
    typeof status?.server_address === 'string' ? status.server_address : null
  const browserOrigin =
    typeof window === 'undefined' ? null : window.location.origin
  const endpoint = resolveTutorialApiBaseUrl(serverAddress, browserOrigin)
  const categories = useMemo(() => buildCategories(endpoint), [endpoint])

  useEffect(() => {
    const params = new URLSearchParams(window.location.search)
    if (params.get('page') === 'site-guide') {
      window.history.replaceState(
        null,
        '',
        '/tutorial?cat=tutorial&page=quick-start'
      )
    }
    const onPopState = () => setSelection(readSelection())
    window.addEventListener('popstate', onPopState)
    return () => window.removeEventListener('popstate', onPopState)
  }, [])

  const defaultSelection: Selection = {
    categoryId: 'tutorial',
    pageId: 'quick-start',
  }
  const requestedSelection = selection || defaultSelection
  const activeCategory =
    categories.find(
      (category) => category.id === requestedSelection.categoryId
    ) || categories[0]
  const activePage =
    activeCategory.pages.find(
      (page) => page.id === requestedSelection.pageId
    ) || activeCategory.pages[0]
  const flatPages = categories.flatMap((category) =>
    category.pages.map((page) => ({ category, page }))
  )
  const activeIndex = flatPages.findIndex(
    (item) => item.page.id === activePage.id
  )
  const previous = activeIndex > 0 ? flatPages[activeIndex - 1] : null
  const next =
    activeIndex >= 0 && activeIndex < flatPages.length - 1
      ? flatPages[activeIndex + 1]
      : null

  const selectPage = (categoryId: string, pageId: string) => {
    setSelection({ categoryId, pageId })
    const params = new URLSearchParams({ cat: categoryId, page: pageId })
    window.history.pushState(null, '', `/tutorial?${params.toString()}`)
    window.scrollTo({ top: 0, behavior: 'smooth' })
  }

  return (
    <PublicLayout showMainContainer={false}>
      <div className='bg-background text-foreground mx-auto min-h-screen max-w-[1160px] px-4 pt-24 pb-7 sm:px-6 lg:grid lg:grid-cols-[260px_minmax(0,1fr)] lg:gap-9 lg:pb-8'>
        <aside className='mb-6 lg:sticky lg:top-24 lg:mb-0 lg:h-[calc(100vh-7rem)] lg:overflow-y-auto lg:pr-2'>
          <div className='text-muted-foreground mb-4 flex items-center gap-3 px-3 text-sm'>
            <Grid2X2 className='h-4 w-4' />
            文档总览
          </div>
          <details className='border-border bg-card rounded-xl border p-3 lg:hidden'>
            <summary className='flex cursor-pointer list-none items-center justify-between text-sm font-medium'>
              <span className='flex items-center gap-2'>
                <Menu className='h-4 w-4' />
                {activeCategory.title} · {activePage.title}
              </span>
              <ChevronDown className='h-4 w-4' />
            </summary>
            <div className='border-border mt-3 space-y-2 border-t pt-3'>
              {categories.flatMap((category) =>
                category.pages.map((page) => (
                  <button
                    key={`${category.id}-${page.id}`}
                    type='button'
                    onClick={() => selectPage(category.id, page.id)}
                    className='hover:bg-muted block w-full rounded-lg px-3 py-2 text-left text-sm'
                  >
                    <span className='text-muted-foreground'>
                      {category.title} /{' '}
                    </span>
                    {page.title}
                  </button>
                ))
              )}
            </div>
          </details>
          <nav className='hidden space-y-2 lg:block'>
            {categories.map((category) => {
              const isActive = category.id === activeCategory.id
              const Icon = category.icon
              return (
                <div key={category.id}>
                  <button
                    type='button'
                    onClick={() =>
                      selectPage(category.id, category.pages[0].id)
                    }
                    className={`flex w-full items-center justify-between rounded-xl px-3 py-2.5 text-sm transition ${isActive ? 'bg-foreground text-background' : 'text-muted-foreground hover:bg-muted hover:text-foreground'}`}
                  >
                    <span className='flex items-center gap-3'>
                      <Icon className='h-4 w-4' />
                      {category.title}
                    </span>
                    <ChevronDown
                      className={`h-3.5 w-3.5 transition ${isActive ? 'rotate-180' : ''}`}
                    />
                  </button>
                  {isActive && (
                    <div className='border-border ml-[22px] border-l py-1 pl-5'>
                      {category.pages.map((page) => (
                        <button
                          key={page.id}
                          type='button'
                          onClick={() => selectPage(category.id, page.id)}
                          className={`block w-full border-l-2 py-2 pl-3 text-left text-[13px] transition ${page.id === activePage.id ? 'border-foreground bg-muted text-foreground -ml-[21px]' : 'text-muted-foreground hover:text-foreground -ml-[21px] border-transparent'}`}
                        >
                          {page.title}
                        </button>
                      ))}
                    </div>
                  )}
                </div>
              )
            })}
          </nav>
        </aside>

        <main className='min-w-0'>
          <div className='text-muted-foreground mb-5 flex flex-wrap items-center gap-2 text-xs'>
            <button
              type='button'
              onClick={() =>
                selectPage(categories[0].id, categories[0].pages[0].id)
              }
              className='hover:text-foreground'
            >
              文档总览
            </button>
            <span>/</span>
            <button
              type='button'
              onClick={() =>
                selectPage(activeCategory.id, activeCategory.pages[0].id)
              }
              className='hover:text-foreground'
            >
              {activeCategory.title}
            </button>
            <span>/</span>
            <span className='text-foreground font-medium'>
              {activePage.title}
            </span>
          </div>

          <article className='border-border bg-card rounded-2xl border px-5 py-8 sm:px-10 sm:py-10 lg:px-12'>
            <div className='border-border border-b pb-7'>
              <div className='text-muted-foreground font-serif text-xs tracking-[0.18em] italic'>
                {activeCategory.title}
              </div>
              <h1 className='mt-3 font-serif text-[36px] font-medium tracking-tight sm:text-[42px]'>
                {activePage.title}
              </h1>
              <p className='text-muted-foreground mt-2 text-sm'>
                {activePage.subtitle}
              </p>
            </div>
            {activePage.description && (
              <p className='text-muted-foreground mt-7 text-[15px] leading-8'>
                {activePage.description}
              </p>
            )}
            <div className='mt-7'>{activePage.content}</div>

            <nav className='border-border mt-10 grid gap-3 border-t pt-6 sm:grid-cols-2'>
              <div>
                {previous && (
                  <button
                    type='button'
                    onClick={() =>
                      selectPage(previous.category.id, previous.page.id)
                    }
                    className='border-border hover:border-foreground group rounded-xl border px-4 py-3 text-left transition'
                  >
                    <span className='text-muted-foreground text-[10px] tracking-[0.16em] italic'>
                      ← 上一篇
                    </span>
                    <span className='group-hover:text-foreground mt-1 block font-serif text-sm'>
                      {previous.page.title}
                    </span>
                  </button>
                )}
              </div>
              <div className='flex justify-end'>
                {next && (
                  <button
                    type='button'
                    onClick={() => selectPage(next.category.id, next.page.id)}
                    className='border-border hover:border-foreground group rounded-xl border px-4 py-3 text-right transition'
                  >
                    <span className='text-muted-foreground text-[10px] tracking-[0.16em] italic'>
                      下一篇 →
                    </span>
                    <span className='group-hover:text-foreground mt-1 block font-serif text-sm'>
                      {next.page.title}
                    </span>
                  </button>
                )}
              </div>
            </nav>
          </article>
        </main>
      </div>

      <a
        href='/about'
        aria-label='联系客服'
        className='border-border bg-card fixed right-5 bottom-5 flex h-11 w-11 items-center justify-center rounded-full border shadow-lg transition hover:-translate-y-0.5'
      >
        <LifeBuoy className='h-4 w-4' />
      </a>
    </PublicLayout>
  )
}
