# Codex Shared Code Handoff

> Reviewable handoff for multiple Codex windows working on this repository. Do not store passwords, tokens, private keys, cookies, SSH key paths, connection secrets, or full chat transcripts here.

## Stable context

- Repository: `new-api`, the source repository inside the relay-operations workspace.
- `/Users/adrian/Documents/Codex/new-api` is a separate restored clone; it is a reference copy, not a synchronized handoff channel.
- Binding repository conventions and protected identifiers remain in `AGENTS.md`.
- Environment and deployment status belongs in `../CODEX_MEMORY.md` and must be re-verified before external operations.

## Current handoff

### 2026-09-23 — 用户端智商测试质感优化完成（本地未上线）

- 在 `/Users/yihu/AI-Workspace/juxing-iq-v6` 的 Default 与 Classic 用户端同步提升鹈鹕存档视觉层级：页面引导区加入已核验来源与原始记录提示、存档时间/来源周期元信息和模型选择区；分组卡片强化分组标题、模型、倍率与描述层级；作品画廊使用轻量双层画布、边框/悬停反馈和关联目标徽标；历史区域整理为带图例的时间记录带；详情抽屉新增来源判定摘要、标准答案/检测答案证据块和更紧凑的记录数据容器。
- 保留原有真实数据链路、三张作品布局、点击作品/历史打开详情、图像原始比例、空作品居中状态、浅色/深色主题和全部后台控制语义；没有重绘、改分、调用模型或改变分组/渠道映射。
- Default `tsc -b`、定向 ESLint（0错误，1条既有 Fast Refresh 警告）、生产构建通过；Classic 生产构建和相关 Prettier 检查通过，`git diff --check` 通过。浏览器本地真实预览确认用户页新头部、分组/画廊层级及作品详情抽屉可见；未部署、未提交、未推送。

### 2026-09-23 — 首次初始化、本地自动导入与空作品状态完成

- 用户“好，继续下一步”授权本地初始化准备。新增受控`service.BootstrapPelicanArchive`和`cmd/pelican-bootstrap`：默认dry-run，计划哈希确认后单事务写入；拒绝非空存档、变化的快照/绑定/分组、重复绑定、无资格成员、非启用root及重复执行；初始化后强制页面隐藏、同步暂停并记录管理员事件，不修改业务渠道。`cmd/pelican-preview --bootstrap --check-only`用最新真实副本通过，产出23分组、8关联、240条记录、32条成员关系；真实数据接口验收、重复执行拒绝和失败回滚通过。
- Bootstrap SQLite、MySQL8.2（与生产同版）、PostgreSQL16矩阵race通过。导入外部周期60改30、目标周期独立改120后，SourceConfig/目标周期自动更新，本站同步间隔保持5分钟；新快照由后台worker自动导入，未点击手动同步，捕获时间从22:02:17推进至22:06:37且关联保持。两台服务器运输timer仍运行。
- Default和Classic浏览器实查真实作品、题目、标准答案21及原答29；8目标跨分组共享真实记录。Default/Classic作品卡片无图或加载失败时改为卡片内居中的图标+“作品暂不可用”空状态，保留比例和整卡详情点击，支持深浅色。Default typecheck/定向ESLint（0错误，1条既有Fast Refresh警告）、Default/Classic production build、Go定向race和全仓build通过。未生产迁移/挂载/蓝绿或提交推送。
- 当前下一步是整理最终发布制品、合并届时最新源码后申请B：生产迁移、目录只读挂载、隐藏状态下slave候选蓝绿验收；用户页面仍未公开。本地预览进程已停止，验收制品与快照在仓库外私有目录。

### 2026-09-23 — A受限运输获批安装完成

- 用户明确“ok”批准上轮A范围及共享维护协作；已安装两台服务器专用账号、固定导出权限和一分钟宿主运输，未获B发布授权。不要重复询问A授权。环境结果见../CODEX_MEMORY.md及docs/architecture/pelican-transport-installation-2026-09-23.md。
- 本地7项Python/壳语法通过，安装脚本哈希一致；实际非空SSH命令、SFTP、PTY、转发及任意sudo拒绝；两轮timer拉取成功8目标240记录，捕获时点推进，失败保留原字节/inode。timer恢复并持续运行。
- 本轮未修改产品源码、UI、业务容器/数据库或本地预览，没有提交推送。下一步仍需本地实现受控首次初始化，验证后申请B迁移/挂载/蓝绿候选；宿主有文件不等于生产应用已经导入。既有展示规则与8项关联确认不变。

### 2026-09-23 — 数据库矩阵与受限运输发布预检查完成

- 用户“可以，进行下一步”授权下完成本地准备及两台服务器只读核验，没有生产写入、提交、推送或上线。GitHub main仍247b243014f3418fe84b398afc1a2299cab8620f，线上仍20260921-load-status且二进制哈希与交接一致、健康无重启。认证账号Piloting06有push权限，admin/maintain=false，不是仓库管理员权限。
- 新增model/pelican_database_test.go；本机真实SQLite3.50.4、MySQL8.2.0（生产同版）/8.4.11、PostgreSQL16.4均通过迁移、导入修订、容量回滚、长文本、并发CAS及旧字段哨兵保留的race验证。仅合成空白隔离库，不是生产全库或最低版本实测。数据库临时解压，未安装系统服务，全部实例已停止。最后model/service/router/preview定向race与Python7项通过。
- tools/pelican-archive/deployment提供固定导出命令、受限SSH/sudo、专用账号运输service/timer及只读目录挂载增量；pull.py支持独立SSH配置。壳语法、非空命令拒绝、SSH配置以及源站sshd的stdin只读解析通过。未安装账号、重载SSH或启动定时器，真实受限登录与两轮运输仍待安装验收。
- 本机生成Linux amd64应用与迁移静态制品，版本20260923-pelican-preflight，未上传或运行Linux候选；哈希见生产预检查文档，不作为最终冻结发布包。
- 首次发布新增门槛：当前迁移工具仅建5表，必须另补受控初始化分组身份、快照和8项关联并保持隐藏，之后slave候选才有数据；该初始化工具尚未实现。禁止把候选临时改master绕过。现网程序为宿主二进制只读挂载，换镜像不等于换程序，需保留旧路径供回退。
- 下一步待用户批准文档A：两台共享服务器安装受限运输，须确认维护协作；A不含业务数据库迁移、容器切流或页面公开。B仍独立待批准。详见docs/architecture/pelican-production-preflight-2026-09-23.md。本地预览仍为05:03拓扑快照，无持续外部拉取；本轮未改UI或产品规则。

### 2026-09-23 — 8项真实关联与生产拓扑副本本地验收完成

- 用户“继续”授权下在既有实施树推进，未获上线授权。05:03:05北京时间通过两台服务器只读获取52渠道/717能力及8目标/240条存档；拓扑白名单不含密钥、上游地址、用户、订阅或计费数据。私有文件留仓库外，未写生产。
- `tools/pelican-archive/confirmed-mappings.json`记录已确认8项，`pull_topology.py`只读一致性导出，`cmd/pelican-preview`新增成对topology/mappings参数及本地user-group。快照时效24h、错误关联整批回滚；预览现在是真实配置副本，非演示ID，非实时生产镜像。Default4199/Classic4200/后端4198已重启接入；会话与banner展示模式/时点/权限边界。生产主程序不包含预览会话入口。
- `PelicanGroups`模型目录只保留有效存档关联模型，避免业务模型先选中造成空结果；两套空状态完善，不改业务路由。8关联对应32成员关系均有效，#83/#90/#36仍业务停用且可展示。管理员9个测试组、本地default账号6个，保留现有用户权限。
- `verify-preview.mjs`固定localhost验收：成员/倍率、7条精选均对应来源最新记录、5份图像逐字节一致、4条跨组共享；实际保存目标隐藏/别名/描述/文案/排序/间隔/暂停/隐藏并恢复。浏览器两套原图详情、21/29原答、Default后台标题保存后两套前台同步，已恢复默认。Default390px浅色与Classic390px深浅色无横溢，桌面主题实查。未改变图库布局。
- Go preview/service/router定向race、Python6项、Default typecheck/改动正式页lint、双前端build、Go全仓build和diff通过；不是全仓行为回归。文档和术语已同步。
- 本地同步保持显示/启用、5分钟；本轮拉取仅单次，外部持续运输未运行。预览拓扑固定05:03快照，不能称持续生产同步。生产关联保存/受限运输、MySQL/PostgreSQL实机、最新基线合并复核、蓝绿候选验收仍待完成；最终生产写入及上线待用户确认。无模型调用、提交或推送。

### 2026-09-23 — 用户确认剩余两对入口等价，8项目标身份齐备

- 用户在阅读两对域名/渠道差异后明确确认“是一样的”：kedaya关联#9，shuai-gpt020关联#36；入口等价依据用户确认，密钥和模型一致依据先前服务器核验，不冒充上游实测。不要再重复请求此项确认。
- 当前8项确定关系：ac→83、aishenji-958→71、dkl-016-ap-gpt-2hao→81、dkl-025→80、opentoken-2469→48、shuxin79→90、kedaya→9、shuai-gpt020→36。#65/#37不是这两项的关联目标。
- 渠道核对报告、方案及验收记录已同步。本轮只更新本地文档，未写生产关联、未改演示预览、未部署。下一步依据确认清单准备真实拓扑本地验收；生产运输配置、三库实测、最新基线复核及蓝绿门槛仍保留，最终上线待独立确认。

### 2026-09-23 — 6项渠道身份确认，停用渠道展示规则变更

- 用户授权继续确认，并明确本站停用但外部仍测试的渠道正常按所属分组展示；前台统一文案，不加“存档样本”等专门标签，业务启停仅在后台说明。这覆盖此前停用即不展示的规则。
- 已修改存档拓扑：保留停用渠道的能力关系可参与，启用渠道的单独禁用能力仍排除；模型撤下、能力缺失、移组、删除、隐藏及来源停用/移除仍阻止对应访问。PelicanGroups补足停用渠道独有的已关联模型，未改业务路由或旧自测。后台新增channel_disabled说明，两套现有组件接入，无前台特殊标签。
- 本地service/router定向race覆盖手动/自动停用、能力联动、结果/详情/作品可读、模型目录可见、移组撤销直链、隐藏不改业务状态。Default类型与修改文件lint、双前端构建、Go全仓构建、diff检查通过。本轮未重新跑浏览器矩阵，旧预览进程未重启，仍不能把演示关联当生产绑定。
- 两台服务器只读身份比对确认当前地址/密钥/模型一致：ac→#83、aishenji-958→#71、dkl-016-ap-gpt-2hao→#81、dkl-025→#80、opentoken-2469→#48、shuxin79→#90。只保存对应关系，不保存凭据或摘要。
- 另2项：kedaya同密钥对应#9但入口不同，同地址#65密钥不同；shuai-gpt020同密钥对应#36但入口不同，同地址#37密钥不同。两个帅API入口匿名status同产品同版本，仍不足证明线路等价；可达鸭两入口status均404。需维护者确认入口等价，未自动批准两项映射。
- 审核报告、方案、术语、验证记录已更新。6项确认仅当前配置，非历史凭据追溯证明。未写生产绑定、未改来源网站/数据库/容器、未调用收费模型、未提交或上线；生产仍待批准且沿既有蓝绿。

### 2026-09-23 — 渠道分组关联核对与本地自动导入补齐

- 本轮在原实施树补齐service/pelican_topology.go，管理报告和公开结果共用当前渠道/成员/模型/ability判定；报告返回分组展示名/倍率/参与资格/阻断原因，渠道字段白名单不加载凭据。Default与Classic后台已接同一报告，30秒刷新且保留未保存草稿；选择新渠道清空旧模型，保存仍经后端校验。
- 本地拓扑回归覆盖共享、改名移组、停用/恢复、模型撤下/恢复、能力禁用、隐藏、解除关联和来源移除；service/router定向race与Go构建通过。双前端构建、Default typecheck/lint（1条既有Fast Refresh警告）通过。浏览器验证两套解除/恢复关联，Classic单目标隐藏/恢复；390px深浅色无页面横溢，沿现有原生组件，图库布局未改。
- pull.py增加60–86400秒循环、可限定cycles、排他锁、单轮失败保留旧快照并重试，3项Python回归通过。预览启动器现启动真实导入worker，本地默认显示/启用；生产默认仍隐藏/暂停。有限两轮真实只读拉取每轮8目标/240条完成并退出；本地间隔1分钟，无手动同步，自动导入04:22:55捕获快照，04:23:21已读回且与文件captured_at完全相同。未留下无限拉取任务；本地worker在预览进程运行期间继续检查文件。
- 实际生产渠道候选只读核对已整理docs/architecture/pelican-channel-audit-2026-09-23.md。同地址多渠道、部分停用，不能仅凭名称或地址认定实际账号/套餐等价；未读取或解密密钥，未写入真实生产映射。预览仍演示渠道分组，不能宣称已完成真实生产端到端绑定。
- 方案、核验记录、工具README和术语已同步。下一步需补外部provider对应实际Channel.Id的身份事实；之后仍有受限生产运输、三库实测、届时最新源码复核及蓝绿验收。生产写入和上线待用户确认；本轮无服务器写入、模型付费调用、提交或推送。

### 2026-09-23 — 原作保真阅读与旧快照问题核对

- 用户要求优化获取信息及展示但保留真实结果，随后提供来源03:05/03:11/03:25截图质疑旧图。核实本地快照获取于02:50，预览同步仍处此前验收的暂停态；源站截图比本地记录新，并非本站重绘导致差异。通过现有SSH只读导出03:52:10快照（8目标、240条，最新id466），在本地后台恢复同步并立即导入。当前画廊为id464/463/462（03:25/03:16/03:11），公开作品接口SHA256与快照SVG逐字节相同。
- 预览当前visible=true、sync_enabled=true、间隔7分钟。本次只手动拉取并导入一次；预览启动器不启动同步worker，外部持续运输也未配置，不能宣称实时跟随来源。正式应用已有导入worker，生产运输与部署仍待批准。
- 两套详情改为原始作品/题目与答案/记录数据，默认作品页，支持适应宽度及150%/200%滚动查看；参考答案与来源检测数字分区、题目保留原文及版本匹配说明，来源判定和截断标记不改。页头改用真实存档获取时间，避免较晚的导入时间造成新鲜度错觉。图廊布局及精选规则未改；不进行审美评分或把29改为21。
- Default类型检查、定向ESLint（0错误、1条既有Fast Refresh警告）、双前端构建与格式检查通过。两套浏览器200%缩放和21/29原答验证，Default390px深色题目页无横向挤压。中英新增Default8条文案，其他语言尚未补译；Classic新增中文沿现有回退。
- 本轮仅本地代码/预览修改与来源服务器只读，未提交、推送或生产部署。

### 2026-09-23 — 作品卡片仅显示图像并自适应原图比例

- 用户要求卡片只显示鹈鹕图像，题目与记录细节点击后再看；图像须贴合边框，不溢出或被固定画框压小。Default/Classic 均移除卡片判定与时间页脚，保留整卡点击及无障碍名称；无图以中性作品不可用占位。
- 图片宽度铺满卡片、高度随原图比例；网格顶部对齐，取消缩略图固定3:2和高度上限，横竖图可不同高度。不裁切、拉伸或修改原SVG，原作品自带文字仍保留；历史与详情布局不变。
- 本地Default类型检查、目标ESLint、两套生产构建及修改文件格式检查通过。浏览器实查Default浅色、Classic深色的真实存档图像卡片与点击详情，标准答案21及原作答可查。未部署、未提交或推送。

### 2026-09-23 — 外部鹈鹕存档接入已进入本地真实数据验收（当前有效状态）

- 用户正式授权面向用户的本地开发，外部网站是唯一测试源；本站不出题、不调用模型、不评分、不控制外部调度。不再要求HTML/SVG动画。用户确认当前标准答案21，历史原答与判定不追溯改写。
- 实施树 `/Users/yihu/AI-Workspace/juxing-iq-v6`，分支 `codex/iq-capability-v6`；未提交、推送或部署。旧V6源码保留，但正式启动、API、双前端入口切换到pelican archive，主程序不再自动迁移旧自测执行表（保留共享分组身份表）。旧执行器HTTP回归通过仅在测试文件注册的历史路由运行。
- 外部迁移服务器SSH只读成功；核实镜像llm-api-bench:2.15.3-balance-alerts1，240条存档/8目标、默认60分钟、当前标准答案21。来源无全局轮次、无历史原题全文；按目标完整记录更新，题目仅按FNV版本标记匹配当前配置并明确标注。来源会改模型名和清理旧数据，本地保留内容修订。
- 新增pkg/model/service/controller pelican archive、私有只读导出与SSH原子拉取、独立迁移工具。快照校验、事务去重、修订恢复、来源身份、暂停/CAS、单目标关联唯一性、100000条/512MiB内容门槛均已实现；不自动删证据。受限运输尚未生产配置。独立拉取与本站导入分离，后台暂停不停止外部测试或宿主运输。
- 外部目标显式关联Channel.Id/实际模型，多分组共享同记录；实时成员、能力、状态和权限校验。UI包含作品上/历史下、最多3个优选目标、真实不足不复制、题目/数据详情、六章说明、隐藏/同步/间隔/映射/分组别名描述排序/折叠文案/原记录。设置写入沿Root权限，管理员只读与预览。
- 本地240条真实记录导入成功、重复导入0新增；152条含SVG，147条可安全预览，5条不通过当前检查但原文仍存。浏览器以blob图片隔离，允许有界无环本地use，拒绝主动代码/外链/循环及扩张炸弹。普通DTO不暴露来源UUID、渠道内部名称、原响应或错误。
- 回归通过：pkg/model/service/router的TestPelican和旧TestCapabilityHTTP定向race（-vet=off），go build ./...，Default typecheck、功能ESLint（0错误、2条Fast Refresh警告）、双前端production build、Python语法、git diff --check。不是全仓行为测试；MySQL/PostgreSQL实机尚未验收。
- 浏览器实查Default真实作品/题目/耗时Token、文案折叠与草稿保持、整行排序、别名及标题保存到前台；Classic作品详情、21标准答案、7分钟间隔保存、隐藏并暂停、隐藏态用户不可见与管理员仍可预览。两套均看过手机390px深色和桌面浅色；更广语言/键盘矩阵未完成。Default新增82项中英文，其他四语言同步为英文回退，未声称已翻译。
- 预览使用内存SQLite和演示分组/渠道，只监听本机：后端4198、Default4199、Classic4200。原始快照保留仓库外私有目录，不提交；预览不是生产映射验证，不发起模型调用。已修复内存库连接寿命导致5分钟后清空的问题。
- 当前方案和核验记录：`docs/architecture/pelican-archive-integration-plan-2026-09-22.md`、`pelican-archive-verification-2026-09-23.md`；部署准备 `tools/pelican-archive/README.md`。上线前仍需真实生产Channel.Id关联、受限运输和网络、三库实测、最新基线合并复核、蓝绿候选验收。生产写入/上线必须待用户确认；当前未向任何服务器写入。

