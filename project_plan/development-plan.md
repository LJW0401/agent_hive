# Agent Hive 移动端适配 开发方案

> 创建日期：2026-04-14
> 状态：已审核
> 版本：v1.1
> 关联需求文档：project_plan/requirements.md

## 1. 技术概述

### 1.1 技术架构

在现有架构基础上扩展移动端支持：

```
┌─────────────────────────────────┐
│           Browser               │
│  ┌───────────┬────────────────┐ │
│  │ isMobile  │                │ │
│  │   = true  │  = false       │ │
│  │     ▼     │     ▼          │ │
│  │ MobileApp │ App (desktop)  │ │
│  │ (Swiper)  │ (2×2 grid)    │ │
│  └───────────┴────────────────┘ │
│            │  WS + REST         │
└────────────┼────────────────────┘
             ▼
┌─────────────────────────────────┐
│         Go Backend              │
│  /api/layout        → layouts   │
│  /api/mobile-layout → mobile_   │
│                       layouts   │
└─────────────────────────────────┘
```

- 前端通过 User-Agent 判断设备类型，渲染不同的顶层组件
- 后端新增 `mobile_layouts` 表和对应 API，与桌面端 `layouts` 表独立
- 移动端复用现有的容器管理、终端 WebSocket、待办 API

### 1.2 技术栈

| 类别 | 选择 | 说明 |
|------|------|------|
| 滑动导航 | Swiper.js | 成熟的触摸滑动库，`touchAngle: 45` 解决手势冲突 |
| 分割面板 | 手写 touch 事件 | 逻辑简单，不需要引入额外库 |
| 设备检测 | navigator.userAgent | 正则匹配手机 UA 关键词 |
| 其余 | 沿用现有技术栈 | React, TypeScript, Tailwind, xterm.js, Go, SQLite |

### 1.3 项目结构变更

```
frontend/src/
├── App.tsx              # 修改：设备检测 → 路由到 MobileApp 或桌面端
├── MobileApp.tsx        # 新增：移动端主组件（Swiper 容器）
├── utils/
│   └── device.ts        # 新增：isMobile() 工具函数
├── components/
│   ├── MobileProjectView.tsx   # 新增：移动端单项目视图
│   └── ResizableSplitPane.tsx  # 新增：可拖拽分割面板
├── api.ts               # 修改：新增 mobile layout API 函数

backend/internal/
├── store/
│   ├── store.go          # 修改：新增 mobile_layouts 表迁移
│   └── mobile_layout.go  # 新增：mobile layout CRUD
├── server/
│   └── server.go         # 修改：新增 mobile layout API 路由
```

### 1.4 安全门控命令

- 前端类型检查：`cd frontend && npx tsc --noEmit`
- 前端构建检查：`cd frontend && npm run build`
- 后端构建检查：`cd backend && go build ./...`

## 2. 开发阶段

### 阶段 1：基础设施（后端 + 设备检测）

**目标**：搭建移动端布局的后端存储和 API，实现前端设备检测和路由分流

**涉及的需求项**：
- 2.1.1 User-Agent 设备识别
- 2.7.1 手机端布局独立存储（存储层）

#### 工作项列表

##### WI-1.1 [S] 后端：新增 mobile_layouts 表
- **描述**：在 `store.go` 的 `migrate` 函数中添加 `mobile_layouts` 表的建表语句，字段为 `container_id`（主键）、`sort_order`（排序序号）
- **验收标准**：
  1. 服务启动后 SQLite 中存在 `mobile_layouts` 表
  2. 安全门控：`cd backend && go build ./...` 通过
- **Notes**：
  - Pattern：与 `layouts` 表结构类似但更简单，移动端只需一维排序
  - Reference：`backend/internal/store/store.go` migrate 函数
  - Hook point：WI-1.2 将基于此表实现 CRUD

