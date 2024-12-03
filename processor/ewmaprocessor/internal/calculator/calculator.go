package calculator

import (
	"crypto/sha256"
	"encoding/hex"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"strings"
	"sync"
	"time"
)

type EWMACalculator struct {
	mu           sync.RWMutex
	averages     map[string]float64
	alpha        float64  // Smoothing factor
	threshold    float64  // Threshold multiplier for filtering
	groupingKeys []string // Resource attributes to group by
}

func NewEWMACalculator(alpha float64, threshold float64, groupingKeys []string) *EWMACalculator {
	return &EWMACalculator{
		averages:     make(map[string]float64),
		alpha:        alpha,
		threshold:    threshold,
		groupingKeys: groupingKeys,
	}
}

// CalculateGroupKey generates a hash based on resource attributes
func (e *EWMACalculator) CalculateGroupKey(resource pcommon.Resource) string {
	var values []string
	attrs := resource.Attributes()

	for _, key := range e.groupingKeys {
		if val, ok := attrs.Get(key); ok {
			values = append(values, val.AsString())
		} else {
			values = append(values, "")
		}
	}

	// Create a hash of the combined values
	hasher := sha256.New()
	hasher.Write([]byte(strings.Join(values, "|")))
	return hex.EncodeToString(hasher.Sum(nil))
}

// UpdateAndCheck updates the EWMA for a group and returns if the duration is within threshold
func (e *EWMACalculator) UpdateAndCheck(groupKey string, duration time.Duration) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	durationMs := float64(duration.Milliseconds())

	currentAvg, exists := e.averages[groupKey]
	if !exists {
		e.averages[groupKey] = durationMs
		return false // First value establishes baseline
	}

	// Calculate new EWMA
	newAvg := (e.alpha * durationMs) + ((1 - e.alpha) * currentAvg)

	// Check if current duration is OUTSIDE threshold percentage of CURRENT average
	// Use currentAvg instead of newAvg to decide if this span is anomalous
	upperBound := currentAvg * (1 + e.threshold)
	lowerBound := currentAvg * (1 - e.threshold)

	// Update the average AFTER checking
	e.averages[groupKey] = newAvg

	// Return true if the duration is OUTSIDE the normal range
	return durationMs > upperBound || durationMs < lowerBound
}