### 2026-09-22 — 用户明确停止旧方案，改为外部鹈鹕存档接入（历史决策，已由上方新确认接续）

- 用户明确要求“停止目前的进度”，最新标准是外部鹈鹕网站将迁移到另一台服务器，中转站读取该网站已经生成并保存的测试存档，再向用户展示。暂停本站自行出题、指定渠道调用、渲染、评审、随机题库及旧 V6 后续开发；以下历史待办不再是继续实施授权。
- 截图中的内网穿透是对方提出的接入建议，尚未核验网络、文件格式、读取协议、稳定身份字段、完整发布标记或迁移地址。不能将“迁移只改地址”当成已验证事实，不能将存档误解为迁入整套测试源码。
- 本站暂停同步/隐藏与外部停测不同；外部周期、停测和自动同步本站渠道的能力均待只读核实。仅有存档时不得伪造评分、题目或调用数据，也不得保留没有实际消费者的旧控制按钮。
- 本地已有未提交修改保留，未回滚/删除源码，未提交/推送/上线。本轮只作停工处理与文档记录，没有连接服务器或读取凭据。检查时 Go 回归/构建进程已不在列表；对三个仍存活的本地 `node ./server.mjs` 进程发出终止，后续匹配检查无残留。未核验它们全部的工作目录，不能把此记录扩大为所有本地服务均已停止。
- 中断前本地源码已新增12个随机动画模板（`juxing-iq-3-animation-pool`）、校准覆盖检查与 renderer 隔离配置/测试。曾观察到带本地 renderer 的五包定向 race 和15项 Node测试通过；最后一条测试＋构建串行命令的完整输出尚未收取，不为旧方案追加验证。Linux容器、人工校准及真实付费验收从未完成。这些均为已暂停的历史实现，不是新方案成果。
- 新方案下一步仅应先只读查明外部存档样例、列表/索引、身份与时间字段、获取方式，再确定同步/分组映射/缓存与展示契约；本轮未执行该检查或新实现。历史 UI 偏好仍是参考，数据与操作含义须按外部实际能力重新校对。生产操作仍待用户确认并沿既有蓝绿流程。

### 2026-09-22 — 用户纠正责任边界：外部网站已接入飓星渠道

- 用户明确补充：不是把外部结果重新映射到本站分组去调度；外部鹈鹕网站已经接入飓星 API 的渠道并完成各渠道测试。本站只读取外部已保存的鹈鹕存档并展示，不能再次分配渠道、发起模型请求或把本站分组作为测试执行前提。
- 已只读打开示例网址 `https://api.shuaiapi.com/intelligence`，页面公开入口要求登录；没有输入账号、密码或绕过登录，也没有读取私有接口。无法从公开页面确认存档格式。
- 新规划文档：`docs/architecture/pelican-archive-integration-plan-2026-09-22.md`，定义来源事实、推荐只读 HTTPS/受限 SFTP/隧道顺序、完成标记、增量同步、整批发布、前端/后台组件和验收条件。核心要求：外部记录中的稳定 ID/分组/渠道字段以实际样例为准，缺失不猜；本站暂停同步/隐藏展示不等同外部停测。

### 2026-09-22 — 动画证据、播放与补评链路（本地模拟验证，未上线）

- 继续在 `/Users/yihu/AI-Workspace/juxing-iq-v6` / `codex/iq-capability-v6` 开发。新题库版本 `juxing-iq-2-animation`，鹈鹕原句逐字保存，移除旧海边/围巾/橙子/太阳条件；逻辑及规则几何保留。随机动画模板规模仍未完成，不能偷换成仅固定题已完成全部题库。
- `tools/capability-renderer`新增Chromium子进程，Playwright1.61.1/Chromium149.0.7827.55。固定600×400、48帧/100ms；服务端重新编码无损APNG、海报、六帧评审拼图，保存HTML哈希/时刻/版本/采样差异。网页不执行上游HTML。CSS/SMIL/JS、静态、外网/本地文件/样式资源/iframe拒绝、无限脚本和客户端取消均在本地自有白名单样例上验证。新增`.gitignore`排除renderer依赖。
- `capabilityRenderItem`供初次执行和持久化补评共用，judge的ImageSHA256绑定拼图；无画面变化不能给运动项通过，有变化也不自动升分。套题/renderer/browser进入校准报告门禁，旧静态报告拒绝。不存在正式人工校准报告，未绕门禁。
- 动画与证据以原run/evaluation鉴权路由的view参数读取；真实SQLite路由验证匿名、隐藏、history关闭、跨run修订及管理员保留访问。Default用户详情/后台原记录/补评共用原生播放器；Classic用户详情有Semi播放器。默认首帧、手动播放/停止、展开后才加载连续帧；Default5项新文案六语已补。保持三图上、历史下布局，方法说明与实际观察窗口一致。
- 本地真实Chromium/resvg＋HTTP模拟上游＋SQLite整轮及恢复通过：生成调用保持12、judge仅3→4，原结果/快照不变。五包定向`-race -vet=off`通过，后续新增动画版本门禁/路由/完整轮次回归再次通过；APNG帧序/CRC/时刻/逐帧像素、静态/非法输出通过。Default typecheck/功能ESLint、双前端production build和最终`go build ./...`通过；不是全仓默认测试或全V6验收。
- 4197预览现接真实本地渲染的“移动圆形”测试制品，顶部明确模拟及非鹈鹕实测。Default浏览器已实查画面移动、停止恢复首帧、六帧展开、390px深浅色及键盘；视口已恢复。开发热更新曾整页重载，非生产运行错误。生成文件在`web/default/dev/animation-fixture`（git忽略），可用README中的可选CAPABILITY_TEST_EXPORT集成测试命令重建，预览入口需要先生成它们。
- Dockerfile补了动画浏览器依赖，但Linux隔离镜像尚未构建/运行；沙箱/userns/seccomp、资源限制、启动阶段中断/崩溃进程回收、字体/镜像与实际judge配置的校准绑定都是发布阻断项。不能把macOS自有样例白名单当作生产隔离验收。详情`tools/capability-renderer/README.md`。
- 未收费调用、未访问或写入生产、未迁移、未提交/推送。后续仍需随机完整题库、人工多帧校准、独立真实验收、三图真实补位、容量/清理、用户独立修订、Classic完整后台/国际化/浏览器、协议矩阵与三库/Redis多worker；最终上线等用户确认并沿历史蓝绿。实施记录和完整方案已更新，不以本地动画链路通过代替这些剩余项。

### 2026-09-22 — 信息密度、分题详情与公开生成数据（本地完成，未上线）

- 按最新六张参考图提升密度，Default 保留三图上/历史下：分组与模型同行、12–13px文字、较紧间距、各题型有评分轮次数；新增底部三栏专业摘要，沿用后台方法标题与隐藏开关，点击进入对应六章节阅读面板。
- Default 拆出 `run-evidence.tsx`，原生 Sheet/Tabs 按点击题型打开；原图、原题、逐项评定、原文、生成时间/用量和匿名记录ID分层呈现。桌面/390px、深浅色、键盘方向键+Enter、点击部分得分色块原题1/2、点击作品6/6、摘要直达第6节已用明确模拟数据实查。预览仍4197，无真实模型调用。
- 公共详情新增严格白名单 `metrics`，只读取 generation 三题；耗时仅 complete 且开始/结束有效时可用，排除judge/渲染。可信上游单对象 usage 的 OpenAI/Responses/Claude/Gemini数字字段按原值展示，0保留；多事件、冲突、非法/估算值与未知终态留空。渠道、URL、key slot、raw usage、内部请求ID不进入该DTO。已过定向 race 和真实路由/SQLite权限泄露回归。
- Classic 新增原生 Semi 时间线/Tooltip、按题型详情及同一生成数据、三栏方法摘要；修正三图同排和小屏单列。排序使用 `set_order` 更新已发布与草稿顺序，成功后移动整行，失败保留原顺序。双前端生产构建通过；Classic尚未浏览器实查，也未完成完整后台矩阵/语言对齐。
- Default 类型、功能 ESLint、双前端构建与新增后端/路由定向race通过；16个本轮标签已补Default六语并同步。全量V6仍未完成，特别是HTML/SVG动画渲染/播放/多帧判分、私有真实验收、题库规模、三图真实补位、清理容量、Classic完整对齐及三库/Redis/付费校准均保留待办。**本轮未实现动画链路，不得把这轮UI进展视为动画完成。**没有生产写入、提交或推送。

### 2026-09-22 — 恢复队列落地及用户明确动画试题（尚未上线）

- 持久化 `CapabilityRecoveryJob` / `CapabilityEvaluation` 已接入 worker：复用已存答案补渲染/首次评审，只追加独立修订，不覆盖源 run 或公开快照；人工 stop、执行版本与 worker owner 阻止旧任务继续。退避、相同证据去重、临时 DB 错误保持待恢复、judge 未知结果不重发、PNG hash 与原题绑定已回归。普通完成任务及时结束恢复项，最后订阅撤销在取消调用前持久化。
- 本地真实 resvg＋模拟 HTTP 上游＋SQLite 回归恢复前后生成次数保持12、judge仅3→4；覆盖重启、短至一分钟的自定义计划、原文已存但检查点未存、未知judge/不同图片拒绝复用。真实路由验证修订图仅管理员/root可读，隐藏仍可后台查，跨run匹配失败404。
- Default 后台运行详情增加原生折叠修订，状态/原题/图像/成绩/原文可查，13个新文案六语齐；模拟浏览器核对深色与390px浅色、展开与键盘焦点。修复侧栏卡片被flex压扁裁切。预览4197仅内存模拟；Classic尚无对应友好恢复面板。
- 最新定向 `-race -vet=off` 的model/service/controller/router/pkg/capabilitytest通过（含本地renderer集成），`go build ./...`与Default类型/lint/格式/生产构建通过。无收费模型调用、生产访问/修改、迁移、提交或推送。
- **用户最新试题要求优先于旧静态设计**：鹈鹕提示词完整为“生成 html，内容是 svg 绘制鹈鹕骑自行车 2D 动画，不进行测试，不使用 skill，不参考本地文件。”其他随机图像沿同模板。此句是被测模型提示词；当前仅静态SVG/PNG实现，不能称已符合动画需求。原固定题不得保留未要求的围巾/橙子等评分约束；须补新题库版本、隔离动画渲染/播放、时序判分与校准。
- 用户询问同版真实功能＋真实渠道、仅自己验收且不向普通用户公开的方式。独立验收实例/库/队列/作品＋限次限额任务进程为建议，尚未创建。现有slave蓝绿候选不跑IQ worker，不得直接切连生产库候选为master。配置自动发现基于实例实际DB；验收配置副本持续跟随生产需另加只读同步，不能称已经自动同步线上。
- 下一步优先按最新动画契约调整题库与渲染/评分设计，准备不公开的真实验收路径；完整题库、三图真实补位、恢复容量/用户端修订、30天清理、Classic完整对齐、三库/Redis多worker、真实授权范围/预算和人工校准仍未完成。详见验收基准与实施核验记录，不能删掉这些差距或宣称全量完成。

### 2026-09-22 — 色块与后台文案折叠本地验证

- Default 时间线改为三行等宽小矩形、20px 色条与32px点击区、本站 success/warning/destructive/muted 语义色、原生 Tooltip 和文字图例。三行共用横向滚动，缺失题型保留位置；手机不撑宽整页。保留用户认可的上三图、下历史布局。
- Default 文案管理复用 SettingsAccordion，三个分区、默认收起、全部展开/收起、自定义项数和未保存提示。浏览器模拟环境验证编辑→收起→展开保留→保存草稿→应用→用户页标题变化。Classic 新增 Semi Collapse 文案管理并接通各文案消费者，构建通过，尚未浏览器验收 Classic。
- 浏览器检查 Default 浅色、深色、390px 手机与三行同步横滚；点击部分得分历史格打开对应轮次原题且成绩一致。修复模拟时间线与详情评分不一致、单渠道组样本数错误；模拟界面不能替代付费实测。预览 4197，仅内存接口，不访问生产。
- 本次 Default typecheck、本功能 ESLint、双前端生产构建、相关格式检查及 git diff --check 通过；`go test -race -vet=off ./model ./service ./controller ./router ./pkg/capabilitytest -run 'Test(Capability|Logic|Questions|SVG)' -count=1` 通过，含真实本地 resvg + 模拟上游与 SQLite 路由权限/时间线回归。
- 全量 V6 仍在开发。完整题库、持久化渲染/评审恢复、30天清理、容量和三库/多worker验证、三图不足补位、Classic完整功能对齐、真实渠道费用测试/人工校准及部署包仍待完成，详见实施核验记录。没有生产写入、提交或推送；上线仍等用户确认。

### 2026-09-22 — 智商测试开发：恢复上下文先核对用户基准

- 必读 `docs/architecture/group-capability-accepted-contract-2026-09-22.md`；它锁定用户已确认方向，不能用临时实现或压缩摘要替换。P2 三图同排在上、分项历史在下为认可布局；P1 两列大卡掉第三张为待纠正偏差。作品/色块点击必须实际打开对应题目证据。专业简介借鉴 P3 的简洁形式，但适配本站实际机制，不照搬未经验证的“同 Codex CLI”“答错即降智”。
- 用户再次明确生产上传、迁移、重启、切流等均等其确认，再沿已有蓝绿规范执行。本地继续开发；不得自动付费调用、提交、推送或上线。
- 工作树 `/Users/yihu/AI-Workspace/juxing-iq-v6`，`codex/iq-capability-v6`，基线 `247b243014f`；原方案工作树不可拿来构建。
- 整轮发布快照、stop 与发布事务顺序、每任务检查点、防重复派发、后台作品证据、history 直链权限已经新增。定向 Go（含实际本地 resvg、模拟上游）和 race、真实路由/鉴权/SQLite 控制回归通过；最新时间线及布局改动仍待本轮后续验收。真实费用测试、全量 V6 仍未完成。

### 2026-09-21 — 全部本地源码归档提交与远程同步

- 用户明确授权上传所有尚未上传的本地源码，目标为已有 `origin`（`glxgo/new-api`）的 `main` 分支。上传前 fetch 确认本地 HEAD 与 origin/main 均为 `84989702a`，无分叉或未推送旧提交。
- 本次归档包含 228 个已跟踪修改文件及 101 个新增源码/测试文件，覆盖此前累计功能与已发布修复。逐文件 SHA-256 核对最新 `20260921-load-status` 发布快照：2548 文件中仅交接文档有更新，业务源码完全一致，沿用该发布已完成的构建与测试结果；`git diff --check` 通过。
- 上传范围排除 8 个本地 Linux 编译产物、`.mimosa/` 工具状态和已忽略的数据库/构建缓存。源码凭据模式检查仅命中测试占位值。已有 GitHub workflows 不因 main 分支 push 触发生产部署，本次不操作生产。
- 此条随归档提交保存；实际 push 成功与远程提交号在上级运营交接中记录，执行结果以远程 main 核验为准。

### 2026-09-21 — 两处修复已完成蓝绿上线 `20260921-load-status`

