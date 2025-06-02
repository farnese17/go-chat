# API 文档

统一前缀`/api/v1`

## 登录与登出

| 端点      | 方法 | 描述     | 认证 | 参数                                                                    |
| --------- | ---- | -------- | ---- | ----------------------------------------------------------------------- |
| `/login`  | POST | 用户登录 | 否   | <pre>{<br>"account":"id/phone/email",<br>"password":"123456"<br>}</pre> |
| `/logout` | POST | 用户登出 | 是   | -                                                                       |

## 用户

用户前缀`/users`

| 端点        | 方法   | 描述             | 认证 | 参数                                                                                                                   |
| ----------- | ------ | ---------------- | ---- | ---------------------------------------------------------------------------------------------------------------------- |
| `/register` | POST   | 用户注册         | 否   | <pre>{<br>"username":"test",<br>"password":"123456",<br>"phone":"12345678901",<br>"email":`"test@mail.com"`<br>}</pre> |
| `/`         | GET    | 获取当前用户信息 | 是   | -                                                                                                                      |
| `/search`   | GET    | 搜索用户         | 是   | `?account=id/phone/email`                                                                                              |
| `/`         | DELETE | 注销账号         | 是   | -                                                                                                                      |
| `/`         | PUT    | 更新当前用户信息 | 是   | <pre>{<br>"field":"avatar/username/phone/email",<br>"value":"value"<br>}</pre>                                         |
| `/password` | PUT    | 更新当前用户密码 | 是   | <pre>{<br>"old":"oldpwd",<br>"new":"newpwd",<br>"comfirm":"newpwd"<br>} </pre>                                         |

## 好友

好友前缀`/friends`

