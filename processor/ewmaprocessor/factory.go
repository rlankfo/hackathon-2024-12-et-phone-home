package ewmaprocessor

import (
	"context"
	"time"

	"github.com/rlankfo/hackathon-2024-12-et-phone-home/processor/ewmaprocessor/internal/buffer"
	"github.com/rlankfo/hackathon-2024-12-et-phone-home/processor/ewmaprocessor/internal/calculator"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
)

const (
	typeStr = "ewmaprocessor"
)

// NewFactory creates a new factory for the processor
func NewFactory() processor.Factory {
	return processor.NewFactory(
		component.MustNewType(typeStr),
		createDefaultConfig,
		processor.WithTraces(createTracesProcessor, component.StabilityLevelDevelopment),
	)
}

// createDefaultConfig creates the default configuration for the processor
func createDefaultConfig() component.Config {
	return &Config{
		GroupingKeys: []string{"service.name"},
		Alpha:        0.2,
		Threshold:    0.25,
	}
}

// createTracesProcessor creates a trace processor based on the config
func createTracesProcessor(
	_ context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (processor.Traces, error) {
	pCfg := cfg.(*Config)

	return &spanProcessor{
		logger:     set.Logger,
		next:       nextConsumer,
		cfg:        pCfg,
		calculator: calculator.NewEWMACalculator(pCfg.Alpha, pCfg.Threshold, pCfg.GroupingKeys),
		spanBuffer: buffer.NewRingBuffer(2*time.Minute, 20, set.Logger),
	}, nil
}
