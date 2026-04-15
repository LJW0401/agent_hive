# Agent Hive 功能增强 开发方案

> 创建日期：2026-04-16
> 状态：已审核
> 版本：v1.1
> 关联需求文档：project_plan/requirements.md

## 1. 技术概述

### 1.1 技术架构

```
┌─────────────────────────────────────────────────────┐
│  CLI 入口 (os.Args 子命令分发)                        │
│  init | install | uninstall | start | stop |         │
│  restart | status | logs | run                       │
├─────────────────────────────────────────────────────┤
│  Config 层 (config.yaml)                             │
│  port / data_dir / token / machines / user / shell   │
├──────────────────────┬──────────────────────────────┤
│  HTTP Server         │  日志层                       │
│  REST API + WS       │  MultiWriter(stdout + file)  │
│  go:embed 前端       │  按天轮转 / 7天保留           │
├──────────────────────┴──────────────────────────────┤
│  Container Manager                                   │
│  PTY Session (用户身份切换: config > 文件归属 > 当前)  │
├─────────────────────────────────────────────────────┤
│  Store (SQLite)  │  Terminal Logs (磁盘文件)         │
└─────────────────────────────────────────────────────┘

前端:
┌─────────────────────────────────────────────────────┐
│  桌面端 App                                          │
│  ProjectContainer: [TodoList | 分割线 | Terminal]    │
│  分割线可拖拽, todo 可隐藏, 最大 2/3                  │
├─────────────────────────────────────────────────────┤
│  移动端 MobileApp (Swiper)                           │
│  MobileProjectView: [Terminal / 拖拽分隔 / TodoList] │
│  底部快捷键栏: ^C ^L Tab Esc ↑↓←→ 粘贴             │
└─────────────────────────────────────────────────────┘
```

### 1.2 技术栈

| 类别 | 选择 | 说明 |
|------|------|------|
| 后端语言 | Go 1.21+ | 现有技术栈 |
| 前端框架 | React + TypeScript + Vite | 现有技术栈 |
| 终端模拟 | xterm.js + FitAddon | 现有技术栈 |
| CSS | Tailwind CSS | 现有技术栈 |
| CLI 解析 | Go 标准库 (os.Args + flag) | 手动子命令分发，不引入第三方库 |
| 日志轮转 | 自实现 io.Writer | 按天轮转，不引入第三方库 |
| PTY 用户切换 | syscall.SysProcAttr.Credential | 进程级 setuid/setgid |
| 前端测试 | Vitest | Vite 生态，dev dependency |
| 后端测试 | go test | Go 标准库 |

### 1.3 项目结构变更

```
backend/
├── cmd/server/main.go          # 重构: 子命令分发入口
└── internal/
    ├── cli/                    # 新增: 子命令实现
    │   ├── init.go             # agent-hive init
    │   ├── install.go          # agent-hive install
    │   ├── uninstall.go        # agent-hive uninstall
    │   ├── service.go          # start/stop/restart/status/logs
    │   └── run.go              # agent-hive run (原 main 逻辑)
    ├── config/config.go        # 修改: 增加 user/shell 字段
    ├── logger/                 # 新增: 日志文件 + 轮转
    │   └── logger.go
    ├── pty/session.go          # 修改: 支持用户身份切换
    └── ...

frontend/src/
├── components/
│   ├── ProjectContainer.tsx    # 修改: 拖拽分割线替代固定 w-48
│   ├── TodoList.tsx            # 修改: 文本换行
│   ├── MobileProjectView.tsx   # 修改: 集成快捷键栏
│   ├── ShortcutBar.tsx         # 新增: 移动端快捷键栏
│   └── PasteModal.tsx          # 新增: 粘贴弹窗
└── ...
```

### 1.4 安全门控命令

- 后端编译检查：`cd backend && go build ./...`
- 后端测试：`cd backend && go test ./...`
- 前端类型检查：`cd frontend && npx tsc --noEmit`
- 前端测试：`cd frontend && npx vitest run`
- 完整构建：`make build`

## 2. 开发阶段

### 阶段 1：前端体验优化

**目标**：桌面端支持拖拽调整待办/终端比例，待办文本自动换行。

**涉及的需求项**：
- 2.1.1 拖拽分割线调整比例
- 2.2.1 待办项自动换行显示

#### 工作项列表

