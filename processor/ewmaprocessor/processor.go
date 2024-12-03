package ewmaprocessor

import (
	"context"
	"github.com/rlankfo/hackathon-2024-12-et-phone-home/processor/ewmaprocessor/internal/calculator"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

// spanProcessor implements the processor interface
type spanProcessor struct {
	logger     *zap.Logger
	next       consumer.Traces
	cfg        *Config
	calculator *calculator.EWMACalculator
}

// Capabilities returns the consumer capabilities
func (p *spanProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// Start implements the component.Component interface
func (p *spanProcessor) Start(_ context.Context, _ component.Host) error {
	return nil
}

// Shutdown implements the component.Component interface
func (p *spanProcessor) Shutdown(context.Context) error {
	return nil
}

// ConsumeTraces processes the span data and forwards to the next consumer
func (p *spanProcessor) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		groupKey := p.calculator.CalculateGroupKey(rs.Resource())

		ilss := rs.ScopeSpans()
		for j := 0; j < ilss.Len(); j++ {
			ils := ilss.At(j)
			spans := ils.Spans()

			// Remove spans that are within the threshold (keep anomalous ones)
			spans.RemoveIf(func(span ptrace.Span) bool {
				duration := time.Duration(span.EndTimestamp() - span.StartTimestamp())

				isAnomaly := p.calculator.UpdateAndCheck(groupKey, duration)
				if !isAnomaly {
					p.logger.Debug("Filtering normal span",
						zap.String("name", span.Name()),
						zap.String("groupKey", groupKey),
						zap.Duration("duration", duration))
				} else {
					p.logger.Debug("Keeping anomalous span",
						zap.String("name", span.Name()),
						zap.String("groupKey", groupKey),
						zap.Duration("duration", duration))
				}
				return !isAnomaly // Remove if NOT anomalous (remove normal spans)
			})
		}
	}

	return p.next.ConsumeTraces(ctx, td)
}
