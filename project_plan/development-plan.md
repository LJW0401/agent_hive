# Agent Hive 开发方案 — 文件浏览模式 & 终端更新

> 创建日期：2026-04-16
> 状态：已审核
> 版本：v1.1
> 关联需求文档：project_plan/requirements.md

## 1. 技术概述

### 1.1 技术架构

```
┌─────────────────────────────────────────────────────────┐
│                     Browser (React)                      │
│  ┌──────────────┐  ┌──────────────┐  ┌───────────────┐  │
│  │ Todo+Terminal │⇄│ File Browser  │  │  Mode Switch  │  │
│  │   (现有)      │  │   (新增)      │  │  (标题栏按钮)  │  │
│  └──────────────┘  └──────┬───────┘  └───────────────┘  │
│                           │                              │
│  ┌────────────────────────┼──────────────────────────┐   │
│  │  FileTree  │  Splitter │  FilePreview             │   │
│  │  (懒加载)   │  (可拖拽)  │  Shiki / Markdown / Img  │   │
│  └────────────┴───────────┴──────────────────────────┘   │
└───────────────────────────┬──────────────────────────────┘
                            │ REST API
┌───────────────────────────┼──────────────────────────────┐
│                     Go Backend                            │
│  ┌─────────────────┐  ┌───┴───────────────────────────┐  │
│  │ Container/Terminal│  │ File API (新增)               │  │
│  │ Manager (现有)    │  │ GET /cwd                     │  │
│  │                  │  │ GET /files?path=              │  │
│  │ PTY Session ─────┼──│ GET /files/content?path=     │  │
│  │  └─ PID() ──────┼──│  └─ /proc/{pid}/cwd          │  │
│  └─────────────────┘  └───────────────────────────────┘  │
│                                                          │
│  安全层：路径遍历防护，二进制检测，大文件截断              │
└──────────────────────────────────────────────────────────┘
```

### 1.2 技术栈

| 类别 | 选择 | 说明 |
|------|------|------|
| 后端 | Go | 现有技术栈 |
| 前端 | React + TypeScript | 现有技术栈 |
| 语法高亮 | Shiki | VS Code 同源高亮质量，异步加载 |
| Markdown 渲染 | react-markdown + remark-gfm | GFM 表格、复选框支持 |
| Markdown 排版 | @tailwindcss/typography | prose 类一键美化 |
| PDF 预览 | react-pdf | 基于 pdf.js 的 React 封装 |
| E2E 测试 | Playwright | 浏览器自动化测试 |

### 1.3 项目结构（新增部分）

```
backend/internal/
├── server/
│   └── server.go          # +文件 API 路由和处理函数
├── container/
│   └── manager.go         # +GetCWD 方法
└── fileutil/
    └── fileutil.go        # 新增：路径安全、二进制检测、文件截断

frontend/src/
├── api.ts                 # +文件 API 调用函数
├── components/
│   ├── FileTree.tsx        # 新增：文件树组件
│   ├── FilePreview.tsx     # 新增：文件内容预览组件
│   ├── FileBrowser.tsx     # 新增：桌面端文件浏览布局（树+内容+分割线）
│   ├── MobileFileBrowser.tsx # 新增：手机端文件浏览（全屏导航）
│   ├── ProjectContainer.tsx  # +模式切换
│   ├── SingleProjectView.tsx # +模式切换
│   └── MobileProjectView.tsx # +模式切换
└── hooks/
    └── useFileBrowser.ts   # 新增：文件浏览状态管理 hook

frontend/
├── e2e/                    # 新增：Playwright 测试目录
│   ├── file-browser.spec.ts
│   └── ...
└── playwright.config.ts    # 新增：Playwright 配置
```

### 1.4 安全门控命令

| 检查项 | 命令 |
|--------|------|
| 后端编译 | `cd backend && PATH="/snap/go/current/bin:$PATH" go build ./...` |
| 后端测试 | `cd backend && PATH="/snap/go/current/bin:$PATH" go test ./...` |
| 前端类型检查 | `cd frontend && npx tsc --noEmit` |
| 前端测试 | `cd frontend && npx vitest run` |
| E2E 冒烟测试 | `cd frontend && npx playwright test` |

## 2. 开发阶段

