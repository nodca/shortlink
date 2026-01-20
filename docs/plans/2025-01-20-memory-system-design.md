# AIFlow 记忆系统设计文档

日期：2025-01-20
状态：待实现
阶段：Phase 1

## 概述

### 项目定位

AIFlow Memory System — 一个让 AI 真正"记住你"的个人助手记忆系统。

### 核心差异化

不是每次都从零开始对话，而是 AI 始终带着对你的理解来交流。

### 设计原则

- 三层分层记忆，越往上越抽象、越稳定
- 自然语言画像优于结构化标签
- RAG 是检索手段，不是记忆结构

## 系统架构

```
┌────────────────────────────────────────────────────────┐
│                    对话层 (Chat)                        │
│         每次对话时组装 context，调用 LLM                 │
└────────────────────────────────────────────────────────┘
                          │
          ┌───────────────┼───────────────┐
          ▼               ▼               ▼
   ┌─────────────┐ ┌─────────────┐ ┌─────────────┐
   │ 核心画像    │ │ 会话摘要    │ │ 原始对话    │
   │ (Profile)   │ │ (Summaries) │ │ (Raw)       │
   │             │ │             │ │             │
   │ 始终携带    │ │ 最近N条     │ │ 按需RAG     │
   └─────────────┘ └─────────────┘ └─────────────┘
          ▲               │               │
          └───────────────┴───────────────┘
                    定期提炼 & 生成
```

### 技术选型

| 组件 | 选择 | 理由 |
|------|------|------|
| 数据库 | PostgreSQL + pgvector | 已有 PG，pgvector 支持向量检索 |
| 向量生成 | 现有 Embedding API | 复用已有基础设施 |
| 摘要/提炼 | DeepSeek 或其他 LLM | 成本可控 |

## Layer 1: 原始对话

### 职责

- 存储完整对话记录
- 为摘要生成提供原始素材
- 支持按需 RAG 检索细节

### 数据模型

```sql
CREATE TABLE conversations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    session_id UUID NOT NULL,
    messages JSONB NOT NULL,
    message_count INT,
    created_at TIMESTAMP DEFAULT NOW(),
    expires_at TIMESTAMP
);

CREATE INDEX idx_conversations_user_session
    ON conversations(user_id, session_id);
CREATE INDEX idx_conversations_expires
    ON conversations(expires_at);
```

### messages 结构

```json
[
  {"role": "user", "content": "我最近在考虑换工作", "timestamp": "2025-01-20T10:00:00Z"},
  {"role": "assistant", "content": "可以聊聊是什么原因吗？", "timestamp": "2025-01-20T10:00:05Z"},
  {"role": "user", "content": "主要是压力太大了...", "timestamp": "2025-01-20T10:01:00Z"}
]
```

### 生命周期

| 事件 | 动作 |
|------|------|
| 对话结束 | 写入一条记录 |
| 摘要生成后 | 可选：标记为"已处理" |
| 到期（7-30天） | 后台任务清理 |

### 清理逻辑

```go
func CleanExpiredConversations(ctx context.Context) error {
    _, err := db.Exec(ctx,
        "DELETE FROM conversations WHERE expires_at < NOW()")
    return err
}
```

## Layer 2: 会话摘要

### 职责

- 保留对话的"精华"，丢弃冗余细节
- 提供中期记忆上下文（最近 N 条）
- 支持 RAG 检索历史话题
- 作为核心画像更新的素材

### 数据模型

```sql
CREATE TABLE session_summaries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    session_id UUID NOT NULL UNIQUE,

    -- 摘要内容
    summary TEXT NOT NULL,
    topics TEXT[] DEFAULT '{}',
    user_intent TEXT,
    emotional_tone TEXT,
    key_facts TEXT[],
    unresolved TEXT,

    -- RAG 支持
    embedding VECTOR(1536),

    -- 元数据
    message_count INT,
    created_at TIMESTAMP DEFAULT NOW(),
    expires_at TIMESTAMP
);

CREATE INDEX idx_summaries_user_time
    ON session_summaries(user_id, created_at DESC);
CREATE INDEX idx_summaries_embedding
    ON session_summaries USING ivfflat(embedding vector_cosine_ops);
```

