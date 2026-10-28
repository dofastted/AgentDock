# 插件宿主接入研究（规划依据）

## 注册与接口

- `engine/plugins/redact/redact.go:19-32` 注册 ID=redact、Requires=settings/gateway、Essential=true、nav.page=masking 与四个客户端资源；Apply 用 `gwmw.Mask(masking.Masker{})`，dispose 时撤销。
- 同文件 `:35-46` 通过 settings.LoadChecked + masking.Wanted 判断在用；读取错误返回运行时，而非假装关闭。
- `engine/internal/gwmw/gwmw.go:76-132` 定义 Masker（JSON/Text/Restore/Writer）、Teller（hits/value）及 ContextTeller（取消等待），不能将与之不同的 JS Agent middleware fail-open 机制套在脱敏上。
- `gwmw.go:172-210,234-276` 后注册的 masker 服务；没有 masker 时最多每秒请求一次 refresh，wanted=true 仍无实现则 ErrUnmasked。`:279-288` 将 panic 转为错误。

## 配置与模型

- `engine/internal/masking/masking.go:21-37` Wanted 是 Secrets/Personal/Words，Options 按每次调用读取设置，传递 Kinds/WordsAnyCase/Rules/Actions/Allow/Shape；RedactModel 单独配置不是 Wanted 的充分条件。
- 同文件 `:42-53` 模型 Find 失败 panic；`:92-121` 支持请求 context 与 hits；`:124-154` 连接 Value、Preview、Restore 和 Writer/Finish。
- Preview 关闭脱敏时临时启用 Secrets/Personal，标记 off；不记忆本次原值（`:134-146`），不等于绝无其他初始化副作用。

## 网关执行与安全边界

- `engine/internal/gateway/redact.go:30-66` 获取 masker，通过 Safely 调 JSONHitsContext/JSONHits/JSON，检测 blocked，有实际替换时挂回复 Writer；失败不返回已改写请求。
- 同文件 `:112-145` 文本路径相同机制；`:148-157` warn 不计替换数量；`:160-193` block 返回 400，masker 无法工作返回 503；`:68-80` Finish 后统计还原位置。
- `redact.go:228-240` 搜索 provider 明确 SkipsRedaction 时原样发送；`:243-270` 主路由及 fallback 或组成员全部允许 unmasked 才可走不脱敏路由，不应描述成绝对所有请求必遮。
- Block 是核心产出 Hit 后网关做的传输决策；直接调用 core Mask 并不会阻断 HTTP。

## 记录和原值查询

- `engine/internal/maskrec/maskrec.go:1-13,33-47,49-87` 日文件记录元信息和 hint，30 天/总计 100MiB；snips、back 和最近 100 请求 sent body 在内存，单份 4MiB、总预算 64MiB。
- `engine/internal/gui/masking.go:59-78,139-180` 暴露 preview/summary/records/value/values/sent/clear；value(s) 会返回仍在内存的真实值并设置 Cache-Control=no-store。该 API 自身不是权限中间件，最终技术路线必须结合宿主路由保护解释，不夸大成任何本地请求都安全。

## 后续验收用例

四目标现有测试覆盖 plugin/masking/gwmw/core。在完整原仓库另运行以下已有 gateway 用例：

- `TestRedactedRequest`（`gateway/redact_test.go:125`）：本地 upstream 观测请求并验证还原。
- `TestBlockedKindHoldsTheRequestBack`（`gateway/mask_actions_test.go:19`）：上游请求计数应不变，返回拦截错误。
- `TestWarnedKindGoesAsWritten`（同文件 `:60`）：warn 原样送出，其余值仍遮。
- `TestSignedThinkingGoesBackAsSigned`（`gateway/redact_roundtrip_test.go:34`）：签名兼容。
- `TestToolCallIDsGoBackAsGiven`（同文件 `:119`）：协议 ID 不破坏。
- `TestMaskingFailsClosed` / `TestSafelyTurnsAPanicIntoAnError`（`gwmw/gwmw_test.go:41,104`）。

这些用例仅定位，尚未执行，不宣称通过。

## 归档建议

完整 core、masking、gwmw、plugins/redact 与四个客户端资源，四目标包必要本地依赖闭包和现有目标测试；额外将 maskrec（完整源码目录）、gateway/redact.go、gateway/masked.go、相关现有 gateway 测试、gui/masking.go、gui/privacymodel.go、GUI 隐私 API 设置处理所在 api.go、provider/unredacted.go 与架构文档作为 reference。

不要为了把 gateway/GUI 参考包做成可编译而盲目带入整个宿主或删测试；README 明示只对四目标解包测试承诺。GUI 保护、pii 具体模型与下载许可等细节在执行阶段补查当前源码并纳入正式技术路线。
