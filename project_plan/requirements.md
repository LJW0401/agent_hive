# Agent Hive 功能增强 需求文档

> 创建日期：2026-04-15
> 状态：已审核
> 版本：v1.3

## 1. 项目概述

### 1.1 项目背景
Agent Hive 是一个基于 Web 的多终端管理器，支持并行运行 AI Agent 或其他终端任务。当前桌面端待办列表与终端的比例固定（待办列表固定 `w-48`），待办项文本过长时会被截断，且程序只能前台运行、缺少系统服务管理能力。本次迭代针对这些体验和运维短板进行增强。

### 1.2 项目目标
1. 桌面端支持拖拽调整待办列表与终端的宽度比例
2. 待办列表项支持自动换行，显示完整内容
3. 提供 systemd 服务管理能力，简化部署运维
4. 程序运行时输出日志文件，便于排查问题
5. 移动端提供快捷键栏和粘贴功能，弥补触屏键盘的操作短板

### 1.3 成功标准
- 桌面端可通过拖拽分割线自由调整待办/终端比例，待办列表可完全隐藏，最大不超过容器宽度 2/3
- 待办项文本完整显示，不再截断
- 可通过 `agent-hive install` 一键注册 systemd 服务，通过子命令管理服务生命周期
- 程序启动后自动在同目录生成运行日志文件
- 移动端可通过快捷键栏输入 ^C、^L、Tab、Esc、方向键等控制字符，可通过粘贴按钮向终端粘贴文本

### 1.4 目标用户
在 Ubuntu/Debian 等 systemd Linux 上部署 Agent Hive 的开发者和运维人员。

### 1.5 核心约束
- 仅支持 systemd 的 Linux 发行版
- 保持单二进制交付的架构（Go embed 前端）
- 不引入额外外部依赖

## 2. 功能需求

### 2.1 桌面端待办列表/终端比例可调

#### 2.1.1 拖拽分割线调整比例
- **描述**：在桌面端项目容器中，待办列表与终端之间增加可拖拽的分割线，用户可通过拖拽调整两者的宽度比例。
- **用户场景**：用户在桌面端使用时，根据当前工作重点（查看待办 vs 操作终端）灵活分配界面空间。
- **输入**：用户在分割线上按住鼠标并左右拖拽。
- **输出**：待办列表和终端区域宽度实时跟随鼠标变化。终端自动 refit 适配新尺寸。
- **验收标准**：
  - Given 桌面端打开一个项目容器, When 用户拖拽分割线向右, Then 待办列表宽度增大，终端宽度减小，终端内容自动适配新尺寸
  - Given 待办列表已展开, When 用户将分割线拖到最左侧, Then 待办列表完全隐藏，终端占满整个区域
  - Given 用户正在拖拽分割线, When 待办列表宽度即将超过容器宽度的 2/3, Then 分割线停止移动，不允许超过该限制
- **优先级**：P0
- **成熟度**：RS3

### 2.2 待办列表文本换行

#### 2.2.1 待办项自动换行显示
- **描述**：待办列表中的待办项文本取消单行截断，改为自动换行显示完整内容。
- **用户场景**：用户添加了较长的待办内容时，能看到完整文本而不是被 `...` 截断。
- **输入**：任意长度的待办项文本。
- **输出**：文本在待办列表宽度范围内自动换行，完整展示。
- **验收标准**：
  - Given 一条待办项文本超过待办列表宽度, When 页面渲染该待办项, Then 文本自动换行显示，无截断
  - Given 待办列表宽度通过拖拽分割线变化, When 宽度变窄, Then 文本重新换行适配新宽度
- **优先级**：P0
- **成熟度**：RS3

### 2.3 Ubuntu 服务管理

#### 2.3.1 init 命令 — 初始化配置文件
- **描述**：`agent-hive init` 命令在当前目录生成 `config.yaml` 配置文件，自动嗅探当前用户和 shell 并写入配置。支持通过命令行参数手动指定 user 和 shell。
- **用户场景**：首次部署时，运行 `agent-hive init` 快速生成配置文件，无需手动编写。
- **输入**：可选参数 `--user <username>`、`--shell <shell-path>`。
- **输出**：生成 `config.yaml`，包含 port、data_dir、token、user、shell 等字段。
- **验收标准**：
  - Given 当前目录无 config.yaml, When 运行 `agent-hive init`, Then 生成 config.yaml，user 字段为当前用户名，shell 字段为该用户的默认 shell
  - Given 运行 `agent-hive init --user penguin --shell /bin/zsh`, When 命令执行, Then config.yaml 中 user 为 penguin，shell 为 /bin/zsh
  - Given 当前目录已有 config.yaml, When 运行 `agent-hive init`, Then 提示文件已存在，询问是否覆盖
  - Given 运行 `agent-hive init --user nonexist`, When 指定的用户在系统中不存在, Then 提示用户不存在，不生成配置文件
  - Given 运行 `agent-hive init --shell /bin/nonexist`, When 指定的 shell 路径不存在, Then 提示 shell 不存在，不生成配置文件