### 阶段 1：终端功能更新

**目标**：修复终端关闭 Bug，移除终端数量上限

**涉及的需求项**：
- 2.1.1 关闭额外终端后未跳转到默认终端
- 2.2.1 取消终端数量上限

**阶段状态**：已完成

##### WI-1.1 [S] Bug 修复 — 关闭终端后自动跳转默认终端
- **描述**：在 `useTerminalTabs` 添加 `useEffect` 守卫，当 `activeTerminalId` 指向不存在的终端时自动回退到默认终端
- **验收标准**：
  1. 关闭当前活动的额外终端后，自动切换到 Agent 标签
  2. 安全门控：`cd frontend && npx vitest run` 通过
- **Notes**：
  - Pattern：useEffect 守卫模式，监听 terminals + activeTerminalId 变化
  - Reference：`frontend/src/hooks/useTerminalTabs.ts`
  - Hook point：所有使用 useTerminalTabs 的组件自动受益

##### WI-1.2 [S] 移除终端数量上限
- **描述**：移除后端 `CreateTerminal` 的 5 个终端限制检查，移除前端 `maxTerminals` prop，"+" 按钮始终可见
- **验收标准**：
  1. 可创建超过 5 个终端，"+" 按钮始终显示
  2. 安全门控：`cd backend && PATH="/snap/go/current/bin:$PATH" go build ./...` + `cd frontend && npx vitest run` 通过
- **Notes**：
  - Reference：`backend/internal/container/manager.go`、`frontend/src/components/TerminalTabBar.tsx`
  - Hook point：`ErrTerminalLimit` 从 server.go 错误处理中移除

##### WI-1.3 [集成门控] 终端功能验证
- **描述**：验证 WI-1.1 ~ WI-1.2 的集成状态
- **验收标准**：
  1. `cd backend && PATH="/snap/go/current/bin:$PATH" go build ./...` 通过
  2. `cd frontend && npx vitest run` 通过
  3. 手动验证：创建多个终端 → 关闭所有额外终端 → 自动跳到 Agent

**阶段验收标准**：
1. Given 用户创建 6+ 个终端，When 查看标签栏，Then "+" 按钮始终可见，标签栏可横向滚动
2. Given 用户关闭所有额外终端，When 最后一个额外终端关闭，Then 自动切换到 Agent 标签
3. 所有安全门控通过

---

### 阶段 2：后端文件 API + 基础设施

**目标**：实现文件系统读取 API，搭建 E2E 测试基础设施

**阶段状态**：已完成

**涉及的需求项**：
- 2.3.1 模式切换（后端部分：CWD 获取）
- 2.3.2 文件树（后端部分：目录列表）
- 2.3.3~2.3.8 文件内容预览（后端部分：文件读取、截断、二进制检测）

##### WI-2.1 [S] 文件工具包 — 路径安全 + 二进制检测 + 文件截断
- **描述**：新建 `backend/internal/fileutil/fileutil.go`，实现：
  - `SafeJoin(base, rel) (string, error)` — 路径拼接+遍历防护（Clean → 检查 `..` 越界 → EvalSymlinks 后二次验证仍在 base 内）
  - `IsBinary(path) bool` — 读取前 512 字节检测是否包含 NUL 字节
  - `ReadTailLines(path, maxLines) (string, truncated bool, error)` — 读取文件最后 N 行
  - `FileType(name) string` — 根据扩展名返回类型（text/markdown/image/pdf/binary）
- **验收标准**：
  1. `SafeJoin("/base", "../etc/passwd")` 返回错误
  2. `SafeJoin("/base", "sub/file.go")` 返回 `/base/sub/file.go`
  3. 二进制文件检测正确识别 ELF/ZIP 等格式
  4. `ReadTailLines` 对超限文件只返回最后 N 行
  5. 安全门控：`go test ./internal/fileutil/...` 通过
- **Notes**：
  - Pattern：独立工具包，无外部依赖
  - Reference：类似 `filepath.Clean` + `strings.Contains` 检查
  - Hook point：被 server.go 文件处理函数调用

