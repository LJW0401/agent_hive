# Agent Hive 多终端与布局增强 开发方案

> 创建日期：2026-04-16
> 状态：已审核
> 版本：v1.1
> 关联需求文档：project_plan/requirements.md

## 1. 技术概述

### 1.1 技术架构

```
                  ┌──────────────────────────────────────────┐
                  │              Frontend (React)             │
                  │                                          │
                  │  App.tsx ─── ProjectContainer ──┐        │
                  │    │          (multi/single)    │        │
                  │    │                      TerminalTabBar │
                  │    │                           │        │
                  │  MobileApp ── MobileProjectView ──┘     │
                  │                                          │
                  └──────┬──────────────┬───────────────────┘
                         │              │
                    REST API      WebSocket
                         │              │
                  ┌──────┴──────────────┴───────────────────┐
                  │              Backend (Go)                 │
                  │                                          │
                  │  Manager ─── Container ─── Terminal[]    │
                  │                  │           ├ session    │
                  │                  │           ├ logFile    │
                  │                  │           └ listeners  │
                  │                                          │
                  │  SQLite: containers + terminals + ...    │
                  └──────────────────────────────────────────┘
```

**核心变更**：将 `Container` 中的单一 PTY session 拆分为 `Terminal` 子结构，每个容器持有 `map[string]*Terminal`。WebSocket 路由增加 `tid` 参数路由到具体终端。

### 1.2 技术栈

| 类别 | 选择 | 说明 |
|------|------|------|
| 后端语言 | Go | 不变 |
| 前端框架 | React + TypeScript | 不变 |
| 终端模拟 | xterm.js | 不变 |
| 数据库 | SQLite (WAL) | 新增 terminals 表 |
| 样式 | Tailwind CSS | 不变 |
| 布局持久化 | localStorage | 桌面布局模式存前端 |
| 进程检测 | /proc/{pid}/stat | Linux 子进程树检测 |

### 1.3 项目结构变更

```
backend/internal/
├── container/
│   ├── manager.go          # 重构：Container 持有 Terminal map
│   └── terminal.go         # 新增：Terminal 子结构（session, logFile, listeners）
├── store/
│   ├── store.go            # 新增 terminals 表 + 迁移逻辑
│   └── terminal.go         # 新增：Terminal CRUD 数据访问
├── server/
│   └── server.go           # 新增终端 CRUD 路由 + 进程检测路由
└── ws/
    └── handler.go          # WebSocket 路由增加 tid 参数

frontend/src/
├── components/
│   ├── TerminalTabBar.tsx  # 新增：终端标签栏（桌面+移动端共用）
│   ├── ConfirmDialog.tsx   # 新增：关闭确认弹窗
│   ├── SingleProjectView.tsx # 新增：单项目全屏视图
│   ├── ProjectContainer.tsx  # 修改：集成标签栏
│   ├── MobileProjectView.tsx # 修改：集成标签栏 + 7/3 布局
│   ├── Terminal.tsx          # 修改：接受 terminalId prop
│   └── ShortcutBar.tsx       # 修改：新增回车键
├── App.tsx                   # 修改：布局切换 + 聚焦追踪
└── api.ts                    # 新增终端 CRUD API 调用
```

### 1.4 安全门控命令

- 后端测试：`cd backend && go test ./...`
- 前端测试：`cd frontend && npx vitest run`
- 全量构建：`make build`

## 2. 开发阶段

### 阶段 1：后端多终端基础

**目标**：完成多终端的后端基础设施，包括数据模型、API、WebSocket 路由和数据迁移

**涉及的需求项**：
- 2.1.2 新建终端（后端部分）
- 2.1.3 关闭终端（后端部分：进程检测）
- 2.1.4 多终端持久化（数据模型 + 迁移）
- 2.1.5 多终端跨设备同步（后端广播）

#### 工作项列表

