# GoChat 后端

基于 Go + Gin + MySQL + Redis + Kafka + WebSocket 的实时即时通讯后端服务。

## 技术栈

- **Web 框架**: Gin
- **数据库**: MySQL (GORM)
- **缓存**: Redis
- **消息队列**: Kafka
- **实时通信**: WebSocket
- **认证**: JWT

## 功能特性

- 用户注册与登录
- 好友关系管理（添加、同意/拒绝、查看列表）
- 实时消息推送（WebSocket）
- 消息历史记录
- 未读消息计数（持久化存储）
- 在线状态显示

## 快速开始

### 环境要求

- Go 1.25+
- MySQL 8.0+
- Redis 6.0+
- Kafka 2.8+

### 配置

修改 `config/config.yaml` 文件：

```yaml
# 数据库配置
db:
  host: "localhost"
  port: 3306
  username: "root"
  password: "password"
  name: "go_chat"

# Redis 配置
redis:
  host: "localhost"
  port: 6379
  password: ""
  db: 0

# Kafka 配置
kafka:
  brokers:
    - "localhost:9092"
  topic: "chat_messages"

# 服务配置
server:
  host: "0.0.0.0"
  port: 8080
```

### 数据库初始化

```sql
-- 创建数据库
CREATE DATABASE go_chat DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

-- 执行数据库迁移（程序启动时自动创建表）
```

### 启动服务

```bash
cd cmd
go run main.go
```

服务启动后，API 文档地址：`http://localhost:8080/swagger/index.html`

## API 接口

### 公共接口（无需认证）

| 方法 | 路径 | 功能 |
|------|------|------|
| POST | `/api/user/register` | 用户注册 |
| POST | `/api/user/login` | 用户登录 |

### 私有接口（需 JWT 认证）

所有私有接口需要在 Header 中携带 Token：

```
Authorization: Bearer <token>
```

| 方法 | 路径 | 功能 |
|------|------|------|
| GET | `/api/user/info` | 获取当前用户信息 |
| GET | `/api/user/search` | 搜索用户 |
| GET | `/api/ws` | 建立 WebSocket 连接 |
| GET | `/api/chat/history` | 获取聊天历史记录 |
| POST | `/api/friend/request` | 发送好友申请 |
| POST | `/api/friend/handle` | 处理好友申请（同意/拒绝） |
| GET | `/api/friend/requests` | 获取待处理的好友申请列表 |
| GET | `/api/friend/list` | 获取好友列表（包含未读计数） |
| POST | `/api/friend/mark-read` | 标记消息已读 |

### 接口详情

#### 1. 用户注册

```http
POST /api/user/register
Content-Type: application/json

{
  "username": "testuser",
  "password": "123456",
  "email": "test@example.com"
}
```

#### 2. 用户登录

```http
POST /api/user/login
Content-Type: application/json

{
  "username": "testuser",
  "password": "123456"
}
```

**响应**:
```json
{
  "code": 0,
  "msg": "登录成功",
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIs...",
    "username": "testuser",
    "nickname": "昵称"
  }
}
```

#### 3. 获取好友列表（带未读计数）

```http
GET /api/friend/list
Authorization: Bearer <token>
```

**响应**:
```json
{
  "code": 0,
  "msg": "success",
  "data": [
    {
      "id": 2,
      "username": "friend1",
      "nickname": "好友昵称",
      "avatar": "https://example.com/avatar.png",
      "online": true,
      "unread_count": 5,
      "last_message_time": 1735737600000
    }
  ]
}
```

#### 4. 标记消息已读

```http
POST /api/friend/mark-read
Authorization: Bearer <token>
Content-Type: application/json

{
  "target_id": 2
}
```

**说明**: 当打开某好友的聊天窗口时调用，将该会话的 `last_read_msg_id` 更新为当前最新消息ID。

#### 5. WebSocket 消息

**连接**:
```http
ws://localhost:8080/api/ws?token=<token>
```

**发送消息**:
```json
{
  "target_id": 2,
  "content": "你好",
  "type": 0,
  "media": 0
}
```

**接收消息**:
```json
{
  "from_user_id": 1,
  "content": "你好",
  "type": 0,
  "media": 0,
  "send_time": 1699999999
}
```

## 项目结构

```
go-chat/
├── cmd/                    # 程序入口
│   └── main.go
├── config/                 # 配置文件
│   └── config.yaml
├── internal/
│   ├── api/                # API 处理器
│   │   ├── user_api.go
│   │   ├── chat_api.go
│   │   └── relation_api.go
│   ├── middleware/         # 中间件
│   │   └── jwt.go
│   ├── models/             # 数据模型
│   │   ├── user.go
│   │   ├── message.go
│   │   └── relation.go
│   ├── pkg/                # 工具包
│   │   ├── initial/        # 初始化
│   │   └── utils/          # 工具函数
│   ├── routers/            # 路由
│   │   └── router.go
│   └── service/            # 业务逻辑
│       ├── relation.go
│       ├── user_service.go
│       ├── chat_manager.go
│       └── dto.go
├── global/                 # 全局变量
│   └── global.go
├── logs/                   # 日志目录
├── data/                   # 数据目录
├── docs/                   # Swagger 文档
├── docker-compose.yml      # Docker 编排
└── go.mod
```

## 数据库表结构

### users 表
- `id`: 用户ID
- `username`: 用户名（唯一）
- `password`: 加密后的密码
- `nickname`: 昵称
- `avatar`: 头像URL
- `email`: 邮箱
- `created_at`: 创建时间

### messages 表
- `id`: 消息ID
- `from_user_id`: 发送者ID
- `to_user_id`: 接收者ID
- `content`: 消息内容
- `type`: 消息类型
- `media`: 媒体类型
- `created_at`: 创建时间

### relations 表
- `id`: 记录ID
- `owner_id`: 关系所有者ID
- `target_id`: 好友ID
- `type`: 关系类型（1=好友，2=拉黑）
- `desc`: 备注名
- `last_read_msg_id`: 该用户在当前会话中已读的最后一条消息ID

### friend_requests 表
- `id`: 申请ID
- `sender_id`: 发送者ID
- `receiver_id`: 接收者ID
- `remark`: 附言
- `status`: 状态（0=待处理，1=已同意，2=已拒绝）
- `created_at`: 创建时间

## Cookbook

### 1. 发送消息流程

```
1. 用户通过 WebSocket 连接到服务器
2. 前端发送消息 JSON 到 WebSocket
3. 服务器验证目标用户在线状态
4. 如果在线，通过 WebSocket 推送给目标用户
5. 消息持久化存储到 MySQL
```

### 2. 未读消息计数逻辑

```
1. 每个好友关系记录包含 last_read_msg_id 字段
2. 获取好友列表时，计算 target 发给我的消息中 id > last_read_msg_id 的数量
3. 当用户打开某好友聊天窗口时，调用 /api/friend/mark-read 更新 last_read_msg_id
```

### 3. 添加好友流程

```
1. 用户 A 发送好友申请 -> POST /api/friend/request
2. 用户 B 收到申请（通过轮询或WebSocket通知）
3. 用户 B 处理申请 -> POST /api/friend/handle
4. 如果同意，创建双向好友关系记录
```

## Docker 部署

```bash
# 使用 Docker Compose 启动所有服务
docker-compose up -d

# 查看日志
docker-compose logs -f go-chat
```

## License

MIT

---

本项目使用 [Claude Code](https://claude.com/claude-code) 和 [Gemini](https://gemini.google.com/) 辅助开发。
