package ewmaprocessor

import (
	"context"
	"github.com/rlankfo/hackathon-2024-12-et-phone-home/processor/ewmaprocessor/internal/calculator"
	"testing"
	"time"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type mockConsumer struct {
	traces []ptrace.Traces
}

func (m *mockConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (m *mockConsumer) ConsumeTraces(_ context.Context, td ptrace.Traces) error {
	m.traces = append(m.traces, td)
	return nil
}

func createTestSpan(startTime, endTime pcommon.Timestamp, name string) ptrace.Span {
	span := ptrace.NewSpan()
	span.SetName(name)
	span.SetStartTimestamp(startTime)
	span.SetEndTimestamp(endTime)
	return span
}

func TestSpanProcessor_ConsumeTraces(t *testing.T) {
	cfg := &Config{
		GroupingKeys: []string{"service.name"},
		Alpha:        0.2,
		Threshold:    0.25,
	}

	mockNext := &mockConsumer{}
	logger := zap.NewExample()

	processor := &spanProcessor{
		logger:     logger,
		next:       mockNext,
		cfg:        cfg,
		calculator: calculator.NewEWMACalculator(cfg.Alpha, cfg.Threshold, cfg.GroupingKeys),
	}

	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()

	attrs := rs.Resource().Attributes()
	attrs.PutStr("service.name", "test-service")

	ils := rs.ScopeSpans().AppendEmpty()

	baseTime := pcommon.NewTimestampFromTime(time.Now())

	spans := []struct {
		duration   time.Duration
		name       string
		shouldKeep bool
	}{
		{100 * time.Millisecond, "baseline", false},
		{110 * time.Millisecond, "normal", false},
		{200 * time.Millisecond, "high", true},
		{50 * time.Millisecond, "low", true},
		{120 * time.Millisecond, "normal2", false},
	}

	t.Log("Adding test spans:")
	expectedKept := 0
	for _, s := range spans {
		span := createTestSpan(
			baseTime,
			baseTime+pcommon.Timestamp(s.duration),
			s.name,
		)
		span.CopyTo(ils.Spans().AppendEmpty())
		if s.shouldKeep {
			expectedKept++
		}
		t.Logf("Added span: name=%s, duration=%v, shouldKeep=%v",
			s.name, s.duration, s.shouldKeep)
	}

	t.Logf("Initial span count: %d", ils.Spans().Len())

	err := processor.ConsumeTraces(context.Background(), traces)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(mockNext.traces) != 1 {
		t.Errorf("expected 1 trace, got %d", len(mockNext.traces))
	}

	processedSpans := mockNext.traces[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans()
	t.Logf("Final span count: %d (expected %d)", processedSpans.Len(), expectedKept)

	for i := 0; i < processedSpans.Len(); i++ {
		span := processedSpans.At(i)
		duration := span.EndTimestamp() - span.StartTimestamp()
		t.Logf("Kept span: name=%s, duration=%v",
			span.Name(), duration.AsTime().Sub(time.Time{}))
	}

	if processedSpans.Len() != expectedKept {
		t.Errorf("expected %d spans to be kept, got %d", expectedKept, processedSpans.Len())
	}
}