##### WI-2.2 [S] 后端测试 — fileutil 单元测试
- **描述**：为 fileutil 包编写单元测试，覆盖路径遍历攻击、二进制检测、大文件截断等边界情况
- **验收标准**：
  1. 路径遍历测试：`..`、符号链接、绝对路径注入
  2. 二进制检测：文本文件、二进制文件、空文件
  3. 截断测试：小于限制、等于限制、超过限制
  4. 安全门控：`go test ./internal/fileutil/...` 通过
- **Notes**：
  - Pattern：表驱动测试（table-driven tests）
  - Reference：`backend/internal/store/terminal_test.go` 作为测试风格参考

##### WI-2.3 [M] CWD + 目录列表 + 文件内容 API
- **描述**：在 server.go 中新增三个 API 端点：
  - `GET /api/containers/{id}/cwd` — 通过 `/proc/{pid}/cwd` 读取 Agent 终端的 CWD
  - `GET /api/containers/{id}/files?path=.` — 列出指定目录的内容（单层），返回 `[{name, type, size}]`，目录排在文件前面
  - `GET /api/containers/{id}/files/content?path=xxx` — 读取文件内容：
    - 文本/代码：返回 `{type: "text", content: "...", truncated: bool, language: "go"}`
    - Markdown：返回 `{type: "markdown", content: "..."}`
    - 图片：返回 `{type: "image", content: "base64...", mimeType: "image/png"}`
    - PDF：返回 `{type: "pdf", content: "base64..."}`
    - 二进制：返回 `{type: "binary"}`
  - Manager 新增 `GetCWD(containerID) (string, error)` 方法
- **验收标准**：
  1. CWD API 返回 Agent 终端的当前工作目录
  2. 目录列表返回正确的文件和子目录信息
  3. 路径参数经过 SafeJoin 安全检查
  4. 大文件自动截断，返回 truncated=true
  5. 二进制文件返回 type="binary"，不返回内容
  6. 安全门控：`cd backend && PATH="/snap/go/current/bin:$PATH" go build ./...` + `cd backend && PATH="/snap/go/current/bin:$PATH" go test ./...` 通过
- **Notes**：
  - Pattern：与 terminal API 相同的路由解析模式（SplitN + HasSuffix）
  - Reference：`server.go` 中 `listTerminals`/`createTerminalHandler` 作为模板
  - Hook point：`Manager.GetCWD` 调用 `Terminal.ProcessPID()` → 读取 `/proc/{pid}/cwd`

##### WI-2.4 [S] 后端测试 — 文件 API 测试
- **描述**：为文件 API 编写测试，使用临时目录作为测试文件系统
- **验收标准**：
  1. CWD 获取测试（mock /proc 或集成测试）
  2. 目录列表：空目录、含文件目录、隐藏文件
  3. 文件内容：文本、markdown、二进制、大文件截断
  4. 路径遍历尝试返回 400 错误
  5. 安全门控：`cd backend && PATH="/snap/go/current/bin:$PATH" go test ./...` 通过
- **Notes**：
  - Pattern：httptest.NewServer + 临时目录
  - Reference：`backend/internal/server/server_test.go`
  - Hook point：验证 fileutil + server handler 集成正确性
- **Notes**：
  - Pattern：httptest.NewServer + 临时目录
  - Reference：`backend/internal/server/server_test.go`

##### WI-2.5 [集成门控] 后端文件 API 集成验证
- **描述**：验证 WI-2.1 ~ WI-2.4 的集成状态
- **验收标准**：
  1. `cd backend && PATH="/snap/go/current/bin:$PATH" go build ./...` 通过
  2. `cd backend && PATH="/snap/go/current/bin:$PATH" go test ./...` 通过
  3. 手动 curl 验证：CWD → 目录列表 → 文件内容 完整链路

##### WI-2.6 [M] Playwright 基础设施搭建
- **描述**：在 frontend 目录下安装和配置 Playwright：
  - `npm install -D @playwright/test`
  - 创建 `playwright.config.ts`（baseURL 指向 localhost:8090）
  - 创建 `e2e/` 目录 + 示例冒烟测试（验证首页加载）
  - 在 package.json 中添加 `test:e2e` 脚本
- **验收标准**：
  1. `npx playwright test` 能正常运行（需要后端服务启动）
  2. 示例测试验证页面标题或容器列表加载
  3. Playwright 配置包含超时、重试、截图等基本设置
  4. 安全门控：`cd frontend && npx playwright test` 通过
