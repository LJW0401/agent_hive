# Learnings

## 2026-04-23

### Bug 修复：reopen 终端丢失历史

- **发现于**：用户手动测试（"后台重启后重新打开终端之前的信息都没了变成了新的终端"）
- **现象**：服务端重启后，容器自动进入 disconnected 态；用户点"重连"，新打开的终端没有任何历史滚动区，像是全新终端。
- **根因**：`reopenTerminal` 用 `O_CREATE|O_WRONLY|O_TRUNC` 打开磁盘上的终端日志文件。虽然 Restore 阶段文件仍然完好，但 reopen 这一步就把历史内容截断；之后前端 WS 连上拉 `ReadHistory` 只能读到空文件。与 `Create` / `CreateTerminal` 用的 `O_APPEND` 形成了不一致。
- **修复**：抽出 `openTerminalLogFile(path)` 辅助函数统一使用 `O_CREATE|O_WRONLY|O_APPEND`，三个入口（Create / CreateTerminal / reopenTerminal）全部走这一条路径；同时在 reopen 时向日志里写一条 ANSI dim 的「终端已重连 yyyy-mm-dd HH:MM:SS」分隔行（`formatReconnectMarker`），让用户在滚动历史时能看到上次会话和本次会话的接缝。
- **回归测试**：`backend/internal/container/reopen_history_test.go` 三个用例：
  1. `TestOpenTerminalLogFilePreservesPriorContent`：直接验证 helper 不截断预存内容。
  2. `TestOpenTerminalLogFileCreatesWhenMissing`：文件不存在时能正常创建（边界）。
  3. `TestReadHistoryAfterReopenIncludesPriorSession`：从 `ReadHistory` 视角验证 reopen 后旧 + 新输出都在。
  修复前三个里的 1 和 3 均失败（已实测），修复后全部通过。
- **为什么原测试没覆盖**：`reopenTerminal` 会 fork 真 PTY，测试基础设施里没有这个路径的用例；历史存续又是"跨进程生命周期"的行为（服务重启才复现），纯单测无法自然触发。异常场景清单只盖了"reopen 成功建立新会话"，漏了"reopen 对副作用文件的影响"。以后写会改动磁盘产物的函数，必须对产物做快照断言。
- **紧急程度**：中（影响用户核心工作流——服务重启后所有终端看起来像崩了）
- **衍生改进建议**（下次处理）：
  1. 历史日志随长时间重连会无限追加，当前 `ReadHistory` 已有 256KB 尾读上限足以遮蔽，但磁盘占用会增长，将来应加日志轮转。
  2. 如果旧会话死在 alternate screen buffer（vim、less 等）里，replay 的 ANSI 会让屏幕残留异常。未来可在 `ReadHistory` 前端加一层"裁剪到最后一个清屏点"的处理。
  3. `container.Manager` 中还有多处直接 `os.OpenFile`/`os.Readlink`，将来若再出现因 flag 不一致导致的 bug，可以考虑引入一个 `fs` 子包统一封装。

### 快速功能：restart-terminal-inherit-cwd

- **类型**：架构洞察（观测模式 vs. 捕获时机）
- **描述**：shell 进程死亡后想"事后回读"它的 cwd 是不现实的——`/proc/<pid>/cwd` 在进程被 reap 后立刻消失，`pumpOutput` 读到 EOF 时 /proc 入口已不可读。本次采用"活期轮询 + 写穿到 DB"的观测模式（每 2s 采样一次 `/proc/<pid>/cwd`，只在值变化时写 DB），空读不覆盖缓存，避免 /proc 抖动清空状态。另一条路是让 shell 主动上报（OSC 7），但需要用户改 shell 配置，不够透明。
- **建议处理方式**：保持。2s 节拍在"短终端丢失窗口"与"DB 写频率"间折中；若观察到写 DB 过于频繁再叠"仅 Wait 退出时落盘"。
- **紧急程度**：低

- **类型**：测试缺口
- **描述**：`pollCWD` 的 ticker + goroutine 生命周期未覆盖真实 PTY 的集成测试（会 fork shell、依赖 /bin/sh）；目前只覆盖了 `observeCWD`/`reopenOpts`/`sessionPID` 三个纯函数 + `UpdateTerminalCWD` 的 DB 往返和老 schema 迁移。
- **建议处理方式**：等到真环境出现"reopen 没继承 cwd"再补集成用例；纯函数层已穷尽分支（首次/重复/空读/空读后恢复）。
- **紧急程度**：低

- **类型**：技术债（架构一致性）
- **描述**：`container.Manager.db` 是具体 `*store.Store` 类型，无法在单测中注入假 store 验证"变化时才写 DB"。当前借 `observeCWD` 返回值 + `if changed && db != nil` 守卫侧面保证，但无法强断言。
- **建议处理方式**：若后续 Manager 的 DB 访问点再增加，抽 `terminalStore` interface 允许测试替换。
- **紧急程度**：低

