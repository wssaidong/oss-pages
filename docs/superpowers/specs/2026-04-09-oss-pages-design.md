# OSS Pages 部署平台设计

## 1. 概述

### 1.1 背景与目标

提供一个类似 Pages 的静态网站部署平台：后端服务接收 zip 包并部署到 S3 兼容存储，配套 CLI 工具完成打包和发布流程。

### 1.2 技术选型

| 组件 | 技术 | 说明 |
|------|------|------|
| 后端 | Go + Gin | 轻量、高性能、单二进制部署 |
| CLI | Go + Cobra | 与后端共享部分代码库 |
| 配置 | viper | 支持 toml + env override |
| 存储 | S3 兼容 | AWS S3 / 阿里云 OSS / 自建 MinIO |
| 数据库 | 无 | 利用 S3 存储元数据文件 |

---

## 2. 架构设计

### 2.1 系统架构

```
┌─────────────┐      POST /deploy       ┌─────────────┐      PutObject       ┌─────────────┐
│  oss-cli    │ ──────────────────────► │  oss-server │ ──────────────────► │  S3 兼容    │
│  (Go CLI)   │      multipart/zip      │  (Go API)   │                     │  Storage    │
└─────────────┘                         └─────────────┘                     └─────────────┘
       │                                       │
       │ read wrangler.toml                    │ read config.yaml
       ▼                                       ▼
┌─────────────┐                         ┌─────────────┐
│  wrangler   │                         │   config    │
│   .toml     │                         │   .yaml     │
└─────────────┘                         └─────────────┘
```

### 2.2 CLI 流程

1. 读取 `wrangler.toml` 获取构建命令和输出目录
2. 执行构建命令（如 `npm run build`）
3. 将 `output_directory` 的**内容**打包为 zip（**不包含顶层目录前缀**）
   - `dist/index.html` → zip 内路径 `index.html`
   - `dist/static/app.js` → zip 内路径 `static/app.js`
4. POST 到后端 `/deploy`，multipart 字段：`project`=项目名，`file`=zip 包

### 2.3 后端流程

1. 接收 zip 包
2. 实时流式解压（内存友好，不落地）
3. 逐文件上传到 S3
4. 返回部署结果（CDN URL）

---

## 3. API 设计

### 3.1 端点列表

| 端点 | 方法 | 描述 |
|------|------|------|
| `/deploy` | POST | 接收 zip 包，部署到 OSS |
| `/projects` | GET | 列出已部署项目 |
| `/projects/:name` | GET | 获取项目详情 |
| `/projects/:name` | DELETE | 删除项目 |

### 3.2 POST /deploy

**请求：**
```
Content-Type: multipart/form-data

字段：
  project: "my-app"       (string, 必填)  项目名
  file:   <zip binary>    (file,   必填)  zip 包
```

> `X-Project-Name` Header 已废弃，统一使用 multipart 表单字段 `project`。

**响应（成功 200）：**
```json
{
  "success": true,
  "project": "my-app",
  "url": "https://cdn.example.com/my-app/",
  "files": 42,
  "deployed_at": "2026-04-09T12:00:00Z"
}
```

**响应（失败 4xx/5xx）：**
```json
{
  "success": false,
  "error": "invalid zip format",
  "code": "INVALID_ZIP"
}
```

### 3.3 GET /projects

**响应：**
```json
{
  "projects": [
    {
      "name": "my-app",
      "url": "https://cdn.example.com/my-app/",
      "deployed_at": "2026-04-09T12:00:00Z"
    }
  ]
}
```

### 3.4 GET /projects/:name

**响应：**
```json
{
  "name": "my-app",
  "url": "https://cdn.example.com/my-app/",
  "files": 42,
  "deployed_at": "2026-04-09T12:00:00Z"
}
```

### 3.5 DELETE /projects/:name

**行为（严格按顺序执行）：**
1. 检查项目是否存在于 `_projects.json`，不存在返回 404
2. **先删除 S3 文件**：列举 `my-app/` 前缀下所有对象，批量删除
3. **再更新元数据**：下载 `_projects.json`，移除项目记录，上传（带 If-Match ETag）
   - 若 ETag 冲突则重试（最多 3 次），每次重新下载最新版本
   - 若重试耗尽，返回 503，提示用户重试（文件已删除，仅元数据残留）

**响应（成功 200）：**
```json
{
  "success": true,
  "deleted": "my-app"
}
```

---

## 4. CLI 命令设计

### 4.1 命令列表

```bash
# 初始化项目（生成 wrangler.toml）
oss-cli init

# 部署（读取 wrangler.toml，执行构建，打包，上传）
oss-cli deploy

# 指定配置文件
oss-cli deploy --config prod.toml

# 查看项目列表
oss-cli projects list

# 查看项目详情
oss-cli projects view <name>

# 删除项目
oss-cli projects delete <name>
```