##### WI-1.1 [M] DB schema：terminals 表 + 数据迁移
- **描述**：在 SQLite 中新增 `terminals` 表 `(id TEXT PK, container_id TEXT, name TEXT, is_default BOOL, sort_order INT, created_at DATETIME)`。启动时检测是否需要迁移：如果 terminals 表为空但存在旧格式日志文件 `{containerID}.log`，为每个容器自动创建默认终端记录，并将日志文件移动到 `{containerID}/{terminalID}.log` 子目录
- **验收标准**：
  1. 新建数据库时 terminals 表正确创建
  2. 旧数据库升级时，每个容器自动生成一条 is_default=true 的终端记录
  3. 旧日志文件迁移到新路径，原文件不残留
  4. 安全门控：`cd backend && go test ./...` 通过
- **Notes**：
  - Pattern：SQLite `CREATE TABLE IF NOT EXISTS` + 检测迁移条件
  - Reference：`backend/internal/store/store.go`（现有建表逻辑）
  - Hook point：`store.New()` 初始化时执行迁移

##### WI-1.2 [S] 测试：DB schema + 数据迁移
- **描述**：为 WI-1.1 编写测试，覆盖新建库、旧库迁移、日志文件迁移三个场景
- **验收标准**：
  1. 测试新库 terminals 表结构正确
  2. 测试旧库迁移后每个容器有一条默认终端记录
  3. 测试日志文件从 `{cid}.log` 迁移到 `{cid}/{tid}.log`
  4. 安全门控：`cd backend && go test ./...` 通过

##### WI-1.3 [M] Terminal CRUD 数据访问层
- **描述**：在 store 包新增 `terminal.go`，提供终端的 CRUD 操作：`CreateTerminal(containerID, name, isDefault) → Terminal`、`ListTerminals(containerID) → []Terminal`、`DeleteTerminal(id)`、`CountTerminals(containerID) → int`
- **验收标准**：
  1. CreateTerminal 正确插入记录，sort_order 自动递增
  2. ListTerminals 按 sort_order 排序返回
  3. DeleteTerminal 删除记录
  4. CountTerminals 返回正确数量
  5. 安全门控：`cd backend && go test ./...` 通过
- **Notes**：
  - Pattern：与现有 `store/todo.go` 风格一致
  - Reference：`backend/internal/store/todo.go`
  - Hook point：被 Manager 和 HTTP handler 调用

##### WI-1.4 [S] 测试：Terminal CRUD 数据访问
- **描述**：为 WI-1.3 的四个方法编写测试
- **验收标准**：
  1. 覆盖创建、列表、删除、计数四个操作
  2. 测试边界：容器无终端时列表为空、删除不存在的记录不报错
  3. 安全门控：`cd backend && go test ./...` 通过

##### WI-1.5 [集成门控] DB 层集成验证
- **描述**：验证 terminals 表 + CRUD + 迁移逻辑的集成状态
- **验收标准**：
  1. `cd backend && go test ./...` 全部通过
  2. 手动验证：用旧版数据目录启动，迁移正确完成

##### WI-1.6 [M] Container/Terminal 结构重构
- **描述**：从 `Container` 中提取 `Terminal` 子结构到新文件 `container/terminal.go`。Terminal 持有 `session *ptypkg.Session`、`logFile *os.File`、`listeners map[*Listener]bool`。Container 改为持有 `terminals map[string]*Terminal`。更新 Manager 的 `Create()`、`Reopen()`、`Delete()` 方法适配多终端。`pumpOutput()` 绑定到 Terminal 级别
- **验收标准**：
  1. 每个容器创建时自动创建一个默认终端
  2. Manager.Create() 返回的容器包含一个可用终端
  3. Manager.Delete() 清理容器下所有终端的 session、logFile
  4. pumpOutput() 在 Terminal 级别运行，输出写入对应终端的 logFile
  5. 安全门控：`cd backend && go test ./...` 通过
- **Notes**：
  - Pattern：组合模式，Container 聚合 Terminal
  - Reference：`backend/internal/container/manager.go`（现有 Container 结构）
  - Hook point：所有使用 Container.session 的地方需改为通过 Terminal 访问

##### WI-1.6b [S] 测试：Container/Terminal 结构重构
- **描述**：为 WI-1.6 编写测试，验证 Container/Terminal 结构重构后的核心行为
- **验收标准**：
  1. 测试 Manager.Create() 创建的容器包含一个默认终端
  2. 测试 Manager.Delete() 清理容器下所有终端的资源
  3. 测试 pumpOutput() 输出写入对应终端的 logFile
  4. 安全门控：`cd backend && go test ./...` 通过