### 摘要示例

```json
{
  "summary": "用户讨论了换工作的想法。当前工作压力大是主要原因，但担心新工作收入下降。还没做决定，想再观察一段时间。",
  "topics": ["职业", "工作压力", "收入"],
  "user_intent": "倾诉和梳理想法，暂不需要具体建议",
  "emotional_tone": "焦虑、犹豫",
  "key_facts": [
    "当前工作压力大",
    "在考虑换工作",
    "担心收入下降"
  ],
  "unresolved": "是否真的要换工作"
}
```

### 生成 Prompt

```
你是一个记忆助手。请根据以下对话，提取结构化摘要。

## 对话内容
{messages}

## 输出要求（JSON 格式）
{
  "summary": "2-3句话概括本次对话的主要内容",
  "topics": ["话题标签，3-5个词"],
  "user_intent": "用户本次对话的核心目的是什么",
  "emotional_tone": "用户的情绪基调，如：平静、焦虑、兴奋、沮丧",
  "key_facts": ["本次对话中提到的重要事实，可用于长期记忆"],
  "unresolved": "对话中未解决的问题，如果没有则为 null"
}

注意：
- 从用户视角提取，不要包含 AI 的观点
- key_facts 只提取客观事实，不要推测
- 如果是闲聊没有实质内容，summary 写"闲聊，无实质内容"
```

### 生成时机

```
对话结束
  → 判断是否有实质内容（消息数 > 2 且非纯闲聊）
  → 调用 LLM 生成摘要
  → 生成 embedding
  → 存入数据库
```

### 使用方式

| 场景 | 方法 |
|------|------|
| 每次对话开始 | 加载最近 5 条摘要，按时间倒序 |
| 用户问"上次聊的..." | RAG 检索相关摘要 |
| 更新核心画像 | 读取最近 N 条未处理摘要 |

## Layer 3: 核心画像

### 职责

- 存储对用户的长期、稳定理解
- 每次对话必定携带，作为 AI 的"底色认知"
- 随着对话积累逐步丰富和修正

### 数据模型

```sql
CREATE TABLE user_profiles (
    user_id UUID PRIMARY KEY,

    -- 核心：自然语言画像
    profile_narrative TEXT NOT NULL DEFAULT '',

    -- 可选：少量结构化字段
    basic_info JSONB DEFAULT '{}',

    -- 更新追踪
    version INT DEFAULT 1,
    last_summary_id UUID,
    updated_at TIMESTAMP DEFAULT NOW(),
    created_at TIMESTAMP DEFAULT NOW()
);
```

### basic_info 结构（仅基础信息）

```json
{
  "name": "小明",
  "occupation": "后端程序员"
}
```

### profile_narrative 示例

```
小明是一位有5年经验的后端程序员，在上海工作。

性格特点：理性但容易焦虑，做决定前会反复权衡。喜欢直接、逻辑清晰的交流，不喜欢空泛的鼓励。

当前状态：正在考虑换工作，主要原因是压力大，但担心收入下降。尚未做出决定。

长期关注：职业发展、工作生活平衡。
```

### 更新机制

**触发条件**（满足任一）：
- 累计 5 条新摘要
- 距上次更新超过 7 天
- 用户主动说了重要信息

**更新流程**：
```
读取当前画像
  → 读取上次更新后的所有新摘要
  → LLM 合并生成新画像
  → 存入数据库，version + 1
```

### 更新 Prompt

```
你是一个用户画像管理助手。

## 当前画像
{current_profile}

## 最近的对话摘要
{recent_summaries}

## 任务
1. 根据新信息更新画像
2. 如果新信息与旧信息冲突，以新信息为准，但保留变化痕迹
3. 不要删除没有被否定的旧信息
4. 用自然语言段落描述，不要用列表

## 输出
直接输出更新后的画像文本（3-5 段）
```

