package config

import "time"

// Settings holds application settings
type Settings struct {
	LabelSelector        string
	UtilizationThreshold float64
	UnusedAge            time.Duration
	EvaluationPeriod     time.Duration
	DelayAfterError      time.Duration
	DelayAfterCordon     time.Duration
}