##### WI-1.1 [S] 待办文本换行
- **描述**：在 `TodoList.tsx` 中移除 `truncate` 类，改为 `break-words whitespace-normal` 实现自动换行。同时调整 item 布局为 `items-start` 使多行文本与复选框顶部对齐。
- **验收标准**：
  1. 超过列表宽度的待办文本自动换行，完整显示
  2. 复选框、拖拽手柄与文本首行顶部对齐
  3. 安全门控：`cd frontend && npx tsc --noEmit` 通过
- **Notes**：
  - Pattern：CSS `word-break: break-word` + flexbox `align-items: flex-start`
  - Reference：`frontend/src/components/TodoList.tsx:197` 的 `truncate` 类
  - Hook point：与 WI-1.3 分割线联动（宽度变化时文本重新换行）

##### WI-1.2 [M] 桌面端拖拽分割线
- **描述**：在 `ProjectContainer.tsx` 中将固定 `w-48` 的 TodoList 改为可变宽度。在 TodoList 和 Terminal 之间添加可拖拽的分割线（4px 宽，`cursor-col-resize`）。通过 `mousedown` + `mousemove` + `mouseup` 事件实现拖拽。使用比例（ratio）而非像素值控制宽度。约束：todo 最大 2/3，可拖到 0 完全隐藏。拖拽结束后触发终端 `fitAddon.fit()`。
- **验收标准**：
  1. 拖拽分割线可实时调整待办/终端宽度比例
  2. 待办列表可完全隐藏（ratio=0），终端占满
  3. 待办列表宽度不超过容器 2/3
  4. 拖拽结束后终端自动 refit
  5. 安全门控：`cd frontend && npx tsc --noEmit` 通过
- **Notes**：
  - Pattern：React state `splitRatio` + `onMouseDown/Move/Up`，参考 MobileProjectView 的上下分割实现
  - Reference：`frontend/src/components/ProjectContainer.tsx:103-116`，`frontend/src/components/MobileProjectView.tsx:59-79`
  - Hook point：Terminal 的 `ResizeObserver` 已存在，会自动触发 refit

##### WI-1.3 [S] 阶段 1 前端测试
- **描述**：配置 Vitest，编写组件测试：(1) TodoList 文本换行验证；(2) 分割线拖拽边界约束（最大 2/3、可隐藏到 0）。
- **验收标准**：
  1. Vitest 配置完成，`npx vitest run` 可执行
  2. 测试覆盖文本换行（无 truncate）和分割线边界约束
  3. 安全门控：`cd frontend && npx vitest run` 通过
- **Notes**：
  - Pattern：`@testing-library/react` + Vitest
  - Reference：`frontend/vite.config.ts` 添加 test 配置
  - Hook point：后续阶段复用 Vitest 配置

##### WI-1.4 [集成门控] 阶段 1 集成验证
- **描述**：验证 WI-1.1 ~ WI-1.3 的集成状态。
- **验收标准**：
  1. `cd frontend && npx tsc --noEmit` 通过
  2. `cd frontend && npx vitest run` 通过
  3. `make build` 构建成功
  4. 浏览器手动验证：拖拽分割线流畅，文本换行正确，隐藏/最大限制生效

**阶段验收标准**：
1. Given 桌面端打开项目容器, When 拖拽分割线向右, Then 待办列表增宽，终端缩窄并自动 refit
2. Given 拖拽分割线到最左, When 松开鼠标, Then 待办列表隐藏，终端占满
3. Given 待办列表宽度接近 2/3, When 继续拖拽, Then 不再增宽
4. Given 待办文本超长, When 页面渲染, Then 文本自动换行完整显示
5. 所有安全门控命令通过

**阶段状态**：已完成
**完成日期**：2026-04-16
**验收结果**：通过
**安全门控**：全部通过
**集成门控**：全部通过
**备注**：文本换行和拖拽分割线功能实现完成，Vitest 测试配置就绪

---

### 阶段 2：后端基础设施

**目标**：重构 CLI 为子命令架构，扩展 config 支持 user/shell，实现日志轮转和 PTY 用户身份切换，实现 init 命令。

**涉及的需求项**：
- 2.3.1 init 命令
- 2.3.7 PTY 以配置用户身份启动
- 2.3.8 run 命令
- 2.4.1 运行日志写入文件
- 2.4.2 日志按天轮转

#### 工作项列表