### 4.2 wrangler.toml 示例

```toml
name = "my-app"
compatibility_date = "2024-01-01"

[pages]
build_command = "npm run build"
output_directory = "dist"

[deploy]
server_url = "https://api.example.com"  # CLI 上传地址
```

### 4.3 Server URL 配置优先级

`deploy` 命令：
1. 命令行参数 `--server`
2. 环境变量 `OSS_SERVER_URL`
3. wrangler.toml 中的 `[deploy].server_url`
4. 默认 `http://localhost:8080`（仅开发环境）

`projects` 子命令（不读 wrangler.toml）：
1. 命令行参数 `--server`
2. 环境变量 `OSS_SERVER_URL`
3. 默认 `http://localhost:8080`

---

## 5. 项目结构

```
oss-pages/
├── cmd/
│   ├── cli/              # CLI 入口
│   │   └── main.go
│   └── server/           # 后端入口
│       └── main.go
├── internal/
│   ├── cli/
│   │   ├── init.go      # oss-cli init
│   │   ├── deploy.go    # oss-cli deploy
│   │   └── projects.go  # oss-cli projects
│   ├── server/
│   │   ├── handler/      # HTTP handlers
│   │   │   ├── deploy.go
│   │   │   └── projects.go
│   │   ├── deployer/    # 解压 + S3 上传
│   │   │   └── deployer.go
│   │   └── storage/     # S3 存储抽象
│   │       └── s3.go
│   └── config/
│       ├── loader.go     # wrangler.toml 解析
│       └── viper.go     # 配置加载
├── pkg/
│   └── s3client/         # S3 客户端封装
├── wrangler.toml.example # 配置示例
├── config.yaml.example   # 服务端配置示例
├── go.mod
└── go.sum
```

---

## 6. 错误处理策略

| 场景 | 客户端处理 | 服务端处理 |
|------|-----------|-----------|
| 构建失败 | 打印 stderr，退出码非零 | N/A |
| 网络中断 | 重试 3 次，指数退避 | N/A |
| zip 格式错误 | 显示错误信息 | 返回 400 INVALID_ZIP |
| S3 上传失败 | 显示错误信息 | 返回 502 UPLOAD_FAILED |
| 项目名冲突 | 显示错误信息 | 返回 409 CONFLICT |
| 项目不存在 | 显示错误信息 | 返回 404 NOT_FOUND |
| 并发部署冲突 | 等待后重试或提示用户 | 返回 409 DEPLOYMENT_IN_PROGRESS |
| 元数据更新失败 | 显示错误信息，建议重试 | 返回 503 META_UPDATE_FAILED |

---

## 7. 配置设计

### 7.1 服务端配置（config.yaml）

```yaml
server:
  port: 8080
  host: "0.0.0.0"

s3:
  endpoint: "https://s3.example.com"
  bucket: "my-bucket"
  region: "us-east-1"
  access_key: ""   # 通过环境变量 S3_ACCESS_KEY 覆盖
  secret_key: ""   # 通过环境变量 S3_SECRET_KEY 覆盖
  path_prefix: ""  # 可选，CDN 路径前缀

# 环境变量覆盖优先级高于配置文件（由 viper AutomaticEnv 实现，非 YAML 变量替换）
```

### 7.2 环境变量

| 变量 | 说明 |
|------|------|
| `S3_ENDPOINT` | S3 端点 |
| `S3_BUCKET` | Bucket 名称 |
| `S3_REGION` | 区域 |
| `S3_ACCESS_KEY` | Access Key |
| `S3_SECRET_KEY` | Secret Key |
| `SERVER_PORT` | 服务端口 |
| `OSS_SERVER_URL` | CLI 用的服务端地址（全局生效） |

---

## 8. S3 路径结构

### 8.1 路径规范

上传到 S3 的文件路径格式：
```
{project_name}/{file_path}
```

示例：
```
my-app/index.html
my-app/static/css/main.css
my-app/static/js/app.js
```

### 8.2 设计原则

- **按项目名隔离**：每个项目有独立命名空间
- **无版本前缀**：始终覆盖更新，CDN 缓存由客户端处理
- **路径遍历防护**：规范 file_path，防止 `../` 逃逸项目目录

### 8.3 部署原子性策略

> S3 不支持目录级别的原子操作，因此采用**非原子 + 可重试**策略：

1. 逐文件上传到目标路径（直接覆盖旧文件）
2. 所有文件上传成功后，更新 `_projects.json` 元数据
3. **失败处理**：若中途失败，用户重新部署即可覆盖不完整文件
4. **关键保证**：单个文件的 PutObject 是原子的，不存在损坏的半写文件