- 已冻结完整共享树 2548 文件，仅本轮 16 个源码/测试文件及交接文档相对上一发布变化。两套前端构建、Default 类型/lint/格式、12 前端测试、真实 Redis 定向 race、全仓 Go（-vet=off、沿用 7 项已知失败排除）通过。新静态产物 SHA-256 `5cde9fb80c4e74c6697c512a91ca058e770fec0e8ec311e8ea6632abf541a2d3`，上传与镜像内一致，无 DDL 或提交/推送。
- 3010 新候选健康、认证接口与上游流式短测通过，1h 真实/探测 12 根五分钟柱、24h/7d 48 柱已生产接口核验。约 11:29 公网切候选；旧正式连接/统计排空后替换，正式健康且认证接口通过，16:32 左右回切。候选等待零连接及统计桶/刷盘周期完成后已停止，保留容器用于恢复。
- 北京时间 16:52 最终验证：正式健康、0 重启、无 OOM，候选 exited，三域名各 5/5 新版本，全部入口为 3000，近五分钟未匹配启动/迁移错误。近 15 分钟真实流量 1,042 条无限额度非零快照、观测最大并发 51/RPM 59；近十分钟 765 条 UA 日志快照一致，生产无本地演示渠道。发布已完成。
- 全部发布证据 `/Users/adrian/Documents/Codex/new-api-release-20260921-load-status/`，final-verification.json、发布记录与回滚步骤齐全；备份/回滚细节见上级记忆。本地预览 4191/4193 保留，无 Git 提交或推送。

### 2026-09-21 — 无限负载快照及 1h 五分钟状态柱（本地完成，未发布）

- `service/concurrency_limiter.go` / `user_rpm_limiter.go` 去掉零额度提前返回；Lua 与本地只在 limit>0 时拒绝，零限额仍使用真实 lease、心跳、幂等释放与滚动分钟计数，普通/独立会员池均覆盖。不改满 ¥1000 的额度规则。前端复用 `formatCapacitySnapshot`，列表/详情/手机显示实际值/∞；旧日志 count=limit=0 保持缺失，不伪造历史数据。
- 模型状态 1h 的真实请求与主动探测为 12 个 300 秒桶，24h/7d 原粒度保留；前端按请求数加权聚合五分钟柱。真实请求优先已有 60 秒渠道桶，查询下界对齐首个显示桶，排除无法拆分的旧粗桶和旧 group fallback，不将半小时/小时样本伪装成五分钟。无需 DDL，不改分组后台采集配置。
- 新测试先复现无限计数返回 0、1h 两柱和前端错误密度；本地与 Redis 7 定向普通/无限/会员隔离/过期/释放/原子性回归及 race 通过。控制器/指标定向回归、12 前端行为测试、tsc、ESLint（0 errors、2 既有 Fast Refresh warnings）、Prettier、Default build 与原生 backend build 通过。首次原生构建与前端 dist 清理冲突导致 embed 缺文件，前端结束后顺序重建通过。未跑全仓回归。
- 浏览器使用真实本地 API 和明确标记 SQLite 样例，确认列表/详情/390px 手机无限计数、1h 真实/探测各十二柱与五分钟时间差，24h 原 48 柱；控制台无 error。截图、原生二进制、预览刷新脚本和验收前 DB 备份位于 `/Users/adrian/Documents/Codex/load-status-fix-20260921/`。预览 4191/4193、后端 `local-load-status-20260921` PID 64919，临时 Redis 已停止。无生产访问/发布/提交/推送，既有共享修改保留。

### 2026-09-21 — 完整工作树已完成蓝绿上线 `20260921-all-local`

- 当前完整共享树已冻结 2548 文件，两套前端、本机 Go Linux/amd64 静态构建、全仓 Go（沿用七项已知基线排除、-vet=off）、定向 race、28 前端测试和变更 lint/格式均通过。产物 SHA-256 `2ed88b6e9ca2183b298e4172c9bc13c3538700a37420a3bb95a66fdb9b36c932`，证据 `/Users/adrian/Documents/Codex/new-api-release-20260921-all-local/`。未提交/推送。
- `channels.group` LONGTEXT、`channel_perf_metrics_v2.bucket_seconds` BIGINT DEFAULT 3600、`idx_channel_perf_channel_bucket(channel_id,bucket_ts)` 已迁移生产，本地先验证同一 DDL。3010 slave 候选健康，管理员渠道统计/钱包折扣/额度策略/模型状态三周期/API Key/日志等 API 与两种流短测通过，三个公网入口各 3/3 新版本 `20260921-all-local`。
- 旧实例 04:45 统计桶的 115 次请求已落库后才替换，正式 3000 健康、同一轮认证业务接口通过，05:03 回切。候选持续零连接，等待现网 15 分钟分组桶结束及下一轮刷盘，05:18 排空脚本成功后停止，未见刷盘失败；容器保留。
- 北京时间 05:19 最终验证正式 running/healthy、0 重启、无 OOM，候选 exited，三个公网域名各 5/5 HTTP 200/新版本，全部 Nginx 入口为 3000。LONGTEXT 和渠道复合索引已确认，170 条分钟桶含 313 次请求；近十分钟 168 条 UA 日志快照不一致为 0，近两分钟消费日志 46 条，近五分钟未见匹配启动/迁移错误。`final-verification.json` 与发布记录已保存；本次包含下方此前本地未发布的两轮功能。
- 数据库/配置备份及 `new-api:rollback-20260920-all-local` 保留，旧版回滚须 `NODE_TYPE=slave` 防止自动迁移缩列，不默认回退业务数据库。更完整运营证据见上级记忆。本地 4191/4193 预览保留，未修改另一个窗口 4188。

### 2026-09-21 — 单渠道分组长文本与管理员渠道统计（本地完成，未发布）

- 相对本轮开始时共享脏树修改 27 个源码/测试/翻译文件，未覆盖先前钱包、限流、UA 或性能改动；增量、基线、哈希、报告、测试与截图在 `/Users/adrian/Documents/Codex/channel-health-20260921/`。未提交/推送/上传、未访问生产。
- 原 `Channel.Group` 的 VARCHAR(64) 限制整个逗号列表总字符数，非分组个数。移除长度/SQL default 标签，使用 MySQL LONGTEXT、PostgreSQL/SQLite TEXT；`BeforeCreate` 保留空值默认 `default`，单建/批量均覆盖。`migrateChannelGroupsToLongText` 在 migrateDB/migrateDBFast 的 AutoMigrate 前执行，幂等扩列保留数据、删除 MySQL 不兼容 SQL 默认，SQLite 不做不支持的 ALTER COLUMN；单个 Ability.Group 规则未变。
- 管理员 `GET /api/channel/metrics`（静态路由在 `/:id` 前、AdminAuth 内）按真实渠道统计，名称/ID 搜索在 SQL 分页前执行，最多 100 条/页，15 秒 Context 与 no-store。只 SELECT ID/name/type/status，无 Key/BaseURL。当前页渠道指标在 SQL 汇总，复用已有成功/缓存/首字计数，不扫 logs。
- 小时窗口截止最新完整分钟，`[end-3600,end)`；自然日固定 UTC+8，可指定历史日期，今天也截止完整分钟。成功率为最终请求、排除内部重试和客户端原因失败，缓存率沿用原 cacheRate 的输入 token 口径，先累加再计算；TTFT 只按实际流式首字样本平均。相应分母为 0 时返回 null/显示 —。
- ChannelPerfMetric 新采集独立为 60 秒桶，公开分组采集周期保持旧配置。表 `channel_perf_metrics_v2` 增加 `bucket_seconds`（旧行保守 default 3600，新行 60；混合 upsert 保留较粗值）及复合索引 `idx_channel_perf_channel_bucket(channel_id,bucket_ts)`。旧桶不推算拆分，包含旧桶/边界重叠显示近似提示。DB+本实例热桶合并，channelMetricsMu 协调查询/单桶 drain+commit，atomicBucket 增加锁以一致读取多项计数。其他实例热数据须刷盘后可见（默认 5 分钟，故障会更迟）。分钟采样增加数据量，未做生产规模长时压测。
- Default 渠道页懒加载“渠道统计”弹窗；按天/最近小时、日期、搜索、分页、错误/空态/采集关闭/历史精度提示；桌面表格与手机三指标卡片，复用 Dialog/Table/Tabs/CountUp/useIsMobile，支持键盘、深浅色、减少动画、六语，无全局 CSS 变更。
- SQLite 与本机 MySQL 8.2 隔离验证 5000 中文分组（>64KB）、能力表、更新、批量、默认值、重复迁移/保留数据；MySQL 还验证指标列迁移、复合索引和聚合 SQL。PostgreSQL 只做 GORM 类型/代码验证，未跑实例迁移。相关 model/perfmetrics/controller/middleware 整包通过（-vet=off，沿用 `TestListModelsTokenLimitIncludesTieredBillingModel` 排除）；初次旧 middleware 两处非字面量 fmt.Errorf vet 错误留档，未扩大修复。定向 race、前端 tsc/变更 lint/格式/构建、最终原生本地 build 通过。
- 真实本地 Chromium 验证数值、null、后端分页/搜索、昨日范围、空态、匿名拒绝、键盘、390px 深色卡片/减少动画、英文，pageErrors 为空。当前预览 `http://127.0.0.1:4191/channels` → 渠道统计；后端 4193 `local-channel-health-20260921`，最终 PID 56624。独立预览库增加 22 个禁用且明确标注的演示渠道/指标，无上游费用，原凭据不变。
- 正式上线前需对届时合并树重新构建验证，并备份/执行渠道长文本及指标列/索引 DDL。旧二进制回滚时避免重新 AutoMigrate 把长字段缩回 VARCHAR(64)。本轮没有生产 DDL 或发布。

### 2026-09-21 — 钱包折扣、默认限流、日志/统计及模型状态七项调整（本地完成，未发布）

- 保留共享树已有修改，本轮相对任务前快照改动 48 个源码/测试/翻译文件；增量补丁、快照、检查日志和截图在 `/Users/adrian/Documents/Codex/wallet-status-20260921/`。未提交、推送、上传、生产访问或部署，没有新增 DDL。
- 普通日志路由固定窗口、工具函数和重置统一为本地今天 00:00 → 当前时间 +1h，保留显式日期及游标分页；使用统计前后端默认 24h。浏览器实际请求参数已验证，不能只改工具函数而遗漏路由中的旧两小时逻辑。
- 账号两项限额分别取原值/全局值与 200 并发、1000 RPM 的较大者；累计有效充值 >= CNY 100000 分时两者为 0（既有限流器无限哨兵值），不受旧 RechargeCapacityEnabled 开关影响。User/UserBase 共用策略、缓存版本 4；会员独立池额度合并不降低账号额度。旧充值容量开关/表从默认主题后台移除，管理员仍可提高全局值。安全限制未绕过。
- 复用合格已支付充值账本：CNY 50/200/500/1000/3000 档对应系数 .99/.98/.97/.96/.95；按付款前累计额解锁，完成当前订单影响下一笔。Epay/Waffo/Pancake/Stripe/Creem 报价与下单接入；叠加已有分组/金额优惠及支持的优惠码，付款手续费在后。Stripe 使用原产品币种动态报价、保留平台优惠码及既有到账额度；Creem 官方单次/限产品折扣 API，展示价由后端给出。未真实扣款。
- 钱包替换为五档折扣进度卡，六种语言；号池公开导航隐藏。模型状态 1h/24h/7d，CountUp 成功率和从左至右增长柱，尊重 reduced motion；主动探测展示真实探测样本成功率，去除渠道数字/名字，保留管理员诊断。完整模型名可键盘选择，复用同组 ModelCard 与 ModelDetailsDrawer，未动全局样式。
- model/controller/middleware/pkg/perf_metrics 整包通过，仅沿用已知 TestListModelsTokenLimitIncludesTieredBillingModel 基线排除；本次新用例及限流器定向 race 通过。24 项前端测试、tsc、变更 ESLint（0 error，3 条既有 refresh warning）、Prettier、生产构建通过。Chromium 真实本地登录/钱包/日志/统计 + 模型状态样例验收通过，含手机暗色、英文钱包、键盘、减少动态效果，无 pageErrors。
- 限制：1h 沿用既有半小时采样，仅 2 桶；24h/7d 48 桶。390px 新卡片/弹窗不溢出，既有全局页头约 419px 溢出按不改全局样式范围保留。真实支付平台与生产尚未验收。
- 本地预览继续使用 4191 前端 / 4193 实际后端和原隔离 SQLite。版本 `local-wallet-status-20260921`，后端 PID 53430，前端 Node 41567；先实时核对再操作。旧样例仍是 9 月 20 日，今天默认日志可能为空，可手动选昨日。本次没有重新播种或导入生产数据。

### 2026-09-20 — 当前完整工作树已蓝绿上线 `20260920-all-local`

- 用户授权将全部本地未上线更新部署。本次发布当前完整共享树，含 UA/Coding、Token/日志游标及用户/渠道/仪表盘性能优化，保留上一版功能；不是此前排除 UA 的独立性能产物。2531 文件快照、相对 9 月 18 日的 113 文件差异清单、日志与发布记录在 `/Users/adrian/Documents/Codex/new-api-release-20260920-all-local/`。未 Git 提交/推送；已有 dirty 状态保留。
- 两套前端生产构建、Default tsc、两套变更文件 lint/格式、5 项前端行为测试通过；全仓 Go 测试沿用上一发布 7 项基线排除后通过。定向 race 宽匹配旧 header override 测试出现并行 gin.SetMode 竞争，确认在未改动的测试中，以 `-parallel=1` 复验通过（被测函数内部并发仍启用），保留初次证据。
- 本机编译静态 Linux/amd64 后端，SHA-256 `19db857db669d08539da9c084b8da42663497f81929e8ffceb93a6d995b916d6`。生产只有 COPY 镜像封装/备份/数据库增量迁移/部署/健康验证，未编译或测试。本地源码冻结后至发布前无漂移。
- MySQL 8.2 本地复制相关生产表定义，审批、组支持、UA 写入、撤回及普通组回归通过；原样增量 SQL 二次本地验证后执行生产四张 client 表与日志/绘图/任务新增字段（INSTANT）。Token 用量覆盖索引在线建立并 EXPLAIN 命中；旧索引及历史日志保留。
- 已完成 3010 独立候选→公网→正式 3000 重建→回切→候选排空停止。真实认证 API、资源、Claude/Responses 短测、公网登录页渲染正常；最终三个域名各 5/5 HTTP 200/新版本，正式健康、restart=0、OOM=false。观察 252 条带快照的新日志、key 不一致 0；不代表真实供应商全矩阵或长时负载保证。
- 发布前完整库/配置备份和 `new-api:rollback-20260918-all-local` 均保留，详细位置与恢复步骤见运营记忆/发布记录。未自动批准客户端或导入本地示例，现有分组名无 Coding，管理员后续启用需要审批与组支持同时配置。

### 2026-09-20 — 实际后端与 Default 本地验收站运行中

- 用户要求本地运行网站。验收地址 `http://127.0.0.1:4191/usage-logs/common`，Bun dev 代理到真实原生 Go 后端 127.0.0.1:4193；独立全新 SQLite，不读取仓库旧数据库或生产配置。运行目录 `/Users/adrian/Documents/Codex/client-ua-20260920/local-preview/`，含二进制、仅修改监听地址的构建 overlay、进程清单、日志、seed.go、run-local.py 和所有者可读的登录凭据文件。凭据不存共享记忆。
- 样例数据明确标注“本地验收”：25 条普通日志、8 条绘图、8 条任务、23 个客户端身份及三种审批状态/Coding 分组策略；没有真实渠道或供应商调用。默认日志时间窗两小时，晚些验收需调大范围。
- `check-browser.mjs` 通过实际登录、原始 UA 详情、后端 Codex 分类过滤及管理员已允许列表，页面错误为空；本次不是模拟 API。初次脚本漏点筛选“展开”及“搜索”，修正脚本流程后通过，未为此修改产品源码。
- 服务已留运行：后端 PID 41565，前端 Bun 进程组 41566 / Node 41567，实际只监听 loopback；操作前先实时核验。其他窗口 4188 服务未动。未上传、提交、推送、部署或访问生产。

### 2026-09-20 — 用户/渠道/仪表盘与路由查询优化已本地验证（未发布）

- 渠道搜索将状态/类型/分组过滤和分页下推 SQL；GROUP BY type 同时推导 total，避免全量 Channel 物化。标签列表单批 IN 查询，标签搜索子查询保留兄弟渠道语义；标签稳定字母排序，渠道排序加 ID 次序。搜索默认 20、上限 100，负数/溢出保护，继续 Omit key。120 条匹配记录的本地案例，完整行加载 120→10；20 标签页面 SQL 23→4。
- 用户/渠道 API 和 React Query 接入 AbortSignal，主数据库分页/计数传递请求 Context；用户编辑抽屉 lazy 且打开时挂载。用户身份补全和容量读取原本已批量查询，未无谓重做。
- `web/default/src/lib/auth-query.ts` 共享路由与欢迎提示身份查询：按 user ID 缓存 15 秒，仅内存；进入已认证路由强制 fresh，失败仍 reset/跳登录。仅网络响应更新 store，防止缓存覆盖新余额；退出后到达的响应不能恢复用户。Chromium 进入用户页并两次筛选的身份请求 4→1，刷新 401 回归通过。
- 仪表盘默认/快捷区间按 30 秒对齐，展示与实际查询一致、自定义日期不量化；`snapshot=true` 显式开启用户/管理员 usage-statistics 相同语义，旧客户端仍精确秒。仪表盘 UNION 两个源内过滤 user_id、外层执行携带 Context，统计请求取消并避免窗口焦点/重试重复扫描。
- Go 定向回归/race、全仓 compile-only（-vet=off）、Default tsc/ESLint/Prettier/build:check、4 项前端测试和 Chromium 模拟验收通过；详细失败复现/测试记录、25 文件增量补丁、源码及两轮合并二进制在 `/Users/adrian/Documents/Codex/admin-query-performance-20260920/`。产物版本 `20260920-admin-query-performance`，SHA-256 `c4471547202a8b2e1e919eed466a0d7074d80a33d2b424e25fa2cd2fcdf01933`。
- 大范围仪表盘冷查询仍可能扫描明细；会员管理员全量生命周期读取、钱包/平台聚合耗时另有证据，未改动其资金/重置语义或开启回填。不能宣称所有慢查询已消除或线上已经提速。
- 隔离产物基于 9 月 18 日发布源码叠加两轮性能优化，不包含并行 UA/Coding 准入；共享工作树和该窗口新交接记录均保留。未上传、部署或变更生产；全量发布需使用最新合并源码重新验证构建。