- **Notes**：
  - Reference：`backend/internal/container/manager.go`、`terminal.go`
  - Hook point：验证重构后与 WI-1.7 API 层的对接基础

##### WI-1.7 [M] Terminal 级别 API + 进程检测
- **描述**：新增 REST API：
  - `GET /api/containers/{id}/terminals` — 列出终端
  - `POST /api/containers/{id}/terminals` — 创建终端（检查上限 5）
  - `DELETE /api/containers/{id}/terminals/{tid}` — 关闭终端（销毁 PTY + 删除记录 + 清理日志）
  - `GET /api/containers/{id}/terminals/{tid}/has-process` — 检测子进程
  
  创建/删除终端后通过 `/ws/notify` 广播 `{"type":"terminals-changed","containerId":"..."}`
- **验收标准**：
  1. 列出终端返回正确列表
  2. 创建终端成功，超过 5 个返回 400 错误
  3. 删除终端销毁 PTY 会话并清理数据
  4. 删除默认终端返回 400 错误
  5. has-process 在有子进程时返回 true，无子进程时返回 false
  6. 创建/删除后广播 terminals-changed 事件
  7. 安全门控：`cd backend && go test ./...` 通过
- **Notes**：
  - Pattern：RESTful 嵌套资源，与现有 todos API 风格一致
  - Reference：`backend/internal/server/server.go`（路由注册）
  - Hook point：进程检测读取 `/proc/{pid}/children` 或遍历 `/proc/{pid}/task/*/children`

##### WI-1.8 [S] 测试：Terminal API + 进程检测
- **描述**：为 WI-1.7 编写测试，覆盖 CRUD 正常/异常路径和进程检测逻辑
- **验收标准**：
  1. 测试创建终端成功 + 上限拒绝
  2. 测试删除普通终端成功 + 删除默认终端被拒绝
  3. 测试进程检测的纯函数逻辑（mock /proc 读取）
  4. 安全门控：`cd backend && go test ./...` 通过

##### WI-1.9 [S] WebSocket 路由更新
- **描述**：修改 `/ws/terminal` 路由，新增 `tid` 查询参数。根据 `id`（容器）和 `tid`（终端）定位到具体 Terminal 的 listeners。不传 tid 时默认连接容器的默认终端（向后兼容）。ReadHistory 根据终端类型使用不同行数上限（默认终端 1000 行，额外终端 200 行）
- **验收标准**：
  1. `?id=c-1&tid=t-1` 正确连接到容器 c-1 的终端 t-1
  2. `?id=c-1`（无 tid）连接到默认终端
  3. 默认终端恢复历史 1000 行，额外终端恢复 200 行
  4. 多设备连接同一终端时，输出实时广播到所有 listeners
  5. 安全门控：`cd backend && go test ./...` 通过
- **Notes**：
  - Pattern：查询参数路由
  - Reference：`backend/internal/ws/handler.go`（现有 HandleTerminal）
  - Hook point：`Terminal.listeners` 替代原 `Container.listeners`

##### WI-1.10 [集成门控] 后端多终端完整验证
- **描述**：验证阶段 1 所有工作项的集成状态
- **验收标准**：
  1. `cd backend && go test ./...` 全部通过
  2. `make build` 构建成功
  3. 手动验证：启动服务，通过 curl 创建终端、列出终端、连接 WebSocket、删除终端

**阶段验收标准**：
1. Given 旧版数据目录，When 新版启动，Then 数据迁移完成，每个容器有一个默认终端
2. Given 一个容器，When 通过 API 创建 5 个终端后再创建，Then 返回 400
3. Given 两个 WebSocket 客户端连接同一终端，When 一个客户端输入，Then 另一个客户端收到输出
4. 所有安全门控通过

**阶段状态**：已完成
**完成日期**：2026-04-16

---

### 阶段 2：前端终端标签页

**目标**：实现终端标签页 UI、新建/关闭终端、懒加载策略和跨设备同步

