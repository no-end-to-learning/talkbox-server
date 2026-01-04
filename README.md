# TalkBox Server

Go + Gin + MySQL 实现的聊天服务端，支持 REST API 和 WebSocket 实时通信。

## 功能特性

- 用户注册/登录（JWT 认证）
- 好友系统（添加、接受、删除）
- 私聊和群聊
- 多种消息类型（文字、图片、视频、文件、卡片）
- @提及和引用回复
- 消息搜索
- WebSocket 实时推送
- Bot API（Token 认证）

## 项目结构

```
server/
├── main.go              # 入口文件
├── config/              # 配置管理
├── database/            # 数据库连接和表创建
├── models/              # 数据模型
├── handlers/            # API 处理器
│   ├── auth.go          # 认证
│   ├── user.go          # 用户
│   ├── friend.go        # 好友
│   ├── conversation.go  # 会话
│   ├── message.go       # 消息
│   ├── file.go          # 文件
│   └── bot.go           # Bot
├── middleware/          # 中间件
│   ├── auth.go          # JWT 认证
│   ├── bot_auth.go      # Bot Token 认证
│   └── cors.go          # CORS
├── websocket/           # WebSocket
│   ├── hub.go           # 连接管理
│   └── client.go        # 客户端处理
└── utils/               # 工具函数
```

## 环境要求

- Go 1.21+
- MySQL 8.0+

## 快速开始

### 1. 配置数据库

```sql
CREATE DATABASE talkbox CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

### 2. 设置环境变量

```bash
export MYSQL_DSN="root:password@tcp(localhost:3306)/talkbox?charset=utf8mb4&parseTime=True&loc=Local"
export JWT_SECRET="your-secret-key"
export PORT=8080
export UPLOAD_DIR="./uploads"
```

### 3. 构建运行

```bash
go mod tidy
go build -o talkbox .
./talkbox
```

## 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| PORT | 服务端口 | 8080 |
| MYSQL_DSN | MySQL 连接字符串 | root:root@tcp(localhost:3306)/talkbox |
| JWT_SECRET | JWT 签名密钥 | talkbox-secret-key |
| UPLOAD_DIR | 文件上传目录 | ./uploads |

## API 文档

### 认证

```bash
# 注册
POST /api/auth/register
Content-Type: application/json
{"username": "user", "password": "123456", "nickname": "昵称"}

# 登录
POST /api/auth/login
Content-Type: application/json
{"username": "user", "password": "123456"}

# 响应
{"code": 0, "data": {"token": "xxx", "user": {...}}}
```

### 用户

```bash
GET    /api/users/me           # 获取当前用户
PUT    /api/users/me           # 更新用户信息
POST   /api/users/me/avatar    # 上传头像
GET    /api/users/search?q=xxx # 搜索用户
```

### 好友

```bash
GET    /api/friends                  # 好友列表
GET    /api/friends/requests         # 好友请求列表
POST   /api/friends/request          # 发送好友请求
POST   /api/friends/accept/:user_id  # 接受好友请求
DELETE /api/friends/:user_id         # 删除好友
```

### 会话

```bash
GET    /api/conversations                       # 会话列表
POST   /api/conversations                       # 创建群聊
POST   /api/conversations/private               # 开始私聊
GET    /api/conversations/:id                   # 会话详情
PUT    /api/conversations/:id                   # 更新群信息
DELETE /api/conversations/:id                   # 解散群

POST   /api/conversations/:id/members           # 邀请成员
DELETE /api/conversations/:id/members/:user_id  # 移除成员
PUT    /api/conversations/:id/members/:user_id  # 设置管理员

POST   /api/conversations/:id/bots/:bot_id      # 添加 Bot
DELETE /api/conversations/:id/bots/:bot_id      # 移除 Bot
```

### 消息

```bash
GET  /api/conversations/:id/messages         # 获取消息
POST /api/conversations/:id/messages         # 发送消息
GET  /api/conversations/:id/messages/search  # 搜索消息
```

### 文件

```bash
POST /api/files/upload    # 上传文件
GET  /files/:filename     # 访问文件
```

### Bot 管理

```bash
GET    /api/bots                    # Bot 列表
POST   /api/bots                    # 创建 Bot
GET    /api/bots/:id                # Bot 详情
PUT    /api/bots/:id                # 更新 Bot
DELETE /api/bots/:id                # 删除 Bot
POST   /api/bots/:id/token          # 重新生成 Token
GET    /api/bots/:id/conversations  # Bot 加入的群
```

### Bot API

Bot 使用独立的 Token 认证：

```bash
# 发送消息
POST /api/bot/conversations/{conversation_id}/messages
Authorization: Bearer <bot_token>
Content-Type: application/json

# 文字消息
{"type": "text", "content": {"text": "Hello!"}}

# 卡片消息
{
  "type": "card",
  "content": {
    "color": "#00CC00",
    "title": "部署通知",
    "content": "生产环境部署成功",
    "note": "版本: v1.2.3",
    "url": "https://example.com"
  }
}
```

## WebSocket

### 连接

```
ws://localhost:8080/ws?token=<jwt_token>
```

### 消息格式

**客户端 → 服务端**
```json
{"action": "ping"}
{"action": "send_message", "conversation_id": "xxx", "type": "text", "content": {"text": "hello"}}
```

**服务端 → 客户端**
```json
{"event": "pong"}
{"event": "new_message", "data": {...}}
{"event": "mentioned", "data": {...}}
```

## 数据库表

- users - 用户
- conversations - 会话
- conversation_members - 会话成员
- messages - 消息
- mentions - @提及
- friendships - 好友关系
- bots - Bot
- bot_conversations - Bot 群关联
- device_tokens - 设备推送 Token

## Docker 部署

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o talkbox .

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/talkbox .
EXPOSE 8080
CMD ["./talkbox"]
```

```yaml
# docker-compose.yml
version: '3.8'
services:
  mysql:
    image: mysql:8.0
    environment:
      MYSQL_ROOT_PASSWORD: ${MYSQL_PASSWORD}
      MYSQL_DATABASE: talkbox
    volumes:
      - mysql_data:/var/lib/mysql

  server:
    build: .
    ports:
      - "8080:8080"
    depends_on:
      - mysql
    environment:
      - MYSQL_DSN=root:${MYSQL_PASSWORD}@tcp(mysql:3306)/talkbox?charset=utf8mb4&parseTime=True&loc=Local
      - JWT_SECRET=${JWT_SECRET}
    volumes:
      - ./uploads:/app/uploads

volumes:
  mysql_data:
```

## Git Commit 规范

### 格式要求

- 使用英文
- 第一行为简短标题（50 字符以内），概括改动内容
- 如有详细说明，空一行后使用列表形式描述
- 不要添加 AI 生成签名（如 `Generated with Claude Code`、`Co-Authored-By` 等）

## License

MIT
