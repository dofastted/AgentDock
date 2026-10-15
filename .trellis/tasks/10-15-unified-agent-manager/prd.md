# AgentDock — Unified Windows/WSL agent manager

## Goal

一个 Windows 桌面应用统一管理 Windows 宿主和所有 WSL 发行版上的 AI CLI / agent：skills、MCP、instructions、hooks、subagents、provider/模型，以及模型调度。不重写，直接组装两个已有项目：**magpie**（`D:/magpie`，引擎 + 自带 UI + 模型调度）和 **cc-switch**（`X:/cc-switch`，Tauri 桌面外壳）。核心价值：在一个地方查看、开关、同步和修复 Windows 与各 WSL 环境中的 agent。

## Background (repository evidence)

- **magpie**（MIT，Go 1.26，`github.com/yetone/magpie` 分支 `main`）：本地 LLM 网关 `127.0.0.1:3425`，支持 OpenAI Chat/Responses、Anthropic、Gemini 等协议的互转；路由策略 smart/order/rotate/usage/pace/weight/manual，以及 failover、affinity（`internal/routes/strategies.go:216-223`，`internal/gateway/fallback.go:38-63`）；library 统一管理 skills/MCP/instructions/hooks/subagents；agent wiring 负责改写 CLI 配置；33 种 agent 支持 WSL 孪生 `id@wsl:<distro>`（`internal/agent/wsl.go:288`）；自带完整 web UI，`-tags nogui` 构建仍可运行 `serve --web`（UI `:3430`）。数据是扁平 JSON。上游更新频繁，通常不接受社区 PR。
- **cc-switch**（MIT，Tauri 2 + React）：前端经约 280 个 invoke 命令紧耦合自己的 Rust 后端，不能直接接到 magpie 上；可复用的是 Tauri 外壳能力：托盘、单实例、window-state、自启动、`CREATE_NO_WINDOW`（锚点见 `research/cc-switch-shell-extraction.md`）。
- magpie 现有的 WSL 缺口：没有环境维度的总览（G1）；hooks/subagents 不能下发到 WSL（G2，`internal/library/hooks.go:133-136`，`subagents.go:93-96`）；网络问题只给文字建议，不会修改 `.wslconfig`（G3，`wsl.go:1416-1424`）；只有全局的 WSL 开关（G4，`settings.go:344-347`）；不能跨环境同步 agent 配置（G5）。
- 本机：Windows 10.0.26300；WSL 为 mirrored 网络；发行版有 Debian 和 docker-desktop。docker-desktop 被上游 `wslrun.Distros` 排除（`internal/wslrun/wslrun.go:413-426`），所以 AgentDock 可管理的只有 Debian，而本会话本身也运行在 Debian 中。Windows 侧有 cc-switch 数据 `C:\Users\Administrator\.cc-switch`；WSL 侧有 magpie 数据 `~/.config/magpie`（没在运行）；Windows 上已装好 cargo/node/pnpm/go。
- 完整调研：`research/*.md`；技术设计：`design.md`；执行计划：`implement.md`。

## Decisions

