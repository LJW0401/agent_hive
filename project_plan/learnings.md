# Learnings

## 2026-04-23

### Bug 修复：重连后出现 `^[[O%` 焦点事件回显

- **发现于**：用户报告（"重连之后输出乱码：── 终端已重连 … ──^[[O% ➜ penguin"）
- **现象**：刷新页面后，marker 行尾紧跟 `^[[O`（ESC+`[O` 的 echoctl 可见渲染）和 zsh 的 `PROMPT_EOL_MARK` `%`，然后才是新 prompt。
- **根因**：日志里有一条上一次会话发过的 `\x1b[?1004h`（启用焦点事件追踪，常由 zsh/p10k 初始化时发）。重连时回放把这条字节送给新 mount 的 xterm.js → xterm.js 激活焦点追踪 → 浏览器标签刷新期间的焦点状态变化让它发 `\x1b[O` 给 WS → Server WriteToPTY → 新 shell 启动瞬间 tty 还在 cooked+ECHOCTL 状态，line discipline 把 `\x1b[O` 回显成可见 `^[[O` 打到屏幕上。`%` 是 zsh 发现前一行没 `\n` 结尾时的提示符前标记。鼠标追踪（`?1000`-`?1006`, `?1015`, `?1016`）同机制，任何鼠标移动都会触发 xterm.js 反向发输入。
- **修复**：扩展上一轮的 `terminalQueryRegex`，把会让 xterm.js 反向发输入的**模式 SET/RESET** 也加进去：`\x1b\[\?(?:100[0-6]|101[56])[hl]`。alt-screen（`?47` / `?1047` / `?1049`）号段在外，anchor 逻辑仍可依赖；bracketed paste (`?2004`)、光标可见性 (`?25`)、行包裹 (`?7`) 等不产生反向输入，也不被触碰。
- **回归测试**：`backend/internal/container/query_strip_test.go` 新增
  - `TestStripTerminalQueriesRemovesFocusAndMouseModes`：9 组 mode + 2 状态共 18 条序列全部剥掉。
  - `TestStripTerminalQueriesPreservesAltScreenAndBenignModes`：6 类 benign 序列全部保留（包括 anchor 三种变体）。
  - `TestReadHistoryStripsFocusModeFromReplay`：端到端验证日志 → ReadHistory 不含 `\x1b[?1004h` / `\x1b[?1002h`。
- **为什么原测试没覆盖**：上一轮的 query strip 测试只覆盖了"终端应答查询"这一类副作用路径，没覆盖"终端因为被启用了某个 mode 而反向发事件"这一类。二者的共同本质是"回放激活了让 xterm.js 变成输入源的状态"，但具体表现截然不同（查询 = 立即回应，mode = 后续事件触发）。协议层测试应该系统化覆盖"xterm.js 何时会成为输入源"——至少包括：应答查询、焦点事件、鼠标事件、bracketed paste 包裹、Kitty keyboard protocol 响应。
- **紧急程度**：中（每次重连都污染屏幕，但不阻断操作；跟随 bb974ae / b50b2da 一起发布比较合适）
- **衍生改进建议**（下次处理）：
  1. 日志里出现的 `\x1b[I`、`\x1b[O`、`\x1b[M...`（X10 鼠标上报）、`\x1b[<...M`（SGR 鼠标）等 **echo 回来的** xterm-to-pty 序列，本来就不该被当成"终端输出"再回放。下一步可以在 pumpOutput 写入 log 的阶段就过滤掉，或者在 readHistoryTailFromFile 里把这几种反向序列也剥掉。
  2. 本次还是在替"xterm.js 状态管理"打补丁。根本解法（上条 learning 里已经提过）是 server-side headless 解析：维护屏幕矩阵，回放发快照而非字节流。
  3. zsh 的 `PROMPT_EOL_MARK %` 其实还算友好 —— 没它的话 `^[[O` 就会跟 marker 贴在一起更难察觉。