### 2026-09-20 — 客户端 UA 快照、日志局部展示与 Coding 白名单已本地完成（未上传）

- 新增 `common/client_identity.go` 严格前缀识别；入口在转发/头覆盖前捕获请求快照。覆盖 Codex 各形态、Claude Code、Pi、OpenCode、ZCode 数字版/unknown、DSH、Go 推断 NewAPI、OpenClaw、Cherry Studio、OpenAI SDK/运行时/浏览器/curl。已知版本升级身份稳定，未知完整原始 UA 做 SHA256；留存内容过滤控制字符并限制 2048 字节，保留截断标志。
- `model/client_identity.go` 新增客户端登记、审批记录、分组支持策略和策略记录四表；审批/策略修改要求理由并用 revision CAS。客户端批准和目标 Coding 分组支持必须同时成立；未知身份不能批准。未配置策略时名称含 coding（不区分大小写）的分组默认受限；其他默认普通，显式策略可设置类型。没有自动批准或生产配置变更。
- 每次请求/重试和公共出站路径重读准入，无正向缓存；自动分组跳过无准入候选，全部被拒时保留专门错误。客户端拒绝使用 `client_not_allowed`/403，不进入渠道封禁、内容风险处罚；预扣失败路径保留退款。普通分组兼容。任务错误包装也保留拒绝语义。
- 普通/错误/绘图/任务/任务结算保存原始快照；旧日志不回填，显示“未记录”。新增 `client_family` 查询在用户范围、COUNT/游标/offset 分页前过滤，统计缓存按分类隔离。与并行性能任务的 `logListContext`/游标分页合并后新增 HTTP 控制器回归，确保没有漏传筛选。
- Default/Classic 都复用原有组件，时间栏显示时间/状态、图标/客户端/Coding，仅栏内底部淡渐变；详情、复制、管理员审批/分组配置、手机/键盘/深浅色及多语言已接入。全站 UA 样例/审批 API 受 AdminAuth 保护；用户仅返回自己的日志快照，快照不保存鉴权头。
- 专项 SQLite/本地 HTTP 回归与 race 通过，验证头改写、撤回后同上下文下次出站为 403 且上游计数不增加、普通组可用、账号/令牌状态和余额不变、零费用日志、用户隔离及分类计数。最终全仓 compile-only、Default tsc、两套构建、改动文件 ESLint/格式检查和 git diff --check 通过（Default 原列文件 2 条 Fast Refresh warning）。扩大整包回归除两项已记录基线失败（priority surcharge、model-list tiered）外通过；未做 MySQL/PostgreSQL 实例迁移验收或真实供应商全矩阵联调。
- Chromium 模拟接口验收含两套界面、Default 普通/绘图/任务、手机/桌面、浅深色、中文英文、键盘焦点/复制、分类过滤、审批理由必填和分组设置。说明、任务起点快照、日志及截图位于 `/Users/adrian/Documents/Codex/client-ua-20260920/`。保留其他窗口修改；没有生成声称纯净的独立任务补丁。
- 本轮未提交、推送、上传、部署或操作生产；上线前先准备客户端审批和分组支持配置，并对届时合并工作树重新验证。UA 可伪造，不代表官方身份或缓存率；Go UA 的 NewAPI 只是本站显示约定。

### 2026-09-20 — API Key / 日志查询性能修复已验证并生成独立产物，未发布

- `controller/token.go` 新增显式 `include_usage=false`，Default 列表/搜索/详情采用此参数，先返回管理元数据；新增受 UserAuth 保护的 `POST /api/token/usage-stats`，最多 100 个 ID、去重、校验当前用户归属及软删除后才进入缓存/查询。旧客户端默认用量字段保持原有语义。前端用量单独 loading/error/stale 状态，避免伪零。
- 新批量接口对 query time 与 cache key 一起量化至 30 秒快照，沿用 local/Redis/singleflight，旧精确时间 model 入口不量化。Token 共享工作与接口等待有 15 秒预算。带 AbortSignal 的新 GET 绕过 axios 全局 in-flight 合并，由 React Query 合并，避免复用已取消请求。
- `pagination=cursor` 接入管理员/用户日志控制器，采用可选 `LogPageMetadata` 取 page_size+1、total=-1、has_more/next_cursor；该路径不 COUNT、不 OFFSET。管理员按 created_at/id、用户按 id，用户 cursor 在 formatUserLogs 重写公开 ID 前生成。legacy 包装器保留原排序与精确总数。分页参数补负数/溢出保护。
- Default 普通日志两小时默认窗口固定进 URL；新分页支持前/后/首页和 page size；精确总数按需查询。统计 key 排除分页/游标、禁自动重试与焦点重查、传递取消；Drawing/Task 保留原时间行为。显式索引迁移增加 `idx_logs_token_usage_quota(token_id,type,settled,created_at,quota)`，AutoMigrate 不自动 DDL。
- 新 controller 回归覆盖不查 logs 的 metadata、Key 归属/删除/去重/快照缓存、双角色游标同秒顺序/插入新行/无 COUNT/OFFSET/用户隔离。定向 race、旧日志/Token/索引回归、隔离版本全仓 compile-only、Default tsc/ESLint/Prettier/build、Chromium 模拟接口桌面/移动分页验证通过。最终共享工作树同一组关键 controller/model 回归也通过；未宣称全仓所有行为测试通过。
- 并行窗口在本轮修改了 `controller/relay.go`、`model/log.go`、`controller/log.go`、router 等客户端准入功能；曾导致混合工作树 GetCode 未定义（后续对方已修正，最终定向编译通过）。本轮未覆盖或修复对方实现。发布产物采用 9 月 18 日已验证源码 + 本次补丁，剔除了并行客户端功能；共享源码仍同时保留双方改动，后续整体上线需重新测试合并结果。
- 产物与说明在 `/Users/adrian/Documents/Codex/api-log-performance-20260920/`，`task-only.patch` 相对 9 月 18 日发布源码、25 文件，在隔离源码反向校验通过，勿在当前工作树重复正向应用。Linux/amd64 静态版本 `20260920-query-performance`，SHA-256 `be0e902cd729194dc81d4f4ee5608ef85947dd4ad38d5c28840981989cd02b90`。未提交、推送、上传或部署。生产诊断与后续发布边界见运营记忆。

### 2026-09-18 — 全量本地改动已完成蓝绿上线，正式版本 `20260918-all-local`

- 用户明确授权“把本地所有改动进行蓝绿部署上线”，替代此前先不上线要求。本次包含全部当前未提交/新增源码、流状态修复与 Claude 思考强度日志修复，Default/Classic 均重新构建；未提交、推送或覆盖其他窗口修改。
- 本地源码快照、2505 文件哈希清单、测试/构建日志、二进制、发布记录与公网/最终验证位于 `/Users/adrian/Documents/Codex/new-api-release-20260918-all-local/`。产物 SHA-256 `8f87c21b8e15c51b321460baa8aeb98bdeac965fdc5f851be2f8c23d52931b3f`，上传前后相同；发布镜像 `new-api:20260918-all-local`。生产只做二进制 COPY 封装，没有编译、安装依赖或运行单元测试。
- Default 类型检查、两个前端构建、改动文件 ESLint（0 errors/2 warnings）和格式检查通过；Go 全仓回归排除前条流式任务已确认的 7 项基线失败后通过，定向 race 与 Linux/amd64 静态构建通过。首次 Go 检查与前端 dist 重建冲突，构建结束后完整重跑通过。
- 实时确认旧公网原由 3010 的 `20260915-spend-limit` 候选承载。本次先启 3011 新候选（slave），健康、静态资源、认证前后业务接口验收通过，渠道 7 Claude 原生流与渠道 71 Responses 流短测均成功；切到候选后重建正式 3000，再切回。三个公网入口各 5/5 返回新版本；两套候选已等待连接和批量更新排空后停止，容器保留用于恢复。
- 最终 2026-09-18 20:21 北京时间核验：正式 `new-api` running/healthy、restart=0、OOM=false，Nginx 三域名及默认入口均指向 3000；停候选后公网复验仍 200/新版本。最近 5 分钟正式日志未见 panic/fatal、迁移失败、database locked 或 unsupported protocol；业务仍观察到上游 502/连接失败及 Spark 模型不支持错误，不能宣称所有上游故障已消除。
- 发布前完整数据库与配置备份：`/opt/newapi/backups/all-local-20260918/`，数据库 gzip 校验通过；原独立回滚镜像 `new-api:rollback-20260915-spend-limit` 内含旧二进制，旧产物仍在 `/opt/newapi/releases/new-api-20260915-spend-limit-linux-amd64`。4 份被 Nginx 重复加载的中转站历史备份已移入该备份目录的 `historical-nginx/`，相关域名冲突消失；不要将历史重复配置放回 sites-enabled。回滚步骤见发布记录，不自动回滚数据库以免覆盖新消费/订单。

### 2026-09-18 — Claude 思考强度日志缺失已本地修复（未部署）

- 根因：`RelayInfo.CaptureEffectiveReasoningEffort` 仅识别 OpenAI 字段和模型后缀，漏读 Claude `output_config.effort`；`ClaudeHelper` 普通/透传分支均缺少采集调用，导致 `other.reasoning_effort` 为空或残留旧值。Default 前端表格和详情已有字段展示支持，无需前端修改。
- `relay/common/relay_info.go` 新增 Claude 字段识别，保持原有 OpenAI 字段优先级、trim/lowercase 与模型后缀兜底；`relay/claude_handler.go` 普通分支在转换、字段过滤与渠道覆盖之后采集，透传分支读取实际原始 body 并处理读取错误。OpenAI→Claude 转换路径复用共享采集器。不改请求参数或计费规则。
- 测试改动为 `relay/common/relay_info_reasoning_test.go`、新增 `relay/claude_reasoning_log_test.go`、`service/billing_outbound_metadata_test.go`。本地 HTTP 上游验证实际出站参数、RelayInfo 与日志 other 一致，覆盖 high/max、模型后缀、渠道覆盖、透传原文、缺省清旧值，并保留 OpenAI/Responses 回归。
- 先复现失败再修复。定向 `go test -race -vet=off ./relay ./relay/common ./service -run '^TestClaudeReasoningEffort|^TestCaptureEffectiveReasoningEffort|^TestPreparePriorityBillingForOutboundCapturesReasoningEffort' -count=1`、全仓 `go test -vet=off ./... -run '^$'`、gofmt、`git diff --check` 与任务补丁反向校验通过。未执行真实供应商联调或生产访问。
- `thinking.budget_tokens` 不等同于命名强度，不做换算；未显式指定 effort 的 adaptive 请求不猜测默认档位；历史未保存字段不能可靠回填。产物在 `/Users/adrian/Documents/Codex/claude-effort-log-20260918/`，含说明、修改前快照、失败/成功日志和 `task-only.patch`（相对此轮开始时工作树，不应重复正向应用）。
- 本轮只改 5 个 Go 文件，保留流式修复与其他未提交改动；未提交、推送、部署或同步恢复副本。用户要求保持本地，发布需另行授权。

### 2026-09-18 — 历史流状态错误本地修复与验证完成（未上线）

- 本任务从 9 月 16 日开始，用户明确要求“修复完后先不上线”。修改仅在本工作树，共 50 个本轮改变的 Go 文件（含 11 个新文件），保留任务开始前已有的 dirty/untracked 修改；未提交、推送、部署或同步恢复副本。
- `relay/helper`：统一检查 SSE/二进制 write、short write、FlushError；公共 `StreamFailure`、协议化错误输出与语义输出标记；scanner 将 DONE 按顺序入队、EOF 后排空处理、BOM/CRLF/多行 JSON/NDJSON、聚合大小限制、非正超时回退，清理前关闭上游并等待所有协程。响应头等待心跳失败取消上游子 context；成功响应 context 延续至 Body.Close。`StreamStatus` 增加 write_error 优先级，logger 共享状态改原子操作以修复 race。
- Responses 原生及 Chat 转换不再吞 `response.failed/error`，保留 stream_read_error 等上游原因；包括 SSE event-only 类型、done/completed 内部 failed/incomplete 状态；提前 EOF/坏 JSON/缺少类型明确失败。已输出后只结束为合法失败事件，不重放或发送成功 DONE；保留既有 Responses 部分输出用量规则。控制器统一重试边界，心跳后按 SSE 写错，二进制输出后不追加 JSON。
- Chat/Claude/Gemini/xAI/Baidu/Dify/图片/音频与旧 Tencent/Cloudflare/Cohere/Coze/Zhipu/Ollama/PaLM 路径传播读取/解析/输出失败。Bedrock SDK 流接入公共 Claude scanner 并传播 stream.Err、绑定取消及等待桥接退出。图片检测坏 JSON/partial 后 EOF；音频二进制读取失败不再成功返回；末尾 DONE 写失败传播。
- 补充 WebSocket：Realtime 的读协程仅提交帧，单事件循环处理 usage/session/write，返回前中断并等待读协程，保留控制器错误写入能力及原有最终用量收尾调用；讯飞同步读取替换旧 channel 循环，完整保留 32 帧并检查错误/终态；火山 TTS 绑定取消、读写截止时间、负序号终态及二进制 payload 长度，检查 FlushError。
- 最终验证通过：Go 1.25.1 的流式定向 `-race`（含真实 HTTP 截断、WebSocket 双向/取消/正常用量、SDK 错误注入）、`go build ./...`、gofmt、`git diff --check`、任务补丁反向校验；`go test -vet=off ./...` 排除以下 7 项已确认基线失败后全部通过：模型列表 tiered 限制、priority surcharge、3 项 OpenAI→Claude 文件转换、图片 JSON 错误文案、图片 multipart stream 参数。全名/原始日志详见报告；没有改动这些用例来隐藏失败。
- 交付目录 `/Users/adrian/Documents/Codex/stream-audit-20260916/`：`流状态错误排查与修复报告.md`、`task-only.patch`、`changed-files.txt`、`verification.json`、`SHA256SUMS`、复现/最终测试日志、修改前源码快照。补丁基于任务开始时脏工作树，不能在当前工作树再次正向应用；快照不能直接解压覆盖当前源码。
- 边界：真实供应商/网络/OOM 故障无法靠网关消除；无强制终态的兼容协议仍接受非空干净 EOF，不能保证语义内容完整；重试终态日志可能包含前次上游终态与后次路由错误，本轮未重设计每次尝试持久化。没有真实供应商全矩阵计费联调、生产候选验收或长时压测。下一步先审阅，未经用户新的部署授权保持本地。

### 2026-09-15 — 用户自定义订阅周期消费限额与隐藏二次确认（本地实现，未部署）

- `UserSubscription` 新增 `spend_limit_period` / `spend_limit_quota` 持久字段，按订阅实例保存可选的 `hour` 或 `day` 周期消费上限；空周期/零额度表示取消限制。该限额只参与请求预扣校验，不修改 `AmountTotal` 或 `AmountCap`，因此不会减少套餐最终可用总额度。
- `model.PreConsumeUserSubscriptionForToken` 在实际预扣前按北京时间计算当前整点或当天 0 点窗口，并从 `subscription_pre_consume_records` 汇总有效用量：`reserved` 计预扣额度，`final` / `provisional` 计最终结算额度，`refunded` 不计；达到/将超过上限时跳过该订阅候选。订阅行锁保证同一实例的并发预扣串行校验。
- 新增用户 API `PATCH /api/subscription/self/instances/:id/spend-limit` 和 `UpdateSelfSubscriptionSpendLimit`，仅允许订阅所有者更新 active 实例；用户订阅摘要新增只读 `spend_limit_used`、`spend_limit_window_start`、`spend_limit_window_end` 用于前端展示当前窗口已用额度。
- Default 订阅管理弹窗新增“周期消费限额”区域：可启用/关闭、选择每小时或每天、输入上限、保存或取消；剩余用量卡片会显示当前周期已用/上限。管理弹窗中的“隐藏此套餐”改为先弹出 `ConfirmDialog`，确认后才调用现有隐藏 API。
- 新增 `model/subscription_spend_limit_test.go`，覆盖北京时间日/小时窗口、预扣上限拦截、配置保存/取消、订阅摘要用额汇总。本轮验证通过：定向 `go test ./model -run 'TestSubscriptionSpendLimit|TestPreConsumeUserSubscriptionEnforcesDailySpendLimit|TestUpdateUserSubscriptionSpendLimit|TestBuildSubscriptionSummariesPopulatesSpendLimitUsage'`；`go test -vet=off ./controller ./router ./service -run '^$'` 编译通过；Default `tsc -b`、修改文件 ESLint、Rsbuild build 和 en/zh JSON 解析通过。
- Go 1.25 `go test ./service` 仍受既有非 constant `fmt.Errorf` vet 报告影响，发布时应继续使用项目现有 `-vet=off` 编译/测试口径或在独立构建中处理基线；本轮未提交、未打包 Linux 产物、未部署生产，也未修改线上数据库或配置。新二进制首次启动时由现有 `AutoMigrate(&UserSubscription{})` 添加两个限额字段。