##### WI-2.1 [M] CLI 子命令框架 + run 命令
- **描述**：重构 `cmd/server/main.go`，将当前启动逻辑提取到 `internal/cli/run.go`。`main()` 改为 `os.Args[1]` 子命令分发。`run` 子命令保持原有 `--config` 和 `--dev` flag。无子命令时输出 usage 帮助信息。
- **验收标准**：
  1. `agent-hive run --config config.yaml` 等同于原来的启动方式
  2. `agent-hive`（无参数）输出子命令帮助列表
  3. 未知子命令输出错误提示和 usage
  4. 安全门控：`cd backend && go build ./...` 通过
- **Notes**：
  - Pattern：`os.Args` switch + 每个子命令独立 `flag.FlagSet`
  - Reference：`backend/cmd/server/main.go` 当前 main 逻辑
  - Hook point：后续 init/install 等命令复用同一分发框架

##### WI-2.2 [S] Config 扩展 — user/shell 字段
- **描述**：在 `config.Config` 结构体中增加 `User string` 和 `Shell string` 字段（yaml tag: `user`、`shell`）。加载 config 后，如果 user/shell 为空，通过 config 文件归属用户推断：`os.Stat(configPath)` → `Sys().(*syscall.Stat_t).Uid` → `user.LookupId` → 用户名和默认 shell。
- **验收标准**：
  1. config.yaml 中设置 user/shell 后能正确解析
  2. 未设置时通过文件 owner 正确推断用户和 shell
  3. 安全门控：`cd backend && go build ./...` 通过
- **Notes**：
  - Pattern：`os/user.LookupId` 获取用户信息，解析 `/etc/passwd` 获取 shell
  - Reference：`backend/internal/config/config.go`
  - Hook point：被 WI-2.5 PTY 用户切换和 WI-2.6 init 命令使用

##### WI-2.3 [M] 日志文件写入 + 按天轮转
- **描述**：新建 `internal/logger/logger.go`，实现 `RotatingWriter` 结构体（实现 `io.Writer`）。每次 Write 时检查日期是否变更，变更时重命名当前文件为 `agent-hive.YYYY-MM-DD.log`，创建新文件，清理 7 天前的旧文件。在 `run` 命令启动时创建 `io.MultiWriter(os.Stdout, rotatingWriter)` 设置为 `log` 包的输出。日志路径为二进制文件所在目录下的 `agent-hive.log`。
- **验收标准**：
  1. 程序启动后在二进制同目录生成 `agent-hive.log`
  2. 日志同时输出到 stdout 和文件
  3. 日志包含时间戳
  4. 重启后追加而非覆盖
  5. 安全门控：`cd backend && go build ./...` 通过
- **Notes**：
  - Pattern：`io.Writer` 接口包装，`sync.Mutex` 保护并发写入
  - Reference：无现有文件，新建 `backend/internal/logger/`
  - Hook point：`run` 命令初始化时调用

##### WI-2.4 [S] 测试 — 日志轮转 + Config 推断
- **描述**：编写 Go 测试：(1) 日志轮转：模拟日期变更触发文件轮转和过期清理；(2) Config 用户推断逻辑：config 显式配置 > 文件归属 > 当前用户的优先级。
- **验收标准**：
  1. `cd backend && go test ./internal/logger/... ./internal/config/...` 通过
  2. 覆盖日志轮转（日期变更、过期清理）和 Config 用户推断优先级
  3. 安全门控：`cd backend && go test ./...` 通过
- **Notes**：
  - Pattern：Go table-driven tests，临时目录模拟文件操作
  - Reference：`backend/internal/logger/`、`backend/internal/config/`
  - Hook point：CI 集成

##### WI-2.5 [M] PTY 用户身份切换
- **描述**：修改 `pty.NewSession()`，接收可选的 user/shell 参数。当 user 不为空且当前进程为 root 时，通过 `user.Lookup(username)` 获取 UID/GID，设置 `cmd.SysProcAttr.Credential`。设置 `cmd.Dir` 为用户 home 目录。重建 `cmd.Env`：HOME、USER、LOGNAME、SHELL、TERM、PATH。Container Manager 创建 PTY 时从 config 读取 user/shell 传入。
- **验收标准**：
  1. config 中 user=penguin, shell=/bin/zsh 时，PTY 以 penguin 用户运行 zsh
  2. 非 root 运行时忽略 user 配置，使用当前用户
  3. 终端中 `whoami` 输出配置的用户名
  4. 安全门控：`cd backend && go build ./...` 通过