**涉及的需求项**：
- 2.1.1 终端标签页 UI
- 2.1.2 新建终端（前端部分）
- 2.1.3 关闭终端（前端部分 + 确认弹窗）
- 2.1.5 多终端跨设备同步（前端部分）
- 2.1.6 终端懒加载

#### 工作项列表

##### WI-2.1 [S] API 调用层 + 类型定义
- **描述**：在 `api.ts` 中新增终端相关 API 调用函数：`listTerminals(containerID)`、`createTerminal(containerID)`、`deleteTerminal(containerID, terminalID)`、`hasProcess(containerID, terminalID)`。新增 TypeScript 类型 `TerminalInfo { id, containerID, name, isDefault, sortOrder, createdAt }`
- **验收标准**：
  1. 类型定义完整，API 函数可正常调用
  2. 安全门控：`cd frontend && npx vitest run` 通过
- **Notes**：
  - Pattern：与现有 `createContainer()`、`listTodos()` 风格一致
  - Reference：`frontend/src/api.ts`
  - Hook point：被 TerminalTabBar 和容器组件调用

##### WI-2.2 [M] TerminalTabBar 组件
- **描述**：新建 `TerminalTabBar.tsx`，桌面端和移动端共用。Props：`terminals: TerminalInfo[]`、`activeId: string`、`onSelect(id)`、`onCreate()`、`onClose(id)`、`maxTerminals: number`。展示标签页列表 + "+"按钮 + "x"关闭按钮。默认终端不显示"x"。终端数达上限时隐藏"+"
- **验收标准**：
  1. 标签页正确渲染，点击切换触发 onSelect
  2. "+"按钮触发 onCreate，终端数达 5 时不显示
  3. 默认终端不显示"x"，额外终端显示"x"
  4. 安全门控：`cd frontend && npx vitest run` 通过
- **Notes**：
  - Pattern：受控组件，状态由父组件管理
  - Reference：现有 `ShortcutBar.tsx` 的布局风格
  - Hook point：嵌入 ProjectContainer 和 MobileProjectView

##### WI-2.3 [S] 测试：TerminalTabBar
- **描述**：为 TerminalTabBar 编写测试，覆盖渲染、切换、新建禁用、默认终端不可关闭
- **验收标准**：
  1. 渲染正确数量的标签
  2. 点击标签触发 onSelect
  3. 5 个终端时"+"不显示
  4. 默认终端无"x"按钮
  5. 安全门控：`cd frontend && npx vitest run` 通过

##### WI-2.4 [S] ConfirmDialog 组件
- **描述**：新建通用确认弹窗组件 `ConfirmDialog.tsx`。Props：`open`、`title`、`message`、`onConfirm()`、`onCancel()`。用于关闭终端时的进程确认
- **验收标准**：
  1. open=true 时显示弹窗，false 时不渲染
  2. 点击确认触发 onConfirm，点击取消触发 onCancel
  3. 安全门控：`cd frontend && npx vitest run` 通过
- **Notes**：
  - Pattern：与现有 PasteModal 结构类似
  - Reference：`frontend/src/components/PasteModal.tsx`

##### WI-2.5 [集成门控] 基础组件验证
- **描述**：验证 API 层 + TerminalTabBar + ConfirmDialog 组件的独立功能
- **验收标准**：
  1. `cd frontend && npx vitest run` 全部通过
  2. 组件可独立渲染、事件回调正常

##### WI-2.6 [M] 集成标签页到 ProjectContainer（桌面端）
- **描述**：修改 `ProjectContainer.tsx`，新增终端列表状态管理。组件挂载时调用 `listTerminals()` 获取终端列表，渲染 TerminalTabBar。activeTerminalId 状态控制当前显示的终端。Terminal 组件接收 `terminalId` prop（新增），WebSocket 连接使用 `?id={cid}&tid={tid}`。新建终端调用 API 后刷新列表。关闭终端先调用 has-process，有进程则弹 ConfirmDialog
- **验收标准**：
  1. 容器渲染时显示终端标签栏和默认终端
  2. 点击标签切换终端内容
  3. "+"新建终端，API 调用成功后新标签出现并自动激活
  4. "x"关闭终端，有进程时弹确认
  5. 安全门控：`cd frontend && npx vitest run` 通过
