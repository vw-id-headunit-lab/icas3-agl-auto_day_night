# 版本化构建输入 / Versioned build inputs

每个固件版本使用独立子目录，目前为 `2995/`：

```text
source/
  2995/
    inputs.json
    viewhandler.jar
    MIBHMI-STD-HCN1.1.134-bin.jar
    osgi.jar
```

`inputs.json` 是该版本的唯一配置：记录文件名、文件／class SHA-256、视图角色、日夜属性、字节码修改点及控件索引与尺寸。
构建器校验输入后直接生成目标 class 定义；运行时校验资源和几何规则也从这份清单生成。

```sh
python3 attach/build.py --version 2995 --jdk /path/to/jdk17 --output /tmp/daynight-2995
```

支持其它固件时，在 `source/<版本>/` 添加对应输入和清单，完成该版本的字节码／运行时适配与验证后，
再把版本加入 `.github/workflows/build.yml` 的矩阵。仅添加文件或改文件名不代表已兼容。

这些 JAR 是固件输入，权利归原权利人；项目 MIT 许可证仅覆盖自编源码。

Each firmware version has its own directory, currently `2995/`. `inputs.json` records the version,
filenames, file/class digests, view roles, bytecode edits and live widget geometry. It is the single
configuration used to validate inputs, patch selected definitions and generate runtime checks/rules.

To support another version, add `source/<version>/`, implement and verify its bytecode/runtime adaptation,
then add it to the build workflow matrix. Adding files or renaming them alone does not establish compatibility.
The firmware JARs retain their original rights; this project's MIT license covers authored code only.
