# Agent Hive

终端智能体并行管理 — 基于 Web 的多终端管理器，支持并行运行 AI Agent 或其他终端任务。

## 功能特性

- **多终端并行** — 2×2 网格布局管理多个终端会话，支持多页切换
- **项目容器** — 每个容器包含独立终端 + 待办清单，可编辑名称
- **完整终端模拟** — 支持 ANSI 颜色、vim、htop 等 TUI 程序（基于 xterm.js）
- **多设备同步** — 终端输入输出、待办事项、容器创建/删除在所有设备间实时同步
- **会话持久化** — 关闭浏览器后终端输出历史和待办数据不丢失，服务重启后可恢复
- **密码认证** — 可选的密码保护访问（通过 config.yaml 配置）
- **拖拽排序** — 容器可拖拽调整位置，待办事项可拖拽排序
- **单文件交付** — 编译为单个二进制文件（~15MB），内嵌前端，部署即复制

## 技术栈

| 层 | 技术 |
|---|------|
| 后端 | Go, creack/pty, gorilla/websocket, SQLite |
| 前端 | React, TypeScript, xterm.js, Tailwind CSS, dnd-kit, Framer Motion |
| 通信 | WebSocket（终端 I/O + 事件广播）, REST API |
| 存储 | SQLite（容器/布局/待办）, 磁盘文件（终端历史）|
| 交付 | go:embed 内嵌前端，单二进制 |

## 快速开始

### 构建

依赖：Go 1.21+（CGO）、Node.js 18+、Make

```bash
make build
```

产物在 `dist/agent-hive`。

### 运行

```bash
cp backend/config.yaml.example config.yaml
# 按需编辑 config.yaml
./dist/agent-hive -config config.yaml
```

浏览器访问 `http://localhost:8090`。局域网其他设备访问 `http://你的IP:8090`。

### 配置

```yaml
port: 8090          # 监听端口
data_dir: ./data    # 数据目录
token: ""           # 认证密码（留空禁用）
machines: []        # 机器白名单（留空允许所有）
```

## 开发

```bash
# 后端
cd backend && go run ./cmd/server/ -dev &

# 前端（热更新）
cd frontend && npm run dev
```

前端 `http://localhost:5173`，API/WS 自动代理到后端 8090。

### 快捷键

| 快捷键 | 功能 |
|--------|------|
| `Ctrl+N` | 新建容器 |
| `Ctrl+←/→` | 切换页面 |

### 项目结构

```
agent_hive/
├── backend/
│   ├── cmd/server/          # 入口
│   └── internal/
│       ├── auth/            # 认证
│       ├── config/          # 配置加载
│       ├── container/       # 容器与 PTY 管理
│       ├── pty/             # PTY 会话
│       ├── server/          # HTTP 路由与 API
│       ├── static/          # go:embed 前端资源
│       ├── store/           # SQLite 存储
│       └── ws/              # WebSocket 处理
├── frontend/
│   └── src/
│       ├── components/      # React 组件
│       ├── api.ts           # API 调用
│       └── App.tsx          # 主应用
├── docs/                    # 功能规划与开发计划
├── Makefile                 # 构建脚本
└── dist/                    # 构建产物
```
