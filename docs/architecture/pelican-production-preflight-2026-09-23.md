# 鹈鹕存档：上线前核验与待批准安装

状态更新：2026-09-23用户已批准A，受限账号和定时运输已安装并验证。详见[安装记录](pelican-transport-installation-2026-09-23.md)。业务数据库、新版应用及用户页面未发布，B不包含在A授权中。下文保留安装前核验及批准范围，描述中的“尚未安装”属于该次预检查时点。

## 已核实的基线

- GitHub `glxgo/new-api` main：`247b243014f3418fe84b398afc1a2299cab8620f`；与实施树基线相同，无新提交需要合并。核查时点为2026-09-23，不保证之后不变化。
- 当前认证账号 `Piloting06`：pull/push/triage=true，admin/maintain=false。属于可写协作者，不是GitHub仓库管理员权限；未改权限或推送。
- 线上容器 `new-api`：`new-api:20260921-load-status`，健康，0重启，无OOM。
- `/api/status`版本 `20260921-load-status`；实际二进制SHA256 `5cde9fb80c4e74c6697c512a91ca058e770fec0e8ec311e8ea6632abf541a2d3`，与交接基线一致。此轮没有重新进行源码可重复构建证明，沿用先前已核验的源码对应关系。
- 正式Compose `/opt/newapi/docker-compose.final.yml`，容器3000映射宿主 `127.0.0.1:3000`；master默认角色。Nginx实际指向3000。当前3010/3011无监听。
- `/new-api`是宿主二进制的只读绑定挂载，当前来源 `/opt/newapi/releases/new-api-20260921-load-status-linux-amd64`。**只换镜像名不会替换此挂载的程序**；发布须同时使用新版本独立路径，禁止覆盖正在运行或回滚使用的旧文件。
- 有历史Nginx3010候选配置；另外20260918候选JSON用3011/slave。实际发布仍沿蓝绿，不把某个历史端口当成永久规定；申请候选部署前须确定端口、配置及切流对象。
- 生产数据库MySQL8.2.0；只读查询确认没有 `pelican_*` / `capability_group_presentations` 新表。本轮未执行迁移。

## 本机数据库实测

临时解压数据库，未安装系统服务；只监听127.0.0.1，使用空白测试库，没有生产DSN或真实账号。MySQL来自Oracle CDN，包MD5与官方值一致；PostgreSQL来自Maven Central的Zonky测试发行包，SHA256与仓库公布值一致。

验收后全部临时数据库实例已停止，并确认测试端口不再监听；临时工具与合成数据目录保留，未创建常驻服务。

| 数据库 | 实际版本 | 验证 |
| --- | --- | --- |
| SQLite | 3.50.4 | 通过 |
| MySQL | 8.2.0（与生产一致） | 通过 |
| MySQL | 8.4.11 | 通过 |
| PostgreSQL | 16.4 | 通过 |

共同验证：只新增5张存档/分组身份表；重复迁移；有记录后再迁移；导入及去重；源记录修订和恢复；容量失败整体回滚；来源变更拒绝；暂停阻止导入；渠道/模型唯一关联；版本冲突；8个并发修改只1个成功；超过64KiB的中文及emoji逐字一致。额外旧表中的fixed_profit_amount、profit_quota哨兵数据保持原值。

这些是独立数据库里的合成场景，不是生产全库克隆、生产容量压测或MySQL5.7/PostgreSQL9.6最低版本实机验证；先前240条真实存档与真实拓扑的完整HTTP验收在本地SQLite完成。不能把两者合称“生产全库三库验收”。

复验入口为 `model/pelican_database_test.go`，仅接受回环IP和 `pelican_test_` 前缀且必须为空库。不读取通用SQL_DSN；未提供专用环境变量时对应数据库明确skip，不伪装成通过。

```sh
PELICAN_TEST_MYSQL_DSN='<new empty loopback test DB>' \
PELICAN_TEST_POSTGRES_DSN='<new empty loopback test DB>' \
go test -race -vet=off ./model -run '^TestPelicanDatabaseMatrix$' -count=1 -v
```

## A：已批准并完成的受限运输安装范围

目的：中转站宿主定时取回已保存的鹈鹕记录，生成私有快照文件。此步不切流、不改现有业务容器、不新建业务数据库表、不向用户开放测试页面。

外部源站为192.227.176.124，SSH实际端口11916。已从中转站宿主验证TCP可达；22端口拒绝连接。SSH直接互通，无需额外公网反向代理或内网穿透。两台均有python3/systemd/ssh，源站有sudo；没有现成的pelican-export/pelican-sync账号。

### 源站安装范围

1. 经用户确认共享服务器维护窗口后，备份SSH及sudo相关配置，保留现有root连接。
2. 新建仅供该功能使用的系统账号 `pelican-export`；shell为/bin/sh，home由root拥有不可被该账号修改。不授予Docker或普通sudo权限。
3. root拥有、普通用户不可写的文件：`export.py`及 `deployment/source-login.sh` 放 `/usr/local/lib/pelican-archive/`；`source-export.sh` 放 `/usr/local/sbin/pelican-export`（0755）。
4. `pelican-export.sudoers`装为 `/etc/sudoers.d/pelican-export`（root:root0440），只允许无参数的固定导出包装程序；先 `visudo -cf`。
5. 新专用公钥放 `/etc/ssh/authorized_keys/pelican-export`，root拥有，行首加`restrict`。此文件只包含新运输公钥，不复制个人root私钥。生成密钥是安装阶段的独立新身份，尚未执行。
6. `pelican-export.sshd.conf`存为 `/etc/ssh/pelican-export.conf`，从现有主配置**末尾**显式Include；不要直接放到主配置开头Include的目录，否则Match作用域可能影响后续全局指令。须检查现有Match顺序，必要时先以`Match all`结束前一条件。通过 `sshd -t` 和 `sshd -T -C user=pelican-export,...` 核实最终配置，再reload，保留旧会话验证root管理入口。
7. 生效后新账号只允许公钥和固定导出，不允许PTY、端口转发、用户rc、口令或任意命令。已用源站sshd对本地配置通过stdin做只读解析，输出符合预期；这不等于已安装或已验证新账号登录。

