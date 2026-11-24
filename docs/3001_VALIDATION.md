# 3001 / HCN1.1.153 实验适配（2026-11-24）

## 输入来源

用户提供原始 `viewhandler.jar`、`MIBHMI-STD-HCN1.1.153-bin.jar` 和 `osgi.jar`，复制到 `source/3001/`：

| 文件 | SHA-256 |
| --- | --- |
| viewhandler.jar | `781a050ef2315797e9a20a578e368948271566208c743c898a95b0b6a56f391b` |
| MIBHMI-STD-HCN1.1.153-bin.jar | `9af3c4a335693cc7baf081dda5b938e6e5fb1dc463bb3dd1799cf2e62740819d` |
| osgi.jar | `69a64d1a1c4dd99b292f9d52af0bb25ad9d6d6b1260f6ce03f7fc7608b43844b` |

3001 原始 `osgi.jar` 大小为 12062 字节，已核对 SHA-256 和逐字节比较，
与 `source/2995/osgi.jar` 以及此前验证所用输入完全一致，仅用于编译接口。
来源已更新至 `source/3001/inputs.json`，此前混合虚拟机验证所用 JAR 字节不变。
完整 3001 根镜像和资源镜像尚未提供。

补齐原始 OSGi 来源后，3001 单文件补丁重新构建通过，21 项 Python／Java 测试、
7 项 Go 测试及 race／vet 检查通过。再次查询原验证 JVM，PID、启动时间及 INSTALLED 状态不变。
本次未更换虚拟机中的补丁，下面的完整交互验证及补丁摘要对应原验证构建。

## 静态比较与构建

两版 viewhandler 的条目集合相同，2834 个条目中有 4 个变化：Maven 元数据两项、
`climate2stamp/C__0.class` 和 `SkinLiteralPool.class`。本补丁十个控制中心目标 class **逐字节相同**，
所以复用相同的属性、指令偏移、控件索引和几何规则，并保留 3001 独立的整个 JAR 摘要门禁。

主 HMI JAR 有 1072 个条目变化，没有新增或删除条目；日夜、SystemBase 和控制中心相关 class
在本次比较范围内没有变化。菜单适配器引用的 SystemBaseAPI、DayNightService、CloneData、
AbstractNodeController、WidgetModel、Invoker 和 ServiceManager 已逐字节核对一致。
编译 3001 补丁和用于独立测试的 bench adapter 均通过，原 JVM 接口自检通过。

REA 的只读 ZIP 清单检查记录为
`ev_0b7d37d19db3a86f7c95605470b3bca842063ac2d4cb93b069d1c40e6e663f67`，
根摘要与上表一致，未观察到完整性矛盾。该 Evidence 仅覆盖容器静态清单，不证明 Java 语义或运行成功。
目标 class 和接口比较使用逐项原始字节核对，运行结果单独记录如下。

## 混合虚拟机验证

原厂 2995 Linux 根镜像和 HMI 资源镜像只读挂载；在 JVM 启动前，将用户提供的**未打补丁的**
3001 主 JAR 与 viewhandler 从 RAM 绑定至原路径。主 JAR 在客体中的路径沿用 `.134` 文件名，
实际字节为上表的 `.153` 摘要。日志报告：

```text
hmisoftware.version:=HCN1.1.153_ICAS_CHN
hmisoftware.base.version:=HCN1.1.153-202511201724
```

启动原菜单后再执行单文件 Attach，**没有在开机时加载补丁**。

| 检查 | 结果 |
| --- | --- |
| 2995 补丁用于 3001 JAR | 正确拒绝，状态 ORIGINAL，退出 1 |
| 3001 补丁动态加载 | INSTALLED，十个类，五个现存节点 |
| 原两行菜单 | 动态变为日间／夜间／自动三行 |
| AUTO 选择及重开 | 保存模式 2，AUTO 选中 |
| 模拟夜间输入 | 保存值 2，原传感接口值 2，深色主题 |
| 模拟日间输入 | 保存值 2，原传感接口值 1，浅色主题 |
| 同外观转手动日间 | 保存值 2→0 |
| 卸载 | 同进程恢复原两行菜单 |
| 再次／重复加载 | 正常，无重复放大 |

整轮 JVM PID `1089`，start tick `448`，RuntimeMXBean start time `1795509886007` 保持不变。
原 3001 JAR 摘要保持不变，磁盘上的 2995 镜像也没有回写。默认临时报告／JAR 清理，锁留在 `/tmp`。

上述混合虚拟机验证所用 3001 补丁 SHA-256：

```text
150bd014e80fd5f90d51d5bc0e2fdc556f9a1df4be4c9de946554a53cfd2a0be
```

## 覆盖边界

以上证明 3001 主 JAR 和 viewhandler 在当前混合 bench 环境中的编译、加载和菜单行为。
**不等于完整 3001 系统验证或实车验证。** 3001 原始系统库、其它资源、共享归档、
原厂启动流程和实车灯光硬件仍需在完整环境中另测。

CI 为 2995 和实验 3001 分别构建，下载时应选择自己的版本；摘要检查不允许交叉安装。
