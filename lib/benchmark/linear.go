package benchmark

import (
	"context"
	"errors"
	"time"
)


// Linear generator
type Linear struct {
	LowerBound          int           `yaml:"lowerBound"`
	UpperBound          int           `yaml:"upperBound"`
	Step                int           `yaml:"step"`
	MinimumMeasurements int           `yaml:"minimumMeasurements"`
	MinimumWindow       time.Duration `yaml:"minimumWindow"`
	currentRPS          int
	config              Config
}

// Generate advances the Generator to the next generation.
// It returns false when the generator no longer has configurations to run.
func (l *Linear) Generate() bool {
	if l.currentRPS < l.LowerBound {
		l.currentRPS = l.LowerBound
		return true
	}
	l.currentRPS += l.Step
	if l.currentRPS > l.UpperBound {
		return false
	}
	return true
}

// GetBenchmark returns the benchmark config for the current generation.
// If called after Generate() returns false, this will result in an error.
func (l *Linear) GetBenchmark() (context.Context, Config, error) {
	if l.currentRPS > l.UpperBound {
		return nil, Config{}, errors.New("No more generations")
	}

	return context.TODO(), Config{
		MinimumWindow: l.MinimumWindow, 
		MinimumMeasurements: l.MinimumMeasurements,
		Rate: l.currentRPS,
		Duration: 0,
	}, nil
}