##### WI-1.2 [S] 后端：mobile layout CRUD
- **描述**：新建 `backend/internal/store/mobile_layout.go`，实现 `GetMobileLayout`、`SetMobileLayout`、`AddMobileLayoutEntry`、`RemoveMobileLayoutEntry` 方法
- **验收标准**：
  1. `GetMobileLayout` 返回按 `sort_order` 排序的条目列表
  2. `SetMobileLayout` 原子替换整个布局
  3. `AddMobileLayoutEntry` 在末尾追加条目
  4. `RemoveMobileLayoutEntry` 删除指定容器条目
  5. 安全门控：`cd backend && go build ./...` 通过
- **Notes**：
  - Pattern：参照 `layout.go` 的实现模式
  - Reference：`backend/internal/store/layout.go`
  - Hook point：WI-1.3 的 API 层将调用这些方法

##### WI-1.3 [S] 后端：mobile layout API 路由
- **描述**：在 `server.go` 中新增 `/api/mobile-layout` 路由（GET 获取、PUT 更新），创建/删除容器时同步操作 `mobile_layouts` 表
- **验收标准**：
  1. `GET /api/mobile-layout` 返回移动端布局 JSON
  2. `PUT /api/mobile-layout` 更新移动端布局
  3. 创建容器时自动在 `mobile_layouts` 末尾追加条目
  4. 删除容器时自动移除 `mobile_layouts` 中的条目
  5. 安全门控：`cd backend && go build ./...` 通过
- **Notes**：
  - Pattern：与 `/api/layout` 路由结构一致
  - Reference：`backend/internal/server/server.go` 中的 layout API
  - Hook point：前端 `api.ts` 将调用这些端点

##### WI-1.4 [S] 测试：后端构建与 API 验证
- **描述**：启动服务，用 curl 验证 mobile layout API 的 GET/PUT 功能正常，创建/删除容器后 mobile layout 自动同步
- **验收标准**：
  1. curl 测试 GET/PUT /api/mobile-layout 返回正确数据
  2. 创建容器后 mobile layout 中出现新条目
  3. 删除容器后 mobile layout 中对应条目消失
  4. 安全门控：`cd backend && go build ./...` 通过

##### WI-1.5 [集成门控] 后端 mobile layout 功能完整
- **描述**：验证 WI-1.1 ~ WI-1.4 的集成状态
- **验收标准**：
  1. `cd backend && go build ./...` 通过
  2. mobile_layouts 表结构正确
  3. API 端点可正常读写
  4. 容器增删自动同步 mobile layout

##### WI-1.6 [S] 前端：isMobile() 工具函数
- **描述**：新建 `frontend/src/utils/device.ts`，导出 `isMobile()` 函数，通过正则匹配 `navigator.userAgent` 中的手机关键词（iPhone, Android + Mobile, iPod 等），平板归类为桌面端
- **验收标准**：
  1. iPhone Safari UA 返回 `true`
  2. Android Chrome（手机）UA 返回 `true`
  3. iPad UA 返回 `false`
  4. 桌面浏览器 UA 返回 `false`
  5. 安全门控：`cd frontend && npx tsc --noEmit` 通过
- **Notes**：
  - Pattern：纯函数，无副作用
  - Reference：常见的移动端 UA 检测正则
  - Hook point：App.tsx 中使用决定渲染哪个组件

##### WI-1.7 [S] 前端：App.tsx 设备路由 + MobileApp 占位
- **描述**：修改 `App.tsx`，在顶层调用 `isMobile()` 判断设备类型。如果是手机则渲染 `MobileApp` 组件（暂时显示占位文本"Mobile View"），否则渲染原有桌面端内容
- **验收标准**：
  1. 手机浏览器访问显示 "Mobile View" 占位页面
  2. 桌面浏览器访问显示原有 2×2 网格
  3. 桌面端功能无任何影响
  4. 安全门控：`cd frontend && npx tsc --noEmit` 通过
- **Notes**：
  - Pattern：条件渲染，顶层分流
  - Reference：`frontend/src/App.tsx`
  - Hook point：阶段 2 将填充 MobileApp 的实际内容

##### WI-1.8 [S] 前端：api.ts 新增 mobile layout 函数
- **描述**：在 `api.ts` 中新增 `MobileLayoutEntry` 接口和 `getMobileLayout()`、`updateMobileLayout()` 函数，对接后端 `/api/mobile-layout`
- **验收标准**：
  1. 接口类型定义正确（containerId, sortOrder）
  2. GET/PUT 函数可正常调用
  3. 安全门控：`cd frontend && npx tsc --noEmit` 通过