- **Notes**：
  - Pattern：容器组件管理子终端状态
  - Reference：`frontend/src/components/ProjectContainer.tsx`
  - Hook point：Terminal.tsx 需增加 terminalId prop

##### WI-2.6b [S] 测试：桌面端标签页集成
- **描述**：为 WI-2.6 编写测试，验证桌面端 ProjectContainer 的标签页集成
- **验收标准**：
  1. 测试容器渲染时显示标签栏和默认终端
  2. 测试点击标签切换 activeTerminalId
  3. 测试"+"新建终端后自动激活新标签
  4. 安全门控：`cd frontend && npx vitest run` 通过
- **Notes**：
  - Reference：`frontend/src/components/ProjectContainer.tsx`
  - Hook point：验证桌面端集成后为移动端集成提供参照

##### WI-2.7 [M] 集成标签页到 MobileProjectView（移动端）
- **描述**：修改 `MobileProjectView.tsx`，与桌面端相同的终端列表管理逻辑。TerminalTabBar 放在终端区域顶部。ShortcutBar 的 `onSend` 需路由到当前激活终端的 `sendData`
- **验收标准**：
  1. 移动端显示终端标签栏
  2. 标签切换、新建、关闭与桌面端行为一致
  3. ShortcutBar 发送数据到当前激活终端
  4. 安全门控：`cd frontend && npx vitest run` 通过
- **Notes**：
  - Reference：`frontend/src/components/MobileProjectView.tsx`
  - Hook point：terminalRef 需按 activeTerminalId 动态绑定

##### WI-2.8 [S] Terminal 组件适配 + 懒加载
- **描述**：修改 `Terminal.tsx`，新增 `terminalId` prop，WebSocket URL 加 `&tid={terminalId}`。实现懒加载：组件接收 `active` prop，默认终端（isDefault=true）始终保持连接；额外终端仅在 active=true 时建立 WebSocket 连接，active=false 时断开并销毁 xterm 实例
- **验收标准**：
  1. terminalId 正确传入 WebSocket URL
  2. 默认终端切换到其他标签后仍保持连接
  3. 额外终端切换离开后断开连接
  4. 额外终端切换回来后重新连接并加载历史
  5. 安全门控：`cd frontend && npx vitest run` 通过
- **Notes**：
  - Pattern：条件渲染 + useEffect 依赖 active 状态
  - Reference：`frontend/src/components/Terminal.tsx`（现有 WebSocket 逻辑）
  - Hook point：active 变化触发 connect/disconnect

##### WI-2.9 [S] 测试：标签页集成 + 懒加载
- **描述**：为桌面端和移动端的标签页集成编写测试，验证懒加载行为
- **验收标准**：
  1. 测试默认终端始终渲染
  2. 测试额外终端仅在激活时渲染
  3. 安全门控：`cd frontend && npx vitest run` 通过

##### WI-2.10 [S] 跨设备终端同步
- **描述**：修改 `App.tsx` 和 `MobileApp.tsx` 中的 `/ws/notify` 处理逻辑，监听 `terminals-changed` 事件。收到事件后，通知对应容器组件刷新终端列表（类似现有 todoRefresh 机制，新增 terminalRefresh 计数器）
- **验收标准**：
  1. 设备 A 创建终端后，设备 B 的标签栏实时更新
  2. 设备 A 关闭终端后，设备 B 的标签栏实时更新
  3. 安全门控：`cd frontend && npx vitest run` 通过
- **Notes**：
  - Pattern：与现有 todos-updated 广播机制一致
  - Reference：`frontend/src/App.tsx`（connectNotifyWS 函数）
  - Hook point：新增 terminalRefresh 状态 + prop 传递

##### WI-2.10b [S] 测试：跨设备终端同步
- **描述**：为 WI-2.10 编写测试，验证 terminals-changed 事件处理逻辑
- **验收标准**：
  1. 测试 /ws/notify 收到 terminals-changed 后 terminalRefresh 计数器递增
  2. 测试 terminalRefresh 变化触发终端列表重新加载
  3. 安全门控：`cd frontend && npx vitest run` 通过
