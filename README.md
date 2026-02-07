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

### 用户模块
- 用户注册与登录
- JWT Token 认证
- 用户信息管理
- 用户搜索

### 好友模块
- 发送好友申请
- 处理好友申请（同意/拒绝）
- 获取好友列表
- 好友在线状态显示
- 未读消息计数

### 聊天模块
- 实时消息推送（WebSocket）
- 消息历史记录
- 消息已读标记
- 心跳保活

### 群组模块
- 创建群组（创建者自动成为群主）
- 群组信息管理
- 群成员管理
- 入群申请处理
- 群主/管理员权限控制
- 群邀请码

---

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

#### 用户模块

| 方法 | 路径 | 功能 |
|------|------|------|
| GET | `/api/user/info` | 获取当前用户信息 |
| GET | `/api/user/profile` | 获取完整用户信息 |
| GET | `/api/user/search` | 搜索用户 |

#### 好友模块

| 方法 | 路径 | 功能 |
|------|------|------|
| POST | `/api/friend/request` | 发送好友申请 |
| POST | `/api/friend/handle` | 处理好友申请（同意/拒绝） |
| GET | `/api/friend/requests` | 获取待处理的好友申请列表 |
| GET | `/api/friend/list` | 获取好友列表（包含未读计数） |
| POST | `/api/friend/mark-read` | 标记消息已读 |

#### 聊天模块

| 方法 | 路径 | 功能 |
|------|------|------|
| GET | `/api/ws` | 建立 WebSocket 连接 |
| GET | `/api/chat/history` | 获取聊天历史记录 |

#### 群组模块

| 方法 | 路径 | 功能 |
|------|------|------|
| POST | `/api/group/create` | 创建群组 |
| GET | `/api/group/info` | 获取群组信息 |
| PUT | `/api/group/info` | 修改群组信息 |
| GET | `/api/group/members` | 获取群成员列表 |
| POST | `/api/group/join` | 发送入群申请 |
| POST | `/api/group/handle-join` | 处理入群申请 |
| GET | `/api/group/requests` | 获取入群申请列表 |
| GET | `/api/group/my-groups` | 获取我的群组列表 |
| POST | `/api/group/quit` | 退出群组 |
| POST | `/api/group/kick` | 踢出群成员 |

---

## WebSocket 消息

### 连接

```
ws://localhost:8080/api/ws?token=<token>
```

### 发送消息

```json
{
  "type": 2,
  "target_id": 2,
  "content": "你好",
  "media": 1
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| type | int | 消息类型：2=单聊，3=群聊 |
| target_id | uint | 接收者ID（群聊为群ID） |
| content | string | 消息内容 |
| media | int | 媒体类型：1=文本，2=图片，3=音频 |

### 接收消息

```json
{
  "from_id": 1,
  "type": 2,
  "content": "你好",
  "send_time": 1699999999
}
```

---

## 快速开始

### 环境要求

- Go 1.25+
- MySQL 8.0+
- Redis 6.0+
- Kafka 2.8+

### 配置

修改 `config/config.yaml` 文件：

```yaml
server:
  host: "0.0.0.0"
  port: 8080
  mode: "debug"

mysql:
  host: "localhost"
  port: 3306
  username: "root"
  password: "password"
  name: "go_chat"
  dsn: "root:123456@tcp(127.0.0.1:3306)/go_chat?charset=utf8mb4&parseTime=True&loc=Local"

redis:
  host: "localhost"
  port: 6379
  password: ""
  db: 0

jwt:
  secret: "your-jwt-secret-key"
  expire: 72

kafka:
  brokers:
    - "localhost:9092"
  topic:
    chat: "chat_message"
    group: "group_message"
    retry: "chat_message_retry"
    dead: "chat_message_dead_letter"
```

### 数据库初始化

```sql
CREATE DATABASE go_chat DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

表结构会在程序启动时自动创建。

### 启动服务

```bash
# 使用 Docker Compose 启动基础设施
docker-compose up -d

# 编译并运行
go build -o go-chat ./cmd/ && ./go-chat

# 或直接运行
go run ./cmd/main.go
```

---

## 项目结构

