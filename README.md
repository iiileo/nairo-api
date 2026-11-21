# Nairo API Service

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](./LICENSE)

> 请勿把 `.env`、密钥、生产域名提交进 Git。

个人收款「免签」后端：用 Android 监控端读取微信 / 支付宝收款通知，签名上报后完成下单匹配、收银台与商户回调。

配套 App：[`iiileo/nairo-android`](https://github.com/iiileo/nairo-android)  
正式 APK：[`iiileo/nairo-android` Releases](https://github.com/iiileo/nairo-android/releases)

> **定位**：本人商户设备上的近实时到账同步，**不是**微信 / 支付宝官方商户接口，不具备官方退款、分账、合规对账等能力。请合法合规使用。

## 功能概览

| 模块 | 说明 |
|------|------|
| Open API | 签名下单、查单、金额微调锁单 |
| 收银台 | 服务端渲染 `/checkout`，支持 `return_url`、iframe `postMessage` |
| App 通道 | 推送入账、HTTP / WebSocket 心跳、幂等、匹配规则下发 |
| Console | 商户端：监控设备、收款码、通知机器人、开发者文档 |
| Admin | 运营端：订单 / 孤儿单 / 回调任务重放等 |
| 安全 | App Secret 密封存储、prod 配置校验、限流 |

技术栈：Go 1.25 · Gin · Ent · MySQL 8 · Uber FX；`index_web` / `console_web` / `admin_web` 构建后 embed 进单一二进制。

## 架构（简图）

```text
商户系统 --签名--> Open API 创建订单 --> 收银台展示收款码
                                      │
付款人扫码支付（个人码）               │
                                      ▼
微信/支付宝通知 --> Nairo App 监听 --> 签名推送 /app/push
                                      │
                                      ▼
                                 金额匹配入账 --> 商户 notify 重试
```

## 快速开始

### 依赖

- Go 1.25+
- MySQL 8.x（本地可用 Docker 单独起库）
- Node.js 20+（改前端时需要；纯跑后端可用已构建 embed 或 `WEB_DEV=true`）

### 配置

```bash
cp .env.example .env
# 按需修改 MySQL / PAY_PUBLIC_BASE_URL 等
# 切勿提交 .env
```

`.env.example` 中的 JWT / 口令仅为**本地开发占位**，生产必须全部换成强随机值（见下方「生产检查清单」）。

### 本地运行

```bash
# 确保 MySQL 可连，库名默认 nairo_app
make run          # 默认 :8088，WEB_DEV=true
# 或
go run ./cmd/server
```

| 入口 | 默认地址 |
|------|----------|
| 官网 / 落地页 | http://127.0.0.1:8088/ |
| 商户 Console | http://127.0.0.1:8088/console/ |
| 管理后台 | http://127.0.0.1:8088/admin_nairo/ （路径可由 `WEB_ADMIN_BASE_PATH` 配置） |
| 健康检查 | http://127.0.0.1:8088/healthz |

本地默认（仅 `SERVER_MODE=local`）：

- Admin：`admin` / `admin123`（首次启动种子）
- 演示商户：`user01` / `user01`（`CONSOLE_BOOTSTRAP_DEMO=true` 时）

### Docker

```bash
cp .env.example .env
docker compose up -d --build
```

Compose 默认把 API 映射到 `8088`，MySQL 通过 `host.docker.internal` 访问宿主机（可用环境变量覆盖）。

### 前端开发

```bash
cd console_web && npm i && npm run dev
# admin_web / index_web 同理
```

生产构建并由后端 embed：见 `Makefile` 中的 web 构建目标。

## 生产检查清单

1. `SERVER_MODE=prod`
2. 设置强随机：`ADMIN_JWT_SECRET`、`CONSOLE_JWT_SECRET`、`ADMIN_BOOTSTRAP_PASSWORD`、`SECURITY_MASTER_KEY`（≥16）
3. `CONSOLE_BOOTSTRAP_DEMO=false`，修改或删除默认 Admin 口令
4. `SMTP_DEV_PRINT=false` + 真实 SMTP；`PAY_PUBLIC_BASE_URL` 为公网 HTTPS
5. 配置 `SERVER_ALLOWED_ORIGINS`（如需要浏览器跨域）
6. 备份：`./scripts/backup_mysql.sh`；恢复：`./scripts/restore_mysql.sh <file.sql.gz>`
7. 冒烟：`./scripts/smoke.sh`，并用真机完成一笔小额收款推送

更多安全说明见 [SECURITY.md](./SECURITY.md)。

## 主要接口（节选）

完整字段与签名算法以 Console「开发者 API」页为准。

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/open/orders` | 商户签名下单（可带 `return_url`） |
| GET | `/api/v1/open/orders/:id` | 查单 |
| GET | `/checkout?order_id=` | 收银台页面 |
| POST | `/app/push/:callback_id` | App 到账推送 |
| GET/WS | `/app/heartbeat` · `/app/heartbeat/ws` | 心跳 |
| GET | `/api/v1/public/app-match-rules` | App 匹配规则（可热更新） |

运维：

```text
GET  /api/v1/admin/ops/summary
GET  /api/v1/admin/notify-jobs
POST /api/v1/admin/notify-jobs/:id/replay
GET  /api/v1/console/notify-jobs
POST /api/v1/console/notify-jobs/:id/replay
GET  /api/v1/console/exports/orders.csv
```

## 目录结构

```text
api_service/
├── cmd/server/          # 入口
├── ent/                 # Ent schema + 生成代码
├── internal/            # API / service / config / 安全组件
├── console_web/         # 商户前端（React）
├── admin_web/           # 管理前端
├── index_web/           # 落地页
├── scripts/             # 备份 / 恢复 / smoke
├── docker-compose.yml
└── .env.example
```

## 开发

```bash
go test ./...
go build -o bin/api_service ./cmd/server

# 修改 ent/schema 后
go run -mod=mod entgo.io/ent/cmd/ent generate ./ent/schema
```

CI：`.github/workflows/ci.yml`（`go test` + `go build`）。

## 相关仓库

| 仓库 | 说明 |
|------|------|
| [nairo-android](https://github.com/iiileo/nairo-android) | Android 源码与 APK Releases |

## License

[MIT](./LICENSE)