导出读取白名单pelican_runs、pelican_targets、pelican_config，不读providers、用户或环境变量。不执行源应用JavaScript，只读取固定grader文本；SQLite mode=ro/query_only及一致性事务。包装程序限制CPU20秒、内存256MiB，输出上限32MiB。数据文件及WAL/SHM当前均存在且可读，不改变其权限、源站调度或测试服务。

部署后必须实查：无命令SSH可以导出；提交`id`/文件读取/子系统等非空命令均拒绝；PTY/转发拒绝；导出前后源数据库内容及源站健康不受影响。检查固定grader路径随源站升级是否变化；不符时导出拒绝，不能改成执行任意脚本。

### 中转站宿主安装范围

1. 新建系统账号/组 `pelican-sync`，无交互登录、无sudo/Docker权限。
2. root拥有 `/usr/local/lib/pelican-archive/pull.py`；配置目录 `/etc/pelican-archive` root:pelican-sync0750；SSH配置root:pelican-sync0640；独立私钥pelican-sync拥有0600，known_hosts按已验证源站主机公钥固定，不接受首次自动信任。
3. 安装 `deployment/ssh_config`，固定11916与专用身份。新身份及主机指纹在私有部署目录保存，不提交仓库。
4. 安装 `pelican-archive-pull.service` 和 `.timer`，先 `systemd-analyze verify`。单次任务45秒超时、256MiB限制；专用用户仅可写其StateDirectory。1分钟运行一次，与后台导入间隔和来源60分钟测试周期分别管理。
5. 首次手动运行service并核对目标数、来源ID、captured_at、权限及快照可读取后，才enable timer。使用 `/var/lib/pelican-archive` 的0700状态目录，快照0600，整文件原子替换。文件失败保留旧快照，服务失败可由systemd状态/journal诊断；尚未接入第三方报警。
6. 不放在 `/opt/newapi` 下，因为该父目录700会阻止专用账号访问。正式应用当前以容器root运行，后续只读挂载该目录即可读取；如果未来改非root容器，需要单独核对UID/GID，不能放宽成公网可读。

安装后核验：定时器至少两次运行完成、captured_at向前推进、文件权限正确；模拟/确认一次受限身份非空命令拒绝；停timer后旧快照保留。源站与中转站均保持原容器健康。

### A的回退

停用新增timer/service，保留私有存档用于核查；撤销新运输公钥及该账号sudo条目，恢复SSH备份并验证配置后reload。不要删除来源数据库、既有账号/密钥、业务Compose/Nginx或测试证据。A不动业务容器，因此无需切换业务流量。

## B：后续独立批准的应用蓝绿发布

当前仅提供 `deployment/compose.archive.yml` 作为挂载和环境变量增量；它不是独立Compose，也不含端口、镜像、凭据或节点角色，必须分别合并到最终核实的正式和候选配置。

首次发布还须解决初始化顺序：先备份、新增5张表，再通过受控初始化流程建立分组身份、导入真实快照和保存8项确认关联，保持用户页面隐藏；之后slave候选才能用只读管理员预览验收。**当前迁移工具只建表，空表后直接启动slave不会自动得到分组或记录。** 不通过临时把候选改master来绕过这一点；受控初始化工具及该步骤验收属于下次发布准备，尚未运行。

随后才申请候选部署与验收（slave、独立日志、仅本地端口），核对health/status/真实原图、管理写拒绝、鉴权、目录原子更新可见；得到用户上线确认后沿既有蓝绿切流、排空、重建3000并切回。应用回退用旧镜像和旧二进制挂载，不物理删除新表或历史利润列；当前增量兼容旧版继续运行。

## 已生成的预检查制品

本机使用Go1.25.1，`CGO_ENABLED=0 GOOS=linux GOARCH=amd64`、`-buildvcs=false -trimpath -ldflags '-s -w ...'`生成静态ELF。完整应用嵌入上一轮已构建验证的两套前端，版本 `20260923-pelican-preflight`。预检查制品未上传，也不是已提交的最终发布包；正式发布须冻结完整源码清单、唯一版本与最终哈希，不能把基线SHA当作新增功能的提交。

| 制品 | SHA256 |
| --- | --- |
| new-api-pelican-preflight-linux-amd64 | a4c445ada00500906d4b4f6f15284c3fd45ac61d83f15c8235ce9d735e4a2f67 |
| pelican-migrate-linux-amd64 | 96d847bee1cdbde55e9536214a9aff1ac13e46c54db53b5ecd450bbf38ea449f |

本地制品位于仓库外私有验收目录。此轮没有Linux候选运行验证；不得将交叉编译成功视作生产验收成功。
