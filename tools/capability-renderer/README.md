# 智商测试作品渲染器

此进程通过 Unix socket 接收已经保存的模型回答。`/render` 继续用于严格静态规则图形；`/animation` 使用独立 Chromium 进程处理 HTML/SVG/CSS/内联 JavaScript，向主应用返回像素帧。原始 HTML 不会在用户浏览器执行。

## 当前动画协议

- Playwright 固定 `1.61.1`；Chromium `149.0.7827.55`；渲染协议 `chromium-animation-1`。后端与人工校准报告同时检查版本，不接受旧静态题库或缺少题目模板覆盖的校准报告。
- 600×400、10 fps，共48帧，采样时间0至4700毫秒。CSS/SMIL和脚本计时使用同一虚拟时间轴。只对观察窗口内的内容作结论。
- 后端逐帧解码和统一重新编码，生成无损 APNG、首帧海报、1800×800六帧评审拼图。评审时刻依次为0、900、1900、2800、3800、4700毫秒；评审调用哈希绑定拼图，不绑定首帧。
- 输入上限400 KiB，48帧累计上限12 MiB；每帧、解码尺寸、版本、返回体大小均在后端再次验证。渲染器单任务运行；外部30秒截止、客户端取消和服务退出会终止任务。Chromium拥有独立进程组，清理同时处理已识别的浏览器与Node子进程。
- 是否发生像素变化只是机械观察；不能据此给骑行动作加分。采样没有变化时，运动要求不得判为满足。动作含义仍由多帧评审及人工校准核验。

## 本地验证

只在工作站或专用构建环境执行，不能在生产安装或编译：

```sh
bun install --frozen-lockfile
node node_modules/playwright/cli.js install chromium
node --test animation.test.mjs renderer.test.mjs server.test.mjs
```

macOS开发验证只允许 `CAPABILITY_RENDERER_FIXTURE_HASHES` 中明确列出的自有样例。不要把未知模型回答加入本机白名单；不要在生产配置此变量。

从项目根目录可运行真实本地 renderer＋HTTP 模拟上游＋SQLite 集成回归，并导出明确模拟的移动圆形供正式前端组件验收：

```sh
CAPABILITY_TEST_NODE=/absolute/path/to/node \
CAPABILITY_TEST_EXPORT=/absolute/project/web/default/dev/animation-fixture \
go test -vet=off ./controller -run TestCapabilityLocalRoundWithRealRenderer -count=1
```

随后使用 `web/default/rsbuild.capability-preview.config.ts` 启动本地预览。生成的图片不进入正式构建入口；它们证明播放链路，不能证明鹈鹕质量或真实模型成绩。

## 生产隔离要求与未验收项

Dockerfile已包含浏览器与系统依赖的构建步骤，尚未在Linux隔离环境实际构建/验收，不能直接视作可上线制品。发布前必须在专用构建环境完成：

1. 非root、独立PID/网络命名空间、`network_mode: none`、只读根文件系统、无业务数据/密钥/宿主敏感目录挂载。仅给Unix socket共享目录与有容量上限的临时目录写权限；明确验证主应用能连接socket且其他无关进程不能连接。
2. 保持Chromium沙箱，限制CPU/内存/PID/临时盘；内核user namespace/seccomp须兼容实际沙箱。不能使用`--no-sandbox`或特权容器换取启动成功。
3. 设置 `CAPABILITY_RENDERER_ISOLATED=true`。进程会核查Linux、非root及没有非loopback网卡；这不替代对只读根、挂载、资源限制和系统调用策略的外部验收。
4. 固定镜像/字体后重新做动画人工校准，报告必须按每个受控题目模板覆盖六类样本，每类至少10个，并绑定题库、渲染器、浏览器和评审配置。验证恶意HTML、浏览器启动阶段取消、OOM、失联、重启后的socket与进程回收。当前本地回归不覆盖Linux这些边界；字体/镜像摘要与实际judge配置的完整校准绑定仍待补。
5. 仅上传经过核验的产物，依照现网蓝绿流程及用户确认执行。默认隐藏且停测；真实付费调用另按获准范围与预算启用。

旧版只支持resvg静态SVG的运行配置不能直接复用为动画生产配置。
