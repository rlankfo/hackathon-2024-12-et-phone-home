package buffer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type Buffer interface {
	AddSpan(span ptrace.Span) error
	GetSpans(traceID string) ([]ptrace.Span, error)
}

type ringBuffer struct {
	client   *redis.Client
	logger   *zap.Logger
	capacity int
	ttl      time.Duration
	traceSet string
}

func NewRingBuffer(url string, ttl time.Duration, cap int, logger *zap.Logger) (Buffer, error) {
	options, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("cannot parse redis URL:%v", err)
	}

	client := redis.NewClient(options)
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("cannot connect to redis :%v", err)
	}

	return &ringBuffer{
		client:   client,
		capacity: cap,
		traceSet: "trace_set",
		ttl:      ttl,
		logger:   logger,
	}, nil
}

func (rb *ringBuffer) AddSpan(span ptrace.Span) error {
	ctx := context.Background()

	spanData, err := json.Marshal(span)
	if err != nil {
		return fmt.Errorf("failed to serialize span: %v", err)
	}

	traceID := span.TraceID()

	spanKey := fmt.Sprintf("trace:%s", traceID)
	if err := rb.client.RPush(ctx, spanKey, spanData).Err(); err != nil {
		return fmt.Errorf("failed to add span to buffer: %v", err)
	}

	if err := rb.client.LPush(ctx, rb.traceSet, traceID).Err(); err != nil {
		return fmt.Errorf("failed to add traceID to trace list: %v", err)
	}

	traceSetSize, err := rb.client.LLen(ctx, rb.traceSet).Result()
	if err != nil {
		return fmt.Errorf("failed to get traceSet size: %v", err)
	}

	if traceSetSize > int64(rb.capacity) {
		oldestTraceID, err := rb.client.RPop(ctx, rb.traceSet).Result()
		if err != nil && err != redis.Nil {
			return fmt.Errorf("failed to pop oldest traceID: %v", err)
		}

		if oldestTraceID != "" {
			rb.client.Del(ctx, fmt.Sprintf("trace:%s", oldestTraceID))
		}
	}

	if err := rb.client.Expire(ctx, spanKey, rb.ttl).Err(); err != nil {
		return fmt.Errorf("failed to set TTL for trace key: %v", err)
	}
	if err := rb.client.Expire(ctx, rb.traceSet, rb.ttl).Err(); err != nil {
		return fmt.Errorf("failed to set TTL for traceSet key: %v", err)
	}

	return nil
}

func (rb *ringBuffer) GetSpans(traceID string) ([]ptrace.Span, error) {
	spanKey := fmt.Sprintf("trace:%s", traceID)
	entries, err := rb.client.LRange(context.Background(), spanKey, 0, -1).Result()
	if err != nil {
		if err == redis.Nil {
			return []ptrace.Span{}, nil
		}
		return nil, fmt.Errorf("failed to fetch spans for traceID %s: %v", traceID, err)
	}

	// Deserialize spans
	var spans []ptrace.Span
	for _, entry := range entries {
		var span ptrace.Span
		if err := json.Unmarshal([]byte(entry), &span); err != nil {
			rb.logger.Warn("Failed to deserialize span: ", zap.Error(err))
			continue
		}
		spans = append(spans, span)
	}

	return spans, nil
}