- **Notes**：
  - Pattern：`syscall.SysProcAttr{Credential: &syscall.Credential{Uid, Gid}}`
  - Reference：`backend/internal/pty/session.go:20-38`，`backend/internal/container/manager.go`
  - Hook point：Manager.Create() 和 Manager.Reopen() 调用 NewSession 时传参

##### WI-2.6 [S] init 命令
- **描述**：实现 `internal/cli/init.go`。嗅探当前用户（`os/user.Current()`）和默认 shell（解析 `/etc/passwd`）。支持 `--user` 和 `--shell` 参数覆盖。校验用户存在性和 shell 路径存在性。生成 config.yaml（包含 port、data_dir、token、user、shell、machines）。文件已存在时提示覆盖确认。
- **验收标准**：
  1. `agent-hive init` 生成 config.yaml，user/shell 为当前用户信息
  2. `--user`/`--shell` 可覆盖默认值
  3. 不存在的用户或 shell 路径给出错误提示
  4. 文件已存在时提示确认
  5. 安全门控：`cd backend && go build ./...` 通过
- **Notes**：
  - Pattern：`flag.FlagSet` 解析子命令参数，`yaml.Marshal` 生成配置文件
  - Reference：`backend/internal/config/config.go` 的 Config 结构体
  - Hook point：生成的 config.yaml 被 run/install 命令使用

##### WI-2.7 [S] 测试 — PTY 用户推断 + init 校验
- **描述**：编写 Go 测试：(1) PTY 用户推断逻辑（非 root 时跳过 setuid）；(2) init 命令参数校验（无效用户/shell 报错）。
- **验收标准**：
  1. `cd backend && go test ./internal/pty/... ./internal/cli/...` 通过
  2. 覆盖 PTY 非 root 路径和 init 参数校验
  3. 安全门控：`cd backend && go test ./...` 通过
- **Notes**：
  - Pattern：Go table-driven tests
  - Reference：`backend/internal/pty/`、`backend/internal/cli/`
  - Hook point：CI 集成

##### WI-2.8 [集成门控] 阶段 2 完整集成验证
- **描述**：验证阶段 2 所有工作项的集成状态。
- **验收标准**：
  1. `cd backend && go build ./...` 通过
  2. `cd backend && go test ./...` 通过
  3. `make build` 构建成功
  4. `agent-hive init` 正确生成 config.yaml
  5. `agent-hive run --config config.yaml` 正常启动，日志写入文件，PTY 用户切换正确

**阶段验收标准**：
1. Given 运行 `agent-hive`（无参数）, When 命令执行, Then 输出子命令帮助列表
2. Given 运行 `agent-hive init`, When 当前目录无 config.yaml, Then 生成配置文件，user/shell 自动嗅探
3. Given config.yaml 配置 user=penguin shell=/bin/zsh, When 以 root 运行 `agent-hive run`, Then PTY 以 penguin 用户运行 zsh
4. Given 程序启动, When 有请求, Then 日志同时输出到 stdout 和 agent-hive.log
5. 所有安全门控命令通过

**阶段状态**：已完成
**完成日期**：2026-04-16
**验收结果**：通过
**安全门控**：全部通过
**集成门控**：全部通过
**备注**：CLI 子命令框架、Config user/shell、日志轮转、PTY 用户切换、init 命令全部完成

---

### 阶段 3：服务管理

**目标**：实现 systemd 服务安装/卸载/管理命令。

**涉及的需求项**：
- 2.3.2 install 命令
- 2.3.3 uninstall 命令
- 2.3.4 start/stop/restart 命令
- 2.3.5 status 命令
- 2.3.6 logs 命令

#### 工作项列表

##### WI-3.1 [M] install 命令
- **描述**：实现 `internal/cli/install.go`。硬编码 systemd service 文件模板，使用 `text/template` 填充二进制路径（`os.Executable()`）和配置文件路径（`--config` 参数或默认同目录 config.yaml）。写入 `/etc/systemd/system/agent-hive.service`。执行 `systemctl daemon-reload` 和 `systemctl enable agent-hive`。检查 root 权限（`os.Getuid() != 0` 报错）。已存在时提示覆盖确认。检查 config 文件存在性。
- **验收标准**：
  1. 以 root 运行 `agent-hive install` 生成 service 文件并 enable
  2. 非 root 运行给出权限错误提示
  3. 服务已存在时提示覆盖确认
  4. config 文件不存在时报错
  5. 安全门控：`cd backend && go build ./...` 通过