- **Notes**：
  - Pattern：参照现有 layout API 函数
  - Reference：`frontend/src/api.ts`
  - Hook point：MobileApp 中将使用这些函数

##### WI-1.9 [S] 测试：前端构建检查 + 设备路由验证
- **描述**：验证前端类型检查和构建通过，在手机浏览器（或 DevTools 模拟）中确认路由分流正确
- **验收标准**：
  1. Chrome DevTools 切换手机 UA 后显示移动端占位页面
  2. 桌面 UA 显示原有网格布局
  3. 安全门控：`cd frontend && npx tsc --noEmit` 通过
  4. 安全门控：`cd frontend && npm run build` 通过

##### WI-1.10 [集成门控] 阶段 1 完整验证
- **描述**：验证后端 API + 前端设备路由 + 类型定义的端到端集成
- **验收标准**：
  1. 所有安全门控命令通过
  2. 手机 UA 访问看到占位页面
  3. 桌面 UA 访问功能不受影响
  4. mobile layout API 可正常读写

**阶段验收标准**：
1. Given 用户使用手机浏览器访问, When 页面加载完成, Then 显示移动端占位页面
2. Given 用户使用桌面浏览器访问, When 页面加载完成, Then 显示原有 2×2 网格布局，无功能退化
3. Given 创建一个新容器, When 查询 mobile layout API, Then 返回包含该容器的条目
4. 所有安全门控命令通过

**阶段状态**：已完成

**完成日期**：2026-04-14
**验收结果**：通过
**安全门控**：全部通过
**集成门控**：全部通过
**备注**：后端 mobile_layouts 表和 API 功能正常，前端设备检测路由分流就绪

---

### 阶段 2：移动端单项目视图

**目标**：实现移动端完整的单项目视图，包含终端、待办事项、可拖拽分割线和项目管理操作

**涉及的需求项**：
- 2.2.1 单项目全屏视图
- 2.2.2 终端与待办区域可拖拽分割
- 2.5.1 移动端项目删除与重命名

#### 工作项列表

##### WI-2.1 [M] MobileApp 组件外壳
- **描述**：实现 `MobileApp.tsx`，包含认证流程（复用 LoginPage）、数据加载（containers, mobile layout）、notify WebSocket 连接。暂时只显示第一个项目的名称
- **验收标准**：
  1. 认证流程与桌面端一致（无密码跳过、有密码需登录）
  2. 加载后正确获取容器列表和移动端布局数据
  3. notify WebSocket 连接正常，收到 `containers-changed` 和 `todos-updated` 事件时刷新数据
  4. 安全门控：`cd frontend && npx tsc --noEmit` 通过
- **Notes**：
  - Pattern：参照 App.tsx 的 auth + data loading 逻辑
  - Reference：`frontend/src/App.tsx` 的 useEffect 和 connectNotifyWS
  - Hook point：WI-2.3 将在此组件内渲染项目视图

##### WI-2.2 [S] 测试：MobileApp 加载验证
- **描述**：在手机浏览器中验证 MobileApp 认证和数据加载正常
- **验收标准**：
  1. 手机端可正常登录
  2. 登录后显示项目名称
  3. 在其他设备创建容器后，手机端自动刷新
  4. 安全门控：`cd frontend && npx tsc --noEmit` 通过

##### WI-2.3 [M] MobileProjectView 组件
- **描述**：新建 `MobileProjectView.tsx`，上方渲染 Terminal 组件，下方渲染 TodoList 组件，默认各占 50% 高度。复用现有的 Terminal 和 TodoList 组件
- **验收标准**：
  1. 终端在上，待办在下，占满可用高度
  2. 终端可输入命令、查看输出
  3. 待办可增删改查
  4. 安全门控：`cd frontend && npx tsc --noEmit` 通过
- **Notes**：
  - Pattern：flex 布局，通过百分比分配高度
  - Reference：`frontend/src/components/Terminal.tsx`, `frontend/src/components/TodoList.tsx`
  - Hook point：WI-2.5 将在此组件中加入分割线