- **Notes**：
  - Pattern：Playwright 标准项目结构
  - Reference：https://playwright.dev/docs/intro
  - Hook point：后续阶段的 E2E 测试基于此基础设施

**阶段验收标准**：
1. Given 后端运行中，When `curl /api/containers/{id}/cwd`，Then 返回 Agent 终端的当前工作目录
2. Given 后端运行中，When `curl /api/containers/{id}/files?path=.`，Then 返回目录内容列表
3. Given 路径含 `..`，When 请求文件 API，Then 返回 400 错误
4. Given 二进制文件，When 请求文件内容，Then 返回 `{type: "binary"}`
5. 所有后端测试通过，Playwright 示例测试通过

---

### 阶段 3：前端文件浏览（桌面端）

**目标**：实现桌面端完整的文件浏览体验

**阶段状态**：已完成

**涉及的需求项**：
- 2.3.1 模式切换（前端部分）
- 2.3.2 文件树
- 2.3.3 代码语法高亮
- 2.3.4 Markdown 渲染
- 2.3.5 图片预览
- 2.3.6 PDF 预览
- 2.3.7 二进制文件提示
- 2.3.8 大文件截断
- 2.3.9 桌面端布局

##### WI-3.1 [S] 前端依赖安装 + API 函数
- **描述**：
  - 安装依赖：`shiki`, `react-markdown`, `remark-gfm`, `@tailwindcss/typography`, `react-pdf`
  - 在 `api.ts` 中添加文件 API 调用函数：`getCWD`, `listFiles`, `getFileContent`
  - 定义 TypeScript 类型：`FileEntry`, `FileContent`
- **验收标准**：
  1. 依赖安装成功，无版本冲突
  2. API 函数类型正确
  3. 安全门控：`npx tsc --noEmit` 通过
- **Notes**：
  - Reference：`api.ts` 中现有函数模式（authHeaders、错误处理）
  - Hook point：被 useFileBrowser hook 和组件使用

##### WI-3.2 [M] 文件树组件
- **描述**：实现 `FileTree.tsx`：
  - 懒加载：展开目录时调用 `listFiles` API 获取子内容
  - 展开/折叠：点击目录切换展开状态
  - 现代化风格：文件/目录图标（lucide-react）、缩进层级线、选中高亮
  - 目录排在文件前面，按名称排序
  - 加载中状态：展开目录时显示 loading indicator
- **验收标准**：
  1. 目录点击展开/折叠正常
  2. 文件点击触发 `onSelect(path)` 回调
  3. 嵌套目录正确缩进
  4. 文件/目录有对应图标
  5. 安全门控：`npx tsc --noEmit` 通过
- **Notes**：
  - Pattern：递归组件，每个节点管理自己的 children 状态
  - Reference：lucide-react 图标（Folder, FolderOpen, File, FileText 等）
  - Hook point：被 FileBrowser 和 MobileFileBrowser 使用

##### WI-3.3 [S] 文件树搜索/过滤
- **描述**：在文件树顶部添加搜索输入框：
  - 输入关键字实时过滤当前已加载的文件/目录
  - 匹配项高亮显示
  - 清空搜索恢复完整树
- **验收标准**：
  1. 输入关键字后，只显示名称匹配的文件/目录及其父目录链
  2. 匹配文本高亮
  3. 清空输入恢复完整树
  4. 安全门控：`npx tsc --noEmit` 通过
- **Notes**：
  - Pattern：受控输入 + 过滤逻辑
  - Reference：FileTree 组件内部状态

##### WI-3.4 [S] 前端测试 — 文件树组件
- **描述**：为 FileTree 编写 Vitest 测试（mock API 调用）
- **验收标准**：
  1. 渲染目录和文件节点
  2. 点击目录触发展开（验证 API 调用）
  3. 搜索过滤正确
  4. 安全门控：`cd frontend && npx vitest run` 通过
- **Notes**：
  - Pattern：vi.mock api 模块，验证异步展开和过滤逻辑
  - Reference：`frontend/src/components/__tests__/TerminalTabBar.test.tsx` 作为测试风格参考
  - Hook point：确保文件树组件独立于后端可测试

