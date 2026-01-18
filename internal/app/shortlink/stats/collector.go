package stats

import "time"

//点击事件
type ClickEvent struct {
	Code      string
	ClickedAt time.Time //点击时间
	IP        string    //点击者的IP
	UserAgent string    //客户端信息（浏览器、操作系统）
	Referer   string    //从哪个页面点击过来的
}

// Collector 收集器接口（方便后续换 Kafka）
type Collector interface {
	Collect(event ClickEvent)
	Close()
}

// ChannelCollector 基于 channel 的收集器
type ChannelCollector struct {
	ch     chan ClickEvent
	closed bool
}

func NewChannelCollector(bufferSize int) *ChannelCollector {
	return &ChannelCollector{
		ch:     make(chan ClickEvent, bufferSize),
		closed: false,
	}
}

func (c *ChannelCollector) Collect(event ClickEvent) {
	if c.closed {
		return
	}
	select {
	case c.ch <- event:
	default:
		// 通道满了，丢弃
	}
}

func (c *ChannelCollector) Events() <-chan ClickEvent {
	return c.ch
}
func (c *ChannelCollector) Close() {
	c.closed = true
	close(c.ch)
}
