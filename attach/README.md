# 单文件 Attach 日夜补丁

车机只需要 **`daynight-patch` 一个可执行文件**。Agent 与辅助代码已嵌入其中，
不需要复制源码、原始固件 JAR 或单独的 Agent JAR。加载和卸载均无需停止或重启 HMI。

## 使用

以 HMI 所属用户或 root 在车机执行：

```sh
chmod +x ./daynight-patch
./daynight-patch          # 自动找到 HMI 并加载
./daynight-patch status   # 查看状态
./daynight-patch remove   # 卸载并恢复原菜单
```

默认识别唯一一个以 `de.vw.mib.MIBMain` 启动的 JVM，也支持虚拟机的
`bench.platform.BenchPlatform`。没有匹配进程或匹配多个时拒绝；多个实例可用 `--pid 1234` 选择。
原控制中心应已初始化。补丁保持原 JAR 不变，结果包含 JVM PID／启动时间和安装状态。

重启 HMI 后补丁消失，需要再执行一次；没有添加自启服务或自动 Attach。
`RECOVERY_REQUIRED` 时应先成功 remove，不能重复 install 掩盖恢复失败。

## 临时文件

- 运行时需要的 JAR 从可执行文件释放到**目标 JVM 的 `/tmp`**，同一 JVM／构建复用相同路径。
- 操作完成并取得结果后，清理临时 JAR；JVM 持有的已打开文件可以继续读取。
- 报告自动使用随机文件名，读取并打印后立即删除，不在 `/data` 下累计。
- 每个 JVM／构建保留一个小型锁目录，防止两个命令同时操作；`/tmp` 清空后移除。
- 若 Attach 超时、结果不确定，临时文件会保留，避免打断尚未结束的加载；都位于 `/tmp`。
- 车机不需要放置构建用的源码或原始 JAR 副本；部署只复制一个补丁可执行文件。

正常运行不需要 `--report`。临时保留诊断结果时，只能指定新的 `/tmp` 文件：

```sh
./daynight-patch status --report /tmp/daynight-status-01.json
```

显式指定的报告会保留；默认自动生成的报告不会保留。已有报告不覆盖。
卸载后已定义的 helper 类和 Attach 原生库可能仍驻留，直至 JVM 退出。

## 在开发机构建

需要 Python 3.11+、Go 1.23+、JDK 17+ ，构建输入位于 `source/<版本>/`。
源码位于 `attach/`，版本输入与所有修改规则位于 `source/<版本>/inputs.json`。
唯一构建入口直接生成 class 定义，菜单只编译一次；构建完成输出补丁和摘要文件：

```sh
python3 attach/build.py \
  --version 2995 \
  --jdk /path/to/jdk17 \
  --output /tmp/daynight-2995
```

输出目录必须不存在：

| 文件 | 用途 |
| --- | --- |
| `daynight-patch` | 唯一需要复制到车机的 Linux ARM64 补丁 |
| `build.json` | 留在开发机，核对补丁 SHA-256 |

匹配输入摘要见根目录 [README](../README.md)。不匹配固件或另一版本 helper 已加载时拒绝，
换构建前先 remove 并重启 JVM。固件输入及生成的修改类保留原权利归属。

## 测试与边界

```sh
cd attach
go test -race ./...
go vet ./...
cd ..
DAYNIGHT_TEST_JDK=/path/to/jdk17 python3 -m unittest discover -s attach/tests -v
```

覆盖进程唯一识别、Attach 协议、临时文件复用／清理、结果验证、事务回退、
过期回调、控件范围、宿主 JVM Attach 和构建边界。虚拟机验证使用原厂 ARM64 JVM。

实车 Attach、真实灯光硬件和长期稳定性尚未验证；原厂权限限制可能使 Attach 失败。
实现不会自动放宽权限、修改原固件或重启车机。详细 review 见 [代码 review](../docs/CODE_REVIEW.md)。

实验版本 3001 使用 `--version 3001`。三个原厂 JAR 均由用户提供，其中 `osgi.jar` 与 2995 逐字节相同，仅用于编译接口。完整 3001 系统和实车未验证；见 [3001 验证](../docs/3001_VALIDATION.md)。
