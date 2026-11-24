# Attach 虚拟机验证（2026-11-24）

最终包在独立 QEMU AGL、原厂 Zulu/OpenJDK 8.0_265 ARM64 JVM 中验证。
目标先从原始镜像正常启动，没有 startup agent、没有 bind 覆盖 viewhandler。
打开原菜单确认只有日间／夜间两行，然后向**已经运行的 JVM**执行 Attach。

| 检查 | 实际结果 |
| --- | --- |
| 原菜单 | 两行，没有 AUTO |
| Attach install | `installed=true`，十个类重定义，五个现存节点刷新 |
| 已创建的下拉菜单 | 三行同时可见，无需重新启动 HMI |
| AUTO 选择 | 原 ASL API 读回保存值 2 |
| AUTO 菜单重开 | 仍显示 AUTO 选中 |
| AUTO + 模拟夜间输入 | 保存值 2，原日夜传感接口值 2，实际深色画面 |
| AUTO + 模拟日间输入 | 保存值 2，原日夜传感接口值 1，实际浅色画面 |
| AUTO 同外观转手动日间 | 显式点击触发一次纠正请求，保存值 2→0 |
| Attach remove | `installed=false`，菜单恢复两行，三个几何尺寸恢复 |
| 再次 install | 三行重新出现 |
| 重复 install | 幂等成功，没有重复挂监听／重复放大几何 |

整轮最终包的 JVM identity 为 `1081@localhost`，RuntimeMXBean start time 为
`1795502130291`，`/proc/1081/stat` start tick 为 `377`。
加载前、加载后、卸载后、再次加载后均一致；未调用 stop/restart，未重启 HMI 或 VM。

磁盘 `viewhandler.jar` 始终为原始 SHA-256：

```text
9bd5764fabe2cf79e5193f2dd39c38954f6170a8f016c229c133e2b8165a1ce1
```

最终验证包：

```text
daynight-agent.jar    2c32d1502afce2bb944653d23c70479b1c1215f3ae3a1e8fcb77932a36b1dd82
daynight-helpers.jar  fa110d404a7c174528c90b0963fd13ed8e99958ee2db8b0b03eaad7f01ff3b68
daynight-attach       d03746cf90752d6a496863c4009575473dc27c89bcad3202533a781c4def8e58
```

使用 Go 1.27.1 / Temurin JDK 21.0.12.1 构建；Agent、helper 和视图定义均为 Java 8 class 格式。
其它工具链构建后的包摘要可能不同，不把相同源码自动视作相同已验证二进制。

源码检查：Attach Go 三项测试（含 race）和 vet 通过；真实宿主 JVM Attach／拒绝路径及
构建输入边界四项 Python 测试通过；原 visibility 工具两项测试在本地原固件输入下通过。
原 Go 菜单构建器 vet/build 通过。CI 配置已更新，但托管 CI 尚未执行。

原始包与界面截图保存在本机忽略目录，未随源码分发。JSON 结果、JVM identity、原 JAR
摘要和原生截图是独立证据；构建报告仍保留 `vm_verified=false`，构建器本身不伪造验收结果。

实车 Attach、跨完整重启保存、原厂共享归档开启时的所有场景、长期稳定性未验证。
模拟信号由独立 bench adapter 发送到原 DSI 接口，不能视作真实大灯硬件验收。
