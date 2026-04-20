# Learnings

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

- **类型**：重构机会
- **描述**：`App.tsx` 已经在 600+ 行规模，里面掺入了拖拽 + 分页 + 键盘快捷键 + 单视图模式 + 通知 WS 等多个关切点。这次又塞进了 pointermove 监听 + dwell 计时器 + 边缘标记 UI，可读性继续下降。
- **建议处理方式**：后续把 "desktop multi-view"、"single-view"、"drag + page flip" 拆成独立 hooks（如 `useDragEdgePageFlip`）。
- **紧急程度**：中

- **类型**：测试缺口
- **描述**：`detectEdgeZone` 已补纯函数测试，但 dwell 计时器 + pointermove + 边缘标记的集成行为未覆盖（需要 jsdom + fake timers 模拟 pointer 事件）。当前仅靠手动 UI 验证。
- **建议处理方式**：等交互确定后再补集成测试，避免反复重写。
- **紧急程度**：低
