# 脱敏插件核心源码与技术路线

本目录交付当前 AgentDock 仓库中 magpie 内置 `redact` 的源码快照和中文实现分析。它是**可复用的 Go 核心 + 宿主接入参考**，不是一个另行开发的独立脱敏服务，也不是 AgentDock 的完整安装源码。

## 阅读与文件

| 文件 | 用途 |
| --- | --- |
| `TECHNICAL-ROUTE.zh-CN.md` | 核心算法、处理流程、源码定位、模型方案、风险及复用路线 |
| `redact-source-and-roadmap.zip` | 按 `engine/...` 原始路径保存的源码、必要本地依赖、测试/夹具、插件客户端资源及本文档 |
| `SOURCE-MANIFEST.sha256` | 每个原始源码/资源文件的 SHA-256，不包含 ZIP 自身 |
| `SOURCE-INDEX.tsv` | 文件分类、字节数和来源路径，区分核心、依赖、集成参考 |
| `verification.txt` | 实际运行命令、输出、测试范围及归档完整性检查 |
| `ARCHIVE.sha256` | ZIP 的 SHA-256；在 ZIP 外提供，避免自引用 |

分析日期：2026-10-28。源码来自打包时的工作树字节，不声称与某个上游版本或 Git commit 完全相同。`SOURCE-MANIFEST.sha256` 是精确快照身份依据。没有修改业务源码，也没有对快照内容做替换、精简函数或格式化。

## 打包边界

- 核心目标：`engine/internal/redact/`、`engine/internal/masking/`、`engine/internal/gwmw/`、`engine/plugins/redact/`，包含原有测试和 testdata。
- 本地依赖：通过四目标的 `go list -tags nogui -deps -test` 解析；包含设置、插件运行时、可选本地模型推理代码及其依赖，共 21 个本地包目录。第三方依赖不 vendor，通过原始 go.mod/go.sum 获取。
- 客户端：`engine/internal/gui/assets/plugins/redact/` 中四个资源文件。
- 集成参考：网关脱敏/观测及相关用例、GUI 隐私 API/模型设置/Web 保护、provider 跳过策略、maskrec 记录实现、既有架构文档。
- 原生模型参考：`engine/native/privacy-filter/` 的 C++ helper、构建脚本、CMake、交叉编译配置和补丁。上游 privacy-filter.cpp/ggml 源码需由原构建脚本按固定版本获取，不包含权重或下载后的二进制。
- 原始源码/资源 350 个文件，合计 2,455,501 字节；详细分类以 SOURCE-INDEX.tsv 为准。

**仅验证四个核心目标包可在解包后运行其现有测试。** `reference` 文件是阅读材料，不是完整的 gateway/gui/provider 包；不要在精简包上用 `go test ./...` 或 `go build .` 判断整个宿主可用。依赖目录附带的其他原有测试也不在四目标验收承诺内。需要完整 AgentDock 应回到原仓库。

排除本机 `.env`、运行态 settings.json、redact.key、真实请求/账号数据、模型权重、依赖缓存、Git/Trellis/代理配置和编译产物；仓库原有测试夹具属于源码，不是本机运行数据。

## 解包、校验、复测

需要 Python 3、Go **1.26.8**（依据 engine/go.mod）以及已缓存的第三方依赖或可用的 Go 模块网络。

在本目录执行，选择不存在的新目录，避免覆盖已有文件：

```sh
sha256sum -c ARCHIVE.sha256
python3 -m zipfile -t redact-source-and-roadmap.zip
python3 -m zipfile -e redact-source-and-roadmap.zip /tmp/redact-source-review
cd /tmp/redact-source-review
sha256sum -c SOURCE-MANIFEST.sha256
cd engine
env -u MAGPIE_EVAL_CACHE go test -count=1 -tags nogui \
  ./internal/redact ./internal/masking ./internal/gwmw ./plugins/redact
```

不要设置 `MAGPIE_EVAL_CACHE`：现有模型评估测试会在设置该变量时调用真实模型并重写回放文件；本次验证使用提交在仓库里的回放，不下载或实跑模型。

所有测试与 smoke 实际结果见 verification.txt。本次没有修改 UI，没有启动桌面窗口；未验证 Windows/macOS 构建，也未测模型真实精度或生产吞吐量。

## 许可和复用

原引擎为 MIT，保留 `engine/LICENSE` 与 `LICENSES/magpie-MIT.txt`，Copyright (c) 2026 yetone。第三方 Go 模块和外部模型各受其自身许可证约束；本归档没有重新授权它们。源码注释将 Privacy Filter 与 AstrLink Guard 标为 Apache-2.0，实际分发权重或外部原生依赖前应以固定版本的上游许可证为准。

核心处于 Go `internal` 路径，不能从任意外部模块直接 import。直接复现实验可在解包后的同一 `engine` 模块内创建临时命令；移植到新工程应调整包边界与模块路径，并保留相应许可证。具体接入顺序和不能丢掉的协议行为见技术路线第 11 节。