- **D1** magpie 作唯一引擎，负责 provider、路由、library、agent wiring 和 WSL 孪生；cc-switch 的 Rust 后端（SQLite、各 CLI 写入器、`:15721` 代理）不再使用。
- **D2** UI = Tauri 外壳 + magpie 自带 UI；不重接 cc-switch 的 React 页面。
- **D3** MVP 的 WSL 范围包括 G1+G4、G5、G3、G2；G6（在 WSL 里安装/升级 CLI）不做。
- **D4** magpie 用 `git subtree` 引入 `engine/` 并跟随上游；改动以新增文件为主，对上游文件的改动记录在 `engine/AGENTDOCK_PATCHES.md`。
- **D5** 独立数据目录 `%APPDATA%\AgentDock\magpie`；首次启动导入 cc-switch 和 WSL magpie 数据；AgentDock 是唯一的 CLI 配置写入方；检测到其他 magpie 进程或 :3425 被占用时只提示，不自动结束。
- **D6** G3 只做切换到 mirrored：备份并改写当前用户的 `.wslconfig`，确认后执行 `wsl --shutdown`；不需要管理员，不改防火墙，不配置 portproxy。
- **D7（待审批）** 品牌与发布：产品名 AgentDock，identifier `com.agentdock.desktop`；只出 Windows x64 NSIS 安装包；MVP 不做自动更新和 deep link；magpie UI 只做最小改名（标题、logo、favicon、登录页）。
- **D8（待审批）** G5 语义：选定一个源 agent 和若干同类目标孪生，复制字段值（model、effort 等）、连接状态和 library 绑定；必须先预览差异，再覆盖目标；不复制凭据和会话；目标发行版处于休眠时，跳过该目标并说明原因。
- **D9（待审批）** G2 命令转换：hook 中的 Windows 盘符路径转换为 `/mnt/<盘>/...`；指向 Windows 可执行文件（`.exe`/`.cmd`/`.ps1`）的 hook 不下发到 WSL，并显示原因；subagent 文件原样复制。

## Requirements

- **R1 外壳**：从 cc-switch 精简出 Tauri 2 应用，包括托盘（显示/隐藏/退出/开机自启）、单实例和窗口状态。启动时拉起内置引擎，主窗口加载引擎 UI；关闭窗口最小化到托盘；退出时引擎进程一并结束（Job Object）。
- **R2 引擎**：magpie nogui 构建作为 sidecar（`serve --web`，`XDG_CONFIG_HOME=%APPDATA%\AgentDock`，固定的 `MAGPIE_WEB_KEY`），保留 magpie 现有的全部页面与模型调度能力。
- **R3 G1 环境总览**：新增「环境」页，列出 Windows 和每个 WSL 发行版的运行状态（running/asleep）、网络模式和 agent 数量，并分开显示两种信号：Windows 行显示「主机监听」（Windows 能否连上网关）；每个运行中的发行版显示「WSL 可达性」，必须**从该发行版内部**探测（`wsl.exe` 有超时上限）。Windows 侧连得上不能当作 WSL 可达，因为防火墙仍可能挡住 WSL（`internal/agent/reach.go:243-247`）。列表和探测都不会启动已停止的发行版；已停止的发行版显示「未探测」和上次结果。
- **R4 G4 按发行版开关**：可以对单个发行版停用或启用 WSL agent 检测和写入；全局开关 `NoWSLAgents` 仍优先。
- **R5 G5 跨环境同步**：按 D8 实现预览和应用。
- **R6 G2 WSL hooks/subagents**：library 中的 hooks 和 subagents 可以绑定到 WSL agent 并写入发行版，规则见 D9。
- **R7 G3 一键切换 mirrored**：按 D6 实现；Windows build < 22621 时禁用并说明原因；支持用最新备份回滚。
- **R8 首次导入**：引擎数据为空时，环境页显示导入引导：cc-switch（使用 magpie 现有导入功能）和运行中发行版里的 magpie `providers.json`（按 id 合并，已有 id 优先，冲突的 id 加 `-wsl` 后缀，从不覆盖已有 key，只读源文件）。
- **R9 双引擎提示**：检测到其他 magpie 进程（Windows 或运行中的发行版）或 :3425 已被占用时，显示提示。
- **R10 品牌**：按 D7；附带 magpie 和 cc-switch 的 MIT 许可声明。

## Acceptance Criteria

