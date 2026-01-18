# 开发者 AI 工作台 - 实现方案

## 项目定位

面向开发者的 AI 工作流平台，聚焦三个核心场景：

| 模块 | 功能 | 典型用例 |
|------|------|----------|
| **研究助手** | 自动化信息收集 + 分析 | 技术选型调研、竞品分析 |
| **内容生成** | 技术博客、文档生成 | 从调研结果生成博客初稿 |
| **代码 Review** | PR 审查、规范检查 | GitHub PR 自动 Review |

### 与通用编排平台的区别

| 维度 | Dify/Coze | 我们的项目 |
|------|-----------|-----------|
| 定位 | 通用 AI 编排 | 开发者垂直场景 |
| 节点 | 通用节点 | 场景化预设工作流 |
| 部署 | 多容器 | 单二进制 |
| 目标用户 | 所有人 | 开发者 |

---

## 核心场景详解

### 场景 1: 研究助手

```
输入：调研 "Go 语言 2024 年 Web 框架对比"

工作流：
┌─────────┐     ┌─────────┐     ┌─────────┐     ┌─────────┐
│ 搜索引擎 │ ──▶ │ 网页抓取 │ ──▶ │ LLM 摘要 │ ──▶ │ 报告生成 │
└─────────┘     └─────────┘     └─────────┘     └─────────┘
     │                                              │
     └──▶ GitHub Trending ──────────────────────────┘

输出：结构化调研报告（Markdown）
- 框架对比表格
- 优缺点分析
- 推荐结论
- 参考链接
```

### 场景 2: 内容生成

```
输入：调研报告 + "写一篇技术博客"

工作流：
┌─────────┐     ┌─────────┐     ┌─────────┐     ┌─────────┐
│ 调研报告 │ ──▶ │ 大纲生成 │ ──▶ │ 分段写作 │ ──▶ │ 润色合并 │
└─────────┘     └─────────┘     └─────────┘     └─────────┘

输出：技术博客初稿
- 符合博客格式
- 代码示例
- SEO 友好标题
```

### 场景 3: 代码 Review（Phase 3 实现）

```
输入：GitHub PR URL

工作流：
┌─────────┐     ┌─────────┐     ┌─────────┐     ┌─────────┐
│ 获取 Diff│ ──▶ │ 代码分析 │ ──▶ │ 规范检查 │ ──▶ │ 生成报告 │
└─────────┘     └─────────┘     └─────────┘     └─────────┘
                     │
                     └── 读取项目规范配置

输出：Review 报告
- 问题列表（按严重程度）
- 改进建议
- 最佳实践参考
```

---

## 技术架构

```
┌─────────────────────────────────────────────────────────────┐
│                        前端                                  │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐          │
│  │ 场景入口    │  │ 工作流画布   │  │ 执行结果    │          │
│  │(快捷操作)   │  │(高级编辑)    │  │(Markdown)   │          │
│  └─────────────┘  └─────────────┘  └─────────────┘          │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                      API 层 (gee)                            │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐          │
│  │ 研究 API    │  │ 内容 API    │  │ Review API  │          │
│  │ /research   │  │ /content    │  │ /review     │          │
│  └─────────────┘  └─────────────┘  └─────────────┘          │
│                         │                                    │
│              ┌──────────┴──────────┐                        │
│              │  工作流 CRUD API     │                        │
│              │  /workflows          │                        │
│              └─────────────────────┘                        │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                     工作流引擎                                │
│  ┌─────────────────────────────────────────────────────┐    │
│  │                   DAG 执行器                         │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                             │
│  ┌─────────────────────────────────────────────────────┐    │
│  │                   节点执行器                         │    │
│  │  ┌─────┐ ┌─────┐ ┌──────┐ ┌──────┐ ┌──────┐        │    │
│  │  │ LLM │ │HTTP │ │Search│ │Scrape│ │GitHub│        │    │
│  │  └─────┘ └─────┘ └──────┘ └──────┘ └──────┘        │    │
│  └─────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                     基础设施（复用 shortlink）                │
│  ┌───────────┐  ┌───────────┐  ┌───────────┐               │
│  │PostgreSQL │  │   Redis   │  │   Kafka   │               │
│  └───────────┘  └───────────┘  └───────────┘               │
└─────────────────────────────────────────────────────────────┘
```

---

## 数据库设计

### workflows 表
```sql
CREATE TABLE workflows (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    description TEXT,
    category    TEXT NOT NULL,  -- research/content/review
    definition  JSONB NOT NULL,
    is_template BOOLEAN DEFAULT false,  -- 预设模板
    created_by  BIGINT REFERENCES users(id),
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW()
);

-- 预设模板示例
INSERT INTO workflows (name, category, is_template, definition) VALUES
('技术调研', 'research', true, '...'),
('博客生成', 'content', true, '...'),
('PR Review', 'review', true, '...');
```