- **Notes**：
  - Reference：`frontend/src/App.tsx`、`MobileApp.tsx`
  - Hook point：与现有 todoRefresh 测试模式一致

##### WI-2.11 [集成门控] 前端多终端完整验证
- **描述**：验证阶段 2 所有工作项的集成状态
- **验收标准**：
  1. `cd frontend && npx vitest run` 全部通过
  2. `make build` 构建成功
  3. 手动验证：桌面端和移动端创建/切换/关闭终端，跨设备同步正常

**阶段验收标准**：
1. Given 桌面端打开一个项目，When 点击"+"创建 3 个终端并切换，Then 标签页切换流畅，终端内容独立
2. Given 额外终端内运行进程，When 点击"x"关闭，Then 弹出确认对话框
3. Given 设备 A 创建终端，When 设备 B 已连接，Then 设备 B 标签栏实时出现新标签
4. Given 用户切换到额外终端再切回默认终端，When 切换完成，Then 默认终端立即显示（无加载延迟）
5. 所有安全门控通过

**阶段状态**：已完成
**完成日期**：2026-04-16

---

### 阶段 3：桌面端布局切换

**目标**：实现多项目/单项目布局切换、项目导航和布局持久化

**涉及的需求项**：
- 2.2.1 布局切换按钮
- 2.2.2 单项目模式

#### 工作项列表

##### WI-3.1 [S] 布局模式状态 + localStorage 持久化
- **描述**：在 `App.tsx` 中新增 `layoutMode` 状态（'multi' | 'single'），初始值从 localStorage 读取，默认 'multi'。变更时写入 localStorage。新增 `focusedContainerId` 状态，追踪最近一次获得终端焦点的容器 ID（通过终端区域的 onFocus/onClick 事件更新）
- **验收标准**：
  1. 刷新页面后 layoutMode 保持
  2. focusedContainerId 在点击/操作终端时正确更新
  3. 安全门控：`cd frontend && npx vitest run` 通过
- **Notes**：
  - Pattern：useState + useEffect(localStorage)
  - Reference：`frontend/src/App.tsx`
  - Hook point：layoutMode 决定渲染 Grid 还是 SingleProjectView

##### WI-3.2 [S] 布局切换按钮 UI
- **描述**：在 App.tsx 顶部栏添加布局切换按钮（图标式：网格图标 ↔ 全屏图标）。点击时切换 layoutMode。切换到单项目模式时，使用 focusedContainerId 确定显示哪个项目（未聚焦则默认第一个）
- **验收标准**：
  1. 按钮可见，图标反映当前模式
  2. 点击切换布局模式
  3. 切换到单项目模式时显示正确的项目
  4. 安全门控：`cd frontend && npx vitest run` 通过
- **Notes**：
  - Reference：App.tsx 顶部栏区域

##### WI-3.2b [S] 测试：布局状态 + 切换按钮
- **描述**：为 WI-3.1 和 WI-3.2 编写测试，验证布局模式状态管理和切换按钮
- **验收标准**：
  1. 测试 layoutMode 在 localStorage 中的读写和初始化
  2. 测试切换按钮点击后 layoutMode 变化
  3. 测试 focusedContainerId 更新逻辑
  4. 安全门控：`cd frontend && npx vitest run` 通过
- **Notes**：
  - Reference：`frontend/src/App.tsx`
  - Hook point：验证状态管理后为 SingleProjectView 提供基础

##### WI-3.3 [M] SingleProjectView 单项目视图
- **描述**：新建 `SingleProjectView.tsx`，占满页面主区域。顶部标题栏：左箭头 + 项目名称 + 右箭头。下方：待办列表（左）+ 终端标签页和终端（右），复用现有 ProjectContainer 的布局逻辑（分割线可拖拽）。Props：`container`、`containers`（全部容器列表，用于导航）、`onNavigate(containerId)`、相关回调
- **验收标准**：
  1. 项目占满页面主区域
  2. 标题栏显示项目名称和左右箭头
  3. 待办列表可见，终端标签页可切换
  4. 分割线可拖拽调整比例
  5. 安全门控：`cd frontend && npx vitest run` 通过
