# 代码 review 与重构（2026-11-24）

## 单文件部署与临时结果

当前构建只输出 `daynight-patch` 和留在开发机的 `build.json`。车机只复制可执行文件，
直接运行自动找到唯一的 HMI，`status` 查询、`remove` 卸载。Agent/helper 仅嵌入补丁，
运行时释放到目标 JVM 的 `/tmp`，操作完成取得结果后删除临时 JAR；默认报告读取后删除。
同一进程／构建复用稳定的临时目录和一个锁，不会每次运行新增持久化文件。
显式诊断 `--report` 只允许新的 `/tmp` 路径，`/data` 和路径穿越均拒绝。

虚拟机验证：只部署一个可执行文件，默认加载、查询、卸载、再次加载以及连续五次查询通过；
执行后临时目录仅剩锁文件，无报告或 JAR。JVM PID `1079`、start tick `377`、start time
`1795505701300` 保持不变。单文件 SHA-256：
`5f96f877ff162322a5fa8644b53051783fe98e5ac5b9a22617ea39896a2b6987`。
当前自动化测试共 22 项通过（Attach Python/Java 13、Go 7、原 Python 工具 2）。

以下为同日 review 和多文件构建阶段的记录，产物以当前构建结果为准。

本次以新 Attach 实现及共用的日夜菜单适配器为重点，同时检查原构建器和离线更新工具。
保留既有固件摘要、文件替换门禁和 Attach 的内存加载方式。

## 已修复的问题

| 级别 | 问题及影响 | 修复与验证 |
| --- | --- | --- |
| P1 | 视图刷新和回退都失败后，旧 `installed` 布尔值仍可能为 true；再次 install 会直接报告成功 | 提取 `PatchSession` 状态机，失败进入 `RECOVERY_REQUIRED`，仅允许 remove 恢复；故障注入覆盖恢复失败、拒绝再次安装、恢复后重试 |
| P1 | 同一次点击的旧读回被新请求取代后，旧回调仍可能补发切换请求 | ASL 回调校验 epoch 和 request id，跨线程字段可见，纠正请求用原子认领；用重构前源码复现失败，新源码通过 |
| P2 | 现存控件只按高度匹配，重名 Background 也不能唯一确定目标 | 同时核对已钉住的视图、CloneData 索引和控件名，写入前验证完整布局；用无关同高度／同名控件回归测试及真实虚拟 HMI 验证 |
| P2 | UI 等待线程被中断时可能提前返回，已开始的修改仍在进行 | 提取 `UiExecutor`；未执行任务可取消，已开始任务等待完成并保留中断状态，测试验证顺序 |
| P2 | 结果目录不可写或已存在时，可能先修改 HMI 再发现无法写结果 | `AgentRequest` 在修改前以 CREATE_NEW 打开结果文件，保持同一描述符，错误 JSON 完整转义控制字符 |
| P2 | 客户端只检查 JSON 的 ok，无法确认是目标 PID／请求动作和已完成状态 | 解析并核对 action、runtime、start time 和状态一致性，拒绝不完整或未完成报告；Go 回归测试覆盖 |
| P2 | 构建复制失败会留下不完整输出目录 | 构建按验证、编译、打包、发布拆分；失败清理本次新目录，不覆盖已存在构建；测试覆盖 |

## 结构调整

- Agent 入口负责固件验证和编排；事务状态、UI 调度、结果文件各自独立。
- `RuntimeViews` 负责更新，`ViewNodes` 负责有限深度的查找，`GeometryRules` 约束修改对象。
- Go CLI 拆成命令／结果验证、Attach 协议、进程身份及 listener 管理三个模块。
- 构建器复用 `firmware.json` 的摘要，检查 Java 8 输出；bundle 身份包含实际 helper 和补丁字节。
- 原 `safe-update/main.go` 仅进行 gofmt，版本摘要、门禁、写入和回退行为未变，Linux ARM64 构建通过。

## 验证与边界

自动化覆盖真实宿主 JVM Attach、回退失败、LinkageError、排队超时、线程中断、过期回调、
结果保护、控件范围、构建失败和 Attach 协议。原 visibility 工具使用本地匹配固件复测。
Go Attach 测试启用 race，两个 Go 模块 vet 通过，旧离线工具交叉编译通过。
本轮合计 19 项自动化测试通过：Attach Python/Java 13 项、Go Attach 4 项、原 Python 工具 2 项。

重构包在原厂 ARM64 JVM 的独立 QEMU AGL 中验证加载、三行菜单、AUTO、模拟日夜变化、
同外观转手动、卸载和重复加载。整轮对比 PID、RuntimeMXBean start time 和原 JAR SHA-256，
原镜像保持只读；测试 JSON、日志、截图和构建产物留在本机忽略目录。
最终虚拟机整轮 identity：PID `1086`，start tick `398`，RuntimeMXBean start time
`1795504388499`；安装、卸载、再次安装均一致。重构前的过期请求问题已用旧源码重现，
不是仅根据代码阅读推测。

实车 Attach、原厂所有共享缓存组合、长期运行及跨重启保持仍未验证。旧离线安装规则不变。
历史 [Attach 验证](ATTACH_VALIDATION.md) 中摘要对应重构前构建，不代表当前产物。

## 回退

运行中使用**相同构建包**执行 `--action remove`；出现 `RECOVERY_REQUIRED` 时先 remove，
不要以重复 install 掩盖恢复失败。Agent 只修改内存，正常重启 HMI 也会清除这次 Attach；
它不提供自动重启。卸载后 helper class 仍可能保留至 JVM 退出，重启后再换另一构建包。
源码改动均可在 `codex/runtime-attach` 的本地工作区查看，未自动提交或推送。
