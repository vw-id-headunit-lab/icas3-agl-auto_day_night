# 部署、手动完整重启与回退

本文描述**旧版离线文件替换**。新的 Attach 版不执行本文的 mask、stop、写入或重启流程，
请使用 [Attach 部署与卸载](../attach/README.md)。不能把“不停止 JVM”的 Attach 流程套用于覆盖打开的 JAR。

适用对象：已取得 AGL root、能够独立恢复系统、且核对过本车固件与文件摘要的维护者。**本文件不是“所有 S2995 均可直接安装”的承诺。** 本轮实车已确认成功，但操作过程中不能使用车辆，中控会暂时不可用。

## 必须遵守的顺序

**先暂存与备份 → runtime-mask → 停止 HMI → 确认没有读者 → 替换与校验 → 用户手动完整重启。** 回退同样先停止服务并检查文件占用。没有例外的在线热替换步骤。

- 不允许向运行中的 `viewhandler.jar` 执行 `cp`、`cat >`、SCP 覆盖或解压覆盖。
- 暂存上传必须使用 `/data/` 下的新目录，不直接上传到 `/vw/hmi/Resources/skin1/`。
- 不能仅凭 `MainPID=0` 就写入；旧 JVM、辅助进程或其它读者可能仍占用文件。
- 不依赖 `cp -a` 保存 SMACK。实车曾出现标签未被保留的情况。
- 不清除／重建原厂共享缓存，不新增桥接 JAR，不修改全局 SMACK 规则。
- 安装工具不会停止／启动服务、修改服务配置或重启机器；这些动作必须按维护流程单独执行。

## 为什么必须防止故障自启

**HMI 改动不要未经验证就直接挂开机自启。如果改动导致 HMI 启动失败或 watchdog 超时，可能触发整机重启；若每次开机仍加载同一故障版本，就可能形成循环重启。**

已检查的原厂 `vw-hmi.service` 为 `Type=notify`，包含 `WatchdogSec=1min` 与 `FailureAction=reboot-immediate`。这说明存在系统重启路径，不意味着每次普通界面错误必定触发重启。循环发生的条件是故障持续存在并重复触发失败处理。不要通过修改 watchdog 或 FailureAction 来掩盖错误，也不要自动反复重启尝试。

本项目不添加额外自启服务。`viewhandler.jar` 仍由原厂 HMI 加载，因此**替换它后原厂下一次启动也会加载新版**。runtime mask 只用于本次离线维护；完整系统重启后 `/run` 被重建，临时 mask 消失，原厂服务恢复按既有流程启动。它不是跨重启隔离机制，不能保证故障版本开机后仍被阻止。

在尝试新改动之前，必须先保留可在 HMI 异常时使用的 SSH／控制台恢复入口和本车正常版备份。对尚未完成验证的改动，先在虚拟环境检查；有条件时在受控、具备恢复能力的环境检查启动，而不是直接投入日常使用。不要新增自动安装、自动恢复或无限重试任务。

## 支持的输入与工具边界

当前构建器只接受以下原始文件：

| 文件 | SHA-256 |
|---|---|
| 原 `viewhandler.jar` | `9bd5764fabe2cf79e5193f2dd39c38954f6170a8f016c229c133e2b8165a1ce1` |
| 原 `MIBHMI-STD-HCN1.1.134-bin.jar` | `c27e37f93b9877c023487af8566e5cb33b9475835fcee6fe1b68f6c8affc7e17` |

**部署工具与构建器的输入范围不同。** 当前发布的 `safe-update/main.go` 原样保留本轮实车使用的安装门禁，`prepare` 只接受车上已有的上一个正常菜单版本：

```text
9113535ba604511c41229bf00201ab4840126966f2fa8f09668f25a564d42c85
```

它不接受任意原厂 JAR，也不接受已经安装的本轮新版作为再次安装起点。**如果你的车没有该正常版本，以下安装步骤会拒绝；不要篡改 baseline、manifest 或去掉门禁。** 原厂首次安装／其它升级起点需要先做独立版本适配，本文未提供通用流程。仓库不分发历史或当前厂商 JAR。

本轮已实车验证的安装结果为：