### 8.4 并发部署保护

同一项目**不支持并发部署**，通过元数据中的部署状态实现互斥：

```json
{
  "name": "my-app",
  "url": "https://cdn.example.com/my-app/",
  "file_count": 42,
  "deployed_at": "2026-04-09T12:00:00Z",
  "deploying": false,
  "deploy_id": ""
}
```

**流程：**
1. 部署开始前，读取 `_projects.json`，检查目标项目 `deploying` 是否为 `true`
2. 若 `deploying == true`，返回 409 `DEPLOYMENT_IN_PROGRESS`
3. 若 `deploying == false`（或项目不存在），设置 `deploying=true` + 生成 `deploy_id`（UUID），上传元数据（If-Match）
4. 执行文件上传
5. 完成后设置 `deploying=false`，更新 `deployed_at` 和 `file_count`

> 首次部署时 `_projects.json` 不存在，直接创建，无需 If-Match。

---

## 9. 元数据设计

### 9.1 方案：S3 元数据文件

在 S3 Bucket 根目录存储 `_projects.json`，包含所有项目元数据：

```json
{
  "projects": [
    {
      "name": "my-app",
      "url": "https://cdn.example.com/my-app/",
      "file_count": 42,
      "deployed_at": "2026-04-09T12:00:00Z"
    }
  ]
}
```

### 9.2 读写流程

**部署时写入：**
1. 获取部署锁（`deploying=true`），若已被锁则返回 409
2. 解压 + 逐文件上传到 S3，统计文件数
3. 下载 `_projects.json`
   - 若不存在（首次部署）：创建空结构
4. UPSERT 项目记录（更新 `deploying=false`, `deployed_at`, `file_count`）
5. 上传 `_projects.json`（带 If-Match ETag 乐观锁，冲突重试最多 3 次）
   - 若重试耗尽，设置 `deploying=false` 并返回 503，**文件已上传成功，仅元数据更新失败**
   - 客户端可通过重新部署或手动 GET+PUT 修复

**查询时读取：**
1. 下载 `_projects.json`
2. 返回给客户端

### 9.3 并发安全

部署时使用 S3 条件写入（If-Match ETag）实现乐观锁，冲突时重试（最多 3 次）。

---

## 10. 数据流设计

### 10.1 部署数据流

```
CLI                           Server                        S3
 │                              │                            │
 │  1. 读取 wrangler.toml       │                            │
 │  2. 执行 build_command       │                            │
 │  3. 打包 output_directory ──►│ POST /deploy                │
 │                              │  4. 流式解压 zip            │
 │                              │  5. 逐文件 PutObject ─────►│
 │                              │                            │
 │◄─────────────────────────────│ 200 OK + URL               │
```

### 10.2 关键实现点

- **流式解压**：使用 `archive/zip` 的 Reader，避免完整解压到磁盘
- **流式上传**：逐文件上传，不等待全部解压完成
- **并发上传**：文件级别并发，默认 `runtime.NumCPU() * 2`，最大 16
- **zip 路径规范**：CLI 打包时 strip 掉顶层目录前缀，zip 内文件均为相对路径
  - `dist/index.html` → zip 内 `index.html` → S3 路径 `my-app/index.html`
- **路径安全**：服务端对每个解压路径调用 `filepath.Clean()` 后校验不含 `..`，防止路径遍历

---

## 11. 安全性考虑

- 无认证设计（按需扩展）
- 文件路径规范化，防止路径遍历攻击（`../../etc/passwd`）
- zip 文件大小限制：**100MB**（Gin middleware 层通过 `MaxMultipartMemory` + `LimitReader` 实现）
- Content-Type 白名单校验（仅允许以下类型）：

| 类型 | MIME |
|------|------|
| HTML | `text/html` |
| CSS | `text/css` |
| JavaScript | `application/javascript`, `text/javascript` |
| JSON | `application/json` |
| 图片 | `image/png`, `image/jpeg`, `image/gif`, `image/svg+xml`, `image/webp`, `image/x-icon` |
| 字体 | `font/woff`, `font/woff2`, `font/ttf`, `font/otf`, `application/font-woff2` |
| WebAssembly | `application/wasm` |
| 其他文本 | `text/plain`, `text/xml`, `application/xml` |

> 未匹配白名单的文件按 `application/octet-stream` 上传并记录警告日志。

---

## 12. 待扩展功能

- [ ] 认证与多用户支持
- [ ] 部署预览（Preview URL）
- [ ] 自定义域名与 SSL
- [ ] 部署回滚
- [ ] 构建日志流式返回