## 数据流

### 对话开始：加载记忆

```
用户发起对话
      │
      ▼
  1. 加载核心画像 (profile_narrative)
  2. 加载最近 5 条会话摘要
  3. 组装 System Prompt
      │
      ▼
   开始对话
```

### System Prompt 组装

```
你是用户的个人 AI 助手。

## 关于用户
{profile_narrative}

## 最近的交流
- 1月18日：讨论了换工作的想法，用户还在犹豫，担心收入问题
- 1月15日：用户说最近压力大，聊了一些减压方法
- 1月10日：帮用户写了一封邮件

## 对话原则
- 基于你对用户的了解来回应
- 不需要每次都重复确认用户的背景信息
- 如果用户提到新信息，自然地记住
```

### 对话中：按需检索

```go
func HandleRecallQuery(ctx context.Context, userID string, query string) ([]Summary, error) {
    queryEmb, _ := embedding.Generate(ctx, query)
    summaries, _ := repo.SearchSummaries(ctx, userID, queryEmb, limit: 3)
    return summaries, nil
}
```

### 对话结束：保存记忆

```
对话结束
      │
      ▼
  1. 判断是否有实质内容
     - 消息数 < 3 → 跳过
     - 纯闲聊 → 跳过
      │ 有实质内容
      ▼
  2. 存储原始对话 (Layer 1)
  3. 生成并存储摘要 (Layer 2)
  4. 检查是否需要更新画像 (Layer 3)
     - 累计 5 条新摘要？
     - 超过 7 天没更新？
     → 触发异步画像更新
```

## 代码结构

### 模块划分

```
internal/app/aiflow/
├── memory/
│   ├── service.go       # 核心服务
│   ├── summarizer.go    # 摘要生成
│   ├── profiler.go      # 画像更新
│   └── repo.go          # 数据库操作
├── chat/
│   └── handler.go       # 对话处理
└── ...
```

### 核心接口

```go
type MemoryService interface {
    // 对话开始时调用
    LoadContext(ctx context.Context, userID string) (*MemoryContext, error)

    // 对话结束时调用
    SaveConversation(ctx context.Context, userID string, messages []Message) error

    // 对话中按需检索
    SearchMemory(ctx context.Context, userID string, query string) ([]Summary, error)

    // 用户主动遗忘
    Forget(ctx context.Context, userID string, scope ForgetScope) error
}

type MemoryContext struct {
    ProfileNarrative  string
    RecentSummaries   []Summary
    SystemPrompt      string
}
```

### 对话 Handler 集成

```go
func (h *ChatHandler) HandleMessage(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
    // 1. 加载记忆
    memCtx, err := h.memory.LoadContext(ctx, req.UserID)
    if err != nil {
        return nil, err
    }

    // 2. 构建对话请求
    llmReq := &llm.Request{
        SystemPrompt: memCtx.SystemPrompt,
        Messages:     req.Messages,
    }

    // 3. 调用 LLM
    resp, err := h.llm.Chat(ctx, llmReq)
    if err != nil {
        return nil, err
    }

    // 4. 如果是对话结束，保存记忆
    if req.IsSessionEnd {
        go h.memory.SaveConversation(ctx, req.UserID, req.Messages)
    }

    return &ChatResponse{Content: resp.Content}, nil
}
```

## 边界情况

### 冷启动：新用户

```
用户首次使用
      │
      ▼
  画像不存在 → 创建空画像
  profile_narrative = "新用户，尚无信息"
      │
      ▼
  正常对话（AI 自然地了解用户）
      │
      ▼
  累计几次对话后 → 生成第一版画像
```

### 摘要生成失败

```go
summary, err := llm.GenerateSummary(ctx, messages)
if err != nil {
    log.Error("summary generation failed", "err", err)

    // 存降级摘要
    fallback := &Summary{Summary: "（摘要生成失败，待重试）"}
    repo.SaveSummary(ctx, userID, sessionID, fallback)

    // 加入重试队列
    queue.EnqueueRetry(ctx, "generate_summary", sessionID)
    return nil
}
```