### 2026-09-15 — 删除虚拟会员的超时收尾尚未实现（只读）

- 当前 `model/virtual_membership.go` 的 `AdminDeleteVirtualMembership` 对任意 pending 预扣记录、pending 续费订单和 pending 主动重置订单均直接报错并阻止删除；没有按 5 分钟 cutoff 自动 `refunded` / `closed` 的逻辑。对应回归测试仍明确断言 pending 预扣会阻止删除。
- 现有超时回收仅位于 `ActiveResetVirtualMembership`：默认按 `TASK_TIMEOUT_MINUTES + 5`（默认 24 小时 + 5 分钟）判断，且只服务主动重置，不进入删除会员链路；历史全 refs 搜索也未发现实现截图三项删除自动收尾的提交。
- 本轮仅做代码、历史提交和测试定位，未修改代码、数据库、构建或生产状态。

### 2026-09-14 — 全量本地改动已蓝绿上线，正式版本 `20260914-all-local`

- 用户明确授权将当前所有未上线本地改动一起上线。本地同时重建 Default/Classic 前端并编译 Linux/amd64 静态二进制：`/Users/adrian/Documents/中转站运营/new-api/new-api-20260914-all-local-linux-amd64`，SHA-256 `b3b2a5168cb6c12469ff8ef69025588f4808b5ff347d6294ee7b2e8df8e4f446`，版本 `20260914-all-local`。构建不包含 `.mimosa` 或历史二进制；源码工作树仍未提交，当前 dirty/untracked 状态需要继续保留。
- 发布前补充了安全检查并纳入本次产物：提现审核只有成功抢占 `pending -> approved/rejected` 的行才处理冻结资金；本金/佣金冻结和审核解冻均增加余额条件，防止并发重复动账；`/v1beta` 流量分类修正；重置日历两个 `useMemo` lint 错误修正。对应定向测试和全仓 compile-only 已通过。
- `site-builder` 生产使用独立候选 `new-api-candidate-all-local-20260914`、127.0.0.1:3010、`NODE_TYPE=slave`、`FRONTEND_BASE_URL` 空值；候选 healthy、restart=0、OOM=false，前端静态资源和受保护 API 401 验收通过。候选承载流量时，正式 3000 已更新 canonical Compose 并重建为新产物；随后 Nginx 实际 `sites-enabled` 配置切回 3000，3010 候选已停止并删除。
- 最终正式 `new-api` healthy、restart=0、OOM=false，仅监听 127.0.0.1:3000，版本 `20260914-all-local`；`token.stellaisle.com`、`api.stellaisle.com`、`direct-token.stellaisle.com` 三个入口各 5/5 返回新版本，首页/发票页/虚拟会员页 200，相关受保护 API 均 401。最近正式日志未发现 panic/fatal/migration failure/database locked/unsupported protocol。
- 发布前备份：`/opt/newapi/backups/all-local-20260914T155304Z/`；旧正式二进制：`/opt/newapi/releases/new-api-20260906-responses-prelude-forward-linux-amd64`；canonical Compose 回滚副本：`/opt/newapi/docker-compose.final.yml.pre-all-local-recreate`；实际启用 Nginx 回滚副本：`/etc/nginx/sites-enabled/new-api.conf.pre-all-local-20260914T155747Z`、`/etc/nginx/sites-enabled/api.stellaisle.com.conf.pre-all-local-20260914T155747Z`。回滚时需同时恢复 Compose 挂载/版本、Nginx upstream 并重建正式容器，保留当前新产物用于再次切换。

### 2026-09-14 — 本地全量改动检查结果（只读，未修复）

- 当前 `new-api` 工作树有 76 个已跟踪修改文件、51 个非 `.mimosa` 未跟踪源码/测试文件、7 个未跟踪 Linux 二进制，以及 5,002 个 `.mimosa` 运行态文件（约 36 MB）。不能使用 `git add -u` 直接提交，否则会漏掉大量未跟踪源码；也不能把二进制和 `.mimosa` 一并提交。
- 验证结果：Go 全仓 compile-only 通过，前端 `tsc -b` 与 Rsbuild 构建通过，新增订阅/发票/用量指标/钱包聚合定向测试和相关 `-race` 通过。完整 `go test -vet=off ./...` 仍有两项既有失败：`controller/TestListModelsTokenLimitIncludesTieredBillingModel` 与 `service/TestPreparePriorityBillingForOutboundReservesTieredPrioritySurcharge`。新增重置日历前端文件 ESLint 有 2 个 `useMemo` inline-function 错误；发票页有 1 个 `useMemo` 依赖 warning。
- 发现待处理实现问题：新增的日志 keyset cursor 目前没有 controller 调用，`GetAllLogsWithContextCursor` / `GetUserLogsWithContextCursor` 实际不可达；`middleware/stats.go` 的先 `/v1` 分支会吞掉 `/v1beta`，Gemini 流量被计入 `relay-v1`；`model/invoice.go` 可开票订单查询限制 5000 条，但注释和“所有历史订单”语义不一致。
- 审阅带修改的提现链路时发现既有资金竞态（非本轮新增）：`FinishWithdraw` 的 `WHERE status=pending` 若 0 行仍返回 nil，controller 不看 `RowsAffected`，并发审核可能重复清冻结或同时退款；`FreezeUserBalance` 也没有“余额足够”条件更新，两个并发申请可能把可用余额扣成负数。发布提现改动前建议一并修复并做并发回归。
- 本轮仅检查、编译、测试和记录，没有修改源码、数据库或生产配置，没有提交或部署。

### 2026-09-14 — 订阅时间统一北京时间与最后周期提示已本地实现（未发布）

- 用户已确认产品口径：订阅相关时间统一按北京时间显示。前端新增 `formatBeijingDate` / `formatBeijingTimestampToDate`，固定 `Asia/Shanghai`，并用于订阅卡片、续费预览、消耗顺序及管理员订阅列表；跨 `UTC`、`America/New_York`、`Asia/Shanghai` 验证 `1789401600` 均显示 `2026/09/15 00:00:00`。
- 后端订阅摘要新增只读字段 `quota_reset_period`、`is_final_reset_cycle`，由套餐快照或存量套餐行计算；仅周期性套餐且没有后续重置边界时标记最后周期，`never` 和已有下次重置均不会误报。
- 用户端两张订阅卡在最后周期显示“当前为最后一个周期，无下次重置”。本地 `go test -vet=off ./model -count=1`、新增摘要回归、`tsc -b`、修改文件 ESLint 和 Rsbuild 构建均通过。
- 本轮未提交、未推送、未打包 Linux 产物、未部署生产，也未修改线上数据库或配置；当前实现只在本地工作树。

### 2026-09-14 — 月卡 #235“中午 12 点重置”定位为浏览器时区展示问题（只读）

- 用户 `sunfengxing`（UID 627）的订阅实例 #235 为套餐 #18、`active`，快照明确为 `daily + quota_reset_anchor=midnight`；实时库值 `last_reset_time=1789315200`、`next_reset_time=1789401600`，换算北京时间分别是 2026-09-14 00:00、2026-09-15 00:00。
- 同一个 `next_reset_time` 在美国东部夏令时显示为 2026-09-14 12:00；用户侧卡片使用 `new Date(next_reset_time * 1000).toLocaleString()`（`web/default/src/features/wallet/components/my-subscriptions-detail.tsx` 与 `subscription-plans-card.tsx`），没有固定 `Asia/Shanghai`，所以会在非北京时区浏览器中把 0 点显示成 12 点。
- 后端重置计算和管理员重置时间均以 `Asia/Shanghai` 为准；`TestCalcNextResetTimeMidnightOverrideAlignsDailyAndWeekly`、`TestMidnightOverrideResetKeepsFutureBoundaryAtMidnight` 已通过。没有发现调度器把 #235 改到中午执行，本轮未改代码、数据库、运行时或生产配置。
- 下一步待确认产品口径：若用户卡也统一展示北京时间，应给上述两处重置时间显示固定 `timeZone: 'Asia/Shanghai'` 并标注“北京时间”；若坚持展示用户本地时间，则应明确标注时区，避免再次被理解为后端锚点漂移。

### 2026-09-13 — Historical code handoff archived and compacted

- All dated code handoff entries through 2026-08-31 were archived to `/Users/adrian/Documents/Codex/中转站运营-CODEX_MEMORY-归档-20260716-20260831.md`; this file was reduced from 492,912 bytes to about 52 KB while retaining the stable context and all 2026-09 entries, including the unreleased `subscription_only` fix.
- No source, test, database, runtime, or production state was changed by this memory-maintenance task.


### 2026-09-13 — "subscription_only" no longer blocks wallet billing in ordinary groups (local fix, not released)

- Bug: `users.setting.billing_preference = "subscription_only"` forced subscription billing for every group in `service.NewBillingSession`. A user with wallet balance but no subscription for the selected ordinary group (production user 388 on group `gpt pro`) received `403 订阅额度不足或未配置订阅: no active subscription` even though wallet billing was allowed.
- Fix: in `service/billing_session.go` the `subscription_only` branch now suppresses wallet billing only inside subscription groups. Plan-designated groups and groups bound to one of the user's active subscription instances stay on the existing hard-boundary path (never wallet); the branch itself keeps subscription billing for an active subscription without a group restriction (`AllowedGroup == ""`, usable in every group) and otherwise falls back to wallet when the group has no subscription entitlement.
- Added `model.HasActiveUnrestrictedUserSubscription(userId)` for that unrestricted-subscription check. It is only queried on the `subscription_only` path, so other preferences keep their previous query count.
- New regression file `service/billing_preference_test.go` (6 cases, own sqlite in-memory DB): ordinary group with only plan-group subscriptions → wallet and wallet quota actually decreases; plan group → subscription; plan group without matching subscription → 403 with no wallet deduction; subscription instance bound to a non-plan group → subscription; unrestricted subscription in an ordinary group → subscription; no subscription at all → wallet.
- Local verification: `gofmt`, `go build -p=1 ./...`, `go test -vet=off ./model/ -count=1` and `go test -vet=off ./service/ -run TestSubscriptionOnly -count=1 -v` all pass with the repo-local Go 1.25.1 toolchain (`/Users/adrian/Documents/中转站运营/.tools/go1.25.1`). The full `./service` package still fails only on the pre-existing unrelated `TestPreparePriorityBillingForOutboundReservesTieredPrioritySurcharge` (expected 200, actual 250) plus the documented Go 1.25 non-constant `fmt.Errorf` vet baseline.
- This worktree already carried other windows' uncommitted changes; the unrelated existing `BillingSession.Settle` zero-delta change and every other dirty file were preserved. No commit, push, artifact upload or deployment was performed, and the production binary still predates this fix; a release must be built from a clean worktree at the deployed baseline plus this fix.

### 2026-09-05 — Full dirty worktree release confirmed live in production

- `site-builder` formal `new-api` currently runs `VERSION=20260905-all-windows`, is healthy with restart count 0 and `OOMKilled=false`, listens only on port 3000, and has no 3010 candidate.
- `/opt/newapi/releases/new-api-20260905-all-windows-linux-amd64` has SHA-256 `73e4612e0ef238c46f4d99333b976457917ab7445b4810676661f752bb8455b4`, matching the local artifact; canonical Compose mounts this artifact.
- `/api/status` on `token.stellaisle.com`, `api.stellaisle.com`, and `direct-token.stellaisle.com` all returned `20260905-all-windows`; no panic/fatal/migration failure/database locked/unsupported protocol appeared in the last 10 minutes of application logs.
- This turn only re-verified production; it did not restart, switch traffic, alter databases, or touch `overseas2`. Log/usage/wallet aggregate read flags remain enabled and background workers remain disabled.

### 2026-09-04 — Virtual-membership Alipay “payment snapshot failed” read-only root cause

- Production `site-builder` currently runs the externally mounted `/opt/newapi/releases/new-api-cpu-v129-linux-amd64-20260904`; this diagnosis only inspected source, Nginx access records, application logs, live orders and payment settings. No code, database, configuration, order, service or traffic was changed.
- At 2026-09-04 14:15:49 CST, the request from `158.101.39.239` to `/api/virtual-membership/epay/pay` returned an 81-byte business-error response; the same source's 14:17:01 retry returned 337 bytes success. Live order #76 (`alipay`, plan #2 single-seat, `money=579`) was created and immediately closed at 14:15:49 while retaining `expected_payment_amount_minor=57900`; order #77 (`wxpay`, plan #2 two-seat, `money=299`, `payment_fee=2.99`, expected 30199 cents) remained pending, matching the fee-dependent behavior.
- Root cause is the snapshot update branch in `controller/virtual_membership_payment_epay.go`: `model.createVirtualMembershipOrderTx` already writes the Epay order's amount snapshot at creation. Alipay has no `fee_rate` in the live `PayMethods`, so `PaymentAmountWithFee` returns zero fee and the same amount; the controller then calls `UpdateVirtualMembershipOrderPaymentExpectation`, whose no-op MySQL UPDATE reports `RowsAffected=0`, interpreted as a failure. The controller closes the order and returns `支付金额快照失败`. The configured 1% WeChat fee changes fields and avoids the false failure.
- This regression was introduced when commit `344b5f1a5d` added payment-channel fees on top of the existing create-time snapshot. The normal subscription path sets its snapshot before insert and does not use the same duplicate UPDATE. Future repair should make the update idempotent/no-op-safe and add a zero-fee Alipay regression; per the user's request, no fix was made in this task.

### 2026-09-04 — Virtual-membership Alipay snapshot false failure fixed locally, not deployed

- `new-api/model/virtual_membership.go` now distinguishes MySQL's no-op UPDATE (`RowsAffected=0`) from a missing/non-pending order in `UpdateVirtualMembershipOrderPaymentExpectation`: it rereads the order and treats the result as idempotent success only when it is still pending and payment fee, amount, currency and commission base all match; fee comparison uses cent precision.
- Added `TestVirtualMembershipPaymentExpectationAllowsIdempotentZeroFeeRetry`, covering an Epay order whose snapshot is written at creation and then repeated with a zero fee while remaining pending. `go test ./model -run 'VirtualMembership' -count=1` and `git diff --check` pass.
- This task changed only the local `new-api` worktree; no commit, push, release artifact, upload or production deployment was performed. Production remains on the v129 artifact pending explicit release authorization.

### 2026-09-04 — v1.2 detailed-log/Token range semantics deployed

- Added v1.2 range semantics and deployed runtime `20260904-cpu-v127`: common detailed logs default to a server-bounded rolling two-hour half-open window; stats expose `effective_start/effective_end`; the Default stats card labels Total Tokens as the current filter range.
- Token list usage remains progressive (`usage_status=loading`); the batch range endpoint enforces ownership, 30s cache/singleflight and a 31-day synchronous limit. It now reads only live detail rows for the selected range so archived full-day buckets cannot over-count arbitrary two-hour or partial-day windows. Legacy today/lifetime fields remain for compatibility, with `range_used_quota` added for the selected-window value.
- Local Docker Linux/amd64 artifact: `/Users/adrian/Documents/中转站运营/new-api/new-api-cpu-v127-linux-amd64-20260904`, SHA-256 `b1d936a13f342daf0074f2d1c80bf80448764d342fb2e7a1dd0e6fb4b366ee22`; Default TypeScript/Rsbuild and targeted token-usage regression passed. Full package runs retain unrelated pre-existing model-list/Redis/test-request failures.
- Production `site-builder` candidate/formal blue-green rollout completed with root-only backup `/opt/newapi/backups/cpu-v127-20260903T203652Z-before/`. Formal `new-api` is healthy, zero restarts, `OOMKilled=false`, only `127.0.0.1:3000` listens, and all three public domains returned `20260904-cpu-v127` 5/5; MySQL/Redis/CPA were not recreated and `overseas2` was untouched.
- Canonical production Compose mounts the v127 artifact. MySQL remains at **3 GiB cgroup cap + 512 MiB InnoDB buffer pool** because the earlier 2 GiB cap hit the documented file-cache stop line; this code release did not force a risky cap reduction.

## Maintenance sections

- Completed: record material code changes and verification.
- In progress: record unfinished code work another window must continue.
- Blockers: record the cause, evidence, and required external condition.
- Next steps: keep only actionable and still-current follow-ups.

### 2026-09-01 — 主动重置仍阻塞的零差额结算根因（只读诊断）

- 生产截图对应会员实例只读核对到 4 条 pending；每条均已有消费日志，日志 quota 与 `pre_consumed` 精确相等，说明请求已结束并记账，不是仍在运行。四条记录约 16–23 小时，尚未达到现有 `TASK_TIMEOUT_MINUTES + 5 分钟` 的默认约 24 小时阈值，所以继续阻塞主动重置。
- 根因在 `BillingSession.Settle`：虚拟会员只有 `delta != 0` 才调用 `funding.Settle(delta)`；当实际消费等于预扣、`delta == 0` 时，`PostConsumeVirtualMembershipDelta(..., 0)` 不会执行，记录永久停在 pending。模型层本来已有 delta=0 时写入 `final_quota=pre_consumed`、状态改 settled 的正确逻辑，但上层跳过了它。
- 当前生产共有 52 条 pending：32 条已有消费日志、20 条无日志；年龄分布为 `<5m` 2 条、`1h–24h` 26 条、`>=24h` 24 条。后续修复应先让零差额也完成虚拟会员结算，并提供“归档已完成记录 + 对真正活动请求显式强制封账”的原子重置流程；不能直接删除 pending。严格意义上的实际用量强制结算在请求仍运行时不可得，如要求精确值必须先取消并等待终态。
- 本轮未修改源码、测试或生产数据。