- **Notes**：
  - Pattern：复用 ProjectContainer 内部布局，外层全屏包装
  - Reference：`frontend/src/components/ProjectContainer.tsx`
  - Hook point：嵌入 App.tsx，layoutMode === 'single' 时渲染

##### WI-3.4 [S] 项目导航 + 快捷键适配
- **描述**：SingleProjectView 标题栏左右箭头点击时切换到上/下一个项目。第一个项目左箭头禁用，最后一个右箭头禁用。修改 App.tsx 中的 Ctrl+←/→ 快捷键逻辑：多项目模式下切换页面（不变），单项目模式下切换项目
- **验收标准**：
  1. 点击左右箭头正确切换项目
  2. 边界项目时箭头禁用
  3. Ctrl+←/→ 在单项目模式下切换项目
  4. Ctrl+←/→ 在多项目模式下仍切换页面
  5. 安全门控：`cd frontend && npx vitest run` 通过
- **Notes**：
  - Reference：`frontend/src/App.tsx`（现有 handleKeyDown）
  - Hook point：layoutMode 条件分支快捷键行为

##### WI-3.5 [S] 测试：单项目视图 + 导航
- **描述**：为 WI-3.3 和 WI-3.4 编写测试，验证单项目视图渲染和项目导航
- **验收标准**：
  1. 测试 SingleProjectView 占满主区域，标题栏显示项目名
  2. 测试导航边界（第一个项目左箭头禁用，最后一个右箭头禁用）
  3. 测试 Ctrl+←/→ 在单项目模式下切换项目
  4. 安全门控：`cd frontend && npx vitest run` 通过
- **Notes**：
  - Reference：`frontend/src/components/SingleProjectView.tsx`
  - Hook point：验证导航逻辑与快捷键适配

##### WI-3.6 [集成门控] 桌面布局完整验证
- **描述**：验证阶段 3 所有工作项的集成状态
- **验收标准**：
  1. `cd frontend && npx vitest run` 全部通过
  2. `make build` 构建成功
  3. 手动验证：切换布局、导航项目、快捷键、刷新后持久化

**阶段验收标准**：
1. Given 多项目模式，When 点击布局切换按钮，Then 切换到单项目模式，显示当前聚焦项目
2. Given 单项目模式显示第一个项目，When 点击左箭头，Then 箭头禁用无响应
3. Given 用户切换到单项目模式后刷新页面，When 页面加载，Then 仍为单项目模式
4. Given 单项目模式，When 按 Ctrl+→，Then 切换到下一个项目
5. 所有安全门控通过

**阶段状态**：未开始

---

### 阶段 4：移动端增强

**目标**：优化移动端布局并增强快捷键栏

**涉及的需求项**：
- 2.3.1 移动端 7/3 布局
- 2.4.1 新增回车键

#### 工作项列表

##### WI-4.1 [M] 移动端 7/3 布局 + 拖动分割线
- **描述**：修改 `MobileProjectView.tsx` 布局：终端区域（含标签栏 + 终端 + 快捷键栏）在上，待办列表在下。默认比例 7:3。支持拖动分割线调整，终端区域最小 40%，待办区域最小 15%。分割线使用水平拖拽手柄
- **验收标准**：
  1. 页面加载时终端在上占 70%，待办在下占 30%
  2. 拖动分割线可调整比例
  3. 终端区域不小于 40%，待办区域不小于 15%
  4. 快捷键栏和终端标签栏在终端区域内
  5. 安全门控：`cd frontend && npx vitest run` 通过
- **Notes**：
  - Pattern：与桌面端 ProjectContainer 的 split 逻辑类似，改为上下方向
  - Reference：`frontend/src/components/MobileProjectView.tsx`、`ProjectContainer.tsx`（分割线逻辑）
  - Hook point：touchmove 事件处理，需与 Swiper 隔离（现有 stopPropagation 方案）

##### WI-4.2 [S] 测试：移动端布局
- **描述**：为移动端 7/3 布局编写测试
- **验收标准**：
  1. 测试默认比例为 70%
  2. 测试终端区域最小 40%、待办最小 15% 的约束
  3. 安全门控：`cd frontend && npx vitest run` 通过