- **优先级**：P0
- **成熟度**：RS3

#### 2.3.2 install 命令 — 安装 systemd 服务
- **描述**：`agent-hive install` 命令自动生成 systemd service 文件并写入 `/etc/systemd/system/agent-hive.service`，执行 `systemctl daemon-reload` 和 `systemctl enable`。service 文件模板硬编码在 Go 代码中，根据当前二进制路径和配置文件路径动态填充。
- **用户场景**：部署完成后，运行 `agent-hive install` 一键注册系统服务。
- **输入**：需要 root 权限（sudo）。可选参数 `--config <path>` 指定配置文件路径。
- **输出**：写入 service 文件，执行 daemon-reload 和 enable。
- **验收标准**：
  - Given 以 root 权限运行 `agent-hive install`, When 命令执行, Then `/etc/systemd/system/agent-hive.service` 被创建，服务被 enable
  - Given 无 root 权限运行 `agent-hive install`, When 命令执行, Then 提示需要 root 权限
  - Given 服务已安装, When 再次运行 `agent-hive install`, Then 提示服务已存在，询问是否覆盖
  - Given `--config` 指定的配置文件不存在, When 运行 `agent-hive install`, Then 提示配置文件不存在，终止安装
- **优先级**：P0
- **成熟度**：RS3

#### 2.3.3 uninstall 命令 — 卸载 systemd 服务
- **描述**：`agent-hive uninstall` 停止服务、disable 服务、删除 service 文件并 daemon-reload。
- **用户场景**：不再需要服务时，一键清理。
- **输入**：需要 root 权限。
- **输出**：服务被停止、禁用，service 文件被删除。
- **验收标准**：
  - Given 服务已安装, When 以 root 权限运行 `agent-hive uninstall`, Then 服务被停止、disable、service 文件删除、daemon-reload
  - Given 服务未安装, When 运行 `agent-hive uninstall`, Then 提示服务未安装
  - Given 无 root 权限运行 `agent-hive uninstall`, When 命令执行, Then 提示需要 root 权限
- **优先级**：P0
- **成熟度**：RS3

#### 2.3.4 start/stop/restart 命令
- **描述**：封装 `systemctl start/stop/restart agent-hive`。
- **用户场景**：日常服务管理。
- **输入**：需要 root 权限。
- **输出**：执行对应的 systemctl 命令并输出结果。
- **验收标准**：
  - Given 服务已安装且未运行, When 运行 `agent-hive start`, Then 服务启动成功
  - Given 服务正在运行, When 运行 `agent-hive stop`, Then 服务停止
  - Given 服务正在运行, When 运行 `agent-hive restart`, Then 服务重启成功
- **优先级**：P0
- **成熟度**：RS3

#### 2.3.5 status 命令
- **描述**：封装 `systemctl status agent-hive`，显示服务运行状态。
- **用户场景**：查看服务是否正常运行。
- **输入**：无需 root 权限。
- **输出**：输出服务状态信息。
- **验收标准**：
  - Given 服务正在运行, When 运行 `agent-hive status`, Then 输出显示 active (running)
- **优先级**：P0
- **成熟度**：RS3

#### 2.3.6 logs 命令
- **描述**：封装 `journalctl -u agent-hive`，查看服务日志。支持 `-f` 参数实时跟踪。
- **用户场景**：排查服务运行问题。
- **输入**：可选参数 `-f`（跟踪模式）、`-n <lines>`（行数）。
- **输出**：输出服务日志。
- **验收标准**：
  - Given 服务已运行过, When 运行 `agent-hive logs`, Then 输出服务日志
  - Given 服务正在运行, When 运行 `agent-hive logs -f`, Then 实时跟踪输出新日志