##### WI-3.5 [集成门控] 文件树集成验证
- **描述**：验证 WI-3.1 ~ WI-3.4 的集成状态
- **验收标准**：
  1. `cd frontend && npx tsc --noEmit` 通过
  2. `cd frontend && npx vitest run` 通过
  3. API 类型与后端响应格式匹配

##### WI-3.6 [M] 文件内容预览 — 代码语法高亮 (Shiki)
- **描述**：实现 `FilePreview.tsx` 的代码预览部分：
  - Shiki 异步加载（`createHighlighter`），使用暗色主题（如 `github-dark`）
  - 根据文件扩展名选择语言
  - 显示行号
  - 大文件截断提示（顶部 banner）
  - 加载中状态（Shiki 初始化 + 文件内容加载）
- **验收标准**：
  1. `.go`, `.ts`, `.py` 等文件正确高亮
  2. 行号显示
  3. 截断文件顶部显示提示信息
  4. Shiki 加载完成前显示 loading 状态
  5. 安全门控：`npx tsc --noEmit` 通过
- **Notes**：
  - Pattern：useEffect + useState 异步初始化 Shiki highlighter，全局单例
  - Reference：Shiki docs — `createHighlighter({ themes, langs })`
  - Hook point：FilePreview 根据 FileContent.type 分发到不同渲染器

##### WI-3.7 [M] 文件内容预览 — Markdown + 图片 + PDF + 二进制
- **描述**：在 `FilePreview.tsx` 中实现剩余预览类型：
  - Markdown：react-markdown + remark-gfm，prose 排版，暗色主题适配
  - 图片：`<img>` 标签，居中显示，最大宽度限制
  - PDF：react-pdf `<Document>` + `<Page>` 组件，支持翻页
  - 二进制：居中提示"不支持预览此文件类型"
  - 无文件选中：空状态提示"选择文件以预览"
- **验收标准**：
  1. Markdown 表格和复选框正确渲染
  2. 图片正常显示
  3. PDF 正常显示，可翻页
  4. 二进制文件显示不支持提示
  5. 安全门控：`npx tsc --noEmit` 通过
- **Notes**：
  - Pattern：条件渲染 switch on content.type
  - Reference：react-markdown, react-pdf 官方文档
  - Hook point：@tailwindcss/typography 的 `prose prose-invert` 类用于 Markdown

##### WI-3.8 [S] 前端测试 — 文件预览组件
- **描述**：为 FilePreview 编写 Vitest 测试（mock Shiki、react-pdf）
- **验收标准**：
  1. 代码文件渲染高亮内容
  2. Markdown 渲染为富文本
  3. 二进制文件显示提示
  4. 截断文件显示提示 banner
  5. 安全门控：`cd frontend && npx vitest run` 通过
- **Notes**：
  - Pattern：vi.mock shiki 和 react-pdf 模块，验证条件渲染分支
  - Reference：`frontend/src/components/__tests__/TerminalTabBar.test.tsx` 作为测试风格参考
  - Hook point：确保各预览类型的渲染路径正确

##### WI-3.9 [集成门控] 文件预览集成验证
- **描述**：验证 WI-3.6 ~ WI-3.8 的集成状态
- **验收标准**：
  1. `cd frontend && npx tsc --noEmit` 通过
  2. `cd frontend && npx vitest run` 通过

##### WI-3.10 [M] 桌面端文件浏览布局 + 模式切换
- **描述**：
  - 实现 `FileBrowser.tsx`：左侧 FileTree + 可拖拽分割线 + 右侧 FilePreview
  - 实现 `useFileBrowser.ts` hook：管理 rootPath、selectedFile、fileContent 状态，内存缓存已加载的文件内容（避免重复请求同一文件）
  - 在 ProjectContainer 和 SingleProjectView 的标题栏添加模式切换按钮（图标：FolderOpen ↔ Terminal）
  - 切换时调用 getCWD API 设置文件树根路径
  - 模式切换不销毁终端组件（display: none 隐藏）
