package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Addr              string
	IdleTimeout       time.Duration // 连接处理完一个请求后等待 IdleTimeout 后依旧没有请求，就会关闭此空闲连接
	ShutdownTimeout   time.Duration // 关闭服务的最长等待时间，超过后强制断开连接
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration

	// 日志配置信息
	LogLevel    slog.Level
	LogFormat   string
	ServiceName string

	PprofEnabled bool
	AdminAddr    string

	// JWT 配置
	JWTSecret string        // HS256 的签名密钥（对称密钥）
	JWTIssuer string        // 签发者标识（iss），用于防止“别的服务签发的 token 被你接受”
	JWTTTL    time.Duration // token 有效期

	OtlpGrpcEndpoint string
	OtlpServiceName  string
	TracingEnabled   bool `env:"TRACING_ENABLED" envDefault:"true"`

	DBDSN string

	//Kafka
	KafkaEnabled bool     `env:"KAFKA_ENABLED" envDefault:"false"`
	KafkaBrokers []string `env:"KAFKA_BROKERS" envSeparator:","`
	KafkaTopic   string   `env:"KAFKA_TOPIC" envDefault:"click-events"`

	//Redis
	RedisAddr     string `env:"REDIS_ADDR" envDefault:"localhost:6379"`
	RedisPassword string `env:"REDIS_PASSWORD" envDefault:""`
	RedisDB       int    `env:"REDIS_DB" envDefault:"0"`

	// RateLimit
	RateLimitEnabled bool `env:"RATELIMIT_ENABLED" envDefault:"true"`

	// AIFlow
	AIFlowEnabled   bool   `env:"AIFLOW_ENABLED" envDefault:"true"`
	DeepSeekAPIKey  string `env:"DEEPSEEK_API_KEY"`
	DeepSeekBaseURL string `env:"DEEPSEEK_BASEURL" envDefault:"https://api.siliconflow.cn/v1"`
	DeepSeekModel   string `env:"DEEPSEEK_MODEL" envDefault:"deepseek-ai/DeepSeek-V3.2"`
	SerpAPIKey      string `env:"SERPAPI_KEY"`

	EmbeddingAPIKey  string `env:"EMBEDDING_API_KEY"`
	EmbeddingBaseURL string `env:"EMBEDDING_BASEURL" envDefault:"https://api.siliconflow.cn/v1"`
	EmbeddingModel   string `env:"EMBEDDING_MODEL" envDefault:"BAAI/bge-m3"`

	AIFlowAgentTimeout        time.Duration
	AIFlowToolTimeout         time.Duration
	AIFlowObservationMaxBytes int
	AIFlowCostPer1KTokens     float64
}

