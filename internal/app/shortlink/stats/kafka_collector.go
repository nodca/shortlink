package stats

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/segmentio/kafka-go"
)

type KafkaCollector struct {
	writer *kafka.Writer
}

func NewKafkaCollector(brokers []string, topic string) *KafkaCollector {
	return &KafkaCollector{
		writer: &kafka.Writer{
			Addr:     kafka.TCP(brokers...),
			Topic:    topic,
			Balancer: &kafka.LeastBytes{},
			Async:    true, // 异步发送
		},
	}
}

func (k *KafkaCollector) Collect(event ClickEvent) {
	data, _ := json.Marshal(event)
	err := k.writer.WriteMessages(context.Background(), kafka.Message{
		Value: data,
	})
	if err != nil {
		slog.Error("kafka write failed", "err", err)
	}
}

func (k *KafkaCollector) Close() {
	k.writer.Close()
}
