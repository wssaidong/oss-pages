# OSS Pages

Pages的静态网站部署平台：Go CLI 打包项目，Go API 接收并部署到 S3 兼容存储。

## 快速开始

### 构建

```bash
cd src
go build -o pages-cli ./cmd/cli
go build -o oss-server ./cmd/server
```

### 部署一个静态网站

```bash
# 1. 初始化项目
./pages-cli init my-site

# 2. 编写代码后部署
./pages-cli deploy --server http://localhost:8080
```

### 启动服务端

```bash
# 复制配置
cp config.yaml.example config.yaml
# 编辑 config.yaml 填入 S3 凭证

# 启动
./oss-server
```

## CLI 命令

| 命令 | 说明 |
|------|------|
| `pages-cli init <name>` | 初始化项目，生成 `wrangler.toml` |
| `pages-cli deploy` | 构建 + 打包 + 上传 |
| `pages-cli push <directory>` | 直接推送本地目录（跳过构建） |
| `pages-cli projects list` | 列出所有已部署项目 |
| `pages-cli projects view <name>` | 查看项目详情 |
| `pages-cli projects delete <name>` | 删除项目 |

## 使用示例

### 初始化项目

```bash
# 使用默认配置初始化
pages-cli init my-blog

# 自定义构建命令和输出目录
pages-cli init my-blog --build-command "vite build" --output-dir build

# 指定服务端地址
pages-cli init my-blog --server https://pages.example.com
```

执行后会在当前目录生成 `wrangler.toml` 配置文件。

### 构建并部署

```bash
# 读取 wrangler.toml，执行构建命令后打包上传
pages-cli deploy

# 指定服务端地址（覆盖 wrangler.toml 中的配置）
pages-cli deploy --server https://pages.example.com

# 使用自定义配置文件
pages-cli deploy -c ./my-config.toml
```

也可以通过环境变量设置服务端地址：

```bash
export OSS_SERVER_URL=https://pages.example.com
pages-cli deploy
```

### 直接推送目录（跳过构建）

```bash
# 将已构建好的 dist 目录直接推送
pages-cli push ./dist

# 推送到指定服务端
pages-cli push ./dist --server https://pages.example.com
```

### 项目管理

```bash
# 列出所有已部署项目
pages-cli projects list --server https://pages.example.com

# 查看项目详情（文件列表、部署时间等）
pages-cli projects view my-blog --server https://pages.example.com

# 删除项目
pages-cli projects delete my-blog --server https://pages.example.com
```

## API 端点

| 端点 | 方法 | 说明 |
|------|------|------|
| `/deploy` | POST | 接收 zip 包，部署到 S3 |
| `/projects` | GET | 列出所有项目 |
| `/projects/:name` | GET | 获取项目详情 |
| `/projects/:name` | DELETE | 删除项目 |

## 项目结构

```
oss-pages/
├── src/
│   ├── cmd/
│   │   ├── cli/           # CLI 入口
│   │   └── server/        # 服务端入口
│   ├── internal/
│   │   ├── cli/           # init, deploy, projects 命令
│   │   ├── config/        # 配置加载
│   │   └── server/
│   │       ├── handler/   # HTTP handlers
│   │       ├── deployer/  # zip 解压 + S3 上传
│   │       └── storage/   # S3 存储抽象
│   ├── pkg/s3client/       # S3 客户端封装
│   └── tests/             # E2E 测试
├── wrangler.toml.example
└── config.yaml.example
```

## 配置

### CLI (wrangler.toml)

```toml
name = "my-app"

[pages]
build_command = "npm run build"
output_directory = "dist"

[deploy]
server_url = "https://api.example.com"
```

### 服务端 (config.yaml)

```yaml
server:
  port: 8080
  host: "0.0.0.0"

s3:
  endpoint: "https://s3.example.com"
  bucket: "my-bucket"
  region: "us-east-1"
  access_key: ""
  secret_key: ""
  path_prefix: ""
```

环境变量优先级高于配置文件（`S3_ACCESS_KEY`、`S3_SECRET_KEY`、`SERVER_PORT` 等）。

## 测试

```bash
cd src
go test ./... -race
```

## 技术栈

- **Go 1.25+** — CLI 和服务端
- **Gin** — HTTP 框架
- **Cobra** — CLI 框架
- **Viper** — 配置管理
- **BurntSushi/toml** — wrangler.toml 解析
- **S3 兼容存储** — AWS S3 / MinIO / 阿里云 OSS