func Load() Config {
	cfg := Config{
		Addr:              ":9999",
		IdleTimeout:       60 * time.Second,
		ShutdownTimeout:   10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,

		LogLevel:    slog.LevelInfo,
		LogFormat:   "json",
		ServiceName: "gee-api",

		PprofEnabled: false,
		AdminAddr:    "127.0.0.1:6060",

		JWTTTL:    12 * time.Hour,
		JWTSecret: "123456",
		JWTIssuer: "123456",

		OtlpGrpcEndpoint: "127.0.0.1:4317",
		OtlpServiceName:  "gee-api",
		TracingEnabled:   true,

		DBDSN: "postgres://days:days@localhost:5432/days?sslmode=disable",

		// Kafka
		KafkaEnabled:  false,
		KafkaBrokers:  []string{"localhost:9092"},
		KafkaTopic:    "click-events",
		RedisAddr:     "localhost:6379",
		RedisPassword: "",
		RedisDB:       0,

		RateLimitEnabled: true,

		// AIFlow
		AIFlowEnabled:   true,
		DeepSeekBaseURL: "https://api.siliconflow.cn/v1",
		DeepSeekModel:   "deepseek-ai/DeepSeek-V3.2",

		EmbeddingBaseURL: "https://api.siliconflow.cn/v1",
		EmbeddingModel:   "BAAI/bge-m3",

		AIFlowAgentTimeout:        120 * time.Second,
		AIFlowToolTimeout:         20 * time.Second,
		AIFlowObservationMaxBytes: 8 * 1024,
		AIFlowCostPer1KTokens:     0,
	}

	_ = godotenv.Load(".env")

	if v, ok := os.LookupEnv("ADDR"); ok && v != "" {
		cfg.Addr = v
	}
	if v, ok := os.LookupEnv("IDLE_TIMEOUT"); ok && v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.IdleTimeout = d
		}
	}
	if v, ok := os.LookupEnv("SHUTDOWN_TIMEOUT"); ok && v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.ShutdownTimeout = d
		}
	}
	if v, ok := os.LookupEnv("READ_HEADER_TIMEOUT"); ok && v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.ReadHeaderTimeout = d
		}
	}
	if v, ok := os.LookupEnv("READ_TIMEOUT"); ok && v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.ReadTimeout = d
		}
	}
	if v, ok := os.LookupEnv("WRITE_TIMEOUT"); ok && v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.WriteTimeout = d
		}
	}

	if v, ok := os.LookupEnv("LOG_LEVEL"); ok && v != "" {
		switch strings.ToLower(v) {
		case "debug":
			cfg.LogLevel = slog.LevelDebug
		case "info":
			cfg.LogLevel = slog.LevelInfo
		case "warn", "warning":
			cfg.LogLevel = slog.LevelWarn
		case "error":
			cfg.LogLevel = slog.LevelError
		default:
			cfg.LogLevel = slog.LevelInfo
		}
	}
	if v, ok := os.LookupEnv("LOG_FORMAT"); ok && v != "" {
		cfg.LogFormat = v
	}
	if v, ok := os.LookupEnv("SERVICE_NAME"); ok && v != "" {
		cfg.ServiceName = v
	}

	if v, ok := os.LookupEnv("PPROF_ENABLED"); ok && v != "" {
		cfg.PprofEnabled = strings.ToLower(v) == "true"
	}
	if v, ok := os.LookupEnv("ADMIN_ADDR"); ok && v != "" {
		cfg.AdminAddr = v
	}

	if v, ok := os.LookupEnv("JWT_SECRET"); ok && v != "" {
		cfg.JWTSecret = v
	}
	if v, ok := os.LookupEnv("JWT_ISSUER"); ok && v != "" {
		cfg.JWTIssuer = v
	}
	if v, ok := os.LookupEnv("JWT_TTL"); ok && v != "" {
		if t, err := time.ParseDuration(v); err == nil {
			cfg.JWTTTL = t
		}
	}

	if v, ok := os.LookupEnv("TRACING_ENABLED"); ok && v != "" {
		cfg.TracingEnabled = strings.ToLower(v) == "true"
	}

	if v, ok := os.LookupEnv("DB_DSN"); ok && v != "" {
		cfg.DBDSN = v
	}

	// Kafka
	if v, ok := os.LookupEnv("KAFKA_ENABLED"); ok && v != "" {
		cfg.KafkaEnabled = strings.ToLower(v) == "true"
	}
	if v, ok := os.LookupEnv("KAFKA_BROKERS"); ok && v != "" {
		cfg.KafkaBrokers = strings.Split(v, ",")
	}
	if v, ok := os.LookupEnv("KAFKA_TOPIC"); ok && v != "" {
		cfg.KafkaTopic = v
	}

	// Redis
	if v, ok := os.LookupEnv("REDIS_ADDR"); ok && v != "" {
		cfg.RedisAddr = v
	}
	if v, ok := os.LookupEnv("REDIS_PASSWORD"); ok && v != "" {
		cfg.RedisPassword = v
	}
	if v, ok := os.LookupEnv("REDIS_DB"); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			cfg.RedisDB = n
		}
	}

	// RateLimit
	if v, ok := os.LookupEnv("RATELIMIT_ENABLED"); ok && v != "" {
		cfg.RateLimitEnabled = strings.ToLower(v) == "true"
	}

	// AIFlow
	if v, ok := os.LookupEnv("AIFLOW_ENABLED"); ok && v != "" {
		cfg.AIFlowEnabled = strings.ToLower(v) == "true"
	}
	if v, ok := os.LookupEnv("DEEPSEEK_API_KEY"); ok && v != "" {
		cfg.DeepSeekAPIKey = v
	}
	if v, ok := os.LookupEnv("DEEPSEEK_BASEURL"); ok && v != "" {
		cfg.DeepSeekBaseURL = v
	}
	if v, ok := os.LookupEnv("DEEPSEEK_MODEL"); ok && v != "" {
		cfg.DeepSeekModel = v
	}
	if v, ok := os.LookupEnv("SERPAPI_KEY"); ok && v != "" {
		cfg.SerpAPIKey = v
	}

	if v, ok := os.LookupEnv("AIFLOW_AGENT_TIMEOUT"); ok && v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.AIFlowAgentTimeout = d
		}
	}
	if v, ok := os.LookupEnv("AIFLOW_TOOL_TIMEOUT"); ok && v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.AIFlowToolTimeout = d
		}
	}
	if v, ok := os.LookupEnv("AIFLOW_OBSERVATION_MAX_BYTES"); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.AIFlowObservationMaxBytes = n
		}
	}
	if v, ok := os.LookupEnv("AIFLOW_COST_PER_1K_TOKENS"); ok && v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 {
			cfg.AIFlowCostPer1KTokens = f
		}
	}

	// Embedding
	if v, ok := os.LookupEnv("EMBEDDING_API_KEY"); ok && v != "" {
		cfg.EmbeddingAPIKey = v
	}
	if v, ok := os.LookupEnv("EMBEDDING_BASEURL"); ok && v != "" {
		cfg.EmbeddingBaseURL = v
	}
	if v, ok := os.LookupEnv("EMBEDDING_MODEL"); ok && v != "" {
		cfg.EmbeddingModel = v
	}

	return cfg
}