### Bug 修复：重连后 prompt 出现 `2026;2$y1;2c...` 重复串

- **发现于**：用户报告（"刷新页面有的终端出现乱码 `➜ penguin 2026;2$y1;2c2026;2$y1;2c…`"）
- **现象**：非 TUI 终端（纯 zsh + oh-my-zsh / p10k）重连后，光标位置被塞进一大段 `2026;2$y1;2c` 重复串，像是有人在连续粘贴。
- **根因**：oh-my-zsh / p10k 在每次 prompt 绘制时发终端能力探测（DA1 `\x1b[c`、DECRQM mode 2026 `\x1b[?2026$p`）。这些查询字节在 shell stdout 上，**被我们的日志记录下来**。TUI anchor 修复把 tail 从 256KB 放大到 1MB 后，日志里攒下了几十次 prompt 查询都进了回放。重连时 xterm.js 把回放字节当**新鲜查询**处理，每次都照章回答；响应 `\x1b[?2026;2$y` / `\x1b[?1;2c` 经 WS 写回 PTY，idle 的 zsh ZLE 把 `ESC-[` 吃掉当 keymap 引导，剩下 `2026;2$y` / `1;2c` 作为字面文字插进命令行缓冲区。
- **修复**：新增 `stripTerminalQueries`，用正则在回放前剥掉只承担"向终端提问"语义的 CSI 序列：DA1/DA2/DA3 (`\x1b[{>=}*c`)、DSR 5/6 (`\x1b[{5,6}n`)、DECRQM 公/私 (`\x1b[{?}?N$p`)、XT version (`\x1b[>*q`)。响应格式因为有 `?` 前缀或 `$y`/`R`/`0n` 等区分位，不会被正则误伤。在 `readHistoryTailFromFile` 最终返回前统一调用，`ReadHistory` 和 `SubscribeWithSnapshot` 两条回放路径同时受益。
- **回归测试**：`backend/internal/container/query_strip_test.go`
  - `TestStripTerminalQueriesRemovesDA1AndDECRQM`：用户报告的两种查询剥离，同时保留周围文本和普通 SGR。
  - `TestStripTerminalQueriesPreservesResponses`：7 种响应（DA1/DA2/DECRQM/DSR）全部不被触动。
  - `TestStripTerminalQueriesCoversCommonFamilies`：11 种查询变体全部剥干净。
  - `TestStripTerminalQueriesEmpty` / `PlainTextUntouched`：空输入 + "c"、"$p"、">q"、".c 文件名" 等类似字节在纯文本里保持原样。
  - `TestReadHistoryStripsTerminalQueriesFromReplay`：5 次 prompt 的模拟日志过 `ReadHistory` 后探测序列全被剥。
- **为什么原测试没覆盖**：先前的 TUI anchor 测试样本只考虑了 1049h + CSI 重绘，没覆盖"shell 在 idle prompt 发探测序列"这种场景；回放对 xterm.js **再触发响应**这条副作用路径（PTY ← WS ← xterm.js）需要理解客户端行为才能想到，光看服务端代码看不出来。协议层测试样本不光要覆盖 TUI/non-TUI，还要区分"纯显示字节"和"期望终端回应的查询字节"。
- **紧急程度**：高（每次重连都污染用户光标位置，交互直接被阻断）
- **衍生改进建议**（下次处理）：
  1. 还有 OSC 色值查询（`\x1b]10;?\x07` 等）、Kitty keyboard protocol 查询（`\x1b[?u`）等更新一些的终端协议也会在回放时重触发响应；未来出现再扩。
  2. 长期真正的解法还是 server-side headless 解析（维护 screen matrix，回放送屏幕快照 + 增量），能一次性绕开所有"回放激活副作用"的家族问题。已在上条 learning 里标记过。
  3. xterm.js 侧如果能加"回放模式"API（不答查询），客户端配合更干净；目前 xterm.js 没提供，server-side 过滤是唯一选项。

