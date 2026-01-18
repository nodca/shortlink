package main

import (
	"fmt"
	"log"
	"os"

	"day.local/internal/llm"

	"github.com/joho/godotenv"
)

func main() {
	// 加载 .env 文件
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	baseURL := os.Getenv("DEEPSEEK_BASEURL")
	model := os.Getenv("DEEPSEEK_MODEL")

	if apiKey == "" {
		log.Fatal("DEEPSEEK_API_KEY is not set")
	}
	if baseURL == "" {
		baseURL = "https://api.siliconflow.cn/v1"
	}
	if model == "" {
		model = "Pro/deepseek-ai/DeepSeek-V3"
	}

	fmt.Printf("Using baseURL: %s, model: %s\n", baseURL, model)

	client := llm.NewDeepSeekClient(apiKey, baseURL, model)

	resp, err := client.Chat("用一句话介绍一下 Go 语言")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Response:", resp)
}
