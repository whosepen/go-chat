# GoChat 后端

基于 Go + Gin + MySQL + Redis + Kafka + WebSocket 的分布式即时通讯后端服务。
本项目已完成微服务化改造，实现了核心业务解耦、配置规范化及高并发消息处理。

前端项目仓库：[github.com/whosepen](https://github.com/whosepen/go-chat)

## 技术栈

- **Web 框架**: Gin
- **RPC 框架**: gRPC (用于 OSS 微服务)
- **数据库**: MySQL (GORM)
- **缓存**: Redis (Cache Aside + Pub/Sub + LRU)
- **消息队列**: Kafka (Sarama)
- **对象存储**: MinIO
- **实时通信**: WebSocket
- **配置管理**: Viper (支持环境变量)
- **认证**: JWT

## 架构概览

系统拆分为三个独立部署的服务：

1.  **API Server (`cmd/server`)**: 
    - 处理 HTTP API 请求。
    - 管理 WebSocket 连接。
    - 生产 Kafka 消息。
    - 监听 Redis Pub/Sub 推送指令。
2.  **Consumer Service (`cmd/consumer`)**: 
    - 消费 Kafka 消息 (群聊/私聊)。
    - 消息持久化 (MySQL)。
    - 消息缓存更新 (Redis Append + LRU)。
    - 触发消息推送 (Publish to Redis)。
3.  **OSS Service (`cmd/oss`)**:
    - 基于 gRPC 提供文件上传凭证服务。
    - 集成 MinIO 对象存储。

## 功能特性

### 用户模块
- 用户注册与登录
- JWT Token 认证
- 用户信息管理与搜索

### 好友模块
- 好友申请与处理
- 好友列表与在线状态
- 未读消息计数

### 聊天模块
- 实时消息推送（WebSocket）
- 消息历史记录漫游 (Redis 热点缓存 + MySQL 持久化)
- 支持文本、图片、文件消息
- 消息已读标记

### 群组模块
- 创建与管理群组
- 群成员管理（踢人/禁言）
- 入群申请处理
- 群消息广播 (Kafka 异步解耦)

### OSS 模块
- 文件上传凭证获取 (Pre-signed URL)
- 头像/聊天文件存储

---

## 快速开始

### 1. 环境准备

确保本地已安装 `docker` 和 `docker-compose`。

```bash
# 启动基础设施 (MySQL, Redis, Kafka, MinIO, Zookeeper)
docker-compose up -d
```

### 2. 配置

配置文件位于 `config/config.yaml`。支持通过环境变量覆盖配置，例如：

```bash
export MYSQL_DSN="root:123456@tcp(127.0.0.1:3306)/go_chat?..."
export KAFKA_ADDR="localhost:9092"
```

### 3. 启动服务

**终端 1: 启动 OSS 服务**
```bash
go run cmd/oss.go
```

**终端 2: 启动 Consumer 服务**
```bash
go run cmd/consumer.go
```

**终端 3: 启动 API Server**
```bash
go run cmd/server.go
```

服务默认监听端口：
- API Server: `:8080`
- OSS gRPC: `:50051`

---

## API 接口文档

### 公共接口
| 方法 | 路径 | 功能 |
|------|------|------|
| POST | `/api/user/register` | 注册 |
| POST | `/api/user/login` | 登录 |

### 私有接口 (需 Header: `token: <jwt>`)

#### 用户 & 社交
| 方法 | 路径 | 功能 |
|------|------|------|
| GET | `/api/user/info` | 个人信息 |
| POST | `/api/friend/request` | 发送好友请求 |
| GET | `/api/friend/list` | 好友列表 |

#### 群组
| 方法 | 路径 | 功能 |
|------|------|------|
| POST | `/api/group/create` | 建群 |
| POST | `/api/group/join` | 申请入群 |
| GET | `/api/group/info` | 群信息 |

#### 文件上传
| 方法 | 路径 | 功能 |
|------|------|------|
| GET | `/api/media/upload` | 获取上传凭证 |

*注：完整接口请参考 `internal/api` 目录或 Swagger 文档。*

---

## 消息流程设计

### 群聊消息流

1.  **Client A** 通过 WebSocket 发送消息。
2.  **Server** 接收消息，进行基础校验（群成员权限等），将消息序列化后投递到 **Kafka** (`group_message` topic)。
3.  **Consumer** 消费 Kafka 消息：
    *   持久化到 **MySQL**。
    *   更新 **Redis** 缓存：使用 `RPush` 追加消息，并执行 `LTrim` 保持最新的 2000 条 (LRU)。
    *   发布通知到 **Redis Pub/Sub** (`chat:push` channel)。
4.  **Server** (所有实例) 收到 Pub/Sub 通知，查找本地连接的群成员 WebSocket，执行推送。
5.  **Client B** 收到消息。

### 历史消息漫游

1.  **Client** 请求 `/api/chat/history`，携带 `last_msg_id`。
2.  **Server**:
    *   若 `last_msg_id == 0` (拉取最新): 优先读取 Redis List。若 Redis 命中，返回最新的 100 条；若未命中，查 MySQL 并重建 Redis 缓存。
    *   若 `last_msg_id > 0` (翻页): 直接查询 MySQL (`id < last_msg_id`)。

---

## 项目结构

```
go-chat/
├── cmd/                    # 入口文件
│   ├── server.go           # API Server
│   ├── consumer.go         # Kafka Consumer
│   └── oss.go              # OSS Service
├── config/                 # 配置文件
├── global/                 # 全局单例
├── internal/
│   ├── api/                # HTTP Handler
│   ├── middleware/         # Gin 中间件
│   ├── models/             # GORM 模型
│   ├── oss/                # OSS 业务逻辑
│   ├── pkg/                # 基础设施初始化
│   ├── repository/         # 数据访问层 (DAO)
│   ├── routers/            # 路由定义
│   └── service/            # 核心业务逻辑
├── proto/                  # Protobuf 定义
└── docker-compose.yml      # 环境编排
```

## License

MIT
