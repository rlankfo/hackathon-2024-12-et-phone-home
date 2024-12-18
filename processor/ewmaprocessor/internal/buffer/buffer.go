package buffer

import (
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type Buffer interface {
	AddSpan(span ptrace.Span) error
	GetSpans(traceID string) ([]ptrace.Span, error)
	DiscardOldTraces()
	MarkTrace(traceID string, anomaly bool) error
}

type traceBufferVal struct {
	anomaly bool
	expiry  time.Time
}

type ringBuffer struct {
	spanBuffer  map[string][]ptrace.Span
	traceBuffer map[string]*traceBufferVal
	logger      *zap.Logger
	capacity    int
	ttl         time.Duration
	mu          sync.Mutex
}

func NewRingBuffer(ttl time.Duration, cap int, logger *zap.Logger) Buffer {
	return &ringBuffer{
		capacity:    cap,
		ttl:         ttl,
		spanBuffer:  make(map[string][]ptrace.Span),
		traceBuffer: make(map[string]*traceBufferVal),
		logger:      logger,
	}
}

func (rb *ringBuffer) MarkTrace(traceID string, anomaly bool) error {
	_, exists := rb.traceBuffer[traceID]
	if !exists {
		return fmt.Errorf("trace does not exist in the buffer")
	}

	rb.traceBuffer[traceID].anomaly = anomaly

	return nil
}

func (rb *ringBuffer) AddSpan(span ptrace.Span) error {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	key := span.TraceID().String()
	expiryTime := time.Now().Add(rb.ttl)
	if _, exists := rb.spanBuffer[key]; !exists {
		rb.spanBuffer[key] = make([]ptrace.Span, 0)
	}
	rb.spanBuffer[key] = append(rb.spanBuffer[key], span)
	rb.traceBuffer[key] = &traceBufferVal{expiry: expiryTime}

	if len(rb.traceBuffer) > rb.capacity {
		oldestKey := rb.getOldestKey()
		if oldestKey != "" {
			rb.removeItem(oldestKey)
		}
	}

	return nil
}

func (rb *ringBuffer) DiscardOldTraces() {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	for traceID, val := range rb.traceBuffer {
		if !val.anomaly && val.expiry.Before(time.Now()) {
			rb.removeItem(traceID)
		}
	}
}

func (rb *ringBuffer) GetSpans(traceID string) ([]ptrace.Span, error) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	spans, exists := rb.spanBuffer[traceID]
	if !exists {
		return nil, fmt.Errorf("no spans found for %s", traceID)
	}

	return spans, nil
}

func (rb *ringBuffer) getOldestKey() string {
	var oldestKey string
	var oldestTime time.Time

	for key, val := range rb.traceBuffer {
		if oldestKey == "" || val.expiry.Before(oldestTime) {
			oldestKey = key
			oldestTime = val.expiry
		}
	}

	return oldestKey
}

func (rb *ringBuffer) removeItem(key string) {
	delete(rb.traceBuffer, key)
	delete(rb.spanBuffer, key)
}
