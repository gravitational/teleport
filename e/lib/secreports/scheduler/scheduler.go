package scheduler

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/e/lib/secreports/limiter"
)

type detailsGetter interface {
	GetDetails(ctx context.Context) (*limiter.Details, error)
}

// Config is the scheduler configuration.
type Config struct {
	// Limiter is the limiter.
	Limiter detailsGetter
	// Clock is the clock.
	Clock clockwork.Clock
	// MinInterval is the minimum interval between reports.
	MinInterval time.Duration
	// ReservedCapacityPercentage is the percentage of total capacity that will not be reserved by the scheduler.
	// This is used to prevent the scheduler draining the limiter completely leaving capacity for user queries.
	ReservedCapacityPercentage float64
}

// CheckAndSetDefaults checks and sets default parameters.
func (s *Config) CheckAndSetDefaults() error {
	if s.Limiter == nil {
		return trace.BadParameter("missing parameter Limiter")
	}
	if s.Clock == nil {
		s.Clock = clockwork.NewRealClock()
	}
	if s.MinInterval == 0 {
		s.MinInterval = time.Hour
	}

	if s.ReservedCapacityPercentage == 0 {
		// Reserve 60% of Limiter capacity for customer user queries.
		// This is to prevent the scheduler draining the limiter completely by internal security report scheduler.
		s.ReservedCapacityPercentage = 0.6
	}
	if s.ReservedCapacityPercentage < 0 || s.ReservedCapacityPercentage > 1 {
		return trace.BadParameter("reserved capacity percentage must be between 0 and 1")
	}
	return nil
}

// New creates a new scheduler.
func New(cfg Config) (*Scheduler, error) {
	return &Scheduler{
		Config: cfg,
	}, nil
}

// Scheduler is the scheduler for the security reports.
type Scheduler struct {
	Config
}

// Next returns the next time when the report should be generated.
func (s *Scheduler) Next(ctx context.Context) (time.Time, error) {
	details, err := s.Limiter.GetDetails(ctx)
	if err != nil {
		if trace.IsNotFound(err) {
			return s.Clock.Now(), nil
		}
		return time.Time{}, trace.Wrap(err)
	}
	if details.Current == 0 {
		return s.Clock.Now(), nil
	}

	if s.ReservedCapacityPercentage != 0 {
		details.Current += uint64(float64(details.Limit) * s.ReservedCapacityPercentage)
	}

	// If the limit is reached, return end time
	// when the capacity is refilled.
	if details.Current >= details.Limit {
		return details.End, nil
	}

	now := s.Clock.Now()
	if now.After(details.End) {
		return s.Clock.Now(), nil
	}
	val := details.End.Sub(details.Start)
	p := float64(details.Current) / float64(details.Limit)
	dur := time.Duration(float64(val) * p)
	triggerTime := details.Start.Add(dur)
	if triggerTime.Before(now) {
		return now, nil
	}
	return triggerTime, nil
}
