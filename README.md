# ICAS3 AGL Day/Night

[简体中文](#简体中文) | [English](#english)

ICAS3 S2995 原厂 AUTO 日夜主题菜单恢复工具。基于 [ICAS3 AGL Root](https://github.com/vw-id-headunit-lab/icas3-agl-root) 项目提供的 AGL root 方法，进一步恢复原车自动日夜主题功能。

## 简体中文

### 当前方案：跟随原车大灯状态

在控制中心恢复原本隐藏的“自动”选项。选择后由原车逻辑处理日夜主题，实车已观察到切换大灯时主题随之切换；大灯置于 AUTO 时，由原车灯光系统判断环境并控制大灯。

本项目只恢复菜单入口，不采集照度或天气数据。阴天、隧道等情况下何时开灯，取决于原车灯光系统。无需额外时间调度、定位、联网或常驻控制程序。

固定时刻、日出日落、自定义策略选择器、Go 控制器及 Java 桥接模块已从当前版本移除。当前版本不修改 HMI 主 JAR、启动脚本、组件注册或预加载缓存，也不包含 AUTO 选中状态修复。

### 验证状态

| 项目 | 状态 |
|---|---|
| 本机虚拟 HMI 显示 AUTO、触控选择 | 已验证 |
| 中国版 S2995 实车完整重启后出现 AUTO | 2026-11-22 已验证 |
| 选择 AUTO 后切换大灯，日夜主题随之变化 | 2026-11-22 用户实车确认 |
| AUTO 在下次完整重启后仍保持控制状态 | 尚待单独验证 |
| 阴天、隧道等不同环境 | 尚未逐项实测，交由原车大灯系统控制 |
| 导航、CarPlay、仪表联动及其它固件版本 | 尚未验证 |

已知现象：选 AUTO 后，菜单可能立即把选中标记显示为当前实际的日间／夜间。已验证车辆在这种情况下仍会随大灯切换主题。当前补丁保留此原厂显示行为。

### 支持范围

已分析的中国版 ICAS3 S2995，HMI HCN1.1.134。需要 AGL root 和本车原件备份。工具仅接受已核对的原始 `viewhandler.jar` 摘要，其它版本需要重新研究。

### 文件与生成方式

- `native-auto-menu/build_patch.py`：生成菜单显示候选文件，不安装、不修改输入。
- `native-auto-menu/test_patch.py`：校验修改范围、原件不变及未知输入拒绝行为。
- `native-auto-menu/VerifyClasses.java`：可选的主机端 class 加载校验器。
- `firmware.json`：版本与文件摘要。

自行提供原件，输出到一个不存在的新目录：

```sh
python3 native-auto-menu/build_patch.py --view-jar /path/to/original/viewhandler.jar --output candidate
```

输出 `viewhandler.candidate.jar` 和 `patch-report.json`。两套控制中心布局的 AUTO 根条目各改一个可见性字节，其它 class 内容保持不变；复用原车文字、点击事件及主题逻辑。

```sh
DAYNIGHT_VIEW_JAR=/path/to/original/viewhandler.jar python3 -m unittest discover -s native-auto-menu
```

未提供厂商原件时，测试会明确跳过依赖原件的范围检查。CI 不下载固件。

### 装车与使用记录

实车已验证的位置为 `/vw/hmi/Resources/skin1/viewhandler.jar`。下拉框一次显示两行，在列表内向上滑动可见第三项“自动”。选择 AUTO 后，按原车大灯状态验证主题变化。

装车前应核对版本和摘要、保存本车原件及标签，并准备独立恢复入口。本项目不提供通用一键安装器。实车发现 `cp -a` 没有保留 SMACK 标签，因此不能据此假定替换文件后权限完整；已验证的更新保留了目标 inode 及原标签。此文件的原访问标签为 `hmi::sub`。

应用或恢复文件后，实车使用完整车机重启验证。单独重启 HMI 服务未建立可靠流程。原 HMI 服务带 `FailureAction=reboot-immediate` 和 watchdog，启动异常可能引发整机重启。

### 回退

使用本车安装前备份恢复 `viewhandler.jar`，保留原文件权限和 SMACK 标签，然后完整重启车机。无需撤销额外调度服务，因为当前方案没有此类服务。

历史桥接方案已撤回：向 `/vw/hmi/lib/` 增加 JAR 会改变 boot classpath，与原厂 `classes.jza` 共享归档不匹配；已在相同 JVM 和缓存输入下复现 `shared class paths mismatch`，导致 JVM 初始化失败。不要按旧版本说明重新安装桥接模块。

### 许可证与边界

自编源码采用 MIT；厂商固件、原厂／修改后 JAR、凭据及车辆数据不随仓库分发。生成的候选文件留在本地。本项目与大众及供应商无隶属或背书关系。

## English

Restores the hidden native AUTO day/night menu on the inspected Chinese ICAS3 S2995 build, building on the AGL root method provided by [ICAS3 AGL Root](https://github.com/vw-id-headunit-lab/icas3-agl-root).

On 2026-11-22 the user confirmed that AUTO appears after a full headunit reboot and the theme follows changes in headlight state. With headlights set to AUTO, environmental decisions remain with the factory lighting system. Weather and tunnel scenarios have not been individually tested. AUTO persistence across another full reboot remains unverified.

The current release contains only the visibility patch generator and checks. Fixed-time/solar policies, the Go controller, Java bridge and custom selector have been removed. The HMI main JAR, startup script, bundle configuration and preload archives remain factory files. The menu may highlight the effective day/night mode after AUTO is selected; the confirmed vehicle still follows headlight changes.

Supply the matching original `viewhandler.jar` locally and run the Python command above. The tool checks the source hash, changes one visibility byte in each of two layout classes, and writes a new candidate without installing it or altering its input. Run tests with `DAYNIGHT_VIEW_JAR`. Preserve same-vehicle backups, original file labels and an independent recovery path; use a full headunit reboot for validation.

The previous bridge installation changed the boot classpath and failed against the factory shared archive with `shared class paths mismatch`; it has been withdrawn. No vendor binaries or firmware are distributed. MIT covers authored code only.