| 端点            | 方法   | 描述         | 认证 | 参数                                                                                       |
| --------------- | ------ | ------------ | ---- | ------------------------------------------------------------------------------------------ |
| `/:id`          | GET    | 获取好友信息 | 是   | `:user_id`                                                                                 |
| `/request/:id`  | POST   | 发起好友请求 | 是   | `:user_id`                                                                                 |
| `/accept/:id`   | PUT    | 接受好友请求 | 是   | `:user_id`                                                                                 |
| `/reject/:id`   | PUT    | 拒绝好友请求 | 是   | `:user_id`                                                                                 |
| `/:id`          | DELETE | 删除好友     | 是   | `:user_id`                                                                                 |
| `/block/:id`    | PUT    | 拉黑用户     | 是   | `:user_id`                                                                                 |
| `/unblock/:id`  | PUT    | 取消拉黑     | 是   | `:user_id`                                                                                 |
| `/remark/:id`   | PUT    | 设置好友备注 | 是   | `:user_id`<br>`?remark=newValue`                                                           |
| `/setgroup/:id` | PUT    | 设置好友分组 | 是   | `:user_id`<br>`?group=newGroup`                                                            |
| `/search`       | GET    | 搜索用户     | 是   | `?value=id/name`<br><pre>{<br>"page_size:10,<br>"last_id":0,<br>"has_more":true<br>}</pre> |
| `/`             | GET    | 获取好友列表 | 是   | -                                                                                          |

## 群组

群组前缀`/groups`

| 端点                            | 方法   | 描述                                             | 认证 | 参数                                                                                                                                                                         |
| ------------------------------- | ------ | ------------------------------------------------ | ---- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `/`                             | POST   | 创建群组                                         | 是   | <pre>{<br>"name":"group_name",<br>"desc":"group_desc"<br>}</pre>                                                                                                             |
| `/`                             | GET    | 获取当前用户群组列表                             | 是   | -                                                                                                                                                                            |
| `/:gid`                         | GET    | 根据 group_id 获取群组信息                       | 是   | `:gid`                                                                                                                                                                       |
| `/search`                       | GET    | 根据 group_name 搜索群组                         | 是   | `?name=group_name` <br><pre>{<br>"page_size:10,<br>"last_id":0,<br>"has_more":true<br>}</pre>                                                                                |
| `/:gid/invitations/:id`         | POST   | 邀请用户，需主要群主或管理员权限                 | 是   | `:group_id`<br>`:user_id`                                                                                                                                                    |
| `/:gid/invitations/accept`      | PUT    | 接受加入邀请                                     | 是   | <pre>{<br>"type":message_type,<br>"from":message_from,<br>"time":message_time,<br>"to":message_to,<br>"extra":message_extra<br>}</pre> 注: message 来自 websocket 收到的邀请 |
| `"/:gid/applications"`          | POST   | 申请加入群组                                     | 是   | `:group_id`                                                                                                                                                                  |
| `/:gid/applications/:id/accept` | PUT    | 接受加入申请，需主要群主或管理员权限             | 是   | `:group_id`<br>`:user_id`                                                                                                                                                    |
| `/:gid/applications/:id/reject` | PUT    | 拒绝加入申请，需主要群主或管理员权限             | 是   | `:group_id`<br>`:user_id`                                                                                                                                                    |
| `/:gid/owner/:id`               | PUT    | 移交群主                                         | 是   | `:group_id`<br>`:user_id`                                                                                                                                                    |
| `/:gid/members/:id`             | GET    | 获取群组成员信息                                 | 是   | `:group_id`<br>`:user_id`                                                                                                                                                    |
| `/:gid/members`                 | GET    | 获取群组成员列表                                 | 是   | `:group_id`                                                                                                                                                                  |
| `/:gid`                         | DELETE | 解散群组,需要群主权限                            | 是   | `:group_id`                                                                                                                                                                  |
| `/:gid`                         | PUT    | 更新群组信息，需主要群主或管理员权限             | 是   | `:group_id`<br>`?field=name/desc`<br>`?value=newValue`                                                                                                                       |
| `/:gid/admins/:id`              | PUT    | 设置或撤销管理员,需要群主权限                    | 是   | `:group_id`<br>`:user_id`<br>`?role=admin/member`<br> <pre>v0.9.0+:<br> admin=2<br> member=3</pre>                                                                           |
| `/:gid/admins/me/resign`        | PUT    | 主动撤销管理员                                   | 是   | `:group_id`                                                                                                                                                                  |
| `/:gid/members/me`              | DELETE | 离开群组                                         | 是   | `:group_id`                                                                                                                                                                  |
| `/:gid/members/:id`             | DELETE | 踢出群组，需主要群主或管理员权限，不能踢出管理员 | 是   | `:group_id`<br>`:user_id`                                                                                                                                                    |
| `/:gid/announces`               | POST   | 发布公告,需要群主或管理员权限                    | 是   | `:group_id`<br><pre>{<br>"group_id":group_id,<br>"content":"something"<br>}</pre>                                                                                            |
| `/:gid/announces`               | GET    | 获取公告,需要在群组内                            | 是   | `:group_id`<br><pre>{<br>"page_size":10,<br>"last_id":0,<br>"has_more":true<br>}</pre>                                                                                       |
| `/:gid/announces/latest`        | GET    | 获取最新一条公告,需要在群组内                    | 是   | `:group_id`                                                                                                                                                                  |
| `/:gid/announces/:id`           | DELETE | 删除一条公告,需要群组或管理员权限                | 是   | `:group_id`<br>`:announce_id`                                                                                                                                                |

## 文件

| 端点                  | 方法   | 描述     | 认证 | 参数                            |
| --------------------- | ------ | -------- | ---- | ------------------------------- |
| `/files`              | POST   | 上传文件 | 是   | form-data: `file:/paht/to/file` |
| `/files/:id`          | GET    | 获取文件 | 否   | `:file_id `                     |
| `/files/download/:id` | GET    | 下载文件 | 否   | `:file_id `                     |
| `/files/delete/:id`   | DELETE | 删除文件 | 是   | `:file_id `                     |

## 管理员

管理 api 前缀`/managers`

| 端点                          | 方法   | 描述                              | 认证 | 参数                                                                                           |
| ----------------------------- | ------ | --------------------------------- | ---- | ---------------------------------------------------------------------------------------------- |
| `/healthy`                    | GET    | 服务状态                          | 是   | `?details=true`(可选)                                                                          |
| `/admins`                     | POST   | 创建管理,需要超级管理员权限       | 否   | <pre>{<br>"username":name,<br>"password":"aaaaaa",<br>"email":`"manager@email.com"`<br>}</pre> |
| `/login`                      | POST   | 管理员登录                        | 否   | <pre>{<br>"id":mgr_id,<br>"password":"aaaaaa"<br>}</pre>                                       |
| `/admins`                     | GET    | 获取管理员列表                    | 是   | <pre>{<br>"page_size":10,<br>"last_id":0,<br>"has_more":true<br>}</pre>                        |
| `/admins/:id`                 | GET    | 获取管理员信息                    | 是   | `:manager_id`                                                                                  |
| `/admins/:id/update/password` | PUT    | 修改管理员密码                    | 是   | `:manager_id`<br><pre>{<br>"new":"newpwd",<br>"comfirm":"newpwd"<br>} </pre>                   |
| `/admins/:id`                 | DELETE | 删除管理员,需要超级管理员权限     | 是   | `:manager_id`                                                                                  |
| `/admins/:id/restore`         | PUT    | 恢复管理员,需要超级管理员权限     | 是   | `:manager_id`                                                                                  |
| `/admins/:id/permissions`     | PUT    | 设置管理员权限,需要超级管理员权限 | 是   | `:manager_id`<br>`?permission=/4/6/7`<br> 4=只读<br>6=读写<br>7=超级管理员                     |
| `/ws/stop`                    | PUT    | 停止 websocket 服务               | 是   | -                                                                                              |
| `/ws/start`                   | PUT    | 启动 websocket 服务               | 是   | -                                                                                              |
| `/config`                     | GET    | 获取配置                          | 是   | -                                                                                              |
| `/config/set`                 | PUT    | 修改配置                          | 是   | <pre>{<br>"section":"commom/cache",<br>"key":"",<br>"value":newValue<br>}</pre>                |
| `/config/save`                | PUT    | 保存配置                          | 是   | -                                                                                              |
| `/users/banned`               | GET    | 获取已封禁用户列表                | 是   | <pre>{<br>"page_size":10,<br>"last_id":0,<br>"has_more":true<br>}</pre>                        |
| `/users/banned/count`         | GET    | 统计封禁用户数量                  | 是   | -                                                                                              |
| `/users/:id/ban/temp`         | PUT    | 临时封禁用户                      | 是   | `:user_id`                                                                                     |
| `/users/:id/ban/perma`        | PUT    | 永久封禁用户                      | 是   | `:user_id`                                                                                     |
| `/users/:id/ban/nopost`       | PUT    | 禁止发布                          | 是   | `:user_id`                                                                                     |
| `/users/:id/ban/mute`         | PUT    | 禁言                              | 是   | `:user_id`                                                                                     |
| `/users/:id/ban/unban`        | PUT    | 撤销封禁                          | 是   | `:user_id`                                                                                     |

## websocket

| 端点  | 方法 | 描述         | 认证 | 参数         |
| ----- | ---- | ------------ | ---- | ------------ |
| `/ws` | GET  | 建立 ws 连接 | 是   | Bearer token |

websocket 服务主动关闭连接: `1012 Service Restart:服务已停止`<br>
websocket 服务不可用: `Error: Unexpected server response: 503`

### 消息类型

|     |                |
| --- | -------------- |
| 100 | 系统消息       |
| 101 | 聊天消息       |
| 102 | 群聊消息       |
| 103 | 黑名单更新消息 |
| 104 | 消息确认消息   |
| 207 | 群组申请消息   |

### 消息结构

| 适用类型            | 结构                                                                                                                                                                                                                                                     |
| ------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 100,101,102,103,207 | <pre>{<br>"type":message_type,<br>"body":<br>{<br>"id":"message_id"(留空),<br>"type":message_type,<br>"from":sender,<br>"to":receiver,<br>"body":"content",<br>"time":unix_time(可留空),<br>"extra":"other"<br>}<br>}</pre><br> 注: 以上留空由中间件填充 |
| 104                 | <pre>{"type":message_type,<br>"body":<br>{<br>"id":"message_id",<br>"type":message_type,<br>"to":receiver,<br>"time":unix_time,<br>}<br>}</pre><br>注: 字段值从收到的消息获取                                                                            |

#### 消息确认

go-chat 在收到消息处理完毕后，会返回一条 104 类型的消息，应该自行设置一个间隔，在没有收到确认消息后重发。
结构如下:

```json
// message来自收到的消息
{
  "type": 104,
  "id": "message_id",
  "to": "message_from" // 只是用于发送
}
```

go-chat 在发送消息后，会等待确认消息，在配置项`check_ack_timeout`时间过后，会重新发送,所以在收到除 104 类型消息外，都应该返回一条确认消息。结构如下:

```json
// message来自收到的消息
{
  "type": 104,
  "body": {
    "type": 104,
    "id": "message_id",
    "time": "message_time",
    "to": "message_to"
  }
}
```

#### 黑名单更新消息

由前端快速拦截
用户登录后获取好友列表<br>`blocked`表示已拉黑的用户<br>`other_blocked`表示拉黑该用户的用户<br>
比如:<br>
通过 http 请求拉黑用户后，如果 go-chat 没有返回错误，应该将用户移到`blocked`分组<br>
之后会通过 websocket 收到一条消息类似`将 xx 添加到黑名单`<br>
对方会收到 103 类型消息,对方在收到消息后应该将`from`移到`other_blocked`<br>
撤销拉黑则是将`blocked`分组的对应用户移除,对方将`other_blocked`分组对应的用户移除<br>
结构如下:

```json
{
  "type": 103,
  "from": "requester_id",
  "to": "target_id",
  "extra": true // 拉黑
}
```

```json
{
  "type": 103,
  "from": "requester_id",
  "to": "target_id",
  "extra": false // 撤销拉黑
}
```
