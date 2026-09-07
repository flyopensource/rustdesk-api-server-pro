# 设备管理实施记录

范围：开发阶段按当前数据结构实现，不做历史版本迁移。每步本地自测并单独提交；真实服务端、官方客户端和主板的端到端联调暂缓。

## 第 1 步：设备停用与启用

- 已实现设备状态、管理接口、Web 状态筛选和确认操作。
- 停用同时禁用设备凭据；普通设备无签名凭据时仍可通过设备状态停用。
- 签名注册/心跳/信息上报及普通心跳/信息上报检查停用状态。
- 操作审计保存在 `device_operation`，与状态变更同一事务，重复请求不重复记审计。
- 状态不表示 hbbs 已断开连接，也不表示客户端已清除缓存策略。
- 本地验证：Go app 与模拟 HTTP/SQLite 测试、Web TypeScript 类型检查。实际联调未执行。

## 第 2 步：无人值守文件权限

- 私有 Android 构建使用 `-PmanagedStorage=true`，本地 release 脚本自动启用；默认构建仍移除存储权限。Android 10 使用 legacy storage，Android 11+ 检查实际 all-files app-op，旧系统检查运行时读写权限。
- 沿用已校验的 Root 执行器与超时机制尝试授权，复检失败不报成功；客户端提供手动授权入口，并定期检查权限。
- 签名心跳持久化 `all_files_access_ready`，Web 展示实际权限状态和上报时间。
- 无人值守且权限就绪时开放主共享存储，保留私有目录访问；阻止路径穿越、越界符号链接和 `Android` 子目录。共享存储根目录只供浏览，批量操作需选择具体子目录。
- 关闭无人值守即关闭共享存储访问通道，不自动撤销 OS 权限，避免误撤销用户此前授权。
- 本地验证：Go app/HTTP/SQLite 测试、Web 类型检查、Dart 分析（只有既有弃用提示）、Kotlin 编译、受管/默认清单合并、独立 Rust 路径边界测试。
- 未完成：完整 Rust/原生 APK 构建、真机授权与远程文件传输、Android 11+ 设备验证。已连接主板为 Android 7.1/API 25，本步未安装或替换 APK、未执行授权命令。

## 第 3 步：Web 别名与地址簿发布

- Web 设备列表提供别名列、搜索、编辑弹窗和发布目标多选；支持中文、修剪首尾空格、清空，拒绝控制字符和超过 128 字的名称。
- 管理员明确选择账号已有的个人地址簿；目标账号需先登录客户端创建地址簿。不发布共享地址簿，不扩大普通账号读取权限。
- 别名与所选 `peer` 的关联及审计在同一事务中保存；以账号、地址簿、实际 ID 唯一索引防重复。
- 接管已有条目只改别名及管理关联，保留密码/标签等；取消目标时，删除本功能创建的条目，已有个人条目保留当前名称并解除管理。
- 官方客户端的逐条和整本地址簿接口仍可读取别名；回写不能覆盖 Web 名称或删除受管条目，但逐条更新仍可保存密码/标签。真正的 Custom ID 与 hbbs 修改不在范围内。
- 设备上报只更新设备报告字段，不覆盖管理别名或策略。
- 本地验证：Go 模拟 HTTP/SQLite 测试（输入、权限、两种读取、旧值回写、重复发布、解除发布、保留私有字段、事务/审计回滚）和 Web 类型检查通过。
- 官方安装包登录、刷新、搜索及实际连接联调暂缓，服务端保存不等于客户端已经刷新。

## 第 4 步：单设备删除

- 仅已停用、离线设备可删除；管理员接口校验完整 RustDesk ID，Web 危险操作弹窗要求手输确认。
- 条件删除复核停用/离线状态，事务内清理设备、凭据和受管地址簿关联，失败全部回滚。
- 分组和设备级配置关联已内置在设备行；不删除共享的设备组/服务器配置。已有个人条目保留并解除管理，仅管理功能新建的条目移除。
- 保留旧审计，新增删除审计包含实际 ID、别名和原组/配置数字 ID，不记录凭据。
- 页面明确提示物理删除不是永久封禁，设备可再次注册，客户端缓存不保证立即删除。
- 本地验证：Go 测试覆盖未授权、未停用、在线、ID 错误、不存在、重复删除、关联清理和审计失败回滚；Web 类型检查通过。未执行实际设备删除或联调。

## 第 5 步：本地总验证

- Go 全量测试与 race 检查通过；新加并发一致性测试在 race 模式重复 5 次通过。
- 重复测试发现 SQLite 多连接写事务的锁升级/提交冲突；生产 SQLite 连接池改为单连接串行事务，测试使用同一连接池配置。MySQL 连接池不变，未连接真实 MySQL 验证。
- Web TypeScript 检查、生产构建通过；Chrome 隔离上下文及全模拟接口页面测试通过，覆盖取消编辑不提交、保存别名与目标参数、停用、删除 ID 输入校验和删除后刷新。
- Dart 分析只有 3 条既有 MaterialStateProperty 弃用提示；受管 Android Kotlin 编译和默认/受管清单合并通过。
- ARMv7 Android Rust `cargo ndk check --release --lib --features flutter,hwcodec` 通过（有编译警告），独立路径边界测试通过；未链接打包新的完整 APK。
- TODO 将已实现、本地测试通过和暂缓联调分开记录。没有对真实 API/hbbs 写入，没有安装 APK 或改变主板权限/文件。

### 复测命令

在 API 仓库的 `backend` 目录：

```sh
go test ./...
go test -race ./app/controller/... ./test/api/...
go test -race -count=5 ./test/api/...
```

在 API 仓库的 `soybean-admin` 目录（模拟页面测试需要本机 Chrome）：

```sh
pnpm typecheck
pnpm build
pnpm exec playwright test -c playwright.mock.config.ts
```

在客户端仓库（沿用已配置的 NDK、VCPKG；Dart 使用 Flutter 3.27.3）：

```sh
VCPKGRS_TRIPLET=arm-neon-android cargo ndk --platform 21 --target armv7-linux-androideabi check --locked --offline --release --lib --features flutter,hwcodec
rustc --edition=2021 --test src/platform/android_storage.rs -o /tmp/rustdesk-storage-tests
/tmp/rustdesk-storage-tests
dart analyze flutter/lib/models/server_model.dart flutter/lib/mobile/pages/server_page.dart
```

在客户端 `flutter/android` 目录，配置 Android Studio JBR 为 `JAVA_HOME`：

```sh
./gradlew :app:compileDebugKotlin -PmanagedStorage=true -Ptarget-platform=android-arm --offline --console=plain
./gradlew :app:processDebugManifest -PmanagedStorage=false -Ptarget-platform=android-arm --offline --console=plain
```

### 分步提交

| 步骤 | 仓库 | 提交 |
| --- | --- | --- |
| 1 停用/启用 | API | `5c3932a` |
| 2 权限状态上报 | API | `76d5e18` |
| 2 Android 权限与文件边界 | 客户端 | `53c572572` |
| 3 Web 别名与地址簿 | API | `d7cbe07` |
| 4 单设备删除 | API | `0290142` |

第 5 步由包含本节的测试/文档提交记录。