- **验收标准**：
  1. 标题栏显示模式切换按钮
  2. 点击切换按钮在 Todo+终端 和 文件浏览 间切换
  3. 文件浏览模式：左侧文件树 + 右侧内容，可拖拽调整比例
  4. 切回终端模式时，终端状态完整保留
  5. 多项目和单项目布局均正常
  6. 安全门控：`npx tsc --noEmit` + `npx vitest run` 通过
- **Notes**：
  - Pattern：与现有 TodoList+Terminal 分割线相同的拖拽模式
  - Reference：`ProjectContainer.tsx` 中的 Splitter 实现
  - Hook point：容器组件通过 viewMode state 控制显示哪个视图

##### WI-3.11 [S] 前端测试 — 模式切换与文件浏览布局
- **描述**：为模式切换和 FileBrowser 布局编写 Vitest 测试（mock API 调用）
- **验收标准**：
  1. 切换按钮点击后视图模式变化
  2. FileBrowser 渲染文件树和预览区域
  3. 切回终端模式后终端组件仍在 DOM 中（display: none）
  4. 安全门控：`cd frontend && npx vitest run` 通过
- **Notes**：
  - Pattern：vi.mock api 模块，验证 viewMode 状态切换
  - Reference：`ProjectContainer.tsx`、`SingleProjectView.tsx`
  - Hook point：验证模式切换不破坏终端连接

##### WI-3.12 [集成门控] 桌面端文件浏览完整验证
- **描述**：验证阶段 3 的完整集成状态
- **验收标准**：
  1. 所有安全门控命令通过
  2. 启动开发服务器，手动验证完整流程：切换模式 → 文件树加载 → 展开目录 → 点击文件 → 内容预览（代码/Markdown/图片）
  3. 多项目模式和单项目模式均正常

**阶段验收标准**：
1. Given 桌面端容器处于 Todo+终端 模式，When 点击标题栏切换按钮，Then 切换到文件浏览模式，文件树加载当前工作目录
2. Given 文件浏览模式中，When 展开目录并点击 `.go` 文件，Then 右侧显示语法高亮的代码
3. Given 文件浏览模式中，When 点击 `README.md`，Then 右侧渲染 Markdown（含表格和复选框）
4. Given 文件浏览模式中，When 点击图片文件，Then 右侧显示图片
5. Given 文件浏览模式中，When 在搜索框输入关键字，Then 文件树过滤显示匹配项
6. Given 文件浏览模式中，When 切回 Todo+终端，Then 终端连接和状态完整保留
7. 所有安全门控和集成门控通过

---

### 阶段 4：手机端适配 + E2E 测试

**目标**：实现手机端文件浏览交互，编写 E2E 冒烟测试

**涉及的需求项**：
- 2.3.10 手机端交互

##### WI-4.1 [M] 手机端文件浏览组件
- **描述**：实现 `MobileFileBrowser.tsx`：
  - 两个全屏视图：文件树视图 + 文件内容视图
  - 文件树视图：全屏显示文件树 + 搜索框，点击文件进入内容视图
  - 文件内容视图：顶部标题栏（返回按钮 + 文件名），下方显示 FilePreview
  - 视图切换动画（左右滑动）
- **验收标准**：
  1. 切换到文件浏览模式后显示全屏文件树
  2. 点击文件进入全屏内容视图
  3. 返回按钮回到文件树
  4. 文件树搜索在手机端正常工作
  5. 安全门控：`npx tsc --noEmit` 通过
- **Notes**：
  - Pattern：内部 state 控制 view（'tree' | 'preview'）
  - Reference：MobileProjectView.tsx 的导航模式
  - Hook point：在 MobileProjectView 中通过模式切换按钮集成

##### WI-4.2 [S] 手机端模式切换集成
- **描述**：在 MobileProjectView 标题栏添加模式切换按钮，集成 MobileFileBrowser
- **验收标准**：
  1. 手机端标题栏显示模式切换按钮
  2. 切换到文件浏览模式正常工作
  3. 切回终端模式时终端状态保留
  4. 安全门控：`npx tsc --noEmit` + `npx vitest run` 通过
- **Notes**：
  - Reference：`MobileProjectView.tsx`
  - Hook point：viewMode state 控制显示 Terminal+Todo 还是 MobileFileBrowser

