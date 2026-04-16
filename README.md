# Agent Hive

终端智能体并行管理 — 基于 Web 的多终端管理器，支持并行运行 AI Agent 或其他终端任务。

## 功能特性

- **多终端并行** — 2×2 网格布局管理多个终端会话，支持多页切换
- **项目容器** — 每个容器包含独立终端 + 待办清单，可编辑名称
- **可调布局** — 桌面端拖拽分割线调整待办/终端比例，待办列表可完全隐藏
- **完整终端模拟** — 支持 ANSI 颜色、vim、htop 等 TUI 程序（基于 xterm.js）
- **移动端适配** — 滑动切换项目、触屏拖拽排序、快捷键栏（^C/^L/Tab/Esc/方向键）、粘贴功能
- **多设备同步** — 终端输入输出、待办事项、容器创建/删除在所有设备间实时同步
- **会话持久化** — 关闭浏览器后终端输出历史和待办数据不丢失，服务重启后可恢复
- **密码认证** — 可选的密码保护访问（通过 config.yaml 配置）
- **拖拽排序** — 容器可拖拽调整位置，待办事项可拖拽排序
- **服务管理** — 内置 systemd 服务安装/卸载/启停命令，一键部署
- **日志管理** — 运行日志自动写入文件，按天轮转，保留 7 天
- **用户切换** — 以 root/systemd 运行时，PTY 自动以配置的普通用户身份启动
- **单文件交付** — 编译为单个二进制文件（~15MB），内嵌前端，部署即复制

## 技术栈

| 层 | 技术 |
|---|------|
| 后端 | Go, creack/pty, gorilla/websocket, SQLite |
| 前端 | React, TypeScript, xterm.js, Tailwind CSS, dnd-kit, Framer Motion, Swiper |
| 通信 | WebSocket（终端 I/O + 事件广播）, REST API |
| 存储 | SQLite（容器/布局/待办）, 磁盘文件（终端历史、运行日志）|
| 测试 | Go test, Vitest |
| 交付 | go:embed 内嵌前端，单二进制 |

## 快速开始

### 构建

依赖：Go 1.21+（CGO）、Node.js 18+、Make

```bash
make build
```

产物在 `dist/agent-hive`。

### 初始化配置

```bash
# 自动检测当前用户和 shell，生成 config.yaml
./dist/agent-hive init

# 或手动指定
./dist/agent-hive init --user penguin --shell /bin/zsh
```

### 运行

```bash
# 前台运行
./dist/agent-hive run --config config.yaml
```

浏览器访问 `http://localhost:8090`。局域网其他设备访问 `http://你的IP:8090`。

### 作为系统服务运行

```bash
# 安装 systemd 服务（需要 sudo）
sudo ./dist/agent-hive install --config /path/to/config.yaml

# 管理服务
sudo ./dist/agent-hive start
sudo ./dist/agent-hive stop
sudo ./dist/agent-hive restart
./dist/agent-hive status
./dist/agent-hive logs -f

# 卸载服务
sudo ./dist/agent-hive uninstall
```

### 配置

```yaml
port: 8090          # 监听端口
data_dir: ./data    # 数据目录
token: ""           # 认证密码（留空禁用）
machines: []        # 机器白名单（留空允许所有）
user: ""            # PTY 运行用户（留空自动检测）
shell: ""           # PTY 使用的 shell（留空自动检测）
```

### CLI 命令一览

| 命令 | 说明 | 需要 root |
|------|------|-----------|
| `agent-hive init` | 生成 config.yaml | 否 |
| `agent-hive run` | 前台启动服务 | 否 |
| `agent-hive install` | 安装 systemd 服务 | 是 |
| `agent-hive uninstall` | 卸载 systemd 服务 | 是 |
| `agent-hive start` | 启动服务 | 是 |
| `agent-hive stop` | 停止服务 | 是 |
| `agent-hive restart` | 重启服务 | 是 |
| `agent-hive status` | 查看服务状态 | 否 |
| `agent-hive logs` | 查看服务日志 | 否 |

## 开发

```bash
# 后端
cd backend && go run ./cmd/server/ run --dev &

# 前端（热更新）
cd frontend && npm run dev
```

前端 `http://localhost:5173`，API/WS 自动代理到后端 8090。

### 测试

```bash
# 后端测试
cd backend && go test ./...

# 前端测试
cd frontend && npx vitest run
```

### 快捷键

**桌面端：**

| 快捷键 | 功能 |
|--------|------|
| `Ctrl+N` | 新建容器 |
| `Ctrl+←/→` | 切换页面 |

**移动端快捷键栏：**

| 按钮 | 功能 |
|------|------|
| `^C` | 中断进程 |
| `^L` | 清屏 |
| `Tab` | 自动补全 |
| `Esc` | 退出 |
| `↑↓←→` | 方向键 |
| `Paste` | 粘贴文本到终端 |

### 项目结构

```
agent_hive/
├── backend/
│   ├── cmd/server/          # 入口（子命令分发）
│   └── internal/
│       ├── auth/            # 认证
│       ├── cli/             # CLI 子命令（run/init/install/...）
│       ├── config/          # 配置加载（含用户推断）
│       ├── container/       # 容器与 PTY 管理
│       ├── logger/          # 日志文件写入与轮转
│       ├── pty/             # PTY 会话（含用户切换）
│       ├── server/          # HTTP 路由与 API
│       ├── static/          # go:embed 前端资源
│       ├── store/           # SQLite 存储
│       └── ws/              # WebSocket 处理
├── frontend/
│   └── src/
│       ├── components/      # React 组件
│       │   ├── Terminal.tsx      # 终端（桌面+移动共用）
│       │   ├── ShortcutBar.tsx   # 移动端快捷键栏
│       │   ├── PasteModal.tsx    # 粘贴弹窗
│       │   └── ...
│       ├── api.ts           # API 调用
│       ├── App.tsx          # 桌面端应用
│       └── MobileApp.tsx    # 移动端应用
├── project_plan/            # 需求文档与开发方案
├── Makefile                 # 构建脚本
└── dist/                    # 构建产物
```
