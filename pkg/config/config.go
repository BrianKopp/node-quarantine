package config

import "time"

type Settings struct {
	LabelSelector        string
	UtilizationThreshold float64
	UnusedAge            time.Duration
	EvaluationPeriod     time.Duration
	DelayAfterError      time.Duration
	DelayAfterCordon     time.Duration
}

func NewFromEnv() Settings {
	return Settings{
		LabelSelector:        "",
		UtilizationThreshold: 0.5,
		UnusedAge:            10 * time.Minute,
	}
}