### workflow_runs 表
```sql
CREATE TABLE workflow_runs (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id  UUID REFERENCES workflows(id),
    status       TEXT NOT NULL DEFAULT 'pending',
    input        JSONB,
    output       JSONB,
    tokens_used  INT DEFAULT 0,      -- token 消耗统计
    cost_usd     DECIMAL(10,6),      -- 成本统计
    started_at   TIMESTAMPTZ,
    finished_at  TIMESTAMPTZ,
    error        TEXT
);
```

### node_runs 表
```sql
CREATE TABLE node_runs (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id       UUID REFERENCES workflow_runs(id),
    node_id      TEXT NOT NULL,
    node_type    TEXT NOT NULL,
    status       TEXT NOT NULL DEFAULT 'pending',
    input        JSONB,
    output       JSONB,
    tokens_used  INT DEFAULT 0,
    started_at   TIMESTAMPTZ,
    finished_at  TIMESTAMPTZ,
    error        TEXT
);
```

---

## 节点类型

### 通用节点

| 节点 | 说明 | 用途 |
|------|------|------|
| **Start** | 入口 | 接收用户输入 |
| **End** | 出口 | 返回结果 |
| **LLM** | 调用大模型 | 分析、生成、摘要 |
| **Condition** | 条件分支 | 流程控制 |

### 场景专用节点

| 节点 | 说明 | 场景 |
|------|------|------|
| **WebSearch** | 搜索引擎 | 研究助手 |
| **WebScrape** | 网页抓取 | 研究助手 |
| **GitHubPR** | 获取 PR diff | 代码 Review |
| **GitHubTrending** | 获取热门仓库 | 研究助手 |
| **Markdown** | 格式化输出 | 内容生成 |

---

## 核心模块设计

### 1. 场景化 API（简化入口）

```go
// 研究助手 - 一键调研
// POST /api/v1/research
type ResearchRequest struct {
    Topic    string   `json:"topic"`     // 调研主题
    Sources  []string `json:"sources"`   // 来源：google, github, hn
    Language string   `json:"language"`  // 输出语言
}

// 内容生成 - 一键生成
// POST /api/v1/content/blog
type BlogRequest struct {
    Topic    string `json:"topic"`     // 主题
    Source   string `json:"source"`    // 参考资料（可选）
    Style    string `json:"style"`     // 风格：tutorial, analysis
    Length   string `json:"length"`    // 长度：short, medium, long
}

// 代码 Review
// POST /api/v1/review
type ReviewRequest struct {
    PRURL    string   `json:"pr_url"`   // GitHub PR URL
    Rules    []string `json:"rules"`    // 检查规则
}
```

### 2. LLM 客户端（多 Provider）

```go
// internal/llm/client.go
type Client interface {
    Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
    ChatStream(ctx context.Context, req *ChatRequest) (<-chan Chunk, error)
}

type ChatRequest struct {
    Model    string    // deepseek-chat, gpt-4o-mini, claude-3-haiku
    Messages []Message
    MaxTokens int
}

type ChatResponse struct {
    Content     string
    TokensUsed  TokenUsage
    Cost        float64
}

// 多 Provider 路由
type Router struct {
    deepseek *DeepSeekClient
    openai   *OpenAIClient
    claude   *ClaudeClient
}

func (r *Router) GetClient(model string) Client {
    switch {
    case strings.HasPrefix(model, "deepseek"):
        return r.deepseek
    case strings.HasPrefix(model, "gpt"):
        return r.openai
    case strings.HasPrefix(model, "claude"):
        return r.claude
    }
    return r.deepseek // 默认最便宜
}
```

### 3. 搜索节点

```go
// internal/workflow/nodes/search.go
type WebSearchNode struct {
    Query   string
    Engine  string // google, bing, duckduckgo
    Limit   int
}

func (n *WebSearchNode) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
    query := RenderTemplate(n.Query, input)

    results, err := search.Search(ctx, n.Engine, query, n.Limit)
    if err != nil {
        return nil, err
    }

    return map[string]any{
        "results": results,  // []SearchResult{Title, URL, Snippet}
    }, nil
}
```

### 4. 网页抓取节点

```go
// internal/workflow/nodes/scrape.go
type WebScrapeNode struct {
    URLs     []string
    Selector string  // CSS 选择器（可选）
}

func (n *WebScrapeNode) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
    var contents []string

    for _, url := range n.URLs {
        content, err := scrape.FetchContent(ctx, url, n.Selector)
        if err != nil {
            continue // 跳过失败的
        }
        contents = append(contents, content)
    }

    return map[string]any{
        "contents": contents,
    }, nil
}
```

---

## 开发路线图

### Phase 1: 核心引擎 + 研究助手（第一优先）

**目标**：完成工作流引擎，实现"输入话题 → 输出调研报告"