##### WI-4.3 [集成门控] 手机端集成验证
- **描述**：验证 WI-4.1 ~ WI-4.2 的集成状态
- **验收标准**：
  1. 所有安全门控通过
  2. 浏览器移动端模拟下验证完整流程

##### WI-4.4 [M] E2E 冒烟测试
- **描述**：编写 Playwright E2E 测试覆盖核心路径：
  - 测试 1：桌面端模式切换 — 点击切换按钮 → 文件树出现 → 切回终端
  - 测试 2：文件浏览完整流程 — 切换模式 → 展开目录 → 点击文件 → 内容显示
  - 测试 3：手机端文件浏览 — 切换模式 → 点击文件 → 内容显示 → 返回
  - 测试 4：终端标签关闭回退 — 创建终端 → 关闭 → 回到 Agent
- **验收标准**：
  1. 所有 E2E 测试通过
  2. 安全门控：`cd frontend && npx playwright test` 通过
- **Notes**：
  - Pattern：Page Object Model 或直接 locator 方式
  - Reference：`frontend/e2e/` 目录（阶段 2 搭建）

##### WI-4.5 [集成门控] 全项目最终验证
- **描述**：全量安全门控 + E2E 测试
- **验收标准**：
  1. `cd backend && PATH="/snap/go/current/bin:$PATH" go build ./...` 通过
  2. `cd backend && PATH="/snap/go/current/bin:$PATH" go test ./...` 通过
  3. `cd frontend && npx tsc --noEmit` 通过
  4. `cd frontend && npx vitest run` 通过
  5. `cd frontend && npx playwright test` 通过

**阶段验收标准**：
1. Given 手机端容器，When 点击模式切换，Then 显示全屏文件树
2. Given 手机端文件树，When 点击文件，Then 进入全屏内容视图，顶部有返回按钮
3. Given 手机端内容视图，When 点击返回，Then 回到文件树
4. 所有 E2E 冒烟测试通过
5. 所有安全门控通过

---

## 3. 风险与应对

| 风险 | 影响 | 概率 | 应对措施 |
|------|------|------|----------|
| Shiki 体积过大影响首屏加载 | 中 | 中 | 异步加载，仅在进入文件浏览模式时初始化；按需加载语言包 |
| 大目录（node_modules）展开慢 | 中 | 高 | 后端单层懒加载，前端展开时显示 loading；后端限制单次返回条目数 |
| /proc/{pid}/cwd 权限问题 | 高 | 低 | 使用与 PTY 相同的用户身份读取；权限不足时返回默认路径 |
| react-pdf worker 配置复杂 | 低 | 中 | 使用 CDN worker 或内联 worker |
| E2E 测试环境不稳定 | 低 | 中 | 设置合理的超时和重试策略 |
| 符号链接绕过路径遍历防护 | 高 | 低 | SafeJoin 中 EvalSymlinks 后二次验证结果路径仍在 base 目录内 |
| 并发读写冲突（Agent 写入中读取文件） | 低 | 中 | 读取失败时优雅降级，显示"文件读取失败"提示而非崩溃 |

## 4. 开发规范

### 4.1 代码规范
- 后端 Go 标准格式化（gofmt）
- 前端 TypeScript 严格模式
- 新文件遵循现有项目风格和命名约定

### 4.2 Git 规范
- 分支：`dev/file-browser`
- 提交信息：中文，每阶段完成后提交
- Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>

### 4.3 文档规范
- README 在功能完成后更新
- 代码注释仅在逻辑不自明处添加

## 5. 工作项统计

| 阶段 | S | M | 集成门控 | 总计 |
|------|---|---|---------|------|
| 阶段 1 | 2 | 0 | 1 | 3 |
| 阶段 2 | 3 | 2 | 1 | 6 |
| 阶段 3 | 5 | 3 | 3 | 11 |
| 阶段 4 | 1 | 2 | 2 | 5 |
| **合计** | **11** | **7** | **7** | **25** |

## 审核记录

| 日期 | 审核人 | 评分 | 结果 | 备注 |
|------|--------|------|------|------|
| 2026-04-16 | AI Assistant | 96/100 | 通过 | v1.0→v1.1：补充 WI-3.11 测试、完善 Notes 和安全门控命令、新增符号链接和并发读写风险、添加文件内容缓存策略 |
