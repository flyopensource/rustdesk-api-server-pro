Rustdesk Api Server Pro
============
[English](https://github.com/rustdesk/rustdesk) | [简体中文](https://github.com/lantongxue/rustdesk-api-server-pro/blob/master/README_CN.md)

这是一个基于开源RustDesk客户端的开源Api服务器，实现了客户端所有Api接口，并提供一个Web-UI用于管理数据。

![Dashboard](./img/1.jpeg "Dashboard")

> 我们致力于用最简单的代码和结构实现功能！

## 特性
- 同步RuskDesk版本（当前适配客户端：1.4.6）
- 纯Go实现所有接口
- 可视化管理界面
    - 国际化支持
    - 统计面板
    - 用户管理
    - 设备组与成员管理
    - 多套 RustDesk Server 配置与全局、设备组、单设备快捷切换
    - 设备最终生效配置预览
    - 两步验证 & 邮件验证码
    - 会话管理
    - 日志审计
- Android 无人值守设备自动注册，无需用户预先登录
- 设备签名鉴权的 heartbeat/sysinfo 上报和加密策略下发
- 单设备无人值守配置和服务器配置远程切换
- 服务器配置按“单设备 → 设备组 → 全局”确定性选择，单设备优先级最高
- 轻量化&跨平台
    - 最小sqlite即可
    - 支持主流操作系统和架构

## Android 无人值守和服务器配置策略

Provisioning Android 客户端首次启动后，从构建时注入的固定地址下载加密 `rud.cfg`，取得初始 API Server 地址并自动注册设备。注册成功后，设备使用自己的签名凭据访问 heartbeat、sysinfo 和策略接口，不依赖用户 Access Token，适用于无人值守主板。

后台设备页面支持：

- 维护多套完整的服务器配置，每套包含 ID Server、Relay Server、Server Key 和永久密码；
- 在全局、设备组或单设备范围快捷选择整套配置；
- 创建设备组并维护成员；一台设备最多属于一个设备组；
- 在单设备上手动开启无人值守并填写 Root 执行器；`auto` 会探测 `su`/`testsu`，也可填写其他单个可执行文件名称或绝对路径；
- 预览指定设备最终生效的服务器配置、来源及无人值守设置；
- 变更后提升全局策略 revision，由 heartbeat 给需要更新的设备下发完整策略；
- 永久密码加密存储，管理查询和预览接口只返回“是否已配置”，不返回明文。

服务器配置不是逐字段合并。每台设备只会选中一套完整配置，选择顺序固定为：单设备直接选择、所属且已启用的设备组、全局默认。范围未选择配置时才继续向上继承；被选择的配置已停用时也会继续回退。无人值守不参与全局或设备组继承，只保存在单设备上。

API Server 属于固定控制通道，不包含在服务器配置模板中，也不能通过策略修改。管理页面显示当前站点地址；Android 客户端实际使用构建时固定地址下载的加密 `rud.cfg` 中的 `provisioning_api_server`。切换服务器配置只改变 ID Server、Relay Server、Server Key 和永久密码。

> **公共仓库安全要求：** 不要把真实 `rud.cfg` 下载地址、SecretBox 密钥、设备初始注册密钥、签名私钥、Server Key 或永久密码写入源码、README、构建日志或 Git 历史。客户端构建值必须使用 GitHub Actions Secrets/Variables 注入；服务端密钥必须使用部署环境的 Secret 注入。构建后的 APK 必然包含客户端启动所需的固定地址和初始密钥，因此 APK 本身也应按部署凭据管理。



## 兼容性声明（RustDesk 1.4.6）
- 当前目标客户端基线：`1.4.6`
- 本轮已覆盖：
    - heartbeat/sysinfo 上报兼容
    - 版本能力门槛（`>=1.4.6` 启用 `translate_mode`）
    - 鉴权载荷兼容（必填严格校验，未知字段容忍）
    - `rustdesk install --version` 同时支持 `1.4.6` 与 `Branch_1.4.6`
- 验证命令：
    - `cd backend && go test ./...`
    - `cd soybean-admin && pnpm typecheck && pnpm lint && pnpm build`

## Playwright E2E（全栈联调）

- 覆盖场景：`login`、`devices`、`users`、`audit`
- 用例目录：`soybean-admin/tests/e2e`

### 前置条件

1. 启动后端并创建管理员账号：

```shell
cd backend
go run . sync
go run . user add admin admin123456 --admin
E2E_SKIP_CAPTCHA=true go run . start
```

2. 安装前端依赖与 Playwright 浏览器：

```shell
cd soybean-admin
pnpm i
npx playwright install chromium
```

### 执行测试

```shell
cd soybean-admin
E2E_ADMIN_USER=admin E2E_ADMIN_PASS=admin123456 pnpm test:e2e
```

### CI

- `build-release.yml` 已支持可选 Playwright 全栈 E2E 任务。
- 当前发布任务只构建 `linux-amd64.zip`，其中包含前端 `dist`、Go API Server 和 `server.yaml`；手动运行上传 Artifact，推送版本 Tag 时同时创建 Release。
- Windows、macOS 和 ARM64 发布包暂不构建，待有对应部署和验证需求后再启用。
- 通过 `workflow_dispatch` 触发时设置 `run_playwright_e2e=true`。

## 使用Docker部署（推荐）
1. 拉取镜像
```shell
docker pull ghcr.io/lantongxue/rustdesk-api-server-pro:latest
```
2. 创建配置
```shell
cat > /your/path/server.yaml <<EOF
signKey: "" # 推荐通过 RUD_API_SIGN_KEY 注入
debugMode: true # debug mode
db:
  driver: "sqlite"
  dsn: "./server.db"
  timeZone: "Asia/Shanghai" # setting the time zone fixes the database creation time problem
  showSql: false

  # driver: "mysql"
  # dsn: "root:123@tcp(localhost:3306)/test?charset=utf8mb4"
httpConfig:
  printRequestLog: true
  port: ":12345" # api server port

smtpConfig:
  host: "127.0.0.1"
  port: 1025
  username: "test"
  password: "test"
  encryption: "none" # none ssl/tls starttls
  from: "test@localhost.com"

jobsConfig:
  deviceCheckJob:
    duration: 30
EOF
```
3. 运行镜像
```shell
docker run \
--name rustdesk-api-server-pro \
-d \
-e ADMIN_USER=admin \ #管理员账号（可选）
-e ADMIN_PASS=yourpassword \ #管理员密码（可选）
-e TZ=Asia/Shanghai \ #必须与 server.yaml 中的"timeZone"设置匹配
-e RUD_API_SIGN_KEY='<至少32位随机字符串>' \
-p 8080:8080 \
-v /your/path/server.yaml:/app/server.yaml \
ghcr.io/lantongxue/rustdesk-api-server-pro:latest
```
4. 添加管理员账号（如果设置了用于初始化管理员账号密码的环境变量，此步骤可以忽略，但我仍推荐你使用此方式创建管理员账号，而不是通过环境变量初始化）
```shell
docker exec rustdesk-api-server-pro rustdesk-api-server-pro user add admin yourpassword --admin
```
> 容器镜像默认监听`8080`端口

> 默认配置文件路径`/app/server.yaml`，可以通过`-v`指定您自己的配置文件

### Docker compose
```yaml
services:
  rustdesk-api-server-pro:
    container_name: rustdesk-api-server-pro
    image: ghcr.io/lantongxue/rustdesk-api-server-pro:latest
    environment:
      - "ADMIN_USER=youruser"
      - "ADMIN_PASS=yourpassword"
      - "TZ=Asia/Shanghai"
      - "RUD_API_SIGN_KEY=${RUD_API_SIGN_KEY:?请配置至少32位随机签名密钥}"
    volumes:
      - ./server.yaml:/app/server.yaml
    network_mode: host
    restart: unless-stopped
```

### 环境变量

| 变量  | 默认值 | 说明 |
| :------------: | :------------: | :------------: |
|ADMIN_USER|-|默认管理员账号|
|ADMIN_PASS|-|默认管理员密码|
|TZ|-|容器操作系统时区；必须与 YAML 文件中的应用设置相匹配|
|RUD_API_SIGN_KEY|-|管理员 Token 签名密钥，至少 32 位；API Server 启动必需|
|RUD_DEVICE_ENROLLMENT_KEY_B64|-|无人值守设备注册使用的 32 字节 Base64 初始密钥|
|RUD_CFG_SIGN_SEED_B64|-|策略签名使用的 Ed25519 私有 seed；必须作为 Secret 保存|
|RUD_CFG_SECRETBOX_KEY_B64|-|策略加密使用的 32 字节 Base64 SecretBox 密钥|
|RUD_CFG_KEY_ID|android-v1|策略密钥版本标识，不是密钥本身|

升级后先运行 `rustdesk-api-server-pro sync` 创建或更新设备凭据和 `strategy_*` 策略表。新的策略实现不会迁移旧设备组或旧策略数据，需要在管理后台重新创建服务器配置、设备组和范围选择；用户、设备、设备凭据和审计等非策略数据保留。Provisioning Android 客户端通过 `POST /api/device/register` 自动注册，之后使用设备签名访问 `/api/device/heartbeat` 和 `/api/device/sysinfo`，不需要用户 Access Token。服务端的 `RUD_DEVICE_ENROLLMENT_KEY_B64` 必须与对应 APK 注入值一致，并应由部署环境的 Secret 管理，不得提交真实值。服务端禁用设备凭据后，该设备不能使用共享初始注册密钥自行恢复。

API Server 启动时会校验 Token 签名密钥。推荐只通过部署 Secret 设置 `RUD_API_SIGN_KEY`；未配置或长度不足 32 位时服务拒绝启动。已有私有部署仍可使用 `server.yaml` 的 `signKey`，环境变量优先级更高。

## 源代码编译
### 必要环境
- Golang >= 1.21.4
- NodeJs ~= 推荐最新LTS版本
- pnpm ~= 最新版

### 编译
1. 获取源码

```shell
git clone https://github.com/lantongxue/rustdesk-api-server-pro.git
```

2. 将 API Server 编译为 Linux amd64 静态可执行文件

```shell
cd backend
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w"
```

关闭 CGO 可避免 Linux 可执行文件依赖编译机的 glibc 版本。

3. 编译前端
```shell
cd soybean-admin && pnpm i && pnpm build
```

### 本地打包部署

安装 Go、Node.js、pnpm 及前端依赖后，在仓库根目录执行：

```shell
make build
```

`make build` 会在关闭 CGO 的情况下编译后端。生成的 Linux 可执行文件为静态链接，可在较旧发行版上运行，不会出现 `GLIBC_2.32 not found` 之类的错误。

输出目录为：

```text
build/
├── rustdesk-api-server-pro
├── server.yaml
└── dist/
```

部署前可在 Linux 上验证可执行文件：

```shell
file build/rustdesk-api-server-pro
ldd build/rustdesk-api-server-pro
```

`file` 应显示 `statically linked`，`ldd` 应显示 `not a dynamic executable`。

`server.yaml` 默认使用相对静态目录 `./dist`。从 `build` 目录运行时，API Server 可以直接提供前端页面；正式日常部署仍建议让 Caddy/Nginx 直接提供 `dist`，并把 `/api` 和 `/admin` 反向代理到仅监听本机的 API Server。

Caddy 示例（请替换域名）：

```caddyfile
api.example.com {
    encode zstd gzip

    @backend path /api /api/* /admin /admin/*
    reverse_proxy @backend 127.0.0.1:12345

    root * /opt/rustdesk-api-server-pro/dist
    try_files {path} /index.html
    file_server
}
```

验证码、登录及所有管理接口都走 `/admin/*`，设备接口走 `/api/*`。如果页面提示 `the backend request error`，先确认新 API Server 已在 `127.0.0.1:12345` 启动，再检查这两个路径是否确实被反向代理，而不是回退到 `index.html`。

```shell
cd build
export RUD_API_SIGN_KEY='<稳定保存的至少32位随机字符串>'
./rustdesk-api-server-pro sync
./rustdesk-api-server-pro user add admin '管理员密码' --admin
./rustdesk-api-server-pro start
```

### 运行

#### api-server
1. 同步数据表结构
```shell
rustdesk-api-server-pro.exe sync
```

2. 创建第一个账号
```shell
rustdesk-api-server-pro.exe user add admin yourpassword --admin
```
> --admin 是一个可选项，启用后添加的用户为管理员用户，否则是普通用户

3. 启动
```shell
rustdesk-api-server-pro.exe start
```
> 默认监听`8080`端口

#### Web管理界面
此步骤你需要一个WEB服务器软件（例如：nginx、apache等），通过将打包后的产物复制到WEB根目录即可。

一般情况下，打包后的产物在`soybean-admin/dist`目录中。

反向代理配置，你需要将在`nginx`或其他WEB服务器中配置反向代理，通过反向代理服务端才能正确访问到接口地址。

下面是`nginx`反向代理的参考配置：
```nginx
#PROXY-START /api for rustdesk client
location ^~ /api
{
    proxy_pass http://127.0.0.1:8080;
    proxy_set_header Host 127.0.0.1;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header REMOTE-HOST $remote_addr;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection $connection_upgrade;
    proxy_http_version 1.1;
    # proxy_hide_header Upgrade;

    add_header X-Cache $upstream_cache_status;
}
#PROXY-END/

#PROXY-START /admin for web-ui
location ^~ /admin
{
    proxy_pass http://127.0.0.1:8080/admin;
    proxy_set_header Host 127.0.0.1;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header REMOTE-HOST $remote_addr;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection $connection_upgrade;
    proxy_http_version 1.1;
    # proxy_hide_header Upgrade;

    add_header X-Cache $upstream_cache_status;
}
#PROXY-END/
```

## CLI命令行
```shell
Usage:
  rustdesk-api-server-pro [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  rustdesk    About rustdesk-server command
  start       Start the api-server
  sync        The api-server database synchronization
  user        User management

Flags:
  -h, --help   help for rustdesk-api-server-pro

Use "rustdesk-api-server-pro [command] --help" for more information about a command.
```

## 后续计划
后续会持续跟进RustDesk客户端，并实现对应接口，这将作为一个长期计划。

## 赞助

如果您觉得此项目对您有所帮助，不妨请开发者喝杯咖啡 :)

![Sponsorship](./soybean-admin/src/assets/imgs/sponsorships.png "Sponsorship")

**感谢您的赞助**

## 开源许可
>您可以查看完整的许可证 [这里](https://github.com/lantongxue/rustdesk-api-server-pro/blob/master/LICENSE)

本项目采用**MIT**许可条款。
