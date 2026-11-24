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

`inputs.json` 记录版本、文件名和 SHA-256。构建器在编译前核对三个文件的摘要，拒绝不匹配的输入。

```sh
python3 attach/build.py --version 2995 --jdk /path/to/jdk17 --output /tmp/daynight-2995
```

支持其它固件时，在 `source/<版本>/` 添加对应输入和清单，完成该版本的字节码／运行时适配与验证后，
再把版本加入 `.github/workflows/build.yml` 的矩阵。仅添加文件或改文件名不代表已兼容。

这些 JAR 是固件输入，权利归原权利人；项目 MIT 许可证仅覆盖自编源码。

Each firmware version has its own directory, currently `2995/`. `inputs.json` records the version,
filenames and SHA-256 digests; all three inputs are verified before compilation.

To support another version, add `source/<version>/`, implement and verify its bytecode/runtime adaptation,
then add it to the build workflow matrix. Adding files or renaming them alone does not establish compatibility.
The firmware JARs retain their original rights; this project's MIT license covers authored code only.
