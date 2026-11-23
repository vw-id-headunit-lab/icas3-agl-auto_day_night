# ICAS3 AGL Auto Day Night

[简体中文](#简体中文) | [English](#english)

中国版 ICAS3 S2995 / HMI HCN1.1.134 原厂日夜主题 AUTO 菜单恢复与状态修复。基于 [ICAS3 AGL Root](https://github.com/vw-id-headunit-lab/icas3-agl-root) 提供的 AGL root 方法进一步开发。

## 简体中文

**2026-11-23：用户在手动完整重启后确认本轮实车验证通过。** 当前版本恢复 AUTO 入口、保持 AUTO 选中状态、一次显示日间／夜间／自动三行，并修复 AUTO 与当前实际主题相同时无法切换到手动模式的问题。

> **部署前必须先停止 HMI，并确认没有 JVM 或其它进程占用目标 JAR，才能替换文件。部署完成后由用户手动完整重启车机。回退也遵循同一流程。禁止在线覆盖运行中的 JAR。**
>
> **HMI 改动不要未经验证就直接挂开机自启。原车服务带 watchdog 和 `FailureAction=reboot-immediate`：启动持续失败或 watchdog 超时可能触发整机重启；若重启后仍加载同一故障版本，可能形成循环重启。应先准备独立 SSH／控制台恢复入口及回退文件，再进行受控验证。** 本项目不新增自启服务，也不关闭或改写原厂 watchdog／失败处理策略。

### 功能与工作方式

- 控制中心显示“日间、夜间、自动”三项，无需滚动查找 AUTO。
- 选择 AUTO 后，菜单显示保存的 AUTO 选择；实际主题仍随原车大灯状态变化。
- 大灯置于 AUTO 时，由原车灯光系统根据环境控制大灯。本项目不自行检测天气、隧道或照度。
- 保留手动日间／夜间选择；即使 AUTO 的实际主题与所点手动模式相同，也能退出 AUTO。
- 无固定时间、日出日落、定位、联网或常驻 Go 控制器。

**装车只替换 `/vw/hmi/Resources/skin1/viewhandler.jar`。** 不修改 HMI 主 JAR、`runHMI.sh`、`bundles.properties`、启动类路径或原厂预加载缓存；不向 `/vw/hmi/lib` 添加桥接 JAR。

Go 负责本地构建及一次性离线文件更新；菜单内运行的是最小 Java 适配器。异步读取原厂保存模式并在 UI 线程刷新，绘制函数不等待同步持久化读取。只在明确点击且读回模式与所选值不一致时，最多补发一次原厂切换请求，避免原厂按“实际主题”去重而跳过手动选择。普通显示读取不会切换模式。实现细节见 [实现与验证](docs/IMPLEMENTATION.md)。

### 验证状态与范围

| 项目 | 记录 |
|---|---|
| AUTO 入口及大灯变化驱动主题 | 2026-11-22 用户实车确认 |
| 三行菜单、AUTO 选中状态 | 本地虚拟 HMI 验证及实车反馈通过 |
| 本轮切换修复 | 2026-11-23 停止 HMI 后部署，用户手动完整重启后确认实车验证无问题 |
| 当前实车候选 SHA-256 | `283ed23a372531d1d2274d459e14b096474dbb717f4a618791bb38148c396460` |
| AUTO 跨另一次完整重启的持续保持 | 尚未单独确认 |
| 长时间运行、极端快速点击、天气／隧道、导航／CarPlay／仪表联动 | 未逐项完成验证 |
| 其它车型、地区、软件版本 | 未验证；不能直接套用 |

这里的“通过”为用户本轮实车反馈，不代表所有功能或长期稳定性均已覆盖。已核对输入摘要见 [firmware.json](firmware.json)。

### 从源码构建

需要 Go 1.23 或更新版本、JDK 17，以及自行提供的对应固件原始 `viewhandler.jar` 与 `MIBHMI-STD-HCN1.1.134-bin.jar`。主 JAR 仅供编译接口使用，**不替换它**。

```sh
cd menu
go run ./cmd/daynightlab \
  --sources . \
  --jdk /path/to/jdk17 \
  --view-jar /path/to/original/viewhandler.jar \
  --hmi-jar /path/to/original/MIBHMI-STD-HCN1.1.134-bin.jar \
  --output /path/to/new-candidate-directory
```

输出目录必须不存在。生成 `viewhandler.candidate.jar` 和 `build.json`；工具核对原件／class 摘要、拒绝未知固件，检查 ZIP 内容和修改范围，不连接车机、不安装、不改输入。候选修改 10 个已有视图 class，加入 4 个自编辅助 class。不同构建工具链可能产生不同压缩包或编译结果，不能仅凭源码相同就宣称文件与实车版本一致。

### 部署与一键回退

**请完整阅读 [部署、手动重启与回退步骤](docs/DEPLOYMENT.md)，不要只复制替换命令。**

固定顺序：

1. 车辆停稳、挂 P 挡，核对版本，准备独立恢复入口。
2. 暂存候选并备份当前正常 JAR，验证双方摘要。
3. 对 `vw-hmi.service` 设置临时 runtime mask，再停止 HMI。
4. 确认 `inactive/dead`、无 JVM、无文件描述符或内存映射占用目标。
5. 离线写入并校验 SHA-256、inode、属主、权限和 `hmi::sub` 标签。
6. **保持 HMI 停止，由用户手动完整重启车机，再验证。** 不以单独重启 HMI 服务替代。

`safe-update/main.go` 是本轮实车使用的离线门禁工具源码。**其 `prepare` 仅接受本轮安装前版本摘要 `9113535…42c85`，不接受任意原厂或第三方版本，也不是所有 S2995 的通用安装器。** 不要改写摘要或跳过门禁强行安装。

本轮已部署车辆保留的一键回退命令：

```sh
ssh root@10.173.189.1 'sh /data/vw-daynight-menu-20261123-clickfix/restore.sh'
```

**仅适用于已执行本轮部署且该备份目录存在的车辆。** 它先停止 HMI，再恢复本车部署前正常版本；执行成功后仍需手动完整重启。其他车辆应按部署文档在自己的备份目录生成对应脚本，不可直接套用这个路径。

### 源码目录与历史方案

| 目录 | 用途 |
|---|---|
| `menu/` | 当前完整菜单补丁的 Go 构建器及 Java 源码 |
| `safe-update/` | 实车使用的一次性 Linux ARM64 离线更新门禁工具 |
| `docs/` | 部署、恢复、实现及验证边界 |
| `native-auto-menu/` | 历史仅恢复 AUTO 可见性的 Python 工具，不包含当前状态／切换／三行修复 |

历史桥接方案已撤回：在 `/vw/hmi/lib/` 添加 JAR 会改变 boot classpath，与原厂共享归档不匹配；已复现 `shared class paths mismatch` 导致 JVM 初始化失败。不要沿用旧方案。在线覆盖已打开的 JAR 也已复现读取异常，不能再使用这种安装或恢复方式。

### 许可证

自编源码采用 MIT。厂商固件、原厂或修改后的 JAR、提取的接口类、共享缓存、车辆数据及凭据不随仓库分发。生成文件留在本地。本项目与大众及供应商无隶属或背书关系。

## English

Restores native headlight-linked AUTO on the inspected Chinese ICAS3 S2995 / HCN1.1.134 build, using the AGL root method provided by [ICAS3 AGL Root](https://github.com/vw-id-headunit-lab/icas3-agl-root). The current adapter preserves the saved AUTO selection, shows all three rows, and handles manual selection when AUTO already produces the same effective theme. The user reported successful vehicle validation after a manual full headunit reboot on 2026-11-23; this is not an exhaustive or long-term reliability claim.

Only `viewhandler.jar` is replaced. The main JAR, startup scripts, bundles, boot classpath and shared archives remain unchanged. Go builds the patch offline and provides a one-shot deployment gate; there is no Go background controller or solar/time policy.

**Stop and runtime-mask HMI, confirm there are no JVM/JAR readers, then apply or restore. Never overwrite a live JAR. After either operation, leave HMI stopped and manually reboot the complete headunit.** Unverified HMI modifications must not be attached directly to startup: the factory watchdog and `FailureAction=reboot-immediate` can cause a reboot loop if the same failing build is loaded at every boot. Preserve an independent recovery channel and same-vehicle backups.

See [deployment and rollback](docs/DEPLOYMENT.md) and [implementation](docs/IMPLEMENTATION.md). The published update tool is pinned to the tested previous version, not a universal installer. No vendor binaries are distributed. MIT covers authored code only.