```text
283ed23a372531d1d2274d459e14b096474dbb717f4a618791bb38148c396460
```

自行构建时应使用实际生成的 `build.json` 与 SHA-256 校验传输和安装一致性；不同 JDK／Go 版本可能导致文件摘要不同。未获得同一文件摘要就不能称为与已测二进制完全一致。部署工具会记录上传候选的摘要，但不替代对候选代码和来源的审查。

## 1. 在本机准备

从仓库根目录构建一次性 Linux ARM64 更新工具：

```sh
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
  go build -o /tmp/daynight-update-linux-arm64 safe-update/main.go
```

按 README 生成候选 JAR。先在虚拟 HMI 检查菜单，并保存 `build.json`。下面以 `root@10.173.189.1` 为示例连接，请替换为自己的车机地址。

车辆停稳、挂 P 挡后，先读状态和当前摘要：

```sh
ssh root@10.173.189.1 'systemctl show vw-hmi.service -p ActiveState -p SubState -p MainPID; sha256sum /vw/hmi/Resources/skin1/viewhandler.jar; df -h /data /vw'
```

确认当前摘要匹配上面的安装起点；保留原文件、权限与标签记录，以及可用的独立恢复通道。不要为绕过版本检查而手工修改 manifest。

## 2. 只暂存与备份，不写运行文件

下面是新安装示例目录，**与本轮实车已存在目录不同**。目录应不存在，避免覆盖历史备份。若命令失败，先检查原因，不要删除旧备份重试。

```sh
ssh root@10.173.189.1 \
  'test ! -e /data/vw-daynight-new-upload && mkdir -m 700 /data/vw-daynight-new-upload'

scp /tmp/daynight-update-linux-arm64 \
  root@10.173.189.1:/data/vw-daynight-new-upload/update
scp /path/to/new-candidate-directory/viewhandler.candidate.jar \
  root@10.173.189.1:/data/vw-daynight-new-upload/candidate.jar
scp safe-update/restore.sh \
  root@10.173.189.1:/data/vw-daynight-new-upload/restore.sh

ssh root@10.173.189.1 '
  chmod 700 /data/vw-daynight-new-upload/update &&
  /data/vw-daynight-new-upload/update prepare \
    /data/vw-daynight-new /data/vw-daynight-new-upload/candidate.jar &&
  cp /data/vw-daynight-new-upload/restore.sh /data/vw-daynight-new/restore.sh &&
  cat /data/vw-daynight-new/manifest.json &&
  sha256sum /data/vw-daynight-new/before.jar /data/vw-daynight-new/candidate.jar
'
```

旧车机没有 SFTP 子系统时，在本机 SCP 命令加 `-O` 使用旧 SCP 协议。不要把工具上传到原厂启动路径。

`prepare` 校验正常版起点，创建新备份目录，复制 `before.jar`、`candidate.jar` 并生成 `manifest.json`。此时不写目标，不停止服务。把本车备份另存到本机，并确认候选摘要与本机构建输出一致。先确认回退脚本及工具确实存在再停止中控。

## 3. 先暂停 HMI，阻止维护期间自动拉起

以下操作会使中控暂时不可用，必须在车辆停稳且允许中断 HMI 时执行：

```sh
ssh root@10.173.189.1 '
  systemctl mask --runtime vw-hmi.service &&
  systemctl stop --no-block vw-hmi.service
'
```

随后检查；若仍为 `deactivating`，等待退出后再检查，**不要立即覆盖文件**：

```sh
ssh root@10.173.189.1 '
  systemctl show vw-hmi.service -p ActiveState -p SubState -p MainPID
  readlink /run/systemd/system/vw-hmi.service
  /data/vw-daynight-new-upload/update inspect /vw/hmi/Resources/skin1/viewhandler.jar
'
```

应看到 `ActiveState=inactive`、`SubState=dead`、`MainPID=0`，runtime 链接指向 `/dev/null`，以及 `Readers: []`。`inspect` 只扫描读者；完整的状态／runtime mask／标签检查由 `apply` 强制执行。