##### WI-2.4 [S] 测试：终端和待办在手机端可用
- **描述**：在手机浏览器中验证终端输入输出和待办操作正常
- **验收标准**：
  1. 终端可触摸聚焦并通过虚拟键盘输入
  2. 待办可添加、勾选、删除、拖拽排序
  3. 多设备间终端和待办实时同步
  4. 安全门控：`cd frontend && npx tsc --noEmit` 通过

##### WI-2.5 [集成门控] 移动端基础视图可用
- **描述**：验证 WI-2.1 ~ WI-2.4 的集成状态
- **验收标准**：
  1. 所有安全门控命令通过
  2. 手机端完整认证 → 加载数据 → 显示项目终端和待办
  3. 终端和待办功能正常

##### WI-2.6 [M] ResizableSplitPane 可拖拽分割线
- **描述**：新建 `ResizableSplitPane.tsx`，在终端和待办之间渲染一条可拖拽的分割条。使用 touch 事件实现拖拽，设置最小高度约束（各自一行内容高度，约 30px）。在 MobileProjectView 中使用
- **验收标准**：
  1. 分割线可上下拖拽，终端和待办高度实时跟随
  2. 拖拽到极限位置时保留最小高度（约 30px）
  3. 拖拽松手后区域高度固定不回弹
  4. 切换项目后恢复默认 50/50 比例
  5. 安全门控：`cd frontend && npx tsc --noEmit` 通过
- **Notes**：
  - Pattern：touchstart/touchmove/touchend 事件，计算 deltaY 更新上下区域比例
  - Reference：无外部依赖
  - Hook point：集成到 MobileProjectView 中

##### WI-2.7 [S] 测试：分割线拖拽验证
- **描述**：在手机浏览器中验证分割线拖拽功能
- **验收标准**：
  1. 手指拖拽分割线流畅无卡顿
  2. 最小高度约束生效
  3. 拖拽分割线不会误触发终端输入或待办操作
  4. 安全门控：`cd frontend && npx tsc --noEmit` 通过

##### WI-2.8 [M] 移动端标题栏（重命名、删除、排序箭头）
- **描述**：在 MobileProjectView 顶部实现标题栏，包含：项目名称、重命名按钮、删除按钮、左右移动箭头（箭头功能在阶段 3 实现，此处先放置禁用状态的按钮）。删除后自动切换到相邻项目
- **验收标准**：
  1. 标题栏显示项目名称
  2. 点击重命名按钮弹出编辑框，修改后名称更新并多设备同步
  3. 点击删除按钮关闭项目，视图切换到前一个项目
  4. 删除唯一项目后显示新建项目页面
  5. 左右箭头按钮已渲染但处于禁用状态
  6. 安全门控：`cd frontend && npx tsc --noEmit` 通过
- **Notes**：
  - Pattern：参照桌面端 ProjectContainer 的 header 操作
  - Reference：`frontend/src/components/ProjectContainer.tsx`
  - Hook point：阶段 3 WI-3.6 将启用箭头功能

##### WI-2.9 [S] 测试：项目管理操作验证
- **描述**：在手机浏览器中验证重命名和删除功能
- **验收标准**：
  1. 重命名后其他设备同步更新
  2. 删除后视图正确切换
  3. 删除唯一项目后显示新建页面
  4. 安全门控：`cd frontend && npx tsc --noEmit` 通过

##### WI-2.10 [集成门控] 阶段 2 完整验证
- **描述**：验证移动端单项目视图的完整功能
- **验收标准**：
  1. 所有安全门控命令通过
  2. 终端 + 待办 + 分割线 + 标题栏全部可用
  3. 多设备同步正常

**阶段验收标准**：
1. Given 用户在手机端打开页面, When 页面加载完成, Then 显示第一个项目，终端在上、待办在下
2. Given 用户拖拽分割线, When 上下拖拽, Then 终端和待办区域高度实时变化，最小高度约束生效
3. Given 用户点击重命名按钮, When 输入新名称确认, Then 名称更新且多设备同步
4. Given 用户点击删除按钮, When 项目被删除, Then 视图切换到相邻项目
5. 所有安全门控命令通过

