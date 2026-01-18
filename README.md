# gee

一个用于学习/打磨的 Go Web 框架内核（路由树 + 中间件 + Context），并包含可运行示例。

## 目录结构

- `gee/`：框架核心库
- `cmd/gee-demo/`：可运行示例（模板 + 静态资源）
- `cmd/api/`：starter 服务入口（S0/S1：配置 + 超时 + 优雅停机）
- `internal/platform/`：gee-starter 的脚手架能力（可复用）
- `internal/app/`：具体业务（不可复用；shortlink/blog 等）
- `playground/`：练习/草稿代码（避免污染主项目）
- `ROADMAP.md`：迭代路线图
- `FRAMEWORK_CORRECTNESS_TODO.md`：框架正确性（C1/C3）细化清单
- `C4_C5_TODO.md`：C4/C5（404/405/兜底 + Bind/统一错误体）细化清单

## 运行示例

在仓库根目录执行：

```bash
go run ./cmd/gee-demo
```

运行 starter（S0/S1）：

```bash
go run ./cmd/api
```

访问：

- `http://localhost:9999/`
- `http://localhost:9999/panic`（测试 Recovery）
