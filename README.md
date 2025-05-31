# go-chat

一个基于 websocket 技术的实时聊天应用程序，支持多人对话、文件发送。

## 主要特性

- 实时消息传递和接收
- 文件和图片共享
- 本地存储
- 群组聊天支持
- 配置热更新
- 中间件支持消息确认和重发机制
- 用户封禁
- 语音通话(未实现)
- 视频通话(未实现)

## 技术栈

- Gin
- Uber/zap 日志框架
- WebSocket 实时通信
- JWT 身份认证
- Bloomfilter 过滤封禁用户
- 文件流处理
- gorm
- Mysql 数据库
- Redis 缓存
- Docker 容器化

## 快速开始

### 拉取代码

```bash
git clone https://github.com/farnese17/go-chat.git
cd ./go-chat
```

### 环境准备

- go 环境
- mysql
- redis

### 编译

```bash
make build
```

#### 指定平台

```bash
make build-linux
make build-darwin
```

### 检查

```bash
./bin/gochat -version
```

### 生成默认配置

```bash
./bin/gochat -generate-config ./data/gochat/config/config.yaml
```

#### 修改数据库连接

```bash
database:
    host: "127.0.0.1"
    port: "3306"
    user: "root"
    password: "123456"
    db_name: "gochat"
cache:
    addr: "127.0.0.1:6379"
    password: ""
    db_num: 0
```

#### 启动

##### 启动后自动创建超级管理员，账号信息打印到终端和日志，请自行修改密码。

##### 注意：当数据库中不存在超级管理员时依然会自动创建

##### 1.配置环境变量(推荐)

```bash
export CHAT_CONFIG=./data/gochat/config/config.yaml
./bin/gochat
```

##### 2.使用启动参数

```bash
./bin/gochat -config ./data/gochat/config/config.yaml
```

##### 3.默认使用当前目录

```bash
./bin/gochat
```

## Docker

### 构建镜像

```bash
docker build -t gochat:`cat VERSION` .
```

### 部署

```bash
docker-compose up -d
```

## API 文档

查看详细的 API 文档请访问 `/api/README.md` 端点

| 端点                       | 方法 | 描述     | 认证 | 参数                                                                                  |
| -------------------------- | ---- | -------- | ---- | ------------------------------------------------------------------------------------- |
| `/api/v1/login`            | POST | 用户登录 | 否   | `{account:'id/phone/email',"password":"123456"}`                                      |
| `/api/v1/register`         | POST | 用户注册 | 否   | `{username:"test","password":"123456","phone":"12345678901","email":"test@mail.com"}` |
| `/api/v1/managers/healthy` | GET  | 健康状态 | 是   | `?details=true`                                                                       |
