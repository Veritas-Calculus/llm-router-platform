# 管理员密码重置

本项目有两种管理员密码重置方式：用户自助的忘记密码流程，以及运维级的强制重置流程。

## 忘记密码流程

登录页的 Forgot Password 会调用 GraphQL `forgotPassword`。后端会创建一个 15 分钟有效、单次使用的 reset token，并通过事务邮件发送重置链接。

这个流程要求根目录 `.env` 中的邮件配置可用：

```env
EMAIL_ENABLED=true
EMAIL_SMTP_HOST=smtp.example.com
EMAIL_SMTP_PORT=587
EMAIL_SMTP_USER=...
EMAIL_SMTP_PASS=...
EMAIL_FROM=noreply@example.com
EMAIL_SMTP_TLS=true
FRONTEND_URL=http://localhost
```

注意：reset token 原文只出现在邮件链接中，数据库只保存 HMAC hash。邮件未配置或发送失败时，不能从数据库中找回可用 token。

## 运维强制重置

当管理员无法收邮件或忘记初始密码时，可以临时覆盖 `ADMIN_EMAIL` / `ADMIN_PASSWORD` 并执行 seed。新密码必须至少 8 位，并包含大小写字母和数字。

Docker Compose：

```bash
docker compose exec \
  -e ADMIN_EMAIL='admin@example.com' \
  -e ADMIN_PASSWORD='NewAdmin123!' \
  server /app/migrate seed
```

本地后端：

```bash
cd server
ADMIN_EMAIL='admin@example.com' ADMIN_PASSWORD='NewAdmin123!' go run ./cmd/migrate seed
```

`migrate seed` 会同步该管理员的密码、确保角色为 `admin` 并启用账号。它不会自动吊销已签发 token，重置后建议再让该账号的旧会话失效：

```bash
docker compose exec postgres sh -lc \
  'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "UPDATE users SET tokens_invalidated_at = NOW() WHERE email = '\''admin@example.com'\'';"'
```

如果是本地直连数据库，也可以执行同等 SQL：

```sql
UPDATE users
SET tokens_invalidated_at = NOW()
WHERE email = 'admin@example.com';
```

## 重要行为

仅修改根目录 `.env` 里的 `ADMIN_PASSWORD` 不一定会改变现有管理员密码：

- `server` 在 `release` 模式启动时只会创建缺失的管理员，不会覆盖已存在管理员的密码。
- `migrate seed` 会把已存在管理员的密码同步为当前 `ADMIN_PASSWORD`。
- 用户在 UI 中修改密码或使用 reset token 重置密码时，会自动吊销旧 token。