- [x] AC1（R1,R2）在本机安装 NSIS 包后启动，几秒内窗口显示 AgentDock 品牌的引擎 UI；托盘退出后 `tasklist` 中没有 `agentdock-engine.exe`；第二次启动只会唤起已有窗口。*已验证：静默安装到 `%LOCALAPPDATA%\AgentDock`；引擎在可用端口就绪；第二次启动后 AgentDock 进程仍只有 1 个；结束外壳后引擎进程数为 0（Job Object）。*
- [x] AC2（R2）引擎数据只写在 `%APPDATA%\AgentDock\magpie`；`~/.config/magpie`（Windows 和 WSL 两侧）都没有被修改。*已验证：`C:\Users\Administrator\.config\magpie` 不存在。*
- [x] AC3（R3）……*已验证：在本机，windows 行主机监听为 up，Debian 的可达性为 ok（via curl，在 Debian 内部探测），页面首次加载返回 not-probed，并在后台发起探测；ok/blocked/unknown/asleep 由 `internal/environments/reach_test.go` 覆盖，其中真实 sh 脚本对 httptest 服务器和已关闭端口均做了测试。*
- [x] AC4（R4）……*已验证：在真实引擎上停用 Debian 后，`/api/state` 中 Debian 孪生为 0，重新启用后恢复为 7；`TestWSLDistroOff`、`TestFindSkipsDistroOff` 覆盖 sessions、wsl.exe 不再被调用，以及 NoWSLAgents 优先。*
- [x] AC5（R5）……*已验证：先预览（connect + model），应用后 Debian `~/.claude/settings.json` 指向 `127.0.0.1:13425`，在 Debian 发起的真实请求出现在引擎 `usage.jsonl` 中（rid 与上游一致；上游账号返回 quota 错误，但路由与记账成立）；随后断开连接，两侧配置已按备份字节级恢复。*
- [x] AC6（R6）……*已验证：在真实 Debian 中，hook 写入为 `/mnt/c/Tools/notify.sh done`；`toast.exe` 被拒绝并返回原因；subagent 文件已写入；删除后文件消失，settings.json 恢复。*
- [x] AC7（R7）……*已验证：`wslconfig_test.go`（6 种布局/编码、无需改动、回滚、旧 Windows）；UI 中关机前列出 `Debian, docker-desktop`，并需要二次确认（已在浏览器中点到确认框后取消，本机未执行关机）。*
- [x] AC8（R8）……*已验证：WSL magpie 导入已在真实引擎上导入 3 个 provider；`import_test.go` 覆盖 id 冲突时改名为 `-wsl`、重复导入不新增、本地文件不可读时不覆盖。cc-switch：`/api/importapps` 在真实数据上发现 `C:\Users\Administrator\.cc-switch\cc-switch.db`，含 26 个 provider；是否实际导入由用户在 UI 中决定。*
- [x] AC9（R9）……*已验证：本机 Docker 中的 magpie 容器占用 :3425 并对未带 key 的请求返回 401；已安装的 AgentDock 返回 `heldElsewhere=true`，页面给出提示，且没有结束该容器；有回归测试 `TestEnvironmentGatewayHeldElsewhere`。*
- [x] AC10 ……*已验证：Windows 全量 `go test -tags nogui ./...` 与上游原版对比，没有新增失败包或测试；唯一差异 `TestFXStateDoesNotWait` 在第一次基线中就失败过，单独运行 5/5 通过（计时受负载影响）；上游原本失败的包（cli、gateway）保持不变。`cargo clippy -D warnings`、`cargo fmt --check` 通过；`cargo test` 在 lib 测试 harness 上因缺少 Common Controls manifest 失败（STATUS_ENTRYPOINT_NOT_FOUND），属已知 Tauri Windows 测试限制。*

## Out of Scope

- G6：在 WSL 内安装或升级 CLI。
- 重写 cc-switch 的 React 页面，或把 cc-switch 后端功能（SQLite、`:15721` 代理、OpenClaw/Hermes 专属页面、Profiles）迁移到 AgentDock。
- NAT 场景的防火墙、portproxy 和 UAC 提权。
- macOS / Linux 桌面包、自动更新、deep link、代码签名。
- 把改动回馈给 magpie 上游。
