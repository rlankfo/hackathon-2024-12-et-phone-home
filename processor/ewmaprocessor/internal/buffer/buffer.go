package buffer

import (
	"fmt"
	"time"

	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type Buffer interface {
	AddSpan(span ptrace.Span) error
	GetSpans(traceID string) ([]ptrace.Span, error)
}

type ringBuffer struct {
	spanBuffer  map[string][]ptrace.Span
	traceBuffer map[string]time.Time
	logger      *zap.Logger
	capacity    int
	ttl         time.Duration
}

func NewRingBuffer(ttl time.Duration, cap int, logger *zap.Logger) Buffer {
	return &ringBuffer{
		capacity:    cap,
		ttl:         ttl,
		spanBuffer:  make(map[string][]ptrace.Span),
		traceBuffer: make(map[string]time.Time),
		logger:      logger,
	}
}

func (rb *ringBuffer) AddSpan(span ptrace.Span) error {
	key := span.TraceID().String()
	expiryTime := time.Now().Add(rb.ttl)
	if _, exists := rb.spanBuffer[key]; !exists {
		rb.spanBuffer[key] = make([]ptrace.Span, 0)
	}
	rb.spanBuffer[key] = append(rb.spanBuffer[key], span)
	rb.traceBuffer[key] = expiryTime

	if len(rb.traceBuffer) > rb.capacity {
		oldestKey := rb.getOldestKey()
		if oldestKey != "" {
			rb.removeItem(oldestKey)
		}
	}

	return nil
}

func (rb *ringBuffer) GetSpans(traceID string) ([]ptrace.Span, error) {
	spans, exists := rb.spanBuffer[traceID]
	if !exists {
		return nil, fmt.Errorf("no spans found for %s", traceID)
	}

	return spans, nil
}

func (rb *ringBuffer) getOldestKey() string {
	var oldestKey string
	var oldestTime time.Time

	for key, ttl := range rb.traceBuffer {
		if oldestKey == "" || ttl.Before(oldestTime) {
			oldestKey = key
			oldestTime = ttl
		}
	}

	return oldestKey
}

func (rb *ringBuffer) removeItem(key string) {
	delete(rb.traceBuffer, key)
	delete(rb.spanBuffer, key)
}