### 2026-09-01 — 主动重置零差额对账与强制封账已在本地完成（未上线）

- `service/BillingSession.Settle` 现在即使 `delta == 0` 也调用资金源结算，确保虚拟会员 pending 记录进入 `settled`；新增幂等回归测试。
- 用户主动重置在会员行锁和 pending 行锁内先按消费日志（`request_id` + `billing_source=virtual_membership`）自动对账并写入 `final_quota`；混合场景会提交已完成对账后再返回结构化 `settlement_in_progress`，不会扣重置次数。
- 主动重置接口支持 `{force:true}`：对仍无消费日志的请求按 `pre_consumed` 记为 `settled/forced_reset` 后清空窗口并扣 1 次；迟到回调保持幂等。新增 `settlement_reason` 字段由 AutoMigrate 兼容 SQLite/MySQL/PostgreSQL。
- Default 两个主动重置入口改为“普通请求 → 活动请求二次确认 → 强制封账”，API 请求跳过全局业务错误 Toast，消除重复提示。Default `tsc -b`、Rsbuild、模型全测、全仓 compile-only、定向 service/model 测试和 `git diff --check` 通过；完整 `go test ./service` 仍有既有 priority billing 断言失败，与本轮无关。
- 当前源码改动仅在本地工作树，未提交、未推送、未构建正式产物、未部署生产，未修改任何线上数据库记录。交接证据：rollout `01a0428e-53ec-79e3-887d-70d74b5ae925`，memory lines `1828-1830`。

### 2026-09-01 — 主动重置零差额对账与强制封账已部署上线

- 按用户明确授权，本地完整工作树完成 Default/Classic 构建、`go test -vet=off ./model`、全仓 compile-only 和 Linux/amd64 静态构建。正式产物 `/opt/newapi/releases/new-api-20260901-vm-reset-settlement-linux-amd64`，SHA-256 `f5862de122a083d9d505b83c8b0a06822f408ea456e31d8e13fc036761fc2323`。
- 发布前备份位于 `/opt/newapi/backups/vm-reset-settlement-20260901T021008Z-before/`，包含约 2.9 GiB MySQL 全量快照（SHA-256 `07ac3c4dc8e0ad4fd108067f1628fef8268ba9e3f33037f1d6a1086de10fbe6d`）、Compose、Nginx、容器检查和旧产物哈希。生产只重建 `new-api`，MySQL/Redis 未重建。
- 3010 候选健康、0 重启、非 OOM；三域名各连续 5/5 返回版本 `20260901-vm-reset-settlement`，未授权 `/api/user/self` 仍为 401。随后正式 3000 重建健康，Nginx 已归一回 3000，候选已停止删除，当前仅监听 `127.0.0.1:3000`。
- 线上数据库已确认 `virtual_membership_pre_consume_records.settlement_reason` 字段存在；正式二进制内版本返回 `20260901-vm-reset-settlement`，最近 15 分钟启动日志无 panic/fatal/migration failure/oom。线上业务数据未执行批量封账或重置，需用户实际点击流程验证历史记录。

### 2026-09-02 — Responses 同渠道重试补齐完整渠道上下文，候选已验证（未切正式）

- 本轮只新增/修改 `controller/relay.go` 和 `controller/relay_stream_retry_test.go`。`getSameChannelRetryWithCapacity` 在复用 Responses 渠道前按 Channel ID 调用 `model.GetChannelById`，恢复简化 Channel 快照缺失的 Key/BaseURL；回归测试验证 `channel #55` 能恢复真实 Key、`http://127.0.0.1:18097` BaseURL 并写回上下文。
- 本地使用 Go 1.26.4 通过控制器定向回归与 `go test -vet=off ./... -run '^$'` 全仓编译检查，`git diff --check` 通过。Linux/amd64 产物为 `/Users/adrian/Documents/Codex/new-api-20260902-same-channel-retry-linux-amd64`，SHA-256 `dd993b452cbfea34f5d31bd935d5e9b1227f9bc52f30428583423cb34623f4f7`。
- site-builder 候选容器 `new-api-candidate-20260902` 挂载同哈希产物，版本 `20260902-same-channel-retry`，healthy、0 重启，监听 `127.0.0.1:3010`；对 #55 的候选探测可稳定观察到真实账号并发 429，但候选最近 60 分钟没有 `unsupported protocol`、相对 URL、`do request failed` 或 500。
- 正式服务仍运行 `20260901-vm-reset-settlement` 于 `127.0.0.1:3000`。本轮尚未提交/推送、未切换 Nginx、未重建正式容器，工作树中的其他既有修改保持不变。

### 2026-09-02 — Responses 同渠道重试修复已正式上线

- 用户确认上线后，正式 Compose `/opt/newapi/docker-compose.final-same-channel-retry-20260902.yml` 增加 `VERSION=20260902-same-channel-retry`；仅重建 `new-api`，MySQL、Redis 和 CPA2 未重建或改配置。正式容器挂载 `/opt/newapi/releases/new-api-20260902-same-channel-retry-linux-amd64`，SHA-256 `dd993b452cbfea34f5d31bd935d5e9b1227f9bc52f30428583423cb34623f4f7`。
- 正式容器 `/api/status` 返回 `20260902-same-channel-retry`，状态 `healthy`、重启 0、`OOMKilled=false`；Nginx 已从 3010 优雅回切 3000，`nginx -t` 成功，当前仅监听 `127.0.0.1:3000`，候选 `new-api-candidate-20260902` 已在连接自然归零后移除。
- 三个现役公网域名 `/api/status` 连续 5/5 返回新版本，未授权 `/api/user/self` 均为 401；切流后的真实 #55 Responses 请求出现 `response.completed`/HTTP 200，最近正式日志未再出现 `unsupported protocol scheme ""`、相对 URL、panic、fatal 或 migration failure。
- 发布前备份目录为 `/opt/newapi/backups/release-same-channel-retry-20260902T025314Z-before/`；本轮源码仍未提交/推送，主工作树中其他既有 dirty 修改保持不变。

### 2026-09-02 — `do request failed` 大面积 500 根因复核（只读）

