# 执行计划

## 启动前

1. 整合 `research/core.md`、`research/integration.md` 和 `research/packaging.md`，以当前源码确认交付边界。
2. 将最终需求收敛写回 PRD，保留需求与验收映射。
3. 为 implement.jsonl/check.jsonl 加入真实 spec/research 上下文并验证任务；向用户呈现最终规划。
4. 仅在用户随后明确批准最终规划时执行 `task.py start`；批准创建任务不等于批准执行。

## 执行顺序

1. 读取 `trellis-before-dev`，复核任务约束及源码快照；不修改产品逻辑。
2. 按依赖解析结果生成明确源码/资源允许清单，保留测试所需 testdata、embed 和平台源码；核对 license。
3. 完成中文技术路线和阅读入口，关键结论附源码行号；注明精简包仅保证指定核心目标可构建，网关/GUI 参考文件不是完整宿主。
4. 在隔离的临时目录运行真实 core API 示例（文本、结构化 JSON、SSE 跨事件还原），不使用实际凭据，不把示例脚本留在业务代码。
5. 执行现有目标包测试与必要网关边界用例，记录完整真实结果。
6. 生成 ZIP 和逐文件 SHA-256 清单，在临时目录解包，对照原始来源与清单验证成员集合/路径/大小/哈希；在解包后的 engine 目录运行目标包测试，发现闭包缺失时补齐真实依赖而非删测试。
7. 将验证结果写入 `verification.txt` 并重新打包，复核文档、清单、归档的一致性；清除本任务临时 smoke 文件和解包目录。
8. 交付最终目录/ZIP 路径、核心结论及实际验证，不做 Git commit。

## 验证命令

在原仓库 `engine/`：

```sh
go version
go list -tags nogui -deps -test ./internal/redact ./internal/masking ./internal/gwmw ./plugins/redact
env -u MAGPIE_EVAL_CACHE go test -tags nogui ./internal/redact ./internal/masking ./internal/gwmw ./plugins/redact
env -u MAGPIE_EVAL_CACHE go test -tags nogui ./internal/gateway -run '^(TestRedactedRequest|TestBlockedKindHoldsTheRequestBack|TestWarnedKindGoesAsWritten|TestSignedThinkingGoesBackAsSigned|TestToolCallIDsGoBackAsGiven)$'
```

在实际完成合成 smoke 实现后运行 `go run`，必须断言恢复后与原始输入相等、JSON 有效、流式输出拼接正确；输出应能让读者观察具体占位符与还原结果。使用临时 HOME/配置目录，避免写入现有用户状态。不要在生产目录创建永久测试来检查打包连线或文档文本。

解包后重复四目标测试；ZIP 校验覆盖 CRC、路径安全、成员集合、来源逐文件 SHA-256、许可证和显式排除项。网关用例已依据 integration.md 核对名称；它们在原仓库运行，精简包的网关/GUI 参考源码不宣称能够独立编译。

## 风险与回退

- 外部模型或网络受限：默认使用已提交的模型回放，不下载权重；保留真实运行限制。
- 精简依赖容易漏掉 embed、汇编和跨平台文件：依赖包目录应按资源边界收录，解包测试验证 Linux 可用；未运行 Windows/macOS 不宣称通过。
- 包含 GUI/API 参考源码会增加边界歧义：清单区分 core/dependency/reference/assets/license，README 明示。
- 只允许新增 `deliverables/redact-analysis/` 与本任务资料；不覆盖已有用户交付。若目录已存在，检查并选新目录，不擅自覆盖。