**阶段状态**：未开始

---

### 阶段 3：滑动导航 + 布局持久化

**目标**：实现 Swiper 滑动切换项目、新建入口、排序功能和移动端布局持久化

**涉及的需求项**：
- 2.3.1 左右滑动切换项目
- 2.4.1 左右按键移动项目位置
- 2.6.1 末尾新建项目页面
- 2.7.1 手机端布局独立存储（前端集成）

#### 工作项列表

##### WI-3.1 [M] Swiper 集成 + 项目滑动切换
- **描述**：安装 swiper 包，在 MobileApp 中用 Swiper 组件包裹项目列表。每个 SwiperSlide 渲染一个 MobileProjectView。配置 `touchAngle: 45` 解决手势冲突
- **验收标准**：
  1. 左右滑动可流畅切换项目
  2. 偏垂直方向的滑动不触发切换，终端内可正常上下滚动
  3. 第一个项目向右滑动有弹性回弹
  4. 安全门控：`cd frontend && npx tsc --noEmit` 通过
- **Notes**：
  - Pattern：Swiper React 组件，`touchAngle: 45` 配置
  - Reference：Swiper.js 官方文档
  - Hook point：WI-3.3 将在末尾添加新建页面

##### WI-3.2 [S] 测试：滑动切换验证
- **描述**：在手机浏览器中测试滑动切换的流畅度和手势冲突
- **验收标准**：
  1. 滑动动画流畅（60fps）
  2. 终端区域垂直滑动不触发项目切换
  3. 不同浏览器（Safari, Chrome）下表现一致
  4. 安全门控：`cd frontend && npx tsc --noEmit` 通过

##### WI-3.3 [S] 末尾新建项目页面
- **描述**：在 Swiper 最后一个 slide 中添加新建项目入口（虚线框按钮），点击后创建项目并将 Swiper 滑动到新项目
- **验收标准**：
  1. 最后一页显示新建项目入口
  2. 点击后创建项目，Swiper 自动滑到新项目
  3. 创建失败时显示错误提示，保持在新建页面
  4. 安全门控：`cd frontend && npx tsc --noEmit` 通过
- **Notes**：
  - Pattern：复用 NewProjectSlot 的样式
  - Reference：`frontend/src/components/NewProjectSlot.tsx`
  - Hook point：创建后需同步更新 mobile layout

##### WI-3.4 [S] 测试：新建项目验证
- **描述**：在手机端创建项目并验证跳转
- **验收标准**：
  1. 滑到末尾可见新建入口
  2. 创建后自动跳转到新项目
  3. 其他设备同步看到新容器
  4. 安全门控：`cd frontend && npx tsc --noEmit` 通过

##### WI-3.5 [集成门控] 滑动导航 + 新建功能完整
- **描述**：验证 WI-3.1 ~ WI-3.4 的集成状态
- **验收标准**：
  1. 所有安全门控命令通过
  2. 滑动切换、手势冲突处理、新建入口全部正常
  3. 多设备同步正常

##### WI-3.6 [M] 左右箭头移动项目顺序
- **描述**：启用标题栏的左右箭头按钮，点击后将当前项目在移动端布局中前移/后移一位。第一个项目禁用左箭头，最后一个项目禁用右箭头。移动后 Swiper 跟随
- **验收标准**：
  1. 点击左箭头，项目前移一位，Swiper 跟随到该项目
  2. 点击右箭头，项目后移一位，Swiper 跟随到该项目
  3. 边界位置箭头禁用
  4. 排序变更同步到后端 mobile layout
  5. 安全门控：`cd frontend && npx tsc --noEmit` 通过
- **Notes**：
  - Pattern：交换相邻条目的 sortOrder，调用 updateMobileLayout
  - Reference：`frontend/src/api.ts` updateMobileLayout
  - Hook point：排序变更需持久化到后端

