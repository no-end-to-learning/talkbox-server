# TalkBox Server

Go + Gin + MySQL 聊天服务端，支持 REST API 和 WebSocket 实时通信。

## 功能特性

- 用户注册/登录（JWT 认证）
- 用户列表（客户端可直接私聊任意用户）
- 私聊和群聊
- 群组管理（成员、管理员、Bot）
- 多种消息类型（文字、图片、视频、文件、卡片）
- @提及和引用回复
- 消息搜索
- WebSocket 实时推送
- Bot API（Token 认证）
- 设备推送 Token 管理

## 项目结构

```
├── main.go              # 应用入口，路由注册
├── config/
│   └── config.go        # 配置加载
├── database/
│   └── mysql.go         # 数据库连接和建表
├── models/
│   ├── user.go          # 用户模型
│   ├── conversation.go  # 会话模型
│   ├── message.go       # 消息模型
│   └── bot.go           # Bot 模型
├── handlers/
│   ├── auth.go          # 认证接口
│   ├── user.go          # 用户接口
│   ├── conversation.go  # 会话接口
│   ├── message.go       # 消息接口
│   ├── file.go          # 文件接口
│   └── bot.go           # Bot 接口
├── middleware/
│   ├── auth.go          # JWT 认证中间件
│   ├── bot_auth.go      # Bot Token 认证中间件
│   └── cors.go          # CORS 中间件
├── websocket/
│   ├── hub.go           # WebSocket 连接管理
│   └── client.go        # WebSocket 客户端处理
└── utils/
    ├── jwt.go           # JWT 工具
    ├── token.go         # Token 生成
    └── response.go      # 响应格式化
```

## 环境要求

- Go 1.24+
- MySQL 8.0+

## 快速开始

### 1. 创建数据库

```sql
CREATE DATABASE talkbox CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

### 2. 设置环境变量

```bash
export PORT=8080
export MYSQL_DSN="root:password@tcp(localhost:3306)/talkbox?charset=utf8mb4&parseTime=True&loc=Local"
export JWT_SECRET="your-secret-key"
export UPLOAD_DIR="./uploads"
export CORS_ALLOWED_ORIGINS="http://localhost:1420,http://localhost:3000"
```

### 3. 运行

```bash
go mod tidy
go run .
```

### 4. 构建

```bash
go build -ldflags="-s -w" -o talkbox .
./talkbox
```

## 环境变量

| 变量 | 必填 | 说明 |
|------|------|------|
| PORT | 是 | 服务端口 |
| MYSQL_DSN | 是 | MySQL 连接字符串 |
| JWT_SECRET | 是 | JWT 签名密钥 |
| UPLOAD_DIR | 是 | 文件上传目录 |
| CORS_ALLOWED_ORIGINS | 是 | 允许的跨域来源 |

## API 接口

### 认证

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/auth/register | 注册 |
| POST | /api/auth/login | 登录 |
| POST | /api/auth/logout | 登出 |
| POST | /api/auth/refresh | 刷新 Token |

### 用户

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/users | 获取所有用户 |
| GET | /api/users/me | 获取当前用户 |
| PUT | /api/users/me | 更新当前用户 |
| POST | /api/users/me/avatar | 上传头像 |
| POST | /api/users/me/device | 注册设备 Token |
| DELETE | /api/users/me/device | 注销设备 Token |
| GET | /api/users/search | 搜索用户 |

### 会话

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/conversations | 会话列表 |
| POST | /api/conversations | 创建群聊 |
| POST | /api/conversations/private | 开始私聊 |
| GET | /api/conversations/:id | 会话详情 |
| PUT | /api/conversations/:id | 更新会话 |
| DELETE | /api/conversations/:id | 删除会话 |
| POST | /api/conversations/:id/members | 添加成员 |
| DELETE | /api/conversations/:id/members/:user_id | 移除成员 |
| PUT | /api/conversations/:id/members/:user_id | 更新成员角色 |
| POST | /api/conversations/:id/bots/:bot_id | 添加 Bot |
| DELETE | /api/conversations/:id/bots/:bot_id | 移除 Bot |

### 消息

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/conversations/:id/messages | 获取消息 |
| POST | /api/conversations/:id/messages | 发送消息 |
| GET | /api/conversations/:id/messages/search | 搜索消息 |

### 文件

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/files/upload | 上传文件 |
| GET | /files/:filename | 访问文件 |

### Bot

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/bots | Bot 列表 |
| POST | /api/bots | 创建 Bot |
| GET | /api/bots/:id | Bot 详情 |
| PUT | /api/bots/:id | 更新 Bot |
| DELETE | /api/bots/:id | 删除 Bot |
| POST | /api/bots/:id/token | 重新生成 Token |
| GET | /api/bots/:id/conversations | Bot 加入的群 |

### Bot API

Bot 使用独立的 Token 认证：

```bash
POST /api/bot/conversations/:conversation_id/messages
Authorization: Bearer <bot_token>
Content-Type: application/json

{"type": "text", "content": {"text": "Hello!"}}
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

## Docker 部署

```bash
# 使用 docker-compose
docker-compose up -d
```

需要创建 `.env` 文件：

```
MYSQL_PASSWORD=your_mysql_password
JWT_SECRET=your_jwt_secret
CORS_ALLOWED_ORIGINS=http://localhost:1420
```

## 技术栈

| 技术 | 版本 | 说明 |
|------|------|------|
| Go | 1.24+ | 编程语言 |
| Gin | 1.11+ | Web 框架 |
| MySQL | 8.0+ | 数据库 |
| gorilla/websocket | 1.5+ | WebSocket |
| golang-jwt | 5.3+ | JWT 认证 |

## License

MIT
