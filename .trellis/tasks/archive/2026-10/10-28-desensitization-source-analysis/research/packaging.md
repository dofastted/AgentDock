# 源码打包边界调查

## 已确认事实

- 入口是 `engine/plugins/redact/redact.go:19-32`：注册内置 `redact`，依赖 `settings` 与 `gateway`，将 `masking.Masker{}` 交给 `gwmw.Mask`；客户端静态资源为 `i18n.js`、`redact.css`、`client.js`、`page.js`。
- 算法主体 `engine/internal/redact/redact.go:19-35` 的生产依赖为标准库与 `golang.org/x/net/publicsuffix`，并非整个宿主。
- 可选模型通过 `engine/internal/redact/model.go:16-25` 的 `Found` 与 `Model` 回调注入；核心文件不直接导入 `internal/pii`。
- 现有外部测试确实导入本地推理实现（`engine/internal/redact/eval_test.go:3-18`），并使用两份回放数据（同文件 `:68-76`）。只复制算法生产源码不足以运行原有完整测试。
- `engine/internal/redact/eval_test.go:40-65` 指定 `MAGPIE_EVAL_CACHE` 时会请求真实模型并重新写入回放；验证必须确保该变量未设置，避免无意下载/重录。
- Go 版本必须按 `engine/go.mod:3` 的 `1.26.8`，不能照根目录 README 的旧版本描述。
- `engine/LICENSE:1-21` 为 MIT，Copyright (c) 2026 yetone，归档应保留。

## 已执行环境与依赖检查

2026-10-28，于 `engine/` 执行：

- `go version` → `go version go1.26.8 linux/amd64`。
- `go list -tags nogui -deps -test -f '{{if not .Standard}}{{.ImportPath}}{{end}}' ./internal/redact ./internal/masking ./internal/gwmw ./plugins/redact` → 成功；这只是依赖解析，不代表测试通过。

本地依赖闭包（除四个目标本身，忽略测试合成包）包括：

`agentenv`、`appdir`、`pii/guard`、`proc`、`steady`、`filememo`、`catalog`、`edit`、`fonts`、`lastgood`、`settings`、`source`、`pii`、`beat`、`filelock`、`pluginrt`、`testenv`。

第三方包来自：`golang.org/x/net`、`golang.org/x/sys`、`github.com/clipperhouse/stringish`、`github.com/clipperhouse/uax29/v2`、`github.com/fsnotify/fsnotify`、`github.com/pelletier/go-toml/v2`、`github.com/tidwall/gjson`、`github.com/tidwall/jsonc`、`github.com/tidwall/match`、`github.com/tidwall/pretty`、`gopkg.in/yaml.v3`。应保留原始 go.mod/go.sum，不另起第二种模块约定。

## 打包设计依据

- 使用显式路径清单，不递归打包整个工作树；保留原始 `engine/...` 路径和原文件字节。
- 提供可运行四个目标包测试所需的本地生产依赖、现有测试和 fixtures/embed/汇编等资源；不随意删掉测试来声称可独立编译。
- 网关、GUI 与 provider 相关源码作为集成参考，清楚说明不构成完整宿主应用，不承诺这些参考包在精简归档中可单独构建。
- 包含 `engine/internal/gui/assets/plugins/redact/` 四个实际插件客户端资源；不以 Go 注册文件替代完整插件源文件。
- 不含 `.env`、settings.json、redact.key、请求日志、模型权重、依赖缓存、二进制和构建输出；测试数据按仓库 fixtures 原样纳入，不混入本机样本。
- 导出逐文件 SHA-256 清单、来源路径、源文件/参考文件分层说明。验证 ZIP 安全路径、成员集合、长度与 SHA-256，并在临时解包目录复核四目标包测试。
- 技术路线明确区分现状、可复用边界和后续移植建议；不将源码归档包装成已完成的独立脱敏服务。

## 尚未执行

尚未运行核心行为、包测试或归档检查；这些属于用户批准最终规划后的执行阶段。
