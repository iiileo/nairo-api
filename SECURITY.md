# Security Policy

## 报告漏洞

请通过 GitHub Issues（可标记为私密讨论）或仓库 Maintainer 私信报告安全问题。
请勿在公开 Issue 中粘贴真实密钥、订单、商户回调 URL 或生产日志。

## 部署注意

- **永远不要**把 `.env`、数据库备份、JWT / `SECURITY_MASTER_KEY`、SMTP、极验密钥提交进 Git。
- `SERVER_MODE=prod` 时必须使用强随机的：
  - `ADMIN_JWT_SECRET`
  - `CONSOLE_JWT_SECRET`
  - `ADMIN_BOOTSTRAP_PASSWORD`
  - `SECURITY_MASTER_KEY`（≥16 字符，用于密封 App Secret）
- 关闭演示账号：`CONSOLE_BOOTSTRAP_DEMO=false`
- 生产环境关闭 `SMTP_DEV_PRINT`，配置真实 SMTP
- `PAY_PUBLIC_BASE_URL` 使用公网 HTTPS
- 定期轮换密钥；轮换 `SECURITY_MASTER_KEY` 前请先确认已有密封数据的迁移方案

## 使用边界

本项目面向**本人商户设备**上的收款通知同步，不能替代微信 / 支付宝官方商户能力。
请勿用于未经授权监控他人设备或通知。
