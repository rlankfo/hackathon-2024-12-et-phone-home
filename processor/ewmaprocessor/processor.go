package ewmaprocessor

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rlankfo/hackathon-2024-12-et-phone-home/processor/ewmaprocessor/internal/buffer"
	"github.com/rlankfo/hackathon-2024-12-et-phone-home/processor/ewmaprocessor/internal/calculator"
	internalmemberlist "github.com/rlankfo/hackathon-2024-12-et-phone-home/processor/ewmaprocessor/internal/memberlist"

	"github.com/hashicorp/memberlist"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

func wait_signal(cancel context.CancelFunc) {
	signal_chan := make(chan os.Signal, 1)
	signal.Notify(signal_chan, syscall.SIGINT)
	for {
		select {
		case s := <-signal_chan:
			log.Printf("signal %s happen", s.String())
			cancel()
		}
	}
}

// getEnv retrieves the value of the environment variable named by the key.
// It returns the value, which will be defaultValue if the variable is not present.
func getEnv(key, defaultValue string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		return defaultValue
	}
	return value
}

// spanProcessor implements the processor interface
type spanProcessor struct {
	logger     *zap.Logger
	next       consumer.Traces
	cfg        *Config
	calculator *calculator.EWMACalculator
	msgCh      chan []byte
	memberlist *internalmemberlist.MyDelegate
	spanBuffer buffer.Buffer
}

// Capabilities returns the consumer capabilities
func (p *spanProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// Start implements the component.Component interface
func (p *spanProcessor) Start(_ context.Context, _ component.Host) error {
	msgCh := make(chan []byte)
	p.msgCh = msgCh

	d := internalmemberlist.NewMyDelegate(&p.msgCh)
	p.memberlist = d

	conf := memberlist.DefaultLocalConfig()
	conf.Name = getEnv("HOSTNAME", "localhost")
	conf.BindPort = 7947 // avoid port confliction
	conf.AdvertisePort = conf.BindPort
	conf.Delegate = d

	list, err := memberlist.Create(conf)
	if err != nil {
		log.Fatal(err)
	}
	// print number of members
	log.Printf("ewma - %d members\n", list.NumMembers())

	local := list.LocalNode()
	list.Join([]string{
		fmt.Sprintf("%s:%d", local.Addr.To4().String(), local.Port),
	})
	log.Printf("ewma - join %s:%d\n", local.Addr.To4().String(), local.Port)

	stopCtx, cancel := context.WithCancel(context.TODO())
	go wait_signal(cancel)
	go p.listen(stopCtx)
	go flushBuffer(stopCtx, time.Minute, p.spanBuffer)

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
					p.spanBuffer.AddSpan(span)
				} else {
					// anomaly found, broadcast message containing the trace id to all nodes
					m := internalmemberlist.AnomalyMessage{
						AnomalyAction: internalmemberlist.AnomalyActionStart,
						GroupKey:      groupKey,
						TraceID:       span.TraceID(),
						SpanID:        span.SpanID(),
					}
					p.memberlist.Broadcasts.QueueBroadcast(m)

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

func flushBuffer(ctx context.Context, interval time.Duration, buf buffer.Buffer) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			buf.DiscardOldTraces()
		case <-ctx.Done():
			return
		}
	}
}

// listen listens for messages from the memberlist and takes actions such as adding to the inventory of spans to look out for
func (p *spanProcessor) listen(stopCtx context.Context) {
	run := true
	for run {
		select {
		case data := <-p.msgCh:
			msg, ok := internalmemberlist.ParseMyBroadcastMessage(data)
			if ok != true {
				continue
			}

			log.Printf("received broadcast msg: action=%d trace_id=%s span_id=%s group_key=%s", msg.AnomalyAction, msg.TraceID, msg.SpanID, msg.GroupKey)
			p.spanBuffer.MarkTrace(msg.TraceID.String(), true)

		case <-stopCtx.Done():
			log.Printf("stop called")
			run = false
		}
	}
}