##### WI-3.7 [S] 移动端布局持久化集成
- **描述**：MobileApp 启动时从后端加载 mobile layout 确定项目顺序，排序变更后实时保存到后端。确保关闭浏览器重新打开后顺序恢复
- **验收标准**：
  1. 启动时按 mobile layout 的 sortOrder 排列项目
  2. 移动项目顺序后，刷新页面顺序不变
  3. 不同手机访问看到相同的排列顺序
  4. 安全门控：`cd frontend && npx tsc --noEmit` 通过
- **Notes**：
  - Pattern：loadData 时 merge containers + mobile layout → sorted list
  - Reference：App.tsx 中桌面端 layout 的加载逻辑
  - Hook point：与 WI-3.6 的排序操作协同

##### WI-3.8 [S] 测试：布局持久化验证
- **描述**：验证移动端布局在刷新、关闭重开、多手机间的一致性
- **验收标准**：
  1. 调整顺序后刷新页面，顺序不变
  2. 手机 A 调整顺序后，手机 B 看到相同顺序
  3. 手机调整不影响桌面端布局
  4. 安全门控：`cd frontend && npx tsc --noEmit` 通过

##### WI-3.9 [集成门控] 阶段 3 完整验证
- **描述**：全功能 E2E 验证
- **验收标准**：
  1. 所有安全门控命令通过
  2. `cd frontend && npm run build` 通过
  3. 滑动切换、新建项目、排序、持久化全部正常
  4. 多设备同步正常
  5. 桌面端无功能退化

**阶段验收标准**：
1. Given 用户在手机端有 3 个项目, When 左右滑动, Then 流畅切换项目，终端区域垂直滑动不误触发
2. Given 用户滑到最后一页, When 点击新建按钮, Then 创建项目并自动跳转
3. Given 用户点击左箭头, When 项目移动后刷新页面, Then 顺序已持久化
4. Given 用户在手机端调整了顺序, When 在桌面端访问, Then 桌面端布局不受影响
5. 所有安全门控命令通过

**阶段状态**：未开始

## 3. 风险与应对

| 风险 | 影响 | 概率 | 应对措施 |
|------|------|------|----------|
| Swiper 与 xterm.js 触摸事件冲突 | 终端区域无法正常滚动或输入 | 中 | 利用 `touchAngle: 45` 配置；必要时对终端区域设置 `noSwiping` class |
| iOS Safari 虚拟键盘弹出导致布局变化 | 分割线位置错乱、终端尺寸异常 | 中 | 使用 `visualViewport` API 监听键盘弹出，动态调整布局高度 |
| 分割线拖拽与 Swiper 水平滑动冲突 | 拖拽分割线时误触发项目切换 | 低 | 分割线 touch 事件中调用 `e.stopPropagation()` 阻止冒泡 |
| 不同浏览器 UA 格式差异导致误判 | 平板被识别为手机或反之 | 低 | 采用成熟的 UA 匹配正则，覆盖主流设备 |

## 4. 开发规范

### 4.1 代码规范
- 沿用现有项目的 TypeScript + Tailwind 风格
- 移动端组件以 `Mobile` 前缀命名，与桌面端组件区分
- 新组件放在 `frontend/src/components/` 目录下

### 4.2 Git 规范
- 在 `feature/mobile-adaptation` 分支上开发
- 提交信息使用中文
- 每个工作项完成后提交一次

### 4.3 文档规范
- 代码中仅在非显而易见的逻辑处添加注释
- 不新增文档文件，README 在最终合并前更新

## 5. 工作项统计

| 阶段 | S | M | 集成门控 | 总计 |
|------|---|---|---------|------|
| 阶段 1 | 8 | 0 | 2 | 10 |
| 阶段 2 | 4 | 4 | 2 | 10 |
| 阶段 3 | 4 | 2 | 3 | 9 |
| **合计** | **16** | **6** | **7** | **29** |

## 审核记录

| 日期 | 审核人 | 评分 | 结果 | 备注 |
|------|--------|------|------|------|
| 2026-04-14 | AI Assistant | 84/100 | 未通过 | 初审：WI-3.5 编号重复、统计表有误、测试 WI 缺少安全门控 |
| 2026-04-14 | AI Assistant | 93/100 | 通过 | 复审：修复编号、更正统计、补齐安全门控命令 |