### 画像冲突处理

策略：新信息优先，保留变化痕迹

```
如果新信息与旧信息冲突：
- 以新信息为准
- 体现变化："用户之前在A公司工作，最近换到了B公司"
```

### Token 超限

```go
func LoadRecentSummaries(ctx context.Context, userID string, tokenBudget int) []Summary {
    summaries := repo.GetRecentSummaries(ctx, userID, limit: 10)

    result := []Summary{}
    usedTokens := 0

    for _, s := range summaries {
        tokens := countTokens(s.Summary)
        if usedTokens + tokens > tokenBudget {
            break
        }
        result = append(result, s)
        usedTokens += tokens
    }

    return result
}
```

### 用户要求遗忘

```go
func ForgetEverything(ctx context.Context, userID string) error {
    repo.DeleteConversations(ctx, userID)
    repo.DeleteSummaries(ctx, userID)
    repo.DeleteProfile(ctx, userID)
    return nil
}

func ForgetTopic(ctx context.Context, userID string, topic string) error {
    repo.DeleteSummariesByTopic(ctx, userID, topic)
    regenerateProfile(ctx, userID)
    return nil
}
```

## 数据库迁移

```sql
-- migrations/015_memory_system.sql

-- Layer 1: 原始对话
CREATE TABLE conversations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    session_id UUID NOT NULL,
    messages JSONB NOT NULL,
    message_count INT,
    created_at TIMESTAMP DEFAULT NOW(),
    expires_at TIMESTAMP
);

-- Layer 2: 会话摘要
CREATE TABLE session_summaries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    session_id UUID NOT NULL UNIQUE,
    summary TEXT NOT NULL,
    topics TEXT[],
    user_intent TEXT,
    emotional_tone TEXT,
    key_facts TEXT[],
    unresolved TEXT,
    embedding VECTOR(1536),
    message_count INT,
    created_at TIMESTAMP DEFAULT NOW(),
    expires_at TIMESTAMP
);

-- Layer 3: 核心画像
CREATE TABLE user_profiles (
    user_id UUID PRIMARY KEY,
    profile_narrative TEXT NOT NULL DEFAULT '',
    basic_info JSONB DEFAULT '{}',
    version INT DEFAULT 1,
    last_summary_id UUID,
    updated_at TIMESTAMP DEFAULT NOW(),
    created_at TIMESTAMP DEFAULT NOW()
);

-- 索引
CREATE INDEX idx_conversations_user ON conversations(user_id, created_at DESC);
CREATE INDEX idx_conversations_expires ON conversations(expires_at);
CREATE INDEX idx_summaries_user ON session_summaries(user_id, created_at DESC);
CREATE INDEX idx_summaries_embedding ON session_summaries
    USING ivfflat(embedding vector_cosine_ops);
```

## 与现有模块集成

### 复用 RAG 基础设施

```
internal/platform/
├── embedding/
│   └── client.go      # Embedding API 封装（复用）
└── vectorstore/
    └── pgvector.go    # 向量存储和检索（复用）

internal/app/aiflow/
├── knowledge/         # 知识库（chunking + embedding + vectorstore）
└── memory/            # 记忆系统（embedding + vectorstore，无 chunking）
```

记忆系统复用 Embedding 和 VectorStore，不需要分块逻辑。

## 成本估算

| 项目 | 估算 |
|------|------|
| 摘要生成（每次对话） | ~1200 tokens → ¥0.001 |
| 画像更新（每5次对话） | ~2000 tokens → ¥0.002 |
| Embedding | ~200 tokens → ¥0.0001 |

每日活跃使用成本约 ¥0.01-0.05，可忽略。

## 后续扩展

Phase 2（后台任务）完成后，可扩展：
- 用户创建的任务也作为画像输入
- 任务偏好纳入核心画像

详见 [头脑风暴记录](./2025-01-20-aiflow-brainstorm.md)
