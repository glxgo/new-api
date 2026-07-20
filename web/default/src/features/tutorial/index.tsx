import {
  useEffect,
  useMemo,
  useState,
  type ElementType,
  type ReactNode,
} from 'react'
import { useQuery } from '@tanstack/react-query'
import {
  ArrowLeft,
  BookOpen,
  CheckCircle2,
  ChevronDown,
  Copy,
  Grid2X2,
  LifeBuoy,
  LockKeyhole,
  Menu,
  TerminalSquare,
  WalletCards,
  WifiOff,
} from 'lucide-react'
import { toast } from 'sonner'
import { Markdown } from '@/components/ui/markdown'
import { Skeleton } from '@/components/ui/skeleton'
import { getTutorialContent } from './api'
import { getSiteGuide } from './content'

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
                访问首页右上角 <InlineCode>登录 / 注册</InlineCode>
                ，完成登录后进入控制台。
              </ArticleSection>
              <ArticleSection title='第 2 步 · 准备额度或套餐'>
                在控制台 <InlineCode>钱包</InlineCode>{' '}
                查看本金、赠金和套餐状态。已有可用额度时可跳过充值。
              </ArticleSection>
              <ArticleSection title='第 3 步 · 创建 API Key'>
                进入 <InlineCode>API 密钥</InlineCode>，点击{' '}
                <InlineCode>创建 API 密钥</InlineCode>
                ，选择需要的分组并保存。完整密钥只展示一次，请立即妥善保存。
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
                用设备名或用途命名，例如 <InlineCode>MacBook-Codex</InlineCode>
                、<InlineCode>团队-设计组</InlineCode>，后续可以快速定位来源。
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
                。可在个人资料页查看当前占用和上限，并填写使用场景与联系方式申请提升。
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
          subtitle: '根据响应状态采取正确动作',
          content: (
            <div className='divide-border border-border divide-y border-y'>
              {[
                ['401', '密钥无效、格式错误或已经撤销'],
                ['403', '账号、分组、模型或 IP 没有权限'],
                ['429', '账号并发达到上限，或上游正在限流'],
                ['5xx', '记录请求 ID，稍后重试或联系管理员'],
              ].map(([code, text]) => (
                <div
                  key={code}
                  className='grid grid-cols-[64px_1fr] gap-4 py-5 text-sm'
                >
                  <code className='text-foreground font-semibold'>{code}</code>
                  <span className='text-muted-foreground'>{text}</span>
                </div>
              ))}
            </div>
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
  const { data, isLoading } = useQuery({
    queryKey: ['tutorial-content'],
    queryFn: getTutorialContent,
  })
  const endpoint =
    typeof window === 'undefined'
      ? 'https://your-domain.example'
      : window.location.origin
  const siteGuide = useMemo(() => getSiteGuide(data?.data), [data])
  const categories = useMemo(() => {
    const builtIn = buildCategories(endpoint)
    if (!siteGuide) return builtIn
    return builtIn.map((category) => {
      if (category.id !== 'tutorial') return category
      return {
        ...category,
        pages: [
          ...category.pages,
          {
            id: siteGuide.id,
            title: siteGuide.title,
            subtitle: siteGuide.subtitle,
            description:
              '请先阅读前面的通用教程；以下是管理员持续维护的本站专属说明，内容会按原始 Markdown 完整展示。',
            content: (
              <Markdown className='prose prose-neutral dark:prose-invert prose-headings:font-serif prose-headings:font-medium prose-h2:mt-9 prose-h2:border-b prose-h2:border-border prose-h2:pb-3 prose-h2:text-[22px] prose-p:leading-8 prose-li:my-1 prose-li:leading-7 prose-a:text-foreground prose-a:underline prose-a:underline-offset-4 prose-blockquote:border-foreground prose-blockquote:bg-muted prose-blockquote:px-5 prose-blockquote:py-2 max-w-none'>
                {siteGuide.content}
              </Markdown>
            ),
          },
        ],
      }
    })
  }, [endpoint, siteGuide])

  useEffect(() => {
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

  if (isLoading) {
    return (
      <div className='bg-background min-h-screen p-6'>
        <div className='mx-auto grid max-w-[1160px] gap-8 pt-20 lg:grid-cols-[260px_1fr]'>
          <Skeleton className='h-[520px] rounded-xl' />
          <Skeleton className='h-[720px] rounded-2xl' />
        </div>
      </div>
    )
  }

  return (
    <div className='bg-background text-foreground min-h-screen'>
      <header className='border-border bg-background/95 sticky top-0 z-40 border-b backdrop-blur'>
        <div className='mx-auto grid h-[68px] max-w-[1160px] grid-cols-[1fr_auto_1fr] items-center px-4 sm:px-6'>
          <a
            href='/'
            className='text-muted-foreground hover:text-foreground flex items-center gap-2 text-xs transition'
          >
            <ArrowLeft className='h-3.5 w-3.5' />
            <span className='hidden sm:inline'>返回首页</span>
          </a>
          <div className='text-center'>
            <div className='text-muted-foreground font-serif text-[9px] tracking-[0.28em] italic'>
              DOCUMENTATION
            </div>
            <div className='mt-1 font-serif text-xs tracking-[0.48em]'>
              使用文档
            </div>
          </div>
          <div className='flex justify-end'>
            <a
              href='/dashboard'
              className='bg-foreground text-background rounded-full px-4 py-2 text-xs font-medium transition hover:opacity-80'
            >
              进入控制台 →
            </a>
          </div>
        </div>
      </header>

      <div className='mx-auto max-w-[1160px] px-4 py-7 sm:px-6 lg:grid lg:grid-cols-[260px_minmax(0,1fr)] lg:gap-9 lg:py-8'>
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
    </div>
  )
}
