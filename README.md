# ICAS3 AGL Auto Day Night

[简体中文](#简体中文) | [English](#english)

## 简体中文

车机只需部署 **`daynight-patch` 一个 Linux ARM64 可执行文件**，直接运行即可自动找到 HMI 并加载补丁。
安装和卸载均无需停止或重启 HMI，原厂 JAR 保持不变。

适配中国版 ICAS3 S2995 / HMI HCN1.1.134，并增加 S3001 / HMI HCN1.1.153 实验构建，恢复日间／夜间／自动三行菜单，并修复 AUTO 选中状态与手动切换。
基于 [ICAS3 AGL Root](https://github.com/vw-id-headunit-lab/icas3-agl-root) 提供的 AGL root 方法开发。

### 快速使用

把构建得到的 `daynight-patch` 复制到车机，以 HMI 所属用户或 root 执行：

```sh
chmod +x ./daynight-patch
./daynight-patch          # 加载补丁
./daynight-patch status   # 查询状态
./daynight-patch remove   # 卸载并恢复原菜单
```

默认自动识别唯一的 HMI JVM，无需指定 PID、JAR 路径或 `--report`。
控制中心应先完成初始化；若提示菜单未就绪，先打开一次控制中心，再运行补丁。
检测到多个 HMI JVM 时可用 `./daynight-patch --pid 1234` 明确选择。

源码、构建依赖和固件输入留在开发机，车机只复制补丁可执行文件；原车系统文件保持原样。
重启 HMI 后补丁消失，需要再次运行；项目不添加自启服务。

### 功能

- 控制中心同时显示“日间、夜间、自动”三行。
- 选择 AUTO 后，菜单保持“自动”选中，实际主题跟随原车日夜信号。
- AUTO 的实际主题与目标手动模式相同时，仍可切回手动日间或夜间。
- 动态更新已经创建的菜单，无需重新启动界面。
- 支持查询、卸载恢复和重复加载；卸载保留用户当前选择的日夜模式。


主题切换使用原厂日夜信号。

### 临时文件与诊断

结果直接输出到终端，正常使用不会在 `/data` 下写入报告：

| 文件 | 位置与清理方式 |
| --- | --- |
| 临时 Agent／helper JAR | 目标 JVM 的 `/tmp`；操作完成、取得结果后删除 |
| 默认报告 | `/tmp` 随机文件；读取并打印后立即删除 |
| 操作锁 | 每个 JVM／构建复用一个小型锁目录，随 `/tmp` 清空 |

若操作超时、结果不确定，会暂留 `/tmp` 文件，供尚未结束的 Agent 完成操作。
需要保留一次诊断结果时，可选用新的 `/tmp` 文件：

```sh
./daynight-patch status --report /tmp/daynight-status-01.json
```

显式指定的报告会保留，已有文件不覆盖；`/data` 下的报告路径会被拒绝。
`RECOVERY_REQUIRED` 表示恢复未完成，先执行 `./daynight-patch remove`。
卸载恢复原视图、控件和监听器，并保留用户选择的日夜模式；helper 类和 Attach 原生库可能保留至 JVM 退出。

详细说明见 [Attach 使用说明](attach/README.md)。

### 支持环境与构建输入

需要 AGL root 和 Java 8 HotSpot Attach。执行用户应为 JVM 所属用户或 root，控制中心应已初始化。

构建需要 Python 3.11+、Go 1.23+、JDK 17+。输入按版本放在 `source/`，仅用于开发机和 CI 编译：

| 固件 | HMI | 输入与规则 | 验证范围 |
| --- | --- | --- | --- |
| 2995 | HCN1.1.134 | [source/2995/inputs.json](source/2995/inputs.json) | 2995 固件虚拟 HMI |
| 3001 | HCN1.1.153 | [source/3001/inputs.json](source/3001/inputs.json) | 实验构建；3001 原始 JAR 在 2995 底层镜像上的混合虚拟机验证 |

3001 的三个原厂 JAR 均由用户提供，来源记录在清单中；其中 `osgi.jar` 已核对与 2995 逐字节相同，仅用于编译接口。完整 3001 根镜像和资源镜像尚未提供。
各版本有独立的原 JAR 摘要校验，不能混用补丁。

不匹配的固件或已存在的其它补丁 helper 会被拒绝。其它车型、地区和软件版本未验证。

### 从源码构建（开发机）

在仓库根目录运行构建命令。输出目录应为新目录。构建器校验目标视图摘要并编译 Attach Agent 和菜单适配器。

```sh
python3 attach/build.py \
  --version 2995 \
  --jdk /path/to/jdk17 \
  --output /tmp/daynight-2995
```

构建 3001 时，将 `--version` 改为 `3001`，输出使用新的目录，例如 `/tmp/daynight-3001`。

输出内容：

| 文件 | 使用位置 |
| --- | --- |
| `daynight-patch` | 复制到车机的唯一补丁文件，已内置 Agent 和辅助代码 |
| `build.json` | 留在开发机，核对补丁 SHA-256 |

构建过程不连接车机、不安装补丁。构建产物保留原固件内容的权利边界。固件输入与生成补丁不属于项目 MIT 授权范围。

### GitHub 自动构建

推送到 `master` 时触发 `Build patch`，也可在 Actions 页面手动运行。流程验证输入摘要、运行测试、
构建 Linux ARM64 单文件补丁，并分别上传 `daynight-patch-2995-linux-arm64`、`daynight-patch-3001-linux-arm64` artifact，保留 14 天。
压缩包包含 `daynight-patch` 和 `build.json`，另附压缩包 SHA-256；车机只需复制补丁文件。

版本输入按 `source/<版本>/` 组织。增加其它版本时需完成适配，再更新构建矩阵；当前启用 `2995` 和实验版本 `3001`。

### 验证状态

2026-11-28 在 S2995 / HCN1.1.134 实车的原厂 Zulu 8 ARM64 JVM 验证了运行时附加、移除恢复和再次附加。再次附加后观察超过一个 60 秒 watchdog 周期，HMI PID、启动时间和原厂 `viewhandler.jar` 摘要一致。

QEMU 台架验证了三行菜单、AUTO 保持选中和切回手动日间。真实灯光联动、长期稳定性和跨重启保存待测。3001 的验证范围见 [3001 验证](docs/3001_VALIDATION.md)。运行时修复的记录见 [Attach 验证](docs/ATTACH_VALIDATION.md)。

### 源码结构

| 路径 | 用途 |
| --- | --- |
| `attach/` | Agent、Attach 客户端、构建器及回归测试 |
| `docs/` | 实现、review 与验证说明 |
| `source/<版本>/` | 对应固件的构建输入与摘要清单 |
| `source/2995/inputs.json` | 文件与 class 摘要、字节码修改点及运行时控件规则 |

### 许可证

自编源码采用 MIT。`source/` 中的固件 JAR 及构建出的修改类仍归原权利人；MIT 仅覆盖自编源码。车辆数据及凭据不随项目分发。本项目与大众及供应商无隶属或背书关系。

## English

Deploy just **`daynight-patch`, a single Linux ARM64 executable**, to the headunit. Run it to detect the HMI JVM and attach the patch automatically.
Installation and removal require no HMI stop or restart; the factory JARs remain unchanged.

Targets the inspected Chinese ICAS3 S2995 / HMI HCN1.1.134 build, with an experimental S3001 / HMI HCN1.1.153 build. It restores the Day / Night / Auto menu and fixes saved AUTO selection and manual switching.
Built on the AGL root method provided by [ICAS3 AGL Root](https://github.com/vw-id-headunit-lab/icas3-agl-root).

### Quick start

Copy the built `daynight-patch` to the headunit and run it as the HMI JVM owner or root:

```sh
chmod +x ./daynight-patch
./daynight-patch          # Install the patch
./daynight-patch status   # Query status
./daynight-patch remove   # Remove and restore the original menu
```

The executable detects the unique HMI JVM automatically. No PID, JAR path or `--report` is required.
The control center must have initialized; if the patch reports that the menu is not ready, open the control center once and run it again.
If multiple HMI JVMs are detected, select one with `./daynight-patch --pid 1234`.

Keep sources, build dependencies and firmware inputs on the development machine. Copy only the patch executable to the headunit; leave factory system files in place.
Restarting HMI clears the patch; run it again to enable it. No startup service is installed.

### Features

- Shows Day, Night and Auto together in the control center.
- Preserves the Auto selection while the effective theme follows the factory day/night signals.
- Allows switching from Auto to the manual mode that already matches the effective theme.
- Updates existing menu widgets without restarting the interface.
- Supports status queries, removal, reinstallation and repeated installation. Removal preserves the user's selected day/night mode.


Theme switching uses the factory day/night signals.

### Temporary files and diagnostics

Results are printed to the terminal. Normal use does not write reports under `/data`:

| File | Location and cleanup |
| --- | --- |
| Temporary Agent / helper JARs | Target JVM's `/tmp`; deleted once the operation completes and its result is read |
| Default report | Random file under `/tmp`; deleted after reading and printing |
| Operation lock | One small directory reused per JVM / build; removed when `/tmp` is cleared |

If an operation times out with an unknown outcome, temporary files are retained under `/tmp` so an Agent still running can finish.
To keep a diagnostic result, optionally specify a new `/tmp` file:

```sh
./daynight-patch status --report /tmp/daynight-status-01.json
```

Explicit reports are retained and existing files are never overwritten. Report paths under `/data` are rejected.
`RECOVERY_REQUIRED` means restoration is incomplete; run `./daynight-patch remove` first.
Removal restores original views, widgets and listeners while preserving the selected day/night mode. Helper classes and Attach native libraries may remain until the JVM exits.

See the [Attach guide](attach/README.md) for details.

### Supported environment and build inputs

Requires AGL root and Java 8 HotSpot Attach. Run as the JVM owner or root after the control center has initialized.

Building requires Python 3.11+, Go 1.23+ and JDK 17+. Inputs are versioned under `source/` and used only by the development machine and CI:

| Firmware | HMI | Inputs and rules | Verification scope |
| --- | --- | --- | --- |
| 2995 | HCN1.1.134 | [source/2995/inputs.json](source/2995/inputs.json) | Virtual HMI using 2995 firmware |
| 3001 | HCN1.1.153 | [source/3001/inputs.json](source/3001/inputs.json) | Experimental build; original 3001 JARs in a hybrid VM with 2995 base images |

All three original 3001 JARs were supplied by the user, with provenance recorded in the manifest. Its `osgi.jar` is byte-identical to the 2995 file and is used only for compile-time interfaces. Complete 3001 root/resource images have not been supplied.
Each build checks its own original JAR digests; patches are not interchangeable.

Mismatched firmware and existing helpers from another patch build are rejected. Other vehicles, regions and software versions have not been verified.

### Build from source (development machine)

Run from the repository root and choose a new output directory. The builder checks the target view digests and compiles the Attach agent and menu adapter.

```sh
python3 attach/build.py \
  --version 2995 \
  --jdk /path/to/jdk17 \
  --output /tmp/daynight-2995
```

For 3001, use `--version 3001` and a new output directory such as `/tmp/daynight-3001`.

Build outputs:

| File | Where it is used |
| --- | --- |
| `daynight-patch` | The only patch file copied to the headunit; embeds the Agent and helper code |
| `build.json` | Kept on the development machine to verify the patch SHA-256 |

Building does not connect to a headunit or install the patch. Firmware inputs and the generated patch retain their original rights and are outside the project’s MIT grant.

### GitHub automatic build

A push to `master` triggers `Build patch`; it can also be started manually from Actions. The workflow verifies input digests,
runs tests, builds the Linux ARM64 single-file patch and uploads separate `daynight-patch-2995-linux-arm64` and `daynight-patch-3001-linux-arm64` artifacts for 14 days.
The archive contains `daynight-patch` and `build.json`, with a separate archive SHA-256 file. Copy only the executable to the headunit.

Versioned inputs live under `source/<version>/`. Implement compatibility before adding a version to the workflow matrix; `2995` and experimental `3001` are enabled.

### Verification

On 2026-11-28, Attach, removal and reattachment passed on an S2995 / HCN1.1.134 head unit running the factory Zulu 8 ARM64 JVM. The JVM PID, start time and factory `viewhandler.jar` digest were identical after reattachment and more than one 60-second watchdog interval.

QEMU testing verified all three menu rows, persistent Auto selection while reopening the menu, and switching back to manual Day. Physical lighting behavior, long-term stability and persistence across reboots still need testing. See the [3001 validation scope](docs/3001_VALIDATION.md) and the [Attach validation record](docs/ATTACH_VALIDATION.md).

### Source layout

| Path | Purpose |
| --- | --- |
| `attach/` | Agent, Attach client, builder and regression tests |
| `docs/` | Implementation, review and verification notes |
| `source/<version>/` | Firmware-specific build inputs and digest manifest |
| `source/2995/inputs.json` | File/class digests, bytecode edits and live widget rules |

### License

Authored source code is licensed under MIT. Firmware JARs under `source/` and derived modified classes retain their original rights; MIT covers authored source code only. Vehicle data and credentials are not distributed. This project is not affiliated with or endorsed by Volkswagen or its suppliers.
