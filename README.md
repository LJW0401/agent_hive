# Agent Hive

终端智能体并行管理 — 基于 Web 的多终端管理器，支持并行运行 AI Agent 或其他终端任务。

## 构建

### 依赖

- Go 1.21+（需要 CGO 支持，用于 SQLite）
- Node.js 18+
- Make

### 一键构建

```bash
make build
```

构建产物在 `dist/agent-hive`，为单个二进制文件（约 15MB），内嵌前端资源。

### 跨平台编译

```bash
make build-linux    # Linux amd64
make build-darwin   # macOS amd64 + arm64
```

## 运行

```bash
# 复制配置文件
cp backend/config.yaml.example config.yaml
# 编辑 config.yaml 设置密码等
vim config.yaml

# 启动
./dist/agent-hive -config config.yaml
```

浏览器访问 `http://localhost:8090` 即可使用。

### 配置说明

```yaml
port: 8090          # 监听端口
data_dir: ./data    # 数据目录（SQLite + 终端历史）
token: ""           # 认证密码（留空则禁用认证）
machines: []        # 机器白名单（留空则允许所有）
```

## 开发

```bash
# 启动开发服务器（前端热更新 + 后端）
cd backend && go run ./cmd/server/ -dev &
cd frontend && npm run dev
```

前端访问 `http://localhost:5173`，API/WebSocket 自动代理到后端 8090 端口。

### 快捷键

| 快捷键 | 功能 |
|--------|------|
| `Ctrl+N` | 新建容器 |
| `Ctrl+←` | 上一页 |
| `Ctrl+→` | 下一页 |