- **优先级**：P1
- **成熟度**：RS3

#### 2.3.7 PTY 以配置用户身份启动
- **描述**：程序读取 config.yaml 中的 `user` 和 `shell` 字段。创建 PTY 会话时，以指定用户身份和 shell 启动。若 config 中未配置 user/shell，则通过 config 文件的归属用户（文件 owner）推断用户，再查找该用户的默认 shell。
- **用户场景**：以 sudo/root 运行 systemd 服务时，终端仍以普通用户的 zsh 启动，而非 root 的 bash。
- **输入**：config.yaml 中 `user` 和 `shell` 字段（可选）。
- **输出**：PTY 进程以指定用户和 shell 运行。
- **验收标准**：
  - Given config.yaml 中 user=penguin, shell=/bin/zsh, When 创建新容器终端, Then PTY 以 penguin 用户身份运行 /bin/zsh
  - Given config.yaml 中未配置 user/shell，config.yaml 归属用户为 penguin, When 创建新容器终端, Then 自动推断用户为 penguin，查找其默认 shell 并使用
  - Given 程序以普通用户（非 root）运行, When 创建新容器终端, Then 直接以当前用户和其 shell 启动（无需 setuid）
- **优先级**：P0
- **成熟度**：RS3

#### 2.3.8 run 命令 — 前台运行
- **描述**：`agent-hive run` 保持当前的前台运行方式，作为默认/开发模式。等同于当前无子命令时的行为。
- **用户场景**：开发调试时直接前台运行。
- **输入**：`--config <path>` 指定配置文件。
- **输出**：前台运行程序，日志输出到终端和日志文件。
- **验收标准**：
  - Given 运行 `agent-hive run --config config.yaml`, When 命令执行, Then 程序前台启动，终端可见日志输出
- **优先级**：P0
- **成熟度**：RS3

### 2.4 程序运行日志文件

#### 2.4.1 运行日志写入文件
- **描述**：程序启动后，将运行日志（HTTP 请求、错误、启动信息等）写入与二进制文件同目录下的日志文件（如 `agent-hive.log`）。
- **用户场景**：服务以 systemd 后台运行时，可通过日志文件排查问题；同时 journalctl 也能看到日志。
- **输入**：无。
- **输出**：日志文件持续写入，包含时间戳和日志级别。
- **验收标准**：
  - Given 程序启动, When 有 HTTP 请求或错误发生, Then 日志被写入二进制文件同目录下的 `agent-hive.log`
  - Given 程序重启, When 日志文件已存在, Then 新日志追加到文件末尾，不覆盖旧日志
- **优先级**：P0
- **成熟度**：RS3

#### 2.4.2 日志按天轮转
- **描述**：日志文件按天轮转，格式如 `agent-hive.log`（当天）、`agent-hive.2026-04-14.log`（历史）。保留最近 7 天的日志，超过 7 天的日志文件自动删除。
- **用户场景**：长期运行的服务不会因日志文件无限增长占满磁盘。
- **输入**：无。
- **输出**：每天自动切换新日志文件，清理过期日志。
- **验收标准**：
  - Given 程序持续运行跨越日期变更, When 新的一天到来, Then 旧日志被重命名为带日期后缀的文件，新日志写入 `agent-hive.log`
  - Given 日志目录中存在 8 天前的日志文件, When 轮转触发, Then 该文件被自动删除
- **优先级**：P0
- **成熟度**：RS3

### 2.5 移动端快捷键栏