若状态失败、进程不退出、权限不足或扫描无法完成，停止安装并诊断，不能使用 `--force`（工具也不提供此选项）。不要为赶进度强制杀进程后盲目写入。

## 4. 离线应用与校验

确认上述条件后：

```sh
ssh root@10.173.189.1 '
  /data/vw-daynight-new-upload/update apply /data/vw-daynight-new &&
  sync &&
  sha256sum /vw/hmi/Resources/skin1/viewhandler.jar &&
  systemctl show vw-hmi.service -p ActiveState -p SubState -p MainPID
'
```

工具检查当前文件与备份摘要匹配、候选摘要匹配、HMI inactive、runtime mask 存在、无厂商 JVM、无目标 inode 的 FD／内存映射读者；不能完整检查时拒绝。写前要求原访问标签为 `hmi::sub`。在读者已停止的条件下保留原 inode 写入内容，写后核对摘要、inode、owner、mode 和 SMACK。

这里保留 inode 是为了保留原权限及标签，**绝不意味着允许向运行中 inode 写入**。写入／验证失败时工具会在可用条件下尝试恢复本次写入前的内容并报错；断电、存储故障或恢复自身失败不受此机制保证。发生错误不要继续启动新版。

只替换目标 JAR，其余原厂文件应与安装前保持一致。更新工具无自动重启、自动启动或自动重复安装行为。

## 5. 部署后必须手动完整重启

**写入和校验完成后，保持 HMI inactive/runtime-masked。由用户手动完整重启车机。** 暂时黑屏在本阶段是计划内的服务停止状态，不应当作运行中新版崩溃。

不要执行 `systemctl restart vw-hmi.service` 来代替完整重启。此前单独重启服务未形成可靠的界面恢复流程；本轮按完整重启路径获得用户实车确认。工具不自动执行 `reboot`。

确认是完整重启而非仅退出界面，可检查 `/proc/sys/kernel/random/boot_id` 是否与重启前不同，以及系统 uptime。`/run` 临时 mask 随系统重启消失，不要改用永久 mask，也不要为了恢复启动删除未确认来源的配置。

启动后由用户检查：

1. 界面和触控正常，日间／夜间／自动三行可见。
2. 手动日间 → AUTO，菜单保持 AUTO；实际主题按当前大灯状态显示。
3. AUTO 且实际日间 → 手动日间，菜单退出 AUTO；夜间做对应检查。
4. AUTO 下改变灯光状态，主题跟随；手动模式仍能主动选择。
5. 关闭后重开菜单，检查选中状态。另一次完整重启后的保存保持需单独记录，不能把一次开机通过算作已覆盖。

若异常，使用 SSH 回退，不要重复触发 HMI 启动试运气。

## 6. 一键回退

### 本轮已安装车辆的原样命令

仅当以下脚本和本车备份存在时使用：

```sh
ssh root@10.173.189.1 'sh /data/vw-daynight-menu-20261123-clickfix/restore.sh'
```

它调用的是本次上传的静态工具，恢复本车 `before.jar`，即上一个正常版本 `9113535…42c85`，**不是恢复全部固件出厂状态**。恢复后仍保持 HMI 停止并需要用户手动完整重启。

### 按本文新目录安装的车辆

仓库的恢复脚本接收本车备份目录和工具路径：

```sh
ssh root@10.173.189.1 \
  'sh /data/vw-daynight-new/restore.sh /data/vw-daynight-new /data/vw-daynight-new-upload/update'
```

脚本 runtime-mask、请求停止服务，最多等 30 秒。若 HMI 未停下则拒绝；之后由工具再次核对无读者、状态、标签与摘要，恢复该备份目录的 `before.jar`，最后同步并显示恢复摘要。它不启动 HMI、不重启机器。

恢复后请手动完整重启。保留备份、manifest、工具及脚本，不要验证一次成功就清理。

工具要求当前目标摘要为该 manifest 的候选摘要，防止覆盖未知状态。因此重复恢复、已被其它工具修改、部分写坏文件、缺失备份、SSH 不可达或持续重启时，不能保证这条命令可用；它会拒绝而不是盲目覆盖。此时需要独立控制台／离线恢复，不要改用在线 `cat` 覆盖，也不要删除门禁。
