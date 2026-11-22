# ICAS3 AGL Day/Night

[简体中文](#简体中文) | [English](#english)

## 概述 / Overview

ICAS3 SW 2995 原车日夜主题研究与控制工具。基于 [ICAS3 AGL Root](https://github.com/vw-id-headunit-lab/icas3-agl-root) 项目提供的 AGL root 方法，进一步开发原厂 AUTO 菜单恢复及日夜主题自动切换功能。

本仓库仅含自编源码、配置示例和固件摘要，不含固件、厂商 JAR、修改后的厂商二进制、凭据、车辆数据或本机配置。

## 简体中文

### 当前状态

| 功能 | 验证状态 |
|---|---|
| 原车菜单恢复“自动” | 本机 QEMU 原厂 JVM/HMI 中文界面验证通过 |
| 点击自动、保存模式 2、重开菜单保持选中 | 虚拟 HMI 验证通过 |
| 模拟 DSI 日夜信号驱动主题双向切换 | 虚拟 HMI 验证通过，非真实车辆信号 |
| 手动日间／夜间 | 虚拟 HMI 验证通过 |
| 固定时刻／离线日出日落控制器 | 本地 Go 测试及 Go→Java 模拟后端联调通过，未部署到虚拟 HMI |
| 真实车机、原厂预加载缓存、冷启动持久化 | 尚未验证 |

### 支持范围与前置条件

目标为已分析的中国版 ICAS3 S2995，HMI HCN1.1.134。补丁生成器校验原始 JAR 的 SHA-256；不能按车型名称推断其它版本兼容。真实部署需要 AGL root、同车原件备份及独立恢复入口。AGL root 不等于 Android root。

### 两种方案

1. 原厂 AUTO：恢复控制中心原有第三项，跟随车辆日夜设计信号。原下拉框一次两行，列表向上滑动可见自动。
2. 自定义自动：固定时间或经纬度离线日出日落，借助同 JVM 桥接模块调用原生主题切换接口。

自定义配置选择 `off / native-auto / fixed / solar`，同一控制器仅使用一个策略；测试原厂菜单时停用自定义调度，避免覆盖手动选择。

### 仓库结构

- `cmd/`、`internal/`：Go 控制器、太阳计算、持久化、回退及测试。
- `hmi/src/`：受 token 保护的回环 Java 桥接模块；默认禁止写入。
- `native-auto-menu/`：原车菜单候选补丁生成器、JVM 验证与方法测试；不包含原厂类。
- `examples/`：默认关闭的配置、定位输入与只读服务模板。
- `scripts/`：构建、测试、候选集成生成、打包和文字选择工具。

### 快速开始

仅 Go 控制器不需要厂商类库：

```sh
go test ./...
go build -o daynight ./cmd/daynight
./daynight --config config.json init
./daynight --config config.json configure --day-start 07:00 --night-start 18:30
./daynight --config config.json select fixed
./daynight --config config.json preview
```

日出日落示例（坐标须换成自己的城市）：

```sh
./daynight --config config.json configure --latitude 31.23 --longitude 121.47 --sunset-offset -15
./daynight --config config.json select solar
./daynight --config config.json preview
```

`preview` 不连接 HMI；`--at` 只允许预览。`status/apply/run/restore` 需要已集成的桥接服务。默认只读，真实写入同时需要桥接 `allow_write=true` 与控制程序 `--write`。桥接模块应运行于原 HMI JVM，另起 Java 进程不能控制现有实例。

### 原车菜单补丁

自行提供对应原件，生成到新目录，输入不会被改写：

```sh
python3 native-auto-menu/build_patch.py --view-jar /path/to/original/viewhandler.jar --output candidate
```

可选保持 AUTO 选中状态修复：

```sh
python3 native-auto-menu/build_patch.py --view-jar /path/to/original/viewhandler.jar --main-jar /path/to/original/main.jar --include-selection-fix --output candidate-with-selection
```

只显示补丁更改两套布局根条目的 `visible=false`；可选状态修复区分保存模式 2 和实际主题 0/1。JVM StackMapTable 同步修正。输出是候选文件，不是刷机包，仓库没有一键实车安装器。

### 构建和验证

Go 测试：`go test -race ./...`。完整桥接构建需自行提供 Go、JDK 17、厂商 HMI/OSGi 编译依赖；输出 Java 8 字节码。通过 `DAYNIGHT_GO`、`DAYNIGHT_JDK`、`DAYNIGHT_HMI_JAR`、`DAYNIGHT_OSGI_JAR`、`DAYNIGHT_BUNDLES`、`DAYNIGHT_RUN_HMI` 配置路径后执行 `sh scripts/build.sh`。

原车菜单测试使用 `DAYNIGHT_VIEW_JAR`、`DAYNIGHT_HMI_JAR`：`python3 -m unittest discover -s native-auto-menu`。

2026-11-22 虚拟机验收使用原固件只读盘、候选 JAR 的 RAM 绑定和真实触控输入；观察到 AUTO 的保存／发布模式均为 2，模拟夜间后 `day=false`，日间后 `day=true`。重开菜单选中保持，手动夜间忽略日间信号。虚拟台架没有启用原车 classes.jza/lookup 优化，持久化服务为内存模拟；不据此推断实车缓存及冷启动行为。

### 卸载与恢复

停止控制器后，可先 `restore` 预览，再显式 `--write restore` 恢复首次改动前模式。`select off` 仅停止接管，不撤销已保存的原厂 AUTO。修改过原车 JAR／启动配置时，用同车安装前备份撤销自己的改动；不能用其它车辆原件覆盖。

### 边界与风险

原 `vw-hmi.service` 有 watchdog 和 `FailureAction=reboot-immediate`，不合适的启动或字节码修改可能导致 HMI 重启。实车部署前必须确认同车备份和独立恢复路径；本项目未完成实车验收。原厂 AUTO 需要有效车辆信号，太阳算法不感知隧道／阴雨。导航、CarPlay、Android 应用和仪表联动未逐项验证。本项目与大众及供应商无隶属或背书关系。

### 许可证

自编源码按 MIT 许可证发布；第三方说明见 `THIRD_PARTY_NOTICES.txt`。厂商文件不随仓库分发。

## English

Experimental ICAS3 S2995 native day/night menu restoration and AGL policy controller. Source-only publication; no original or modified vendor binaries, firmware, credentials or machine-specific configuration.

Building on the AGL root method provided by [ICAS3 AGL Root](https://github.com/vw-id-headunit-lab/icas3-agl-root), this project develops native AUTO menu restoration and automatic day/night theme switching.

Native AUTO menu visibility, touch selection, saved mode 2, retained menu selection, simulated vehicle-signal day/night switching and manual modes were verified in a local QEMU guest running the original JVM and HMI. Real-vehicle behavior, production preload caches and cold-boot persistence remain unverified. Fixed-time and offline solar policies passed host tests and a simulated Java backend integration, but were not deployed into that VM session.

Run `go test ./...` and build `./cmd/daynight` for firmware-independent work. `preview` is offline; writes require both the controller flag and a write-enabled bridge in the original HMI JVM. Supply matching vendor inputs locally to generate menu candidates with `native-auto-menu/build_patch.py`; source hashes are checked and input files remain unchanged. Never treat generated candidates as a complete flash package. Keep same-device backups and an independent recovery path. See the Chinese sections for commands, scope and rollback behavior. MIT applies to authored code only.