##### WI-4.3 [S] 快捷键栏新增回车键
- **描述**：在 `ShortcutBar.tsx` 的快捷键列表中新增回车键按钮 `{label: 'Enter', value: '\r'}`
- **验收标准**：
  1. 快捷键栏显示 Enter 按钮
  2. 点击发送 `\r` 到终端
  3. 安全门控：`cd frontend && npx vitest run` 通过
- **Notes**：
  - Reference：`frontend/src/components/ShortcutBar.tsx`
  - Hook point：添加到 shortcuts 数组

##### WI-4.4 [S] 测试：回车键
- **描述**：为回车键按钮编写测试
- **验收标准**：
  1. 测试 Enter 按钮存在且点击发送 `\r`
  2. 安全门控：`cd frontend && npx vitest run` 通过

##### WI-4.5 [集成门控] 移动端增强完整验证
- **描述**：验证阶段 4 所有工作项的集成状态
- **验收标准**：
  1. `cd frontend && npx vitest run` 全部通过
  2. `make build` 构建成功
  3. 手动验证：移动端布局比例、分割线拖动、回车键功能

**阶段验收标准**：
1. Given 移动端访问，When 页面加载，Then 终端区域在上约占 70%，待办在下约占 30%
2. Given 移动端页面已加载，When 拖动分割线到极限，Then 终端最小 40%，待办最小 15%
3. Given 移动端快捷键栏可见，When 点击 Enter 按钮，Then 终端收到回车输入
4. 所有安全门控通过

**阶段状态**：未开始

---

## 3. 风险与应对

| 风险 | 影响 | 概率 | 应对措施 |
|------|------|------|----------|
| 多终端 WebSocket 并发导致资源消耗增大 | 性能下降 | 低 | 懒加载策略限制同时连接数；默认终端常驻，额外终端按需连接 |
| 旧数据迁移失败（日志文件损坏或权限问题） | 数据丢失 | 低 | 迁移前备份日志文件；迁移失败记录日志但不阻塞启动 |
| 进程检测 /proc 在不同 Linux 发行版上行为差异 | 关闭确认不准确 | 中 | 使用 `os.FindProcess` + `/proc/{pid}/children` 双重检测；检测失败时默认弹确认 |
| 移动端拖动分割线与 Swiper 手势冲突 | 交互异常 | 中 | 复用现有 stopPropagation 隔离方案；分割线区域设置 `touch-action: none` |
| 单项目模式下 Terminal resize 事件触发时机 | 终端尺寸错误 | 低 | 复用现有 requestAnimationFrame 延迟方案 |

## 4. 开发规范

### 4.1 代码规范
- Go：标准 gofmt 格式化，导出函数必须有注释
- TypeScript：项目现有 ESLint 配置
- 新组件使用函数式组件 + hooks
- 样式使用 Tailwind CSS utility classes

### 4.2 Git 规范
- 分支：从 main 创建 `dev/multi-terminal` 分支
- 提交：每个阶段完成后提交一次，提交信息使用中文
- Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>

### 4.3 文档规范
- 代码注释：仅在逻辑不自明处添加
- API 变更：更新 README 的 CLI 命令一览（如有变化）

## 5. 工作项统计

| 阶段 | S | M | 集成门控 | 总计 |
|------|---|---|---------|------|
| 阶段 1：后端多终端基础 | 5 | 3 | 2 | 10 |
| 阶段 2：前端终端标签页 | 7 | 3 | 2 | 12 |
| 阶段 3：桌面端布局切换 | 5 | 1 | 1 | 7 |
| 阶段 4：移动端增强 | 3 | 1 | 1 | 5 |
| **合计** | **20** | **8** | **6** | **34** |

## 审核记录

| 日期 | 审核人 | 评分 | 结果 | 备注 |
|------|--------|------|------|------|
| 2026-04-16 | AI Assistant | 88/100 → 95/100 | 通过 | 插入 4 个测试项修复测试交织问题（WI-1.6b、WI-2.6b、WI-2.10b、WI-3.2b） |
