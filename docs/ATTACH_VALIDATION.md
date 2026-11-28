# Attach 验证

## S2995 实车与台架验证（2026-11-28）

- 修复保留 HotSpot Attach。Agent 在原厂 UI 线程更新菜单实例，移除时恢复控件、数据接口及点击监听器。构建出的 Agent 不申请 JVM 类重定义或重转换权限。
- 车机使用 HCN1.1.134 和原厂 Zulu 8 ARM64 JVM。通过 BetterMEB 包装安装后，附加返回 `INSTALLED`，停止后返回 `ORIGINAL`，再次启动返回 `INSTALLED`。观察 65.5 秒后状态仍正常，HMI JVM 的 PID 和启动时间一致，原厂 `viewhandler.jar` 保持原始摘要。
- 测试期间临时设置 HMI 服务的失败动作为 `none`，保留 60 秒 watchdog。验证后恢复原厂失败动作，期间 HMI 和车机均未重启。
- QEMU 台架显示日间、夜间和自动三行。实测选择 AUTO、重开菜单保持选中、再切回手动日间。真实灯光联动、长期稳定性和跨重启保存待测。
- 本轮实车使用的 2995 单文件补丁 SHA-256 为 `57c88928371efc67de0b5ed7624483a1a83d04a899be5ca000b23e8c03316d8d`。3001 仅构建验证。

## 历史 HotSwap 实现的虚拟机验证（2026-11-24）

### 验证环境

独立 QEMU AGL，原厂 Zulu/OpenJDK 8.0_265 ARM64 JVM；先启动原始固件，再执行单文件补丁。
没有 startup agent，没有绑定替换 viewhandler。开发机使用 Go 1.27.1 / JDK 21.0.12.1，
CI 使用 Go 1.23 / JDK 17；输出 Agent/helper 和目标定义均为 Java 8 class。

### 收敛后的构建

- 入口为 `python3 attach/build.py --version 2995 --jdk ... --output ...`。
- 配置只来自 `source/2995/inputs.json`。
- 构建直接生成十个目标定义，不产生完整候选 JAR，菜单适配器只编译一次。
- 十个目标定义与收敛前已验证实现逐字节一致。
- 运行时几何规则与 class 字节修改使用同一份配置，测试覆盖修改该配置后两端同步变化。

### 实际验证

| 项目 | 结果 |
| --- | --- |
| 动态加载 | 两行原菜单变三行，五个已创建节点刷新 |
| AUTO 选择 | 原 ASL API 读回保存值 2 |
| 模拟夜间输入 | 保存值 2，原日夜传感值 2，深色画面 |
| 模拟日间输入 | 保存值 2，原日夜传感值 1，浅色画面 |
| 同外观转手动日间 | 保存值 2→0 |
| remove | 恢复原两行菜单 |
| 再次／重复加载 | 正常、幂等，不重复放大控件 |
| 临时文件 | 完成后无报告或 JAR，仅剩操作锁 |

整轮 JVM PID 为 `1080`，start tick `381`，RuntimeMXBean start time 为
`1795508519324`。安装、卸载、再次加载中 identity 保持一致，没有重启 HMI。
磁盘 `viewhandler.jar` 始终匹配版本清单的原始 SHA-256。

本机验证单文件 SHA-256：

```text
4d482b2d6f96ad700576cd8b474389b3e63d0193aa395903fae8d3a6a4ee2cd0
```

不同工具链可能产生不同包摘要。独立 JSON、原生截图和日志保留在本机忽略目录；
构建器仍输出 `vm_verified=false`，不会根据编译成功伪造 UI 或实车验收。

### 自动化与边界

当时 27 项测试通过，含 Python/Java 20 项、Go 7 项。Go race 与 vet 通过。
测试包含直接 class 构建、版本规则、未知固件拒绝、类范围、事务恢复、过期回调、
UI 排队／中断、临时文件、Attach 协议和真实宿主 JVM Attach。

CI 在 `master` 推送时运行，单独执行的源码测试和最终构建都复用同一个 Attach 模块。
这轮历史验证只覆盖虚拟机。实车结果见上方 2026-11-28 的记录。
