package gofakes3

import "time"

type TimeSource interface {
	Now() time.Time
	Since(time.Time) time.Duration
}

type TimeSourceAdvancer interface {
	TimeSource
	Advance(by time.Duration)
}

// FixedTimeSource provides a source of time that always returns the
// specified time.
func FixedTimeSource(at time.Time) TimeSourceAdvancer {
	return &fixedTimeSource{time: at}
}

func DefaultTimeSource() TimeSource {
	return &locatedTimeSource{
		// XXX: uses time.FixedZone to 'fake' the GMT timezone that S3 uses
		// (which is basically just UTC with a different name) to avoid
		// time.LoadLocation, which requires zoneinfo.zip to be available and
		// can break spectacularly on Windows (https://github.com/golang/go/issues/21881)
		// or Docker.
		timeLocation: time.FixedZone("GMT", 0),
	}
}

type locatedTimeSource struct {
	timeLocation *time.Location
}

func (l *locatedTimeSource) Now() time.Time {
	return time.Now().In(l.timeLocation)
}

func (l *locatedTimeSource) Since(t time.Time) time.Duration {
	return time.Since(t)
}

type fixedTimeSource struct {
	time time.Time
}

func (l *fixedTimeSource) Now() time.Time {
	return l.time
}

func (l *fixedTimeSource) Since(t time.Time) time.Duration {
	return l.time.Sub(t)
}

func (l *fixedTimeSource) Advance(by time.Duration) {
	l.time = l.time.Add(by)
}
