# Days

一个基于自研轻量级 Web 框架构建的 Go Web 应用，包含短链接服务及数据分析功能。

## 功能特性

- **短链接服务**：创建和管理短链接，支持点击追踪
- **数据分析**：基于 Kafka 或 Channel 的实时点击统计
- **多级缓存**：Redis + 本地缓存，布隆过滤器优化不存在 key 的查询
- **分布式限流**：基于 Redis 的分布式限流
- **可观测性**：Prometheus 指标、Grafana 仪表盘、OpenTelemetry 链路追踪
- **JWT 认证**：HS256 JWT Token 安全认证

## 技术栈

- **语言**：Go 1.21+
- **数据库**：PostgreSQL
- **缓存**：Redis
- **消息队列**：Kafka（可选）
- **监控**：Prometheus + Grafana
- **链路追踪**：OpenTelemetry (Jaeger/Tempo)
- **前端**：Astro

## 项目结构

```
├── cmd/
│   └── api/              # 应用入口
├── gee/                  # 自研轻量级 Web 框架
├── internal/
│   ├── app/
│   │   └── shortlink/    # 短链接业务逻辑
│   └── platform/         # 可复用的基础设施组件
├── migrations/           # 数据库迁移
├── grafana/              # Grafana 仪表盘配置
└── prometheus.yml        # Prometheus 配置
```

## 快速开始

### 环境要求

- Go 1.21+
- PostgreSQL
- Redis
- Docker & Docker Compose（可选）

### 安装步骤

1. 克隆仓库：

```bash
git clone https://github.com/yourusername/days.git
cd days
```

2. 复制并配置环境变量：

```bash
cp .env.example .env
# 编辑 .env 填入你的配置
```

3. 使用 Docker Compose 启动依赖服务：

```bash
docker-compose up -d
```

4. 运行数据库迁移（启动时自动执行或手动执行）。

5. 启动应用：

```bash
go run ./cmd/api
```

### 接口地址

- **公开 API**：`http://localhost:9999`
- **管理/指标**：`http://localhost:6060`（仅内网访问）
  - `/metrics` - Prometheus 指标
  - `/readyz` - 就绪探针
  - `/version` - 版本信息

## 配置说明

完整配置项请参考 `.env.example`：

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `ADDR` | 公开服务地址 | `:9999` |
| `ADMIN_ADDR` | 管理服务地址 | `127.0.0.1:6060` |
| `DB_DSN` | PostgreSQL 连接字符串 | - |
| `REDIS_ADDR` | Redis 地址 | `localhost:6379` |
| `RATELIMIT_ENABLED` | 启用限流 | `true` |
| `TRACING_ENABLED` | 启用链路追踪 | `false` |

## 许可证

MIT