- **Notes**：
  - Pattern：`text/template` 渲染 service 文件，`exec.Command("systemctl", ...)` 执行命令
  - Reference：阶段 2 的 CLI 框架
  - Hook point：被 uninstall/start/stop 等命令依赖（需要已安装的服务）

##### WI-3.2 [S] uninstall 命令
- **描述**：实现 `internal/cli/uninstall.go`。执行 `systemctl stop`、`systemctl disable`、删除 service 文件、`daemon-reload`。检查 root 权限。检查 service 文件是否存在（未安装时提示）。
- **验收标准**：
  1. 以 root 运行 `agent-hive uninstall` 完成停止、禁用、删除、reload
  2. 服务未安装时提示
  3. 非 root 给出权限错误
  4. 安全门控：`cd backend && go build ./...` 通过
- **Notes**：
  - Pattern：顺序执行 systemctl 命令，忽略 stop 失败（可能已停止）
  - Reference：`internal/cli/install.go` 的 root 检查逻辑
  - Hook point：与 install 互为逆操作

##### WI-3.3 [S] 测试 — 模板渲染 + 权限检查
- **描述**：编写 Go 测试：(1) service 文件模板渲染（验证路径填充正确）；(2) root 权限检查逻辑；(3) service 文件存在性检查。不实际调用 systemctl。
- **验收标准**：
  1. `cd backend && go test ./internal/cli/...` 通过
  2. 覆盖模板渲染、权限检查、存在性检查
  3. 安全门控：`cd backend && go test ./...` 通过
- **Notes**：
  - Pattern：抽取 systemctl 调用为可注入接口，测试中使用 mock
  - Reference：`internal/cli/install.go`、`internal/cli/uninstall.go`
  - Hook point：CI 集成

##### WI-3.4 [集成门控] install/uninstall 集成验证
- **描述**：验证 WI-3.1 ~ WI-3.3 的集成状态。
- **验收标准**：
  1. `cd backend && go build ./...` 通过
  2. `cd backend && go test ./...` 通过
  3. `make build` 构建成功
  4. 手动验证：install 生成 service 文件 → uninstall 清理

##### WI-3.5 [M] start/stop/restart/status/logs 命令
- **描述**：实现 `internal/cli/service.go`。每个子命令封装对应的 systemctl/journalctl 调用。start/stop/restart 检查 root 权限。status 无需 root。logs 支持 `-f` 和 `-n` 参数，使用 `exec.Command` 并将 stdout/stderr 连接到当前终端（`cmd.Stdout = os.Stdout`）。
- **验收标准**：
  1. `agent-hive start/stop/restart` 正确调用 systemctl
  2. `agent-hive status` 显示服务状态
  3. `agent-hive logs` 输出日志，`-f` 可实时跟踪
  4. start/stop/restart 非 root 给出权限错误
  5. 安全门控：`cd backend && go build ./...` 通过
- **Notes**：
  - Pattern：`exec.Command` + `cmd.Stdout/Stderr = os.Stdout/Stderr` 透传输出
  - Reference：`internal/cli/install.go` 的 systemctl 调用模式
  - Hook point：logs 的 `-f` 模式需要 `cmd.Run()`（阻塞直到用户 Ctrl+C）

##### WI-3.6 [S] 测试 — service 命令
- **描述**：编写 Go 测试：(1) start/stop/restart 的 root 权限检查；(2) status 无需 root 验证；(3) logs 参数解析（-f、-n）。不实际调用 systemctl。
- **验收标准**：
  1. `cd backend && go test ./internal/cli/...` 通过
  2. 覆盖权限检查和参数解析
  3. 安全门控：`cd backend && go test ./...` 通过
- **Notes**：
  - Pattern：mock systemctl 调用接口
  - Reference：`internal/cli/service.go`
  - Hook point：CI 集成

##### WI-3.7 [集成门控] 阶段 3 完整集成验证
- **描述**：验证阶段 3 所有工作项的集成状态。
- **验收标准**：
  1. `cd backend && go build ./...` 通过
  2. `cd backend && go test ./...` 通过
  3. `make build` 构建成功
  4. 手动验证完整生命周期：init → install → start → status → restart → logs → stop → uninstall

