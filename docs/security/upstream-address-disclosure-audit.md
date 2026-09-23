# 上游地址泄露排查与修复验收

日期：2026-09-23。代码基线：V6 `ba026c4a5`（生产同步基线 `247b243014f`）。状态：已并入 V6 发布分支，待提交、推送及蓝绿候选验收。

用户已确认截图中的地址是后台渠道上游地址，而非客户端配置的公开 API 入口。本次没有取得该次线上请求的原始响应及服务端关联日志，因此不能把截图唯一归因于某一条代码分支；以下缺陷均已通过当前源码检查或本地故障注入确认。

## 排查结论

泄露防护存在多个出口缺口，单独修改网页提示无法保护第三方客户端收到的响应。

| 范围 | 原有问题 | 本次处理 |
| --- | --- | --- |
| 普通 OpenAI／Claude 错误 | 主要仅对 `message` 脱敏；`type`、`param`、`code`、`metadata` 可包含完整地址；计数错误存在豁免 | 出站时覆盖全部错误字段及嵌套扩展；取消计数错误豁免；内部原始错误保留 |
| Responses、Chat、Claude、图片 SSE | 多条路径通过 `CustomEvent` 直接发送原始错误 JSON | 在共享 SSE 输出出口清理错误对象、顶层错误事件、Responses 内嵌错误和 incomplete 原因 |
| Realtime WebSocket | 上游消息可直接转发给客户端 | 在上游到客户端方向清理错误；客户端发给上游的请求保持原样 |
| 异步视频／Suno／Midjourney | 任务失败原因、原始失败数据及图片代理错误可携带地址 | 清理面向用户的失败投影和任务错误；失败原因不再被误当作旧版视频结果 URL |
| 用户日志与历史记录 | 历史错误内容及 `other` 扩展可能已有地址 | 查询输出时脱敏，继续移除管理员调试字段；无需修改或删除历史数据库记录 |
| 网站业务接口 | 多处直接将 `err.Error()` 交给页面 | 统一错误帮助函数及旧式控制器 JSON 错误字段增加服务端脱敏；包括登录、支付、渠道测试、设置等路径 |
| 响应头和原始网关错误 | 部分出口复制上游响应头；Location、诊断头、Cookie 等可能揭示来源 | 协议、缓存、限额及必要请求追踪头使用白名单并检查值；502 等非 JSON 错误转换为通用 JSON 错误 |
| Panic／中间件拒绝请求 | 全局崩溃回调和中间件错误帮助函数可回显异常原文 | 统一通用 panic 提示；已开始的 SSE 终止而不追加 JSON；中间件 message／code／description 脱敏 |
| Sora／OpenAI TTS／MiniMax | HTTP 200 内嵌业务错误存在独立直出路径 | Sora 提交错误、TTS JSON 和 HTML 错误、MiniMax base_resp 错误清理；数据库原始任务数据保留 |
| 渠道探测历史 | 旧错误消息及错误代码可能包含地址 | 新写入及查询投影清理；历史行不修改；即时探测与分组摘要一致处理 |
| 图片／视频下载 | HTTP 200 也可能包含网关错误页，直接作为二进制返回 | 有界预读并识别文本／JSON／HTML 诊断，返回通用失败；成功媒体字节和关闭行为保留 |
| Default／Classic 前端 | 页面提示直接使用后端返回消息 | 检查了两套前端错误展示入口；修复位于服务端，使页面及外部 API 客户端共用保护 |

排查以路由、错误封装、各适配器输出、共享流式出口、任务与日志接口为线索进行了全仓静态搜索和相关代码核对；动态验证集中在上述共享出口与有明确风险的处理器，并非逐一调用所有真实供应商。

## 修复行为

新增 `common/public_error.go`。完整 HTTP／HTTPS／WebSocket URL、常见编码 URL、IPv4、IPv6、DNS／拨号错误地址和域名被替换为 `[redacted]`，不保留完整上游端点。错误内的嵌套 JSON、扩展字段和常见凭据字段也经过清理。补充识别 success:false、失败状态、数组及 data 内嵌错误和 MiniMax base_resp；错误对象内独立 URL／endpoint 字段直接删除。混合百分号、HTML 实体、Unicode 转义、零宽字符和国际化域名均有覆盖；编码归一最多四轮，仍在变化时回退通用错误。

示例使用虚构域名，不记录实际渠道地址：

```text
修复前：502 Bad Gateway, url: https://private.example/v1/responses
修复后：502 Bad Gateway
```

按用户确认的展示要求，错误文本中从 `url:` 开始的尾部诊断信息整段移除，包括后面的 `cf-ray`；只显示前面的错误内容，不显示 `url: [redacted]`。只有 URL 而没有错误正文时返回通用错误，普通 URL 参数校验文案仍保留。其他未标注为 URL 尾部的地址继续脱敏。