- **类型**：架构洞察（schema 演进）
- **描述**：SQLite 没有 `ALTER TABLE ADD COLUMN IF NOT EXISTS`，本次用 `PRAGMA table_info` 探测列存在再决定是否 ALTER，是 SQLite 下幂等加列的标准做法。测试覆盖了"幂等"与"老库升级"两类。
- **建议处理方式**：之后再加列复用 `addTerminalLastCWDColumn` 模式。
- **紧急程度**：低

## 2026-04-20

### 快速功能：todo-insert-at-top

- **类型**：测试缺口
- **描述**：`backend/internal/store/todo.go` 之前无单元测试；本次新增 `todo_test.go` 覆盖 CreateTodo 的排序行为（新建在顶部、跨容器隔离、reorder 之后仍在顶部、空容器首条）。
- **建议处理方式**：保持，后续给 UpdateTodo/DeleteTodo/ReorderTodos 也补上测试。
- **紧急程度**：低

- **类型**：技术债（范围外）
- **描述**：`npm run lint` 存在 11 个 pre-existing 错误 + 4 个警告，涉及 `useTerminalTabs.ts` 等文件的 exhaustive-deps / 类型定义，与本次改动无关，未清理。
- **建议处理方式**：单独起一次 lint 清理任务。
- **紧急程度**：低

- **类型**：架构洞察
- **描述**：`sort_order` 采用 int 空间两端扩展（min-1 / max+1）的策略，理论上可无限制地新建而不需重排；`ReorderTodos` 已将其规范化到 `[0..N-1]`，因此长期也不会累积过大偏移。当前选择用 min-1 让新 todo 置顶，和已有 max+1 置底逻辑在设计上对称。
- **建议处理方式**：无需行动，文档化以便未来改动时保持一致。
- **紧急程度**：低

### 快速功能：mobile-delete-visible

- **类型**：架构洞察
- **描述**：用 Tailwind `md:` 断点（≥768px）区分"触屏/鼠标"是惯例但不严格：大尺寸触屏平板按 md 规则进入 hover-only 分支，会再次出现"按钮不可见"的摩擦。若未来上触屏平板用户，可改用 `@media (hover: hover)` 媒体查询替换 `md:` 判定。
- **建议处理方式**：当前按桌面/手机两分类足够；收到平板用户反馈时再切换。
- **紧急程度**：低

### 快速功能：drag-edge-page-flip

- **类型**：架构洞察（主动降级）
- **描述**：用户原诉求是"拖到边缘换页但不中断拖拽"。当前 `App.tsx` 页面切换用 `<AnimatePresence mode="wait" key={currentPage}>` 整页卸载重建，这天然会把 dnd-kit 的 draggable 节点摘除、终止拖拽。要真正做到"不中断"，需要重构渲染拓扑：把被拖项从页面 key 中剥离出去（独立在顶层 mount）或放弃 AnimatePresence 的 mode="wait"。这是 L 级改造。
- **建议处理方式**：当前版本降级为"悬停 600ms 自动触发换页 + 接受拖拽中断"。如未来收集到用户反馈必须不中断，走 `/dev-plan` 重构拓扑。
- **紧急程度**：低

- **类型**：产品决策（跨页精确定位）
- **描述**：跨页搬运的完整工作流拆成两步，而非一步到位：(1) 拖到边缘悬停触发 / 或点标题栏 ←→ 箭头按钮，容器落到邻页边缘槽 (position 0 或 PAGE_SIZE-1)；(2) 在目标页内用原生 dnd-kit 拖拽重排到精确槽位。不支持"一次拖拽落到对页任意位置"——该交互必须等 /dev-plan 做无中断跨页拖拽后才能上。
- **建议处理方式**：若收到用户反馈频繁"两步走太麻烦"，再升级。当前在 README / 用户文档里说明此工作流。
- **紧急程度**：低

- **类型**：重构机会
- **描述**：`App.tsx` 已经在 600+ 行规模，里面掺入了拖拽 + 分页 + 键盘快捷键 + 单视图模式 + 通知 WS 等多个关切点。这次又塞进了 pointermove 监听 + dwell 计时器 + 边缘标记 UI，可读性继续下降。
- **建议处理方式**：后续把 "desktop multi-view"、"single-view"、"drag + page flip" 拆成独立 hooks（如 `useDragEdgePageFlip`）。
- **紧急程度**：中

- **类型**：测试缺口
- **描述**：`detectEdgeZone` 已补纯函数测试，但 dwell 计时器 + pointermove + 边缘标记的集成行为未覆盖（需要 jsdom + fake timers 模拟 pointer 事件）。当前仅靠手动 UI 验证。
- **建议处理方式**：等交互确定后再补集成测试，避免反复重写。
- **紧急程度**：低
