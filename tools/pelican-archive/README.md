# 鹈鹕存档接入工具与部署准备

本目录是不含凭据的存档导出、运输和部署工具。2026-09-23获批完成受限运输安装；正式业务应用仍未发布。

2026-09-23更新：已实测SQLite3.50.4、MySQL8.2.0/8.4.11、PostgreSQL16.4。`deployment/`中的受限SSH及独立systemd定时器已安装，新账号认证、两轮定时拉取和失败保留旧文件已验证，见[安装记录](../../docs/architecture/pelican-transport-installation-2026-09-23.md)。Compose只读挂载增量尚未应用；首次生产初始化及蓝绿仍待准备与批准，见[上线前核验](../../docs/architecture/pelican-production-preflight-2026-09-23.md)。

## 本地验收

- `pull.py --audit-export --host <existing-alias> --source-id bench-primary --output /private/archive.json --database <verified-db> --grader <verified-grader>`：只读导出，实际存档放仓库外。
- `go run ./cmd/pelican-preview --archive /private/archive.json`：仅127.0.0.1:4198、内存库、真实记录和演示关联，不连接生产业务库。
- 真实配置验收：先运行 `python3 tools/pelican-archive/pull_topology.py --host <verified-host> --port <verified-port> --identity-file <private-key> --known-hosts <verified-known-hosts> --output /private/topology.json`，通过现有SSH对生产MySQL进行一次只读一致性导出。仅白名单渠道/能力/分组配置，不读取密钥、上游地址、用户、订阅或计费数据；快照保存在仓库外。
- `go run ./cmd/pelican-preview --archive /private/archive.json --topology /private/topology.json --mappings tools/pelican-archive/confirmed-mappings.json`：使用8项已经确认的稳定编号关联；两个参数必须同时给出，启动失败不回退演示关联。可加 `--check-only` 检查完整导入与重复导入。拓扑有效期24小时，此模式不自动刷新生产拓扑。
- `--user-group default` 指定独立本地测试账号的分组；普通用户页面沿现有可用分组权限，管理后台“用户预览”可查全部分组。不复制真实用户或订阅，不扩大正式权限。
- `node tools/pelican-archive/verify-preview.mjs /private/topology.json /private/archive.json tools/pelican-archive/confirmed-mappings.json`：固定访问本机4198，核对渠道/倍率/成员、停用资格、原图逐字节、最新记录与跨组共享，并操作本地隐藏/暂停/排序/别名/文案/间隔。结束时恢复原设置；不要与手动编辑同时运行。验收脚本不会接受远程服务地址。
- 在 web/default 执行 `bun x rsbuild dev --config rsbuild.pelican-preview.config.ts`，入口4199。
- 在 web/classic 执行同名配置，入口4200。
- 预览程序的本地专用会话接口不进入生产主程序。不要将预览端口转发公网。快照超过24小时会被拒绝，需重新只读获取。
- 预览现启动真实导入 worker，默认显示并启用同步；这些初始设置只写本地内存库。生产仍默认隐藏、暂停。同步间隔由后台设置，worker 每30秒检查；改间隔不会让外部重新测试。
- `pull.py` 默认只拉取一次；增加 `--interval-seconds 60` 可持续运输，允许60–86400秒；`--cycles 2` 用于有限轮验收，0表示持续运行至停止。实际外部测试周期、运输周期和本站导入周期是三个不同概念。
- 同一输出路径有排他文件锁；单轮失败保留旧快照并在下一周期重试，异常信息不输出远端正文或命令参数。Ctrl-C停止运输不会清除已导入结果。运行 `python3 -m unittest discover -s tools/pelican-archive -p 'test_*.py'` 验证失败恢复、来源拒绝和并发锁。

## 受限运输（已完成首次获批安装）

1. 外部服务器专用导出账号固定执行审查后的export.py，须能读取SQLite及当前WAL、指定grader文件。不要给数据库目录写权限或读取providers配置的权限。
2. 可由root拥有的固定包装程序以来源应用身份执行只读查询，SSH账号仅获得执行该精确命令的权限。authorized_keys设置固定command与restrict，禁止交互shell、PTY和转发；验证传入其他命令无效，并限制替代认证路径。共享服务器安装前确认维护者协作窗口。
3. 中转站宿主使用独立拉取身份、私有SSH配置和已确认known_hosts。执行 `python3 /installed/pull.py --host <restricted-alias> --source-id bench-primary --output <private-directory>/archive.json`。可由宿主定时器每分钟运行，以flock防重叠。
4. 后台暂停的是本站导入，不停止上述运输或外部测试。不要把运输周期解释成模型测试周期。
5. 应用只读挂载快照**目录**，不能只挂载单文件，否则原子改名后可能读旧inode。配置 `PELICAN_ARCHIVE_FILE` 和 `PELICAN_SOURCE_ID=bench-primary`；根据实际容器UID授予读取权限。应用不持有SSH私钥，不放宽为公网可读。
6. 来源迁移、恢复数据库或重置记录ID须核对身份，不能用旧source_id静默接另一套数据。

## 蓝绿发布顺序

须重新只读核对服务器当前Compose、Nginx、镜像及切流脚本，以下不是已执行记录。

1. 合并届时最新生产基线，本地完成回归、双前端构建、Go构建、三库迁移实测，记录提交、版本及产物SHA256。
2. 备份数据库和部署配置，保留旧镜像及回滚命令。本地构建cmd/pelican-migrate，获准后只新增存档与分组身份表，不删列、不生产编译。
3. 候选3010使用NODE_TYPE=slave、独立日志，不执行同步或管理写入。受限验收页面、权限、容器健康及status。
4. 逐一核对外部provider UUID与本站Channel.Id/模型/能力关系，不能仅凭同名。页面和同步默认关闭，先管理员预览。
5. 等用户确认后，按现有流程切流到候选、排空旧3000、重建正式实例、验收、切回3000。保持唯一同步master。
6. 正式开启同步与用户页面通过后台执行。异常时隐藏/暂停本功能，必要时旧镜像回滚；无需删除新增表。

## 运行界限

导出或导入失败保留旧数据。后台同时关注来源捕获时间、真实测试时间和本站导入时间，不能只凭导入成功声称测试更新。

证据内容上限100000条修订或512MiB，触限返回archive_capacity_reached，不自动清理证据。数据库索引、事件、备份另占空间，需外部容量监控及后续经确认的归档策略。

当前无外部停测/改题API；源码保留的旧V6执行模块不接正式路由，不启动worker，不自动建旧自测表。
