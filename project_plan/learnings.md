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