**阶段验收标准**：
1. Given 以 root 运行 `agent-hive install`, When 命令执行, Then service 文件被创建并 enable
2. Given 服务已安装, When 运行 `agent-hive start`, Then 服务启动，status 显示 active
3. Given 服务运行中, When 运行 `agent-hive logs -f`, Then 实时输出日志
4. Given 服务已安装, When 运行 `agent-hive uninstall`, Then 服务停止、service 文件删除
5. Given 非 root 运行 install/start/stop, When 命令执行, Then 给出权限错误提示
6. 所有安全门控命令通过

**阶段状态**：已完成
**完成日期**：2026-04-16
**验收结果**：通过
**安全门控**：全部通过
**集成门控**：全部通过
**备注**：install/uninstall/start/stop/restart/status/logs 全部实现

---

### 阶段 4：移动端快捷键栏

**目标**：移动端添加快捷键栏和粘贴功能。

**涉及的需求项**：
- 2.5.1 快捷键按钮栏
- 2.5.2 粘贴按钮

#### 工作项列表

##### WI-4.1 [M] 快捷键按钮栏组件
- **描述**：新建 `frontend/src/components/ShortcutBar.tsx`。固定在终端底部，包含按钮：`^C`（`\x03`）、`^L`（`\x0c`）、`Tab`（`\t`）、`Esc`（`\x1b`）、`↑`（`\x1b[A`）、`↓`（`\x1b[B`）、`←`（`\x1b[D`）、`→`（`\x1b[C`）、粘贴按钮。栏容器 `overflow-x-auto` 实现水平滚动，`touch-action: pan-x` 阻止事件冒泡到 Swiper。接收 `onSend: (data: string) => void` 回调。在 `MobileProjectView.tsx` 中集成到终端区域底部。
- **验收标准**：
  1. 移动端终端底部显示快捷键栏
  2. 点击各按钮发送正确的控制字符
  3. 快捷键栏可水平滑动，不触发 Swiper 页面切换
  4. 安全门控：`cd frontend && npx tsc --noEmit` 通过
- **Notes**：
  - Pattern：按钮数组 `{label, value}` 映射，`onTouchStart` 中 `e.stopPropagation()` 隔离 Swiper
  - Reference：`frontend/src/components/MobileProjectView.tsx` 中 Terminal 和 TodoList 的布局
  - Hook point：通过 Terminal 组件暴露的 WebSocket 发送数据，或通过 `term.onData` 回调

##### WI-4.2 [S] 粘贴弹窗组件
- **描述**：新建 `frontend/src/components/PasteModal.tsx`。点击快捷键栏的粘贴按钮弹出 modal。包含 `<textarea>` 输入框（支持多行粘贴）、确认和取消按钮。确认时将内容通过 `onSend` 发送到终端，清空并关闭。取消或点击遮罩层关闭。空内容点确认直接关闭。
- **验收标准**：
  1. 点击粘贴按钮弹出弹窗
  2. 粘贴文本并确认，内容发送到终端
  3. 取消或点击外部关闭弹窗
  4. 空内容确认不发送，直接关闭
  5. 安全门控：`cd frontend && npx tsc --noEmit` 通过
- **Notes**：
  - Pattern：React portal 或 absolute 定位 modal，`autoFocus` 让 textarea 自动聚焦方便粘贴
  - Reference：无现有 modal 组件，新建
  - Hook point：被 ShortcutBar 中的粘贴按钮触发

##### WI-4.3 [S] Terminal 组件适配 — 暴露发送接口
- **描述**：修改 `Terminal.tsx`，通过 `useImperativeHandle` + `forwardRef` 暴露 `sendData(data: string)` 方法，内部调用 `ws.send(new TextEncoder().encode(data))`。MobileProjectView 通过 ref 获取此方法，传给 ShortcutBar 的 `onSend`。
- **验收标准**：
  1. Terminal 暴露 `sendData` 方法
  2. 快捷键栏和粘贴弹窗通过此方法发送数据到终端
  3. 桌面端 Terminal 不受影响
  4. 安全门控：`cd frontend && npx tsc --noEmit` 通过
- **Notes**：
  - Pattern：`React.forwardRef` + `useImperativeHandle`
  - Reference：`frontend/src/components/Terminal.tsx:94-98` 的 `onData` 和 ws.send
  - Hook point：被 MobileProjectView 使用

##### WI-4.4 [集成门控] 快捷键栏 + 粘贴集成验证
- **描述**：验证 WI-4.1 ~ WI-4.3 的集成状态。
- **验收标准**：
  1. `cd frontend && npx tsc --noEmit` 通过
  2. 移动端快捷键栏和粘贴弹窗正常工作
  3. 快捷键栏水平滑动不影响 Swiper
  4. 桌面端不显示快捷键栏