- 生产实时核对确认 CPA2（`cpa2.service`，`172.20.0.1:18097`）在 9 月 1–2 日共 5 次达到 `MemoryMax=3G` 并被 cgroup OOM killer 杀死：09-01 11:34:31、14:19:40；09-02 09:27:56、10:33:44、10:51:53（CEST）。内核记录 `Memory cgroup out of memory: Killed process ... (cli-proxy-api)`；服务自动重启，当前 active/running、`NRestarts=5`，峰值约 3,221,237,760 字节、上限 3,221,225,472 字节。
- CPA2 被杀期间 `18097` 短暂不可用，New API 的连接拒绝/EOF/context canceled 最终被包装成 `status_code=500, upstream error: do request failed`，解释了故障的集中爆发与自动恢复后的正常。最近窗口还存在 CPA2 上游 `502 Our servers are currently overloaded` 和下游 `context canceled`，不能把所有 500/502 都合并为 OOM。
- 当前生产代码仍保留 Responses 在无独立备用渠道且尚未转发 typed event 时的同渠道重试；`common.RetryTimes=3` 最多产生 4 次外层尝试。#55 无并发/RPM限制，CPA2 为 `request-retry: 3`、`max-retry-credentials: 0`、25 个账号；长请求重放和 CPA2 内部账号轮次叠加会放大内存/上游压力。已确认的完整 Channel reload 修复没有制造 URL 为空问题，生产未再出现相对 URL/`unsupported protocol scheme`。
- 公开 [CLIProxyAPI Issue #5265](https://github.com/router-for-me/CLIProxyAPI/issues/5265) 与当前短 429 冷却导致重试风暴/OOM 的形态高度吻合；[PR #5271](https://github.com/router-for-me/CLIProxyAPI/pull/5271) 仍 open、目标为 `dev`，v7.2.147 release changelog 未包含该修复。
- 本轮仅做只读诊断，未修改代码、生产配置、数据库或容器。后续应先限制/取消 #55 同渠道外层重放并设置有限并发，再降低 CPA2 内部 `request-retry`、增加 429 最小冷却和账号轮次限制；提高 `MemoryMax` 只能作为缓解，不能替代重试治理。

### 2026-09-02 — #55 同渠道最多 1 次、CPA2 8G/retry=2 已上线

- 按用户确认，New API 的 Responses 同渠道重试限制为最多 1 次：外层循环只在 `sameChannelRetryCount < 1` 时允许 `getSameChannelRetryWithCapacity`，完整 Channel reload 修复保留；新增 `shouldUseSameChannelRetry` 与回归测试。本地 Go 1.26.4 控制器定向测试、全仓 compile-only、`git diff --check` 通过。
- Linux/amd64 产物 SHA-256 `ef165ecb5ef28cc3b467a4c3a5638532ae4246d63d84c5b48a3b5439d6e7043d`；正式 Compose `/opt/newapi/docker-compose.final-cpa2-memory-retry1-20260902.yml`，VERSION=`20260902-cpa2-memory-retry1`。候选经独立 Compose 项目 `newapi-cand-cpa2-memory-retry1` 验证后切回 3000，正式 healthy、0 重启、非 OOM，三公网入口返回新版本。
- 生产 CPA2 运行参数已改：`MemoryMax=8G`、`request-retry=2`，服务 active/running、25 账号加载、`/healthz` 200。变更前快照位于 `/opt/cpa2/backups/cpa2-memory-retry1-20260902T095217Z/` 与 `/opt/newapi/backups/cpa2-memory-retry1-20260902T1003Z/`。
- 过程记录：默认 Compose 项目内重建候选曾导致 3000 短暂无监听并出现 502，已立即恢复并改用独立项目完成发布；3010 候选容器仍在自然排空阶段，连接归零后需移除。源码仍未提交/推送，其他既有 dirty 修改保留。
- 后续更新：3010 候选已在活动连接归零后停止并移除，正式仅监听 127.0.0.1:3000；Nginx 已回切 3000，三公网入口 200，CPA2 active/running、`MemoryMax=8G`、`request-retry=2`。

### 2026-09-02 — 号池公开、发票中心与虚拟会员重置日历本地实现完成（未部署）

- 号池：`service/cpa_platform_usage.go` 支持 CPA1 与可选 CPA2 并行抓取、合并匿名账号与模型用量；账号 `plan_type` 不再序列化，新增粗粒度 `source`。Default 号池卡片已移除账号等级标签。
- 发票：新增 `model/invoice.go`、`controller/invoice.go`、Default 发票中心和 RootAuth 发票审核路由。仅成功、外部支付且 `amount>0` 的钱包充值可申请；实际支付快照优先、历史订单兼容回退；一笔 TopUp 由唯一索引最多占用一张发票，多订单仅允许同币种合并；申请/审核通知失败不回滚状态。
- 重置日历：新增 `VirtualMembershipResetCalendarEntry`、用户/管理员 API 与 Default 日历/维护面板；月度次数只汇总管理员填写的 `count`，展示和输入统一按 `Asia/Shanghai`，不读取主动重置按钮次数。
- 迁移已加入发票表与日历表 AutoMigrate/Fast migration；新增发票、日历、CPA 回归覆盖筛选、重复占用、跨用户/跨币种、审核状态更新、月份边界、管理员次数和匿名字段。
- 本地验证：Go `go build ./...`、`go test -vet=off ./model -count=1`、发票/日历/CPA 定向测试、全仓 compile-only、Default `tsc -b`、Rsbuild、平台用量 Node 回归和 `git diff --check` 均通过。版权检查仅剩既有无关文件头漂移。
- 当前改动未提交、未推送、未构建正式发布产物、未部署生产；工作树中的其他窗口既有修改保持不变。

### 2026-09-03 — 号池公开、发票中心与虚拟会员重置日历已蓝绿上线

- 本地完整工作树已通过 Go model/service/controller 定向测试、Default/Classic 前端生产构建、Default TypeScript、Go Linux/amd64 静态构建与 `git diff --check`；正式产物 `/opt/newapi/releases/new-api-20260903-invoice-reset-calendar-linux-amd64`，SHA-256 `5a8bfb625a1b0d0ef0d713aab17bdbf695bce4f96044e8ef99577bcfdf2fab`。
- 发布前 root-only 备份为 `/opt/newapi/backups/release-invoice-reset-calendar-20260902T162302Z-before/`，包含约 3.68GB MySQL 一致性快照、现用/候选 Compose、Nginx、容器检查、旧二进制哈希和最终校验清单；最终 `SHA256SUMS.final` 哈希为 `c33781e2deeb8b9e51c08467c2742ddf4b89c722f4bcd17c96041c0f4eeaf9ca`。
- 独立 3010 候选已接入 CPA1/CPA2 只读管理配置并通过 healthy、0 重启、非 OOM 验收，启动日志确认 `instances=2`；候选迁移创建 `invoice_applications`、`invoice_application_orders`、`virtual_membership_reset_calendar_entries` 三张表，未写入业务申请或日历记录。
- Nginx 按 3010 候选 → 三域名连续 5/5 验收 → 3000 旧连接自然归零 → 正式 3000 重建 → 回切 3000 → 候选排空移除完成。正式 `new-api` healthy、0 重启、`OOMKilled=false`，MySQL/Redis 未重建；仅监听 3000，canonical Compose 已指向新产物并包含 CPA2 接线。
- 三个公网域名 `/api/status` 均连续 5/5 返回 `20260903-invoice-reset-calendar`，`/console` 均为 200；发票/重置日历 API 未登录均为 401；Nginx `nginx -t` 通过。发布后无 panic、fatal、migration failure、database locked 或 unsupported protocol；有一条既有渠道探针 context deadline，不影响发布健康状态。
- CPA2 生产服务 active/running、`MemoryMax=8G`、`NRestarts=0`、`/healthz` 200；CPA1 今日模型摘要接口返回 200（5 个模型），CPA2 对应插件摘要接口返回 404，因此模型统计可能保持 partial；账号容量读取与 CPA1/CPA2 账号合并不受影响。源码仍未提交/推送，主工作树其他 dirty 修改保持不变。

### 2026-09-03 — CPU 92.6%/503 与 token 统计慢查询只读诊断

- `middleware/performance.go` 的 `SystemPerformanceCheck` 对 `/v1` 请求读取 `common.GetSystemStatus()`；当 `int(status.CPUUsage) > CPUThreshold` 时返回 `system_cpu_overloaded`/HTTP 503。`common/system_monitor.go` 的后台监控每 5 秒调用 `gopsutil/cpu.Percent(0,false)`，因此用户看到的 92.6%/90% 文案是 New API 自身的 CPU 门禁，不是 Nginx 或上游响应。
- 生产近 2 小时 GIN 503 以 `/v1/responses` 为主（1,234 条，另有 `/v1/chat/completions` 10 条），Responses 503 中位耗时约 0.197 ms、P95 约 0.553 ms；这种微秒级快速返回与 middleware Abort 一致。应用日志没有逐条记录错误 JSON，故统计用于确认模式而非重构每条正文。
- 503 文案中的 `(MISSING)`/`%!` 是独立格式化问题：`performance.go` 把含字面 `%` 的已翻译字符串直接交给无参数 `fmt.Errorf`；不影响 CPU 门禁判断，本轮未修复。
- 直接负载来源是重复并发的 `/api/token/?keyword=softcodex-*` 请求（同一来源每轮约 4 个搜索，单轮约 1 分 40 秒–2 分钟）。`controller/token.go` 会调用 `model.AttachTokenUsageStats`；`model/token_usage_stats.go` 随后对 `logs` 做今日、生命周期和归档三次 `GROUP BY` 聚合。实时 processlist 同时观察到 5 条相同日用量查询运行 59–68 秒，慢 SQL 日志记录同类查询约 86–93 秒/生命周期查询约 14.5 秒。
- 生产 `logs` 约 996,523 行（数据约 2,996.8 MiB、索引约 770.5 MiB）。EXPLAIN 显示日聚合使用单列 `idx_logs_token_id`、估算扫描约 1,008,732 行（过滤率 0.09%）；生命周期聚合估算扫描约 504,366 行并使用临时表。当前索引没有覆盖 `(token_id,type,settled,created_at)` 的复合索引，这是本轮 CPU/数据库压力的主要代码级线索。
- 资源核对：`new-api` 当前约 1.05–1.26 GiB/8 GiB、重启 0、未 OOM；CPA2 为 systemd `cpa2.service`（不是 Docker 容器），当前约 2.84–2.95 GiB/8 GiB、历史峰值约 5.20 GiB、`NRestarts=0`；`cpa2-manager-plus` 约 556–578 MiB/1 GiB、无 OOM kill。MySQL 容器仅 1 GiB 上限、当前约 881 MiB，且今日 09:11 因 memcg OOM 杀过 `mysqld` 后自动重启；InnoDB buffer pool 仅 128 MiB、`pages_free=0`、`Innodb_buffer_pool_wait_free=40,579`，应优先处理。
- 12:33（服务器时区）追加快照：load 8.17/8.58/8.58，Docker stats 瞬时 `new-api` 155.8%、`new-api-mysql` 214.6%、`cpa-manager-plus` 95.7%、`cpa2-manager-plus` 14.2%；随后 Manager 回落到约 8.6%，显示其 CPU 为突发贡献。`/api/token` 四路轮询仍持续，最近一轮响应约 1 分 39 秒–2 分 21 秒。
- 日志页后续只读核对：Default `getDefaultTimeRange` 和 Classic 日志 hook 当前都默认“当天 00:00 到当前后 1 小时”；普通日志打开会做列表计数/分页读取，并自动调用统计接口。生产近 3 小时匿名汇总中，`/api/log/stat` 约 47 次、平均约 8.7 秒、最慢约 17.7 秒；`/api/log/self/stat` 约 61 次、平均约 3.1 秒；管理员列表约 65 次、平均约 2.1 秒。同期 `/api/token/` 约 159 次、平均约 11.1 秒，日志页是显著次级贡献者而非 token 热点的替代解释。
- 数据库当前窗口内，最近 2 小时约 34,541 条日志/34,083 条消费日志，最近 24 小时约 217,863/213,927 条；滚动 2 小时输入行约少 84%。无其他筛选的消费统计 EXPLAIN 从 24 小时的全表扫描（估算约 117 万行）变为 2 小时的范围索引（估算约 6.7 万行）。建议后续同时修改 Default/Classic 默认范围与 `/api/log`、`/api/log/stat` 无时间参数时的服务端回退，保留显式历史查询；目前尚未改代码或生产。
- MySQL 仍是 1 GiB cgroup/128 MiB buffer pool；后续快照 `memory.current` 约 1.061 GB、几乎触顶，`memory.events max` 约 1026 万、buffer-pool `wait_free` 约 4.9 万。用户提出 2 GiB cgroup/1 GiB buffer pool 目标；1 GiB pool 相比当前会增加约 896 MiB 常驻预算，在 2 GiB 限额内偏紧，实施前应先降查询负载并分阶段观察。
- 本轮只读检查生产日志、运行态、MySQL EXPLAIN/状态和源码，未修改代码、配置、数据库、容器或流量。建议先治理 token 页面轮询/并发，再在备份低峰窗口增加复合索引并重新评估 MySQL cgroup/buffer pool；不要仅提高 90% 门槛。

### 2026-09-04 — CPU 优化 PRD 待审批，源码未实施

- 运营根目录已生成审批稿 `/Users/adrian/Documents/Codex/中转站CPU负载系统性优化_PRD.md`。本轮只把源码核对结果整理为候选方案：Token 列表/usage 解耦、日志默认 2 小时、查询取消/舱壁、复合索引、`RecordConsumeLog`/`quota_data` 热路径计量、重试/后台任务预算、CPU 分层准入和 MySQL 内存分阶段；没有修改源码或生产。
- 关键代码事实需以生产挂载二进制重新核验：`/api/token/` 兼容根路径不读取 `keyword`，正式搜索路由为 `/api/token/search`；`queryUsedQuota` 的 `logType/settled` 语义、日志统计重复扫描、连接池默认值和 `RecordConsumeLog` 同步余额/日志写入均列入回归与性能验证。
- 当前工作树仍有其他窗口既有 dirty 修改；本 PRD 不代表这些修改已测试、提交或上线。后续若获批，先在本地/独立环境验证和构建，再按生产主机身份门禁、备份、低峰、候选和回滚流程执行。

### 2026-09-04 — CPU 优化 PRD v1.1 的代码审查补充

- 代码审查补充了几个待量化热点：`GetAllTokens`/搜索的 `CountUserTokens` 与模糊匹配 COUNT；`GetUserLogs`/`GetLogByTokenId` 的 `formatUserLogs` 对 `Other` 逐行解析/重序列化；`common/relayGoPool`、`RelayCtxGo`、`gopool.Go` 的近乎无界任务提交；以及 `DataExportEnabled` 下 `LogQuotaData`/`SaveQuotaDataCache` 的队列和锁写放大。
- `SetRelayRouter` 当前先经过 `APIIngressResolver → DecompressRequestMiddleware → BodyStorageCleanup → StatsMiddleware`，CPU 门禁随后按多个 relay route class 挂载。任何“解压前准入”必须先验证鉴权/请求体依赖并按 `/v1`、`/mj`、`/suno` 等 class 分层，不能直接交换中间件顺序。
- Token usage 批量端点在 PRD 中改用示例路径 `/api/token/usage-stats` 或 `/api/token/usage/batch`，避开现有 `/api/usage/token/` 与动态路由；复合索引验证需先读取实际 MySQL 版本，不能假设 `INVISIBLE INDEX`/`EXPLAIN ANALYZE` 可用。
- 以上均为待审批、待本地/独立环境验证的方案；本轮未修改这些代码，也未改变生产运行态。

### 2026-09-04 — CPU 优化 PRD v1.2 用户决策输入

- 用户确认目标主机/公网映射仍为 `site-builder`；Token usage 允许渐进加载并在迁移期显示 `loading/stale`。
- 普通详细日志默认窗口定为最近 2 小时；Token 总用量继续是当天 00:00–当前的“今日累计”，不能把两种时间范围混为一个指标。今日累计 CPU 优化依赖 usage 解耦、复合索引、批量/缓存或日聚合，不会因明细窗口缩短自动下降。
- 企业用户共享 API Key，因此不做按 Key/IP 的硬限流或封禁；保护改为全机、渠道、route class、请求类型和重试预算级别。详细日志保留当前默认策略。
- MySQL 第一阶段目标确定为 2 GiB cgroup cap + 512 MiB buffer pool；Manager SQLite 清理/VACUUM/压缩暂缓。查询取消与 SQLite/WAL/SHM/key 恢复演练仅作为后续内部技术验证，不代表本轮代码实现。
- 以上产品决策已写入运营目录 PRD；本轮仍未修改源码或生产运行态。

### 2026-09-04 — v1.2 普通日志两小时默认已在本地实现

- 本地修改 `controller/log.go`：`/api/log`、`/api/log/self`、`/api/log/stat`、`/api/log/self/stat` 对缺失、非法、倒置或明显未来的时间参数回退最近两小时；财务流水接口保持原有 30/31 天逻辑。统计响应新增 `effective_start`、`effective_end`、`usage_label=selected_range`。
- Default 普通日志筛选器与查询参数改为最近两小时默认；Drawing/Task 继续使用旧 helper。Classic 普通日志表单与无表单值回退也改为最近两小时。
- 新增后端时间范围归一化回归测试和 Default 前端时间范围测试；本地 Default TypeScript 检查通过，`git diff --check` 通过。当前环境没有 Go/gofmt/前端测试运行器，Go 测试与前端 node:test 尚未执行。
- 本轮仅修改本地源码和运营 PRD，未提交、未推送、未构建正式产物、未改生产数据库/索引/容器/Nginx/内存/流量。Token 总用量“跟随详细日志筛选范围”的产品口径已同步修订 PRD；Token 管理页旧字段尚未改为范围统计，需后续设计共享筛选上下文或明确仅指日志统计卡片。

### 2026-09-04 — CPU v1.2 首批优化已上线 site-builder

- 本地完成并验证：普通日志最近两小时默认及服务端兜底、统计 effective range、Token 列表 usage 解耦与批量用量接口（30 秒缓存/singleflight、归属校验）、logs 复合索引定义、负数分页保护、CPU 503 文案格式化修复、详细日志统计全局并发舱壁。
- Go 定向测试（`-vet=off`）通过，Default/Classic Rsbuild 和 Default TypeScript 通过；Linux/amd64 产物 SHA-256 为 `22f8c7018a3b114a28ad84479c0f33ebeea23f5adbbae976be7c654159799448`。
- 生产 `site-builder` 已完成备份 `/opt/newapi/backups/cpu-v12-20260903T184157Z/`（MySQL dump SHA-256 `e08f1abc1533c81ea188737e5bb4a808c43e4561666a029059433d141ce429ac`），候选端口 3010 验收后切回正式 3000；三公网域名连续 5/5 返回 `20260904-cpu-v121`，正式 new-api healthy、0 重启、非 OOM。
- MySQL 8.2 已创建 `idx_logs_token_usage(token_id,type,settled,created_at)`，buffer pool 在线调整为 512 MiB。2 GiB cap 下 cgroup 长期贴顶（主要为可回收文件页缓存），按停止线临时提高为 3 GiB 并同步 canonical Compose；当前 `memory.events oom=0/oom_kill=0`、Threads_running=2。后续需观察 24–72 小时，再决定是否能安全回落 2 GiB。
- 当前生产版本为 `20260904-cpu-v121`，正式挂载 `/opt/newapi/releases/new-api-cpu-v121-linux-amd64`；未触碰 `overseas2`、CPA 配置或 Manager SQLite。源码仍未提交/推送，工作树其他 dirty 修改保持不变。

### 2026-09-04 — CPU v1.2 v123 正式版本完成上线

- 在 v121 基础上补齐 API Key 页最近日志范围记忆：Default 普通日志筛选器将范围写入 `new-api-usage-logs-range`，Keys 页批量 usage 查询读取该范围；无范围时安全回退最近 2 小时。
- 本地重新执行 Go 定向回归（`-vet=off`）、Default TypeScript、Default/Classic Rsbuild 和 Linux/amd64 静态构建；最终产物 `/opt/newapi/releases/new-api-cpu-v123-linux-amd64` SHA-256 `aa935218b66c326ae49922c559304fec8f7a73aaca44d3d698ec96c88eb49cb9`。
- 生产候选 `3010` 健康后切换正式 `3000`，三域名均返回 `20260904-cpu-v123`，正式容器 healthy、0 重启、非 OOM；候选已移除。canonical Compose 已持久化 `VERSION=20260904-cpu-v123`、MySQL `mem_limit=3g`、`innodb-buffer-pool-size=512M`。
- 当前 MySQL 复合索引存在，buffer pool=512 MiB，`Threads_running=2`，`memory.events oom=0/oom_kill=0`；2 GiB cap 曾持续贴顶，因此按停止线保留 3 GiB 上限，后续需 24–72 小时观察再决定回落。

### 2026-09-04 — CPU v1.2 v125 保留任务修复并完成正式上线

- `model/log_retention.go` 已将详细日志归档 OR 条件拆为 error 与 settled consume 两条索引友好查询，按最早 `id` 合并批次；`service/log_retention_task.go` 增加每批 3 秒 context deadline、整轮 10 秒预算，避免高负载时连续占用数据库。
- 本地 Go 1.26.5 容器 gofmt、`go test -vet=off ./model ./service` 定向回归、全仓 `go test -vet=off ./... -run '^$'` 和 `git diff --check` 通过；产物 `/Users/adrian/Documents/中转站运营/new-api/new-api-cpu-v125-linux-amd64-20260904` SHA-256 `acf2a628115cb873e9dac6ef896d989dd4ec79d87fe399d12a5cd1e2ae2a7914`。
- 生产正式版本为 `20260904-cpu-v125`，canonical Compose 已挂载 `/opt/newapi/releases/new-api-cpu-v125-linux-amd64-20260904`；候选 3010 验收后切回 3000 并移除候选。正式 `new-api` healthy、0 重启、非 OOM；三公网域名 `/api/status` 均返回 v125。MySQL/Redis 未重建，其他窗口既有 dirty 修改未覆盖。

### 2026-09-04 — v128 候选切换事故与恢复（502 事件）

- 本地 v128（`20260904-cpu-v128`）Linux/amd64 产物 SHA-256 `9d97437f3137bc5ad4ca08b1ea7d049de6ddabc5ac4011fa5784246a31e1bf84` 已构建并上传，`VERSION` 已恢复为空。
- 事故根因：`/opt/newapi` 下候选 Compose 与 canonical Compose 同属目录项目 `newapi`、服务名同为 `new-api`，候选 `up -d --no-deps` 被 Compose 视为同一服务替换，删除正式 3000 容器后 502。
- 恢复：以 canonical `/opt/newapi/docker-compose.final.yml` 重建正式 v127，`new-api` healthy、restart=0、OOM=false、仅监听 3000；发布前备份 `/opt/newapi/backups/cpu-v128-20260903T210341Z-before/` 完整且 gzip -t 通过。三公网域名 api/token/direct-token `/api/status` 各 5/5 返回 200。
- 后续 v128 蓝绿必须使用独立 compose 项目名（如 `docker compose -p newapi-cpu-v128-candidate`）承载 3010 候选，避免与正式项目 `newapi` 共享服务身份。

### 2026-09-04 — v129 上线：抑制 axios 主动取消 toast，v1.2 正式部署

- 本地修改 `web/default/src/lib/api.ts`：响应拦截器对 `axios.isCancel(error)` 直接放行，不再把 route/unmount/filter 变化导致的主动取消弹成 `canceled`。日志列表与统计、API Key 用量已接入 `AbortSignal`，故此修复覆盖实际触发面。
- 产物 `new-api-cpu-v129-linux-amd64-20260904` SHA-256 `5ad20025198e71556e55aa03361dc9b6bdeda96f68c9f90ab2a551ef366f1e40`；Default `tsc -b` 与 Docker Linux/amd64 构建通过，镜像内版本 `20260904-cpu-v129`。
- 生产候选以 `docker compose -p newapi-cpu-v129-candidate -f docker-compose.cpu-v129-candidate.yml up -d --no-deps new-api` 启动，与正式项目 `newapi` 隔离，未再触发正式容器替换。完整蓝绿（备份→3010 候选验收→Nginx 切 3010→3000 排空重建→回切→移除候选）完成。
- 当前正式 v129 healthy、restart=0、OOM=false，canonical Compose 已持久化 v129 挂载与版本；三公网域名 `/api/status` 各 5/5 返回 `20260904-cpu-v129`。MySQL/Redis 未重建，VERSION 已恢复为空。

### 2026-09-04 — 大日志用户查询弹“当前用户的日志查询正在处理”根因（只读）

- 现网 `site-builder` 正式 `/api/status` 为 `20260904-cpu-v129`。管理员从 Safari 查询 `username=aihub`、`type=0`、整日范围时，`/api/log/` 的 COUNT/分页和 `/api/log/stat` 的聚合均实测约 4.3–5.0 秒；慢 SQL 分别为 `SELECT count(*) FROM logs WHERE username='aihub' ...` 与按 username/type/created_at 的 SUM，均受 5 秒 context deadline 约束。
- 截至本次核对，`aihub` 在该日期范围有 50,662 行、生命周期 386,526 行。MySQL 8.2 的 EXPLAIN 对 COUNT 与 SUM 都选择单列 `idx_logs_username`，估算扫描约 627,960 行并 `Using where`；现有 `idx_user_type_created_at` 以 `user_id` 为首列，不能加速管理员按 username 的查询。
- `controller/log.go` 的 `detailedLogQueryDeadline` 使用全局 2 槽位加按 `c.GetInt("id")` 的单用户非阻塞锁；管理员查询按操作者 ID 归组。第二个并发 `/api/log/` 或 `/api/log/stat` 会立即返回 429 文案“当前用户的日志查询正在处理，请稍后重试”，而持锁查询达到 5 秒则返回另一类“日志查询超时”429。
- 生产 GIN/Nginx 记录已捕获同一管理员 IP 在 `aihub` 全日查询期间的 429：有立即返回约 1–2ms 的锁冲突，也有约 5.0s 的超时；同一页面还会分别请求列表与统计，因此慢查询时两者重叠即可触发截图提示。未修改源码、数据库、配置或流量。

### 2026-09-04 — 日志查询超时本地修复（未上线）

- `model.Log` 新增管理员查询索引：`idx_username_created_at(username,created_at,id)` 支持按用户名的列表/COUNT/排序，`idx_username_type_created_at(username,type,created_at)` 支持统计聚合；索引回归已补充。
- Default 日志列表查询关闭 React Query 自动重试和窗口焦点重取，并把 AbortSignal 传入请求；统计查询等待列表请求（含路由预取）完成后再启动，同样关闭自动重试，避免列表/统计重叠触发后端单操作者锁。
- 本地验证：Go `controller` 时间范围回归、`model` 日志分页/统计缓存/索引回归与 model compile-only 通过；Default `tsc -b`、定向 ESLint、Prettier、Rsbuild 通过；`git diff --check` 通过。
- 本轮只修改当前本地 dirty 工作树，未提交、未推送、未构建生产产物、未修改线上数据库/索引/配置、未重启或切换生产流量。现有其他窗口修改保持不变。

### 2026-09-05 — 大陆 IP 白名单取消单账号数量限制（仅本地）

- `model/identity_access.go` 已移除 `MainlandIPAllowlistMaxPerUser=10`、`ErrWhitelistLimit` 及新增白名单前的数量检查；同一企业/教育身份账号现在可继续添加任意数量的不同 IP，重复地址仍保持幂等，身份校验、精确地址和数据库唯一约束不变。
- `model/identity_access_test.go` 新增回归测试，验证同一账号连续添加 25 个不同 IP 均成功且有效记录数为 25。
- 本轮仅修改本地源码，未提交、未推送、未构建或部署生产，未修改线上数据库/配置/流量。当前环境未安装 Go/gofmt，已完成 `git diff --check`；Go 测试尚未执行。

### 2026-09-05 — 双机日志架构评估与代码边界（只读）

- `model/main.go` 的 `LOG_SQL_DSN` 只建立一个独立 `LOG_DB` 连接；`NODE_TYPE=slave` 仅跳过迁移/部分后台任务，不会把日志读请求自动路由到副本。当前 `site-builder` 与 `overseas2` 均未设置 `LOG_SQL_DSN`。
- `model/log.go` 的 `RecordConsumeLog` 仍在请求热路径同步执行 `LOG_DB.Create`；迁移到跨区日志库会增加至少一次网络往返和远端 INSERT/索引/redo 等待。日志列表查询通过 `LIMIT page_size+1`（服务端 page size 上限 100）返回当前页，统计接口主要返回聚合标量；数据库内部扫描的未返回行不会经网络传给应用。
- 因此“本地日志主库 + 远端异步只读副本”可避免每次写入跨区 RTT，但只能卸载副本上的读扫描，不能卸载主库写入和索引维护；要真正生效必须增加独立读端点/路由（或谨慎配置 ProxySQL），并处理复制延迟、读后写一致性和副本故障回退。此前针对管理员大用户查询的组合索引及前端请求编排修复仍是本地未上线改动，与本次架构评估无关。
- 不能把“只返回分页结果”套用到全部 `LOG_DB` 使用者：`model/platform_usage.go`、`model/log.go` 的部分平台用量/财务汇总路径通过 `Rows()` 将匹配字段逐行拉到 Go 端再聚合；`dashboard_traffic.go` 也会读取明细/聚合行。若日志库跨区，这些路径可能产生较大结果集和额外应用 CPU，需在迁移前单独盘点。
- 本轮未修改源码、未提交/推送、未构建或部署生产。

### 2026-09-05 — 钱包消费流水专用聚合本地实施完成（未上线）

- 新增 `model/wallet_consume_daily_aggregate.go` 与 `service/wallet_consume_aggregate_task.go`：按服务端自然日聚合 `LogTypeConsume` 的 request/quota，并保存当天最后一笔 `(created_at, log_id)` 的可空 `balance_after`；覆盖表、checkpoint、幂等整日替换和 master-only 有界 worker 均已实现。
- `GetUserFinancialConsumeDailyWithContext` 在 `WALLET_CONSUME_AGGREGATE_READ_ENABLE=true` 且完整封存日覆盖可用时优先读专用表；当前日、partial day、覆盖缺失、schema/查询错误均回退旧精确路径。入口已统一处理 nil context。worker/read 默认 feature-off，不能因启动或发日志自动扫描/写聚合。
- `model/log_retention.go` 为 `UsageLogDailyAggregate` 增加 `LastLogId` 并在归档聚合时保留源日志 ID；`model/usage_metric_migration.go` 的显式迁移包含旧归档表和钱包三张投影表。钱包投影 source version 为 `wallet-consume-v2`，checkpoint 为 `wallet-consume-daily-v2`，避免从旧版本水位误判已回填。
- 归档 schema 探测使用受 mutex 保护的按 `*gorm.DB` 缓存；旧归档 `last_log_id=0` 使用归档行 ID 兼容兜底。原始 `logs` 仍是审计真相，钱包聚合只用于展示读取，不参与扣费/结算。
- 回归覆盖空日、partial day、raw/archive 重叠、同秒 ID、NULL/0 余额、归档边界、幂等 checkpoint、nil context 和显式迁移。通过钱包/财务流水/归档/迁移定向测试（含 `-race`）、完整 model 测试、全仓 compile-only、gofmt 与 `git diff --check`。
- 本地云端快照验证环境由隔离 MySQL 容器提供；代码与数据库均未提交、推送或上线，生产 `site-builder`/`overseas2` 未执行 DDL、回填、重启或流量切换。历史归档缺少源 ID 的严格同秒对账仍需后续受控重建后再宣称完成。

### 2026-09-05 — 日志/钱包聚合收尾与本地验证状态

- 钱包 reader 已改为按自然日局部 fallback：完整 coverage 日读取专用聚合，缺失/失效/partial 日只合并对应 legacy 区间；coverage 或投影查询出错时整段回退旧路径。overlap 使 coverage 失效时若 UPDATE 失败会显式返回错误，不再静默保留旧 complete 标记。
- 钱包 worker 采用自然日 `next_day_start` checkpoint；每个成功日立即持久化，取消/超时不会错误推进游标或 `LastReconciledAt`。相关局部 fallback、覆盖失效、checkpoint/取消回归已通过。
- `Log` 的四个新增大表复合索引已从普通 `AutoMigrate` tag 拆出，新增 `LOG_QUERY_INDEX_MIGRATION`（默认关闭）和显式 GORM migrator；共享 DB 与独立 `LOG_SQL_DSN` 初始化路径均只在显式开关开启时建索引。
- PRD 已更新为 v0.6，明确自然日 checkpoint、显式索引迁移和旧 archive schema 兼容边界。旧 archive 缺 `last_log_id` 且开关关闭时仍跳过隐式 ALTER；该策略只覆盖当前已知新增字段，未来 schema 漂移需独立迁移。
- 本地快照实际已有约 27 个完整 coverage 日；2026-09-04 用户 512 原始 9,381 行/quota 54,498,012 与投影完全一致，逐用户 request/quota 差异 0。验证容器仅绑定 localhost:13306、重启 0，说明已同步更新。
- 最终验证：定向 model/service/controller、wallet/legacy/archive/index/usage 回归，wallet 相关 `-race`，全仓 Go compile-only，Default TypeScript，gofmt 与 `git diff --check` 均通过。代码、快照和投影仍未提交/推送/上线；未连接或操作 `overseas2`。

### 2026-09-05 — 全工作树蓝绿发布准备受远程执行通道阻塞

- 用户明确要求把当前工作树其他窗口改动一并发布；当前候选包含完整 dirty 工作树，不仅是日志/钱包聚合改动。
- 本地已完成 gofmt、Go model/service/controller 定向测试、Default TypeScript、Default/Classic Rsbuild、全仓 Go compile-only 和 `git diff --check`。静态 Linux/amd64 产物 `new-api-20260905-all-windows-linux-amd64` SHA-256 为 `73e4612e0ef238c46f4d99333b976457917ab7445b4810676661f752bb8455b4`。
- 尝试读取生产 `site-builder` 状态时，普通 SSH 被沙箱拒绝；升级 SSH 请求因本地 Codex 上游通道 HTTP 503 被自动拒绝。没有上传产物、生产 DDL/迁移、容器重启或 Nginx 切流；`overseas2` 未触碰。
- 恢复远程执行能力后，先重新读取生产状态并使用独立 Compose 项目名承载 3010 候选，再执行蓝绿发布；不得复用正式 `newapi` 项目名启动候选。

### 2026-09-05 — Responses 流中断根因已本地复现并修复（未上线）

- 本地回归复现了 Codex Responses 的直接回归：已转发 `response.output_text.delta` 后，上游发送瞬时顶层 `type=error`（如 `server_is_overloaded`）；原代码先具备转换为 `response.failed` 的条件，却又在后续 Codex 瞬时错误抑制条件中把该事件吞掉，客户端只收到半截 SSE，因而报告 “Text model stream was interrupted after receiving …”。该行为来自 `6006ffcc0` 引入的终态恢复组合，既有测试只覆盖输出前/prelude 错误。
- `relay/channel/openai/relay_responses.go` 已调整为：Codex 在已有下游输出后遇到 `error/response.error` 时生成并转发协议有效的 `response.failed`，保留 `skipRetry` 防止重复输出；仅在无法安全转换时抑制原始瞬时 `event:error`。新增回归覆盖 output-after-transient 场景，并保留 output-before/prelude 透明重试行为。
- `relay/helper/stream_scanner.go` 已将 scanner 生命周期 goroutine 恢复为独立 `gopool.Go`，避免当前其他窗口未提交的 `common.RelayCtxGo` 512 槽准入池在高并发下阻塞流启动；同时修复裸 `[DONE]` 被错误切片为 `]`，补充 scanner 回归测试。
- 定向测试通过：`go test -vet=off ./relay/helper ./relay/common ./relay/channel/openai ./controller -run 'Test(StreamScannerHandler|OaiResponses|CanRetrySame|ShouldRetry)' -count=1`。更宽的整包测试存在与本次无关的既有基线失败（i18n 初始化、图片错误文案、model-list/SQLite/日志表测试环境等），未归因于本修复；`git diff --check` 通过。
- 本轮只修改本地 `new-api` 工作树，未提交、未推送、未构建或部署生产，未修改生产配置、数据库、容器、Nginx 或流量。

### 2026-09-05 — Responses 流修复已蓝绿上线

- 用户明确授权上线。包含 Responses 终态修复、scanner 生命周期恢复和裸 `[DONE]` 兼容修复的本地 Linux/amd64 产物已重新构建并校验：`new-api-20260905-all-windows-stream-fix-linux-amd64`，SHA-256 `4bbc24aaaad43b67977675736e86dcc1b4e7368d62313cf74f45b0ae2d4b153f`。
- 生产 `site-builder` 先保留正式旧容器 3000，再以独立容器 `new-api-candidate`、3010 端口和独立日志目录启动候选；候选健康检查通过，三个域名各 5/5 返回 `success=true`、版本 `20260905-all-windows-stream-fix`。
- 候选验收后 Nginx 从 3000 切到 3010；canonical Compose 更新为新 release，正式 3000 在流量仍由候选承载时重建并健康；随后 Nginx 切回 3000，三个域名再次各 5/5 返回新版本，候选已停止并删除。
- 最终正式状态：`new-api` running/healthy、restart=0、OOMKilled=false，仅监听 `127.0.0.1:3000`；最近 5 分钟应用日志未见 panic/fatal/migration failure/database locked/unsupported protocol，Nginx 错误日志未见 upstream/502/503。旧产物 `73e4612e...` 仍保留在 `/opt/newapi/releases/`，发布前备份位于 `/opt/newapi/backups/stream-fix-20260905T120624Z-before/` 与切流备份 `/opt/newapi/backups/stream-fix-20260905T121125Z-cutover/`。

### 2026-09-06 — Responses 前导事件修复与 CPA2 重试关闭已部署

- `relay/channel/openai/relay_responses.go` 的本地修复通过定向测试：`response.created/in_progress/queued` 立即转发；前导事件已暴露后不再透明重放；Codex 瞬时错误在已有输出后转换为协议有效的 `response.failed`。Linux/amd64 产物为 `new-api-20260906-responses-prelude-forward-linux-amd64`，SHA-256 `81aec56bb632b72c4d2a1a56724a511ad3075085ce29394c97d7a71857aa40ff`。
- 生产先以独立 Compose 项目承载 3010 候选，真实流追踪看到 `first_event_ms` 约 0.6–2.1 秒且 `forwarded_events=2`；验收后正式 3000 已重建并切回，候选已清理。三个公网入口均返回版本 `20260906-responses-prelude-forward`，正式容器 healthy、restart=0、OOM=false。
- CPA2 `/opt/cpa2/config.yaml` 的 `request-retry` 已按用户确认从 1 调为 0并重启；15 分钟观察健康 15/15、`NRestarts=0`，内存约 315–556 MiB。回滚配置保存在 `/opt/cpa2/backups/request-retry0-20260906T111640Z/`。
- 旧生产产物仍保留可回滚；后续监控应同时关注 `upstream_frt`/外部 TTFT、成功率、502/503/429、CPA2 重启和 `forwarded_events`，避免仅用成功样本 P90 判断。

### 2026-09-22 — 智商测试 V6 开发进行中（尚未交付/上线）

- 用户授权按 V6 审查并实施；开发工作树 `/Users/yihu/AI-Workspace/juxing-iq-v6`，分支 `codex/iq-capability-v6`，基于重新核验的 GitHub main `247b243014f3418fe84b398afc1a2299cab8620f`。原文档工作树保留，业务开发不在旧基线进行。
- 已开始实现独立配置/分组 UUID、CAS 控制与展示草稿、发现清单、题目/严格答案解析/SVG 白名单、指定渠道执行器、保守预算/派发记录、Redis 租约调度、授权结果接口及 Default 原生组件页面。均为未提交的开发中代码，尚未满足 V6 全量验收。
- 新执行器的本地 HTTP fixture 已验证指定渠道、目标分组、模型映射、请求内容、完整回答及 usage；题目 oracle/SVG/终止条件定向测试通过。Default 初轮 TypeScript 与 Rsbuild 通过。完整回归、Classic、视觉验收、三数据库、真实评审校准和真实收费链路尚未完成。
- 真实模型范围/验收费用上限已向用户询问，尚未获得；未发起收费模型调用。执行默认隐藏/停止，部署执行门禁与校准门禁保留。
- 服务器只读复查：正式版本仍为 `20260921-load-status`、健康，正式监听3000，历史候选停止但保留，蓝绿候选配置/备份可定位。没有生产写入、发布、重启、DDL、提交或推送。新表需独立新增迁移后才能验收 slave 候选；仍沿既有3000/3010蓝绿、连接及指标排空流程。
- 下一步：完成执行器/并发/权限/控制回归并修复、补齐 V6 控制矩阵和双前端、完善文档/验收报告；任何未通过项必须明列，不能把 mock 或页面构建成功称为生产可用。

### 2026-09-23 — 用户端来源存档与方法说明质感优化（本地未上线）

- Default 与 Classic 的来源存档区和测试方法说明区完成视觉层级优化：顶部渐变强调线、内层边界、来源状态徽标、分组信息块、模型选择器、章节编号卡、悬停反馈、证据层级和双主题适配均沿用 new-api 原生组件与现有主题变量。
- 未改变外部测试来源、真实图像、标准答案 21、原始检测答案、分组/渠道映射、精选规则、后台开关或作品不可用状态；本轮只改变用户端呈现。
- 通过 Default TypeScript、定向 ESLint、Default/Classic production build、两端定向 Prettier 与 `git diff --check`。环境没有 Bun，改用项目现有 `node_modules/.bin` 中已安装的同版本工具执行，未安装依赖或修改锁文件。
- 浏览器本地预览已确认 Default/Classic 页面和详情抽屉可用，生产尚未挂载、切流、提交或推送。

### 2026-09-23 — 开发进度提交已推送，生产蓝绿等待 SSH 恢复

- 当前分支 `codex/iq-capability-v6` 已提交为 `ba026c4a5`（智商测试与鹈鹕存档接入、双前端、管理端、迁移/预览/验收工具及本轮 UI 优化），并成功推送到 `origin/codex/iq-capability-v6`；远程 `main` 未修改，未创建或合并 PR。
- 已核对用户入口为 Default `/intelligence-test`（通用侧栏中位于使用日志与模型状态之间），Classic `/console/intelligence-test`；管理入口为 Default 系统设置 → 运维 → 智商测试，Classic 运维设置中的智商测试卡片。
- 生产蓝绿尚未执行。只读 SSH 到历史 `site-builder` 别名落到 `site-builder:22` 后被对端关闭，历史 `stellaisle-image`（50.118.185.139:41862）也被对端关闭；未上传、未迁移、未重启、未切流。恢复有效 SSH 后仍须重新核对正式3000、候选端口、Compose、Nginx、备份与最新生产基线，再按蓝绿流程发布。

### 2026-09-23 — 智商测试 V6 错误脱敏修复与蓝绿上线完成

- 安全修复提交 `f009a47913aed7b00282b1a7e32ee8f8ba2f8bb3` 已推送到 `origin/codex/iq-capability-v6`；修复公开错误、SSE/WebSocket、任务/媒体响应及日志/响应头中的上游 URL、域名、IP、端口等诊断泄露，并保留内部日志诊断。远程 `main` 仍为 `247b243014f`，没有强推或直接改写 main；当前工作树另有4个仅格式化/测试文件的未提交修改，未进入本次制品。
- 本地制品已验证并上传：应用 `new-api-20260923-iq-pelican-ui-error-safe-linux-amd64` SHA-256 `2fa430b460a2da99bcc2473dddc8563ad194449128c920fa5c91958fe1a3b3e7`；迁移和初始化工具分别为 `33e1cb29a17015f98475ede8fbb775fc8ccc7f2ba3aebddd65c9137bc714d5ac`、`77c23747586f57c74c9cbbefecf3adfc40e384b19595afb4e38a603f79b60fff`。本地 Go 构建、定向安全测试和 `git diff --check` 通过；4个无关基线测试失败未归因于本修复。
- 正式发布版本为 `20260923-iq-pelican-ui-error-safe`。按既有流程完成备份 → 3010 `NODE_TYPE=slave` 候选 → 候选健康与前端/API/鉴权验收 → Nginx 切 3010 → 3000 连接自然排空 → 正式 3000 仅应用容器重建 → 健康验收 → Nginx 回切 3000 → 候选优雅停止。MySQL/Redis 未重建，生产未编译；正式 Compose 已持久化新二进制和 `/var/lib/pelican-archive` 只读挂载，`PELICAN_SOURCE_ID=bench-primary`。
- 最终正式状态：`new-api` healthy、restart=0、OOM=false，仅监听 `127.0.0.1:3000`；`token.stellaisle.com`、`direct-token.stellaisle.com`、`api.stellaisle.com` 的 `/api/status` 和 `/intelligence-test` 各5/5通过，新版本一致；3010无监听。未授权 Pelican 接口保持401且响应未含上游地址模式。
- 生产数据库存档保持 `pelican_targets=8`、`pelican_records=240`、`capability_group_presentations=23`；首次上线状态仍 `visible=0`、`sync_enabled=0`，不会向用户公开或启动同步。回滚前配置和制品位于 `/opt/newapi/backups/release-20260923-iq-pelican-ui-error-safe-20260923T002801Z-before` 及后续切流备份目录，旧二进制未删除。
