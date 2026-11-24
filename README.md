# ICAS3 AGL Auto Day Night

[简体中文](#简体中文) | [English](#english)

## 简体中文

车机只需部署 **`daynight-patch` 一个 Linux ARM64 可执行文件**，直接运行即可自动找到 HMI 并加载补丁。
安装和卸载均无需停止或重启 HMI，原厂 JAR 保持不变。

适用于中国版 ICAS3 S2995 / HMI HCN1.1.134，恢复日间／夜间／自动三行菜单，并修复 AUTO 选中状态与手动切换。
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

需要 AGL root、Java 8 HotSpot Attach、类重定义和类重转换能力。执行用户应为 JVM 所属用户或 root，控制中心应已初始化。

构建需要 Python 3.11+、Go 1.23+、JDK 17+。当前输入文件已放在 `source/2995/`，仅用于开发机和 CI 编译：

| 文件 | SHA-256 / 用途 |
| --- | --- |
| 原始 `viewhandler.jar` | `9bd5764fabe2cf79e5193f2dd39c38954f6170a8f016c229c133e2b8165a1ce1` |
| 原始 `MIBHMI-STD-HCN1.1.134-bin.jar` | `c27e37f93b9877c023487af8566e5cb33b9475835fcee6fe1b68f6c8affc7e17` |
| 同一固件的 `osgi.jar` | 仅用于本地编译接口 |

不匹配的固件或已存在的其它补丁 helper 会被拒绝。其它车型、地区和软件版本未验证。

### 从源码构建（开发机）

从仓库根目录执行，输出目录必须不存在。唯一构建链直接生成内存 class 补丁，菜单适配器只编译一次：

```sh
python3 attach/build.py \
  --version 2995 \
  --jdk /path/to/jdk17 \
  --output /tmp/daynight-2995
```

输出内容：

| 文件 | 使用位置 |
| --- | --- |
| `daynight-patch` | 复制到车机的唯一补丁文件，已内置 Agent 和辅助代码 |
| `build.json` | 留在开发机，核对补丁 SHA-256 |

构建过程不连接车机、不安装补丁。构建产物保留原固件内容的权利边界。固件输入与生成补丁不属于项目 MIT 授权范围。

### GitHub 自动构建

推送到 `master` 时触发 `Build patch`，也可在 Actions 页面手动运行。流程验证输入摘要、运行测试、
构建 Linux ARM64 单文件补丁，并上传 `daynight-patch-2995-linux-arm64` artifact，保留 14 天。
压缩包包含 `daynight-patch` 和 `build.json`，另附压缩包 SHA-256；车机只需复制补丁文件。

版本输入按 `source/<版本>/` 组织。增加其它版本时需完成适配，再更新构建矩阵；当前只启用 `2995`。

### 验证状态

2026-11-24，在 QEMU AGL / 原厂 Zulu 8.48.51.251 ARM64 JVM 中验证：

- 已运行的菜单从两行动态变为三行，AUTO 选中及菜单重开正常。
- 模拟日夜信号可切换实际主题，AUTO 保存值保持不变。
- AUTO 可切回相同外观的手动日间或夜间模式。
- 卸载恢复、再次加载和重复加载正常，JVM PID 与启动时间保持不变。
- 原 `viewhandler.jar` SHA-256 保持不变。
- 27 项自动化测试通过，Go race 检查、vet 和 Linux ARM64 构建通过。

实车 Attach、真实灯光硬件、长期稳定性和跨重启保存尚未验证。虚拟机结果不代表实车验收。
修复与验证详情见 [代码 review 与重构](docs/CODE_REVIEW.md)。

### 源码结构

| 路径 | 用途 |
| --- | --- |
| `attach/` | Agent、Attach 客户端、构建器及回归测试 |
| `docs/` | 实现、review 与验证说明 |
| `source/2995/` | 当前固件的构建输入与摘要清单 |
| `source/2995/inputs.json` | 文件与 class 摘要、字节码修改点及运行时控件规则 |

### 许可证

自编源码采用 MIT。`source/2995/` 中的固件 JAR 及构建出的修改类仍归原权利人；MIT 仅覆盖自编源码。车辆数据及凭据不随项目分发。本项目与大众及供应商无隶属或背书关系。

## English

Deploy just **`daynight-patch`, a single Linux ARM64 executable**, to the headunit. Run it to detect the HMI JVM and attach the patch automatically.
Installation and removal require no HMI stop or restart; the factory JARs remain unchanged.

Targets the inspected Chinese ICAS3 S2995 / HMI HCN1.1.134 build. It restores the Day / Night / Auto menu and fixes saved AUTO selection and manual switching.
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

Requires AGL root, Java 8 HotSpot Attach, class redefinition and retransformation. Run as the JVM owner or root, after the control center has initialized.

Building requires Python 3.11+, Go 1.23+ and JDK 17+. The current inputs are stored under `source/2995/` and used only by the development machine and CI:

| File | SHA-256 / purpose |
| --- | --- |
| Original `viewhandler.jar` | `9bd5764fabe2cf79e5193f2dd39c38954f6170a8f016c229c133e2b8165a1ce1` |
| Original `MIBHMI-STD-HCN1.1.134-bin.jar` | `c27e37f93b9877c023487af8566e5cb33b9475835fcee6fe1b68f6c8affc7e17` |
| `osgi.jar` from the same firmware | Local compilation interfaces only |

Mismatched firmware and existing helpers from another patch build are rejected. Other vehicles, regions and software versions have not been verified.

### Build from source (development machine)

Run from the repository root. The output directory must not already exist. One build pipeline generates in-memory class definitions directly and compiles the menu adapter once:

```sh
python3 attach/build.py \
  --version 2995 \
  --jdk /path/to/jdk17 \
  --output /tmp/daynight-2995
```

Build outputs:

| File | Where it is used |
| --- | --- |
| `daynight-patch` | The only patch file copied to the headunit; embeds the Agent and helper code |
| `build.json` | Kept on the development machine to verify the patch SHA-256 |

Building does not connect to a headunit or install the patch. Firmware inputs and the generated patch retain their original rights and are outside the project’s MIT grant.

### GitHub automatic build

A push to `master` triggers `Build patch`; it can also be started manually from Actions. The workflow verifies input digests,
runs tests, builds the Linux ARM64 single-file patch and uploads the `daynight-patch-2995-linux-arm64` artifact for 14 days.
The archive contains `daynight-patch` and `build.json`, with a separate archive SHA-256 file. Copy only the executable to the headunit.

Versioned inputs live under `source/<version>/`. Implement compatibility before adding a version to the workflow matrix; only `2995` is currently enabled.

### Verification

Verified on 2026-11-24 in QEMU AGL with the factory Zulu 8.48.51.251 ARM64 JVM:

- A running two-row menu becomes three rows; Auto selection and menu reopening work.
- Simulated day/night signals change the effective theme while preserving the saved Auto mode.
- Auto can switch to manual Day or Night even when the effective theme already matches.
- Removal, reinstallation and repeated installation work with unchanged JVM PID and start time.
- The original `viewhandler.jar` SHA-256 remains unchanged.
- All 27 automated tests pass, along with Go race checks, vet and Linux ARM64 builds.

Real-vehicle Attach, physical lighting hardware, long-term stability and persistence across reboots have not been verified. VM results do not establish vehicle validation.
See the [code review and refactoring record](docs/CODE_REVIEW.md) for fixes and verification details.

### Source layout

| Path | Purpose |
| --- | --- |
| `attach/` | Agent, Attach client, builder and regression tests |
| `docs/` | Implementation, review and verification notes |
| `source/2995/` | Current firmware build inputs and digest manifest |
| `source/2995/inputs.json` | File/class digests, bytecode edits and live widget rules |

### License

Authored source code is licensed under MIT. Firmware JARs under `source/2995/` and derived modified classes retain their original rights; MIT covers authored source code only. Vehicle data and credentials are not distributed. This project is not affiliated with or endorsed by Volkswagen or its suppliers.