HTTP 502、限流等状态语义继续保留，现有重试／退款决策仍由原始内部错误驱动。正常回答中的链接、成功图片和视频地址、输出内容及用量不会作为错误文本整体清理。原始错误仍可供内部诊断使用；受权限保护的管理员渠道配置属于明确的信息展示，不属于本次需要隐藏的公开错误输出。

这次修复针对信息泄露。上游本身的 502 故障仍需要另行排查，不会因地址脱敏而自动恢复。

## 验证记录

- 使用本地 Go 工具链；未在生产机器构建、安装依赖或运行测试。
- 首轮新增 15 个顶层回归测试，涉及错误全部字段、IPv4／IPv6／编码／异常 URL、HTTP／HTML 错误、SSE、真实 Responses 与图片流处理器、WebSocket、任务、历史日志、成功内容和压缩媒体保留。
- 修改前的复现测试确认 `type`、`param`、`code`、`metadata` 泄露测试上游地址；修复后通过。
- 全部后端业务包 `go build` 通过，包含 router、controller、relay、service、model 及其依赖。
- 对 common、types、service、relay/helper、relay/channel/openai、model、controller、middleware、relay 运行完整包测试，发现下列 4 个基线失败；使用 Go overlay 将修改文件还原为 HEAD 后对照复跑，四项同样失败。
- 明确跳过这 4 项后，9 个包 **505 个顶层测试、673 项含子用例的测试全部通过**。
- URL 尾部展示调整另新增 1 个顶层回归测试（含 7 种文案及 JSON 出口验证），8 个相关包的 PublicError／Sanitize 专项测试通过；上述完整包测试数量为首轮验证记录。
- 第三轮复核补齐上述旁路。11 个相关包专项测试通过；明确跳过相同四项已知基线问题后，完整相关包测试 **518 个顶层测试、693 项含子用例通过**。新增实际 Gin panic／SSE 终止、中间件拒绝、Sora 提交、TTS、MiniMax fallback、SQLite 历史探测查询及媒体预读回归。
- 全部后端业务包 `go build ./...` 通过；Default/Classic 前端未因本次后端修复改动，沿用 V6 发布前已经通过的构建结果。未在生产机编译或安装依赖。
- `gofmt` 和 `git diff --check` 通过。

四个已对照确认的基线问题：

1. `TestPreparePriorityBillingForOutboundReservesTieredPrioritySurcharge`：现有优先计费断言期望 200、实际 250。
2. `TestGetAndValidOpenAIImageRequestMultipartStream`：无效参数分支的 i18n 初始化导致空指针。
3. `TestOpenaiImageHandlersReturnJSONError`：断言仍期望英文原文，实际行为已经中文化。
4. `TestListModelsTokenLimitIncludesTieredBillingModel`：已有模型列表／计费配置断言失败。

复跑命令（使用环境中实际 Go 路径）：

```sh
go test -vet=off ./common ./types ./service ./relay/helper \
  ./relay/channel/openai ./relay/channel/minimax ./relay/channel/task/sora \
  ./model ./controller ./middleware ./relay \
  -skip '^Test(PreparePriorityBillingForOutboundReservesTieredPrioritySurcharge|GetAndValidOpenAIImageRequestMultipartStream|OpenaiImageHandlersReturnJSONError|ListModelsTokenLimitIncludesTieredBillingModel)$' \
  -count=1
```

## 尚未执行的线上验收

本轮尚未修改生产配置、数据库、Nginx 或 CDN，也没有逐个访问实际供应商制造故障。正式发布仍需用包含 V6 前端资源的完整发布制品进行候选验收。

正式发布前，需要从完整累积代码快照构建候选产物，并在候选环境验证：502 JSON／HTML、超时和 DNS 失败、流中途失败、任务查询及历史日志；同时确认响应头和响应体都不含测试上游地址。还需核对入口代理／CDN 自行生成的错误页和重定向，这些不经过应用层脱敏。

新响应头白名单会移除供应商自定义诊断头；依赖额外自定义头的客户端需在候选环境确认。错误信息中的帮助链接也会被脱敏；正常成功内容中的链接不受影响。已经保存到外部客户端的旧报错不会被服务器追溯修改。

## 后续维护约束

新增适配器应复用公共错误出口与协议脱敏，不直接把供应商异常、HTTP 错误正文或诊断响应头透传。新增独立 SSE／WebSocket／媒体／任务输出时，应补充包含虚构上游地址的失败回归，同时验证成功内容仍保留。

本次结论是已识别代码出口与上述本地用例通过，不等于对任意未知供应商错误格式或任意编码作绝对保证。媒体检查会拒绝应为二进制却返回文本的响应；自定义供应商若依赖文本媒体或额外响应头，应在候选环境验证兼容性。实际生产链路与入口代理／CDN 验收仍待部署前完成。