### Bug 修复：Claude Code 大 history 下实时输出被截断

- **发现于**：用户观察（"在 claude code 运行过程中会截断在运行的命令处"），在上一个 TUI anchor 修复（5f98df6）合入后暴露。
- **现象**：Claude Code 正在跑（alt-screen 内持续重绘），用户重连 WS，回放结束后屏幕停在"某条命令正在运行"的一帧，之后的实时字节再不刷新；再过几秒才恢复正常。
- **根因**：`ws/handler.go` 按「`ReadHistory` → 发送 history → `AddListener`」的顺序推进。`ReadHistory` 之后到 `AddListener` 之间的字节：(1) 已经被 pumpOutput 在 `t.mu` 保护下写进日志文件（但我们已完成文件快照）；(2) 广播时 listener 还没注册，根本没被塞给新连接。旧代码 256KB history 时这个窗口是毫秒级、几乎不可感知；TUI anchor 修复把 history 放大到可达 10MB 后，窗口拉到数百毫秒，正好覆盖 Claude Code 跑一个命令的活跃输出期，于是"截断在运行的命令处"。
- **修复**：
  - 新增 `Manager.SubscribeWithSnapshot`：在 `t.mu` 保护下**原子地**做两件事——`os.Stat` 日志文件拿到 `snapSize`、注册 listener。因为 pumpOutput 也在 `t.mu` 下写文件 + 遍历 listener，所以任何一次 pumpOutput 写要么"整体发生在我们 Lock 之前"（bytes 已在 snapSize 内 → 被 history 读到）、要么"整体发生在我们 Unlock 之后"（listener 已可见 → 走广播），不可能夹在中间丢。
  - 把 `readHistoryFile` 里 "stat 再读 tail" 拆成 `readHistoryTailFromFile(f, upTo, lineLimit)`，让 `SubscribeWithSnapshot` 用 `snapSize` 严格封顶。新增字节进不来 history，也就不会和 listener 重复。
  - `ws/handler.go` 改走 `SubscribeWithSnapshot`，并加一条 `pending` 队列：从订阅成功到 history 发完这段时间的 listener 回调先进队列，发完 history 后按序 drain，再把 `historySent` 翻成 true 让后续回调直通。这保证线格式顺序：history → drain buffer → 直通。
- **回归测试**：`backend/internal/container/subscribe_test.go`
  - `TestSubscribeWithSnapshotClosesTheRace`：种 HISTORY_BYTES，订阅后用 `simulatePumpOutput`（严格按 pumpOutput 的 lock 顺序）追加 LIVE_BYTES_A/B；断言 history 包含前者、listener 收到后者、不出现重复。
  - `TestSubscribeWithSnapshotUnknownContainer/Terminal`：未知 id 返回明确 error，不泄漏 listener。
  - `TestSubscribeWithSnapshotFreshLogNoContent`：全新终端（日志文件还不存在）订阅能正常返回空 history + 可用 listener，之后的写入能到达 listener。
- **为什么原测试没覆盖**：`ws/handler.go` 从来没有自动化测试（需要 WebSocket + 真 PTY 才能端到端）；Manager 层的并发契约（"pumpOutput 与订阅方必须在同一把锁下协调 snapshot 和 listener 注册"）也没有显式断言。Manager 与 ws/handler 之间一直是"隐式契约"，一方变更没法被另一方的测试拦截。这类横跨模块的并发不变量，必须有一层专门的 contract test（像本次的 `simulatePumpOutput` 就是这种角色）。
- **紧急程度**：高（上个 TUI fix 把偶发小 race 变成每次 Claude Code 重连必现的数据丢失）
- **衍生改进建议**（下次处理）：
  1. `Listener.Send` 仍然是 lossy 的（channel cap 64，满了默认丢）；若再叠上 WS 客户端慢，pending drain 仍可能被截断。应把 `Listener` 改成带容量保护的阻塞/背压模型，或为 WS listener 单独加一条大缓冲。
  2. `Manager.ReadHistory` 现在只剩"非连接态终端送 history"一个调用点，将来可以并进 `SubscribeWithSnapshot`（对 disconnected 返回 listener=nil）减少 API 表面。
  3. `ws/handler.go` 目前没有单测，这次的隐性契约破坏本应被端到端测试发现——未来应引入一个基于 `httptest` + WebSocket 的集成测试框架，至少覆盖 reconnect / disconnect / 大 history 场景。

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
### Bug 修复：TUI (Claude Code / vim) 重连后只剩几十行

