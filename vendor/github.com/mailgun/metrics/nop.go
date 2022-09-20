package metrics

import (
	"time"
)

// nopclient does nothing when called, useful in tests
type nopclient struct {
}

func (s *nopclient) Close() error {
	return nil
}

func (s *nopclient) Inc(stat interface{}, value int64, rate float32) error {
	return nil
}

func (s *nopclient) Dec(stat interface{}, value int64, rate float32) error {
	return nil
}

func (s *nopclient) Gauge(stat interface{}, value int64, rate float32) error {
	return nil
}

func (s *nopclient) GaugeDelta(stat interface{}, value int64, rate float32) error {
	return nil
}

func (s *nopclient) Timing(stat interface{}, delta int64, rate float32) error {
	return nil
}

func (s *nopclient) UniqueString(stat interface{}, value string, rate float32) error {
	return nil
}

func (s *nopclient) UniqueInt64(stat interface{}, value int64, rate float32) error {
	return nil
}

func (s *nopclient) SetPrefix(prefix string) {

}

func (s *nopclient) TimingMs(stat interface{}, d time.Duration, rate float32) error {
	return nil
}

func (s *nopclient) ReportRuntimeMetrics(prefix string, rate float32) error {
	return nil
}

func (s *nopclient) Metric(p ...string) Metric {
	return NewMetric("", p...)
}

func NewNop() Client {
	return &nopclient{}
}