#### 2.5.1 快捷键按钮栏
- **描述**：在移动端终端底部添加一行固定的快捷键栏，包含常用快捷键按钮：`^C`、`^L`、`Tab`、`Esc`、`↑`、`↓`、`←`、`→`。快捷键栏可左右滑动浏览。点击按钮后向终端发送对应的控制字符。按钮上 Ctrl 组合键统一显示为 `^` 前缀格式。
- **用户场景**：手机端缺少物理键盘，无法输入 Ctrl 组合键和功能键，通过快捷键栏快速输入常用控制字符。
- **输入**：用户点击快捷键按钮。
- **输出**：对应的控制字符被发送到终端 WebSocket 连接。
- **验收标准**：
  - Given 移动端打开一个项目终端, When 页面加载完成, Then 终端底部显示快捷键栏，包含 ^C、^L、Tab、Esc、↑、↓、←、→ 按钮
  - Given 快捷键栏可见, When 用户点击 ^C 按钮, Then 终端收到 `\x03` 控制字符（中断当前进程）
  - Given 快捷键栏可见, When 用户点击 ^L 按钮, Then 终端收到 `\x0c` 控制字符（清屏）
  - Given 快捷键栏可见, When 用户点击 Tab 按钮, Then 终端收到 `\t` 字符（自动补全）
  - Given 快捷键栏可见, When 用户点击 Esc 按钮, Then 终端收到 `\x1b` 字符
  - Given 快捷键栏可见, When 用户点击 ↑ 按钮, Then 终端收到 `\x1b[A`（上方向键）
  - Given 快捷键栏可见, When 用户点击 ↓ 按钮, Then 终端收到 `\x1b[B`（下方向键）
  - Given 快捷键栏可见, When 用户点击 ← 按钮, Then 终端收到 `\x1b[D`（左方向键）
  - Given 快捷键栏可见, When 用户点击 → 按钮, Then 终端收到 `\x1b[C`（右方向键）
  - Given 快捷键按钮数量超出屏幕宽度, When 用户在快捷键栏上左右滑动, Then 栏内容可水平滚动，不触发 Swiper 页面切换
- **优先级**：P0
- **成熟度**：RS3

#### 2.5.2 粘贴按钮
- **描述**：快捷键栏中包含一个粘贴按钮，点击后弹出输入框。用户在输入框中粘贴文本内容，确认后将内容发送到终端。用于绕过移动端浏览器对 canvas 元素的剪贴板限制。
- **用户场景**：用户需要在手机端向终端粘贴命令或文本（如从其他 App 复制的命令），但 xterm.js canvas 无法直接接收系统粘贴。
- **输入**：用户点击粘贴按钮，在弹出框中粘贴文本并确认。
- **输出**：文本内容被发送到终端 WebSocket 连接。
- **验收标准**：
  - Given 移动端快捷键栏可见, When 用户点击粘贴按钮, Then 弹出包含文本输入框和确认按钮的弹窗
  - Given 粘贴弹窗已打开, When 用户在输入框中粘贴文本并点击确认, Then 文本内容被发送到终端，弹窗关闭
  - Given 粘贴弹窗已打开, When 用户点击取消或弹窗外区域, Then 弹窗关闭，不发送任何内容
  - Given 粘贴弹窗已打开, When 用户未输入任何内容直接点击确认, Then 弹窗关闭，不发送任何内容
- **优先级**：P0
- **成熟度**：RS3

## 3. 非功能需求

### 3.1 性能要求
- 拖拽分割线时终端 refit 应流畅，无明显卡顿（throttle 到 ~60fps）
- 日志文件写入不应阻塞主程序运行

### 3.2 安全要求
- PTY 用户切换不得通过 su/sudo 等子进程方式实现，须在进程级别直接设置用户身份
- install/uninstall 命令需要 root 权限，非 root 运行时给出明确错误提示

### 3.3 兼容性要求
- 服务管理仅支持 systemd 的 Linux 发行版（Ubuntu/Debian 等）
- 前端拖拽分割线仅桌面端，移动端保持现有上下拖拽分割线不变

### 3.4 技术约束
- 保持单二进制交付架构
- Go 后端，React + TypeScript 前端
- systemd service 文件模板硬编码在 Go 代码中

## 4. 术语表

| 术语 | 定义 |
|------|------|
| PTY | 伪终端（Pseudo Terminal），用于模拟终端会话 |
| systemd | Linux 系统和服务管理器 |
| service 文件 | systemd 的服务配置文件（.service） |
| refit | xterm.js 的 FitAddon 重新计算终端行列数以适配容器尺寸 |

## 5. 开放问题

（无）

## 6. 需求成熟度汇总

| 等级 | P0 数量 | P1 数量 | P2 数量 |
|------|---------|---------|---------|
| RS3  | 13      | 1       | 0       |

## 审核记录

| 日期 | 审核人 | 评分 | 结果 | 备注 |
|------|--------|------|------|------|
| 2026-04-15 | AI Assistant | 96/100 | 通过 | 补充异常路径验收标准，统一参数风格，修正 P0 计数，NFR 移除实现细节 |
| 2026-04-16 | AI Assistant | 97/100 | 通过 | 新增移动端快捷键栏(2.5)，补充项目目标/成功标准，Swiper 冲突隔离，空粘贴验收标准 |