- [ ] 数据库 schema 和 migration
- [ ] DAG 执行引擎（拓扑排序、并行执行）
- [ ] 基础节点：Start、End、LLM、Condition
- [ ] 变量系统（模板渲染 `{{node.field}}`）
- [ ] LLM 客户端（先支持 DeepSeek）
- [ ] 搜索节点（WebSearch）
- [ ] 抓取节点（WebScrape）
- [ ] 研究助手预设工作流
- [ ] 场景化 API：`POST /api/v1/research`

**验收标准**：
```bash
curl -X POST http://localhost:9999/api/v1/research \
  -H "Content-Type: application/json" \
  -d '{"topic": "Go vs Rust 2024 对比", "sources": ["google"]}'

# 返回：结构化调研报告（Markdown）
```

### Phase 2: 内容生成

**目标**：从调研结果生成技术博客

- [ ] Markdown 格式化节点
- [ ] 博客生成预设工作流（大纲 → 分段 → 润色）
- [ ] 场景化 API：`POST /api/v1/content/blog`
- [ ] 多种输出格式（Markdown、HTML）

**验收标准**：
```bash
curl -X POST http://localhost:9999/api/v1/content/blog \
  -H "Content-Type: application/json" \
  -d '{"topic": "Redis 缓存最佳实践", "style": "tutorial"}'

# 返回：技术博客初稿（Markdown）
```

### Phase 3: 代码 Review

**目标**：GitHub PR 自动 Review

- [ ] GitHub API 集成（获取 PR diff）
- [ ] 代码分析 prompt 模板
- [ ] 规范配置（可自定义规则）
- [ ] GitHubPR 节点
- [ ] Review 预设工作流
- [ ] 场景化 API：`POST /api/v1/review`

**验收标准**：
```bash
curl -X POST http://localhost:9999/api/v1/review \
  -H "Content-Type: application/json" \
  -d '{"pr_url": "https://github.com/user/repo/pull/123"}'

# 返回：Review 报告（问题列表 + 建议）
```

### Phase 4: 前端 + 增强

- [ ] 场景入口页面（快捷操作）
- [ ] 工作流可视化编辑（Vue Flow）
- [ ] 执行历史和结果查看
- [ ] SSE 流式输出
- [ ] Token 消耗和成本统计

---

## 目录结构

```
internal/
├── workflow/
│   ├── engine/
│   │   ├── dag.go          # DAG 构建和拓扑排序
│   │   ├── executor.go     # 工作流执行器
│   │   └── variables.go    # 变量系统
│   ├── nodes/
│   │   ├── interface.go    # 节点接口
│   │   ├── start.go
│   │   ├── end.go
│   │   ├── llm.go
│   │   ├── condition.go
│   │   ├── websearch.go
│   │   ├── webscrape.go
│   │   ├── githubpr.go
│   │   └── markdown.go
│   ├── repo/
│   │   ├── workflows.go    # 工作流 CRUD
│   │   └── runs.go         # 执行记录
│   └── httpapi/
│       ├── workflows.go    # 工作流 API
│       ├── research.go     # 研究助手 API
│       ├── content.go      # 内容生成 API
│       └── review.go       # 代码 Review API
├── llm/
│   ├── client.go           # LLM 客户端接口
│   ├── deepseek.go
│   ├── openai.go
│   └── router.go           # 多 Provider 路由
└── search/
    ├── google.go           # Google 搜索
    └── scrape.go           # 网页抓取
```

---

## LLM 成本估算

| 模型 | 输入价格 | 输出价格 | 适用场景 |
|------|----------|----------|----------|
| DeepSeek Chat | $0.14/M | $0.28/M | 默认，成本低 |
| GPT-4o-mini | $0.15/M | $0.6/M | 需要更好效果时 |
| Claude 3 Haiku | $0.25/M | $1.25/M | 长文本处理 |

**单次调研估算**（DeepSeek）：
- 搜索结果摘要：~2000 tokens → $0.0006
- 报告生成：~3000 tokens → $0.0008
- **合计：约 $0.001/次**

**月成本**（日均 10 次使用）：~$0.3/月

---

## 技术决策

| 决策 | 选择 | 理由 |
|------|------|------|
| 默认 LLM | DeepSeek | 最便宜，中文效果好 |
| 搜索 API | DuckDuckGo | 免费，无需 API key |
| 网页抓取 | colly | Go 原生，性能好 |
| JS 执行 | goja | 暂不需要，Phase 4 再加 |
| 多租户 | 单用户 | MVP 足够 |

---

## 验证方式

### 单元测试
```bash
go test ./internal/workflow/... -v
```

### 集成测试
```bash
# 启动服务
go run ./cmd/api

# 测试研究助手
curl -X POST http://localhost:9999/api/v1/research \
  -d '{"topic": "test topic"}'
```

### 端到端测试
```bash
# k6 压测（可选）
k6 run test/k6/research_load.js
```
