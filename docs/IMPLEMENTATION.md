# 实现结构

项目只有一条 Attach 构建链和一个车机入口。车机执行 `daynight-patch`，开发机执行
`python3 attach/build.py --version 2995 --jdk ... --output ...`。

## 单一版本配置

`source/<版本>/inputs.json` 同时提供：

- 三个构建输入 JAR 的文件名和 SHA-256。
- 目标视图类的名称、摘要、角色和布局分组。
- AUTO 可见性的字节位置及预期指令。
- 三个下拉控件的固定索引、名称、原尺寸、目标尺寸和字节码位置。
- 日夜属性编号、属性读取次数和行对应的模式值。

构建时验证该清单，生成 Agent 的校验资源和 `GeometryRules`。字节码修改与已创建控件的
运行时修改读取同一份配置。增加版本需要对应清单和适配验证，不靠改目录名推断兼容。

## 构建流程

1. 校验所选版本的三个 JAR 和目标 class 摘要。
2. 从原 JAR 只提取目标 class，在内存／临时目录执行可见性、几何与 ASM 视图方法修改。
3. 编译一次菜单适配器与运行时 helper；生成几何规则并一起编译为 Java 8。
4. 编译 Agent，打包目标定义、原 class 摘要、固件摘要和 helper 摘要。
5. 用 Go 交叉编译 Linux ARM64 客户端，将两个临时 JAR 嵌入单文件补丁。
6. 输出 `daynight-patch` 与 `build.json`，删除构建临时目录。

不会生成完整候选 viewhandler JAR，不会重复编译菜单适配器，也不回写原固件。
Python 负责唯一构建编排，JDK 负责 class 编译／修改，Go 只负责车机可执行入口。

## 运行时加载和回退

客户端定位唯一的 HMI JVM，在目标 `/tmp` 释放内嵌文件，通过 HotSpot Attach 加载
`agentmain`。Agent 根据内嵌的版本配置校验原 JAR、原 class 与 class loader，捕获内存定义，
再在原 UI dispatcher 中一次性重定义目标类并刷新现存控件。

`PatchSession` 区分 ORIGINAL、INSTALLING、INSTALLED、RESTORING、RECOVERY_REQUIRED。
回退失败时保留原定义供 remove 重试，拒绝重复 install 伪装成功。未开始的 UI 请求可取消；
已开始的修改等待结果后再返回。

卸载恢复原类、控件几何和监听器，用户当前模式保持不变。helper 类与原生 Attach 库可能保留
到 JVM 退出。默认报告和临时 JAR 在读取结果后清理，锁文件留在 `/tmp` 供串行操作复用。

## 菜单状态

唯一菜单实现为 `attach/java/DayNightSelection.java`。保存模式通过原 ASL API 异步读取，
结果回到 UI 线程；绘制函数不等待同步持久化。明确点击后有限次读回，必要时最多补发一次
原厂设置请求，修复 AUTO 与实际主题相同时切不回手动的问题。

epoch、请求序号和点击标识用于拒绝过期结果；卸载或新请求后，旧回调不再设置模式。
AUTO 的实际主题继续由原厂日夜输入驱动，适配器没有天气、定位或时间策略。
