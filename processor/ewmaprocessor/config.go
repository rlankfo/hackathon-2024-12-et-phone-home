package ewmaprocessor

import "fmt"

type Config struct {
	GroupingKeys []string `mapstructure:"grouping_keys"`
	Alpha        float64  `mapstructure:"alpha"`     // Smoothing factor between 0 and 1
	Threshold    float64  `mapstructure:"threshold"` // Percentage as decimal (e.g., 0.25 for 25%)
}

// Validate checks if the processor configuration is valid
func (cfg *Config) Validate() error {
	if len(cfg.GroupingKeys) == 0 {
		return fmt.Errorf("grouping_keys must not be empty")
	}
	if cfg.Alpha <= 0 || cfg.Alpha > 1 {
		return fmt.Errorf("alpha must be between 0 and 1")
	}
	if cfg.Threshold <= 0 || cfg.Threshold >= 1 {
		return fmt.Errorf("threshold must be between 0 and 1 (e.g., 0.25 for 25%% deviation)")
	}
	return nil
}