##### WI-4.5 [S] 阶段 4 前端测试
- **描述**：编写 Vitest 测试：(1) ShortcutBar 各按钮点击触发正确的 onSend 值；(2) PasteModal 确认/取消/空内容行为。
- **验收标准**：
  1. `cd frontend && npx vitest run` 通过
  2. 覆盖所有快捷键映射和粘贴弹窗行为
  3. 安全门控：`cd frontend && npx vitest run` 通过
- **Notes**：
  - Pattern：`@testing-library/react` 模拟点击和输入
  - Reference：WI-1.3 的 Vitest 配置
  - Hook point：CI 集成

##### WI-4.6 [集成门控] 阶段 4 完整集成验证
- **描述**：验证阶段 4 所有工作项的集成状态。
- **验收标准**：
  1. `cd frontend && npx tsc --noEmit` 通过
  2. `cd frontend && npx vitest run` 通过
  3. `make build` 构建成功
  4. 手机浏览器验证：快捷键栏各按钮功能正常，粘贴弹窗正常，Swiper 不受干扰

**阶段验收标准**：
1. Given 移动端打开项目, When 页面加载, Then 终端底部显示快捷键栏
2. Given 终端运行 `cat`, When 点击 ^C 按钮, Then 进程被中断
3. Given 终端运行 shell, When 点击 Tab 按钮, Then 触发自动补全
4. Given 快捷键栏, When 水平滑动, Then 栏内容滚动，不切换 Swiper 页面
5. Given 点击粘贴按钮, When 输入文本并确认, Then 文本发送到终端
6. Given 粘贴弹窗, When 空内容点确认, Then 弹窗关闭，不发送
7. 所有安全门控命令通过

**阶段状态**：已完成
**完成日期**：2026-04-16
**验收结果**：通过
**安全门控**：全部通过
**集成门控**：全部通过
**备注**：快捷键栏（9按钮）+ 粘贴弹窗 + Terminal forwardRef 适配完成

---

## 3. 风险与应对

| 风险 | 影响 | 概率 | 应对措施 |
|------|------|------|----------|
| PTY setuid 需要 root 权限，开发环境无法测试 | 用户切换功能无法在开发机验证 | 中 | 非 root 时跳过 setuid，仅 root 时生效；测试中 mock Credential 设置 |
| 快捷键栏水平滑动与 Swiper 页面切换冲突 | 快捷键栏滑动误触发页面切换 | 中 | `e.stopPropagation()` + `touch-action: pan-x` 隔离事件；Swiper `touchAngle: 45` 已有角度限制 |
| systemctl 命令在 CI/容器环境不可用 | 服务管理命令无法自动化测试 | 高 | 抽取 systemctl 调用为接口，测试中 mock；手动验证实际 systemd 集成 |
| 日志轮转在跨天边界的并发写入 | 轮转期间日志丢失 | 低 | mutex 保护轮转操作，确保原子性 |

## 4. 开发规范

### 4.1 代码规范
- Go：遵循 Go 官方代码风格，`gofmt` 格式化
- TypeScript：遵循项目 ESLint 配置
- CSS：使用 Tailwind utility classes，避免自定义 CSS

### 4.2 Git 规范
- 在 main 分支上开发
- 提交信息使用中文
- 每个工作项完成后提交，粒度与工作项对齐

### 4.3 文档规范
- 代码注释仅在逻辑不自明时添加
- 不添加额外 README 或文档文件

## 5. 工作项统计

| 阶段 | S | M | 集成门控 | 总计 |
|------|---|---|---------|------|
| 阶段 1：前端体验优化 | 2 | 1 | 1 | 4 |
| 阶段 2：后端基础设施 | 4 | 3 | 1 | 8 |
| 阶段 3：服务管理 | 3 | 2 | 2 | 7 |
| 阶段 4：移动端快捷键栏 | 3 | 1 | 2 | 6 |
| **合计** | **12** | **7** | **6** | **25** |

## 审核记录

| 日期 | 审核人 | 评分 | 结果 | 备注 |
|------|--------|------|------|------|
| 2026-04-16 | AI Assistant | 92/100 | 通过 | 测试交织改进：拆分测试项紧跟功能，WI-3.3 改为 M，阶段 3 增加 1 项 |