```
go-chat/
├── cmd/                    # 程序入口
│   └── main.go
├── config/                 # 配置文件
│   └── config.yaml
├── docker-compose.yml      # Docker 编排
├── global/                 # 全局变量
│   └── global.go
├── internal/
│   ├── api/                # API 处理器
│   │   ├── user_api.go
│   │   ├── chat_api.go
│   │   ├── relation_api.go
│   │   └── group_api.go
│   ├── middleware/         # 中间件
│   │   └── jwt.go
│   ├── models/             # 数据模型
│   │   ├── user.go
│   │   ├── message.go
│   │   ├── relation.go
│   │   ├── group.go
│   │   ├── group_request.go
│   │   └── friend_request.go
│   ├── pkg/                # 工具包
│   │   ├── initial/        # 初始化
│   │   │   ├── config.go
│   │   │   ├── db.go
│   │   │   ├── redis.go
│   │   │   ├── kafka.go
│   │   │   └── logger.go
│   │   ├── utils/          # 工具函数
│   │   │   ├── response.go
│   │   │   ├── jwt.go
│   │   │   └── hashids.go
│   │   └── protocol/       # 协议定义
│   │       └── protocol.go
│   ├── routers/            # 路由
│   │   └── router.go
│   └── service/            # 业务逻辑
│       ├── user_service.go
│       ├── relation.go
│       ├── chat_manager.go
│       ├── group_service.go
│       ├── history_msg.go
│       ├── consumer.go
│       ├── dto.go
│       ├── converter.go
│       └── redis_key.go
├── markdown/               # 文档
│   └── API文档.md
├── logs/                   # 日志目录
├── data/                   # 数据目录
├── docs/                   # Swagger 文档
├── go.mod
└── README.md
```

---

## 数据库表结构

### users 表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | uint | 用户ID |
| username | string | 用户名（唯一） |
| password | string | 加密后的密码 |
| nickname | string | 昵称 |
| avatar | string | 头像URL |
| email | string | 邮箱 |
| last_login | time | 最后登录时间 |
| created_at | time | 创建时间 |
| updated_at | time | 更新时间 |

### messages 表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | uint | 消息ID |
| from_user_id | uint | 发送者ID |
| to_user_id | uint | 接收者ID（群ID） |
| content | string | 消息内容 |
| type | int | 消息类型：1=单聊，2=群聊 |
| media | int | 媒体类型：1=文本，2=图片，3=音频 |
| created_at | time | 创建时间 |

### relations 表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | uint | 记录ID |
| owner_id | uint | 关系所有者ID |
| target_id | uint | 好友ID |
| type | int | 关系类型：1=好友，2=拉黑 |
| remark | string | 备注名 |
| last_read_msg_id | uint | 已读消息ID |
| created_at | time | 创建时间 |

### friend_requests 表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | uint | 申请ID |
| sender_id | uint | 发送者ID |
| receiver_id | uint | 接收者ID |
| remark | string | 附言 |
| status | int | 状态：0=待处理，1=已同意，2=已拒绝 |
| created_at | time | 创建时间 |

### groups 表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | uint | 群组ID |
| code | string | 邀请码（用于分享加入） |
| name | string | 群名称 |
| icon | string | 群图标 |
| desc | string | 群描述 |
| owner_id | uint | 群主ID |
| member_count | int | 成员数量 |
| created_at | time | 创建时间 |
| updated_at | time | 更新时间 |

### group_members 表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | uint | 记录ID |
| group_id | uint | 群组ID |
| user_id | uint | 用户ID |
| nickname | string | 群内昵称 |
| role | int | 角色：1=群主，2=管理员，3=普通成员 |
| mute | int | 禁言状态：0=正常，1=禁言 |
| join_time | time | 入群时间 |

### group_requests 表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | uint | 申请ID |
| group_id | uint | 群组ID |
| user_id | uint | 申请人ID |
| remark | string | 附言 |
| status | int | 状态：0=待处理，1=已同意，2=已拒绝 |
| created_at | time | 创建时间 |

---

## 消息流程

### 单聊消息

```
1. 客户端通过 WebSocket 发送消息
2. 服务器验证目标用户在线状态
3. 如果在线，通过 WebSocket 推送给目标用户
4. 消息持久化存储到 MySQL
```

### 群聊消息

```
1. 客户端通过 WebSocket 发送群消息
2. 消息发送到 Kafka
3. Kafka Consumer 消费消息
4. 持久化到数据库
5. 推送给所有在线群成员
```

### 未读消息计数

```
1. 每个好友关系记录包含 last_read_msg_id 字段
2. 获取好友列表时，计算目标发给我的消息中 id > last_read_msg_id 的数量
3. 当用户打开某好友聊天窗口时，调用 /api/friend/mark-read 更新 last_read_msg_id
```

---

## License

MIT