- **发现于**：用户手动观察（"有时候只恢复了不到 50 行"）
- **现象**：在终端里跑 Claude Code / vim 等 TUI 时，reopen 后 xterm.js 只显示半截屏幕（20-50 行），滚动历史基本没了。非 TUI 场景（纯 bash）不受影响。
- **根因**：TUI 用 `\x1b[?1049h` 切到备用屏，之后用 cursor-addressable 重绘输出大量 ANSI 字节但极少 `\n`。旧的 `readHistoryFile` 只取最后 256KB + 按行数截断——TUI 输出很容易让 256KB 窗口落在 `\x1b[?1049h` 之后，replay 时 xterm.js 看不到"进备用屏"的开关，落在主屏回放残余 ANSI → 屏幕错乱。同时字节窗口切在半截 ANSI 序列里也会让 xterm.js 解析失败。
- **修复**：三处改动
  1. `readHistoryFile` 改成"anchor-aware"：在 8MB 窗口里找最近一个 `\x1b[?{47,1047,1049}{h,l}` 开关，如有就从开关处开始返回；没有再走字节/行数兜底。字节兜底从 256KB 抬到 1MB。硬上限 10MB 防跑飞。
  2. WS handler 回放前加 `\x1bc` (RIS)，让 xterm.js 从零态开始处理 anchor 之后的字节。
  3. 把 `findLastAltScreenAnchor` / `trimToLastLines` 抽成纯函数方便单测。
- **回归测试**：
  - `TestReadHistoryAnchorsOnAltScreenToggle`：550KB 含 1049h 的 log，anchor 丢失前失败（历史里 `[?1049h` 不见），修复后通过。
  - `TestReadHistoryAnchorsOnLatestToggle`：1049h + 1049l 交错时 anchor 取最后一个（1049l），丢掉早期的 1049h，修复前失败。
  - `TestReadHistoryNoToggleUsesByteCap`：无 anchor 时按 1MB 字节上限返回。
  - 5 个纯函数测试：anchor 多变体 / 无匹配 / 行数 under/at/zero limit。
  - 原有 `TestReadHistoryDefaultTerminalLimit` / `Extra` / `CapsByBytes` 三个回归仍通过。
- **为什么原测试没覆盖**：原异常清单只有"很多行"和"很多字节"两种样本，没有"ANSI heavy、无 newline、有 alt-screen 切换"这种 TUI 场景。对终端类产物测试要有"协议层"的样本（至少覆盖 alt-screen toggle、CSI 序列、可能切在转义序列中间）。
- **紧急程度**：中（TUI 用户的核心体验被破坏）
- **衍生改进建议**（下次处理）：
  1. Anchor 之后的字节如果还是切在半截 CSI 序列里（anchor 之前的部分截掉），仍可能让 xterm.js 吃到残骸。可在 anchor 前再吃 1 字节是否 `\x1b`，做"序列对齐"。
  2. 10MB 硬上限对重度 TUI 用户可能还是紧。长期应该做"按屏快照"（xterm-headless 解析到 screen matrix，存快照 + 增量）而不是全字节回放。
  3. `ReadHistory` 目前只有 `ws/handler.go` 一个调用点——如果后续有"导出历史"之类的功能要读原始字节而不要 anchor/reset，要分两套 API。

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
