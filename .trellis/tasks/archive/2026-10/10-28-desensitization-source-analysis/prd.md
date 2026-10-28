# 脱敏插件源码分析与技术路线打包

## Goal

分析当前仓库脱敏插件的真实实现，交付可审阅、可校验并支持核心复用的源码与中文技术路线，避免只提取插件入口或把参考源码误当独立产品。

## Background

- 用户已同意创建任务并明确批准最终规划，执行与验证均已完成。
- 当前仓库包含 AgentDock 的 magpie 引擎。内置插件位于 `engine/plugins/redact/`，算法主体位于 `engine/internal/redact/`，配置适配位于 `engine/internal/masking/`。
- 现有架构资料为 `engine/docs/subsystems/plugin-runtime.md:217-228`、`engine/docs/subsystems/gateway-routing.md:119-126`。

## Requirements

- R1：以当前仓库内置隐私脱敏插件为对象，追踪注册、配置、请求脱敏、响应还原与网关失败策略。
- R2：基于实际源码分析识别规则、自定义规则、占位符与内存映射、结构化 JSON、流式还原及可选本地模型。
- R3：输出中文技术路线，标明文件与行号、适用边界、风险及复用所需依赖；区分既有实现和建议。
- R4：在 `deliverables/redact-analysis/` 提供源码与技术路线 ZIP、阅读入口、逐文件 SHA-256 清单及验证记录；源码保留原始路径、必要测试/测试数据、依赖声明和许可证。
- R5：只新增交付和任务资料，不修改插件及宿主业务逻辑，不打包本机配置、凭据、运行数据、依赖缓存或构建产物。

## Acceptance Criteria

- [x] A1（R1、R2）：核心链路结论已写入技术路线并附当前源码路径/行号；明确区分插件注册、核心算法及宿主。
- [x] A2（R3）：中文文档已包含架构、处理流程、算法边界、复用路线；实际运行输出见 verification.txt。
- [x] A3（R4、R5）：ZIP 355 成员，其中 350 个原始来源文件共 2,455,501 bytes；CRC、唯一成员、安全路径、来源逐字节、逐项 SHA-256、解包一致性及包外校验通过；未包含运行态凭据或配置。
- [x] A4（R2、R4）：原仓库与精简解包四目标现有测试均通过；原仓库五个网关边界用例通过；go run 已确认文本/JSON/嵌套参数/SSE 往返及 warn，未下载模型。

## Out of Scope

- 不开发新的脱敏算法、独立插件产品、安装包、界面或模型下载器。
- 不将凭据保护等同于绝对不泄露承诺；白名单、提醒动作、跳过脱敏等边界须如实分析。
- 不在本任务提交 Git commit。

## Technical Notes

- 当前工具链与 `engine/go.mod:3` 一致，均为 Go 1.26.8；含测试依赖闭包为 21 个本地包目录，四目标测试及实际 smoke、ZIP 解包/哈希校验已通过。
- 重要实现边界已有当前源码依据：标签与内存映射 `engine/internal/redact/redact.go:490-584`；重复文本缓存 `engine/internal/redact/known.go:215-277`；JSON/签名 `engine/internal/redact/json.go:368-448`；流式写入 `engine/internal/redact/writer.go:9-127`；失败与动作 `engine/internal/gateway/redact.go:30-66,148-193`。
- 最终技术路线已说明 40-bit 标签的潜在碰撞、map 淘汰不保证先后、保形当前最小长度 8、原值查询 API 与明确跳过路径；仅分析现状，没有修复这些行为。
- 交付入口：`deliverables/redact-analysis/README.zh-CN.md`；归档：`redact-source-and-roadmap.zip`，942,113 bytes；SHA-256：`b1ce4b6fcdad1096dcf914394b008e60a990451204aafc36ef2690eefc4c81c2`。未运行真实模型、桌面 UI、完整仓库全部测试及 Windows/macOS 构建。
