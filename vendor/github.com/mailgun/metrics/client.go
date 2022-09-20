package metrics

import (
	"bytes"
	"fmt"
	"math/rand"

	"runtime"
	"strconv"

	"sync"
	"time"
)

type Client interface {
	// Close closes the connection and cleans up.
	Close() error

	// Increments a statsd count type.
	// stat is a string name for the metric.
	// value is the integer value
	// rate is the sample rate (0.0 to 1.0)
	Inc(stat interface{}, value int64, rate float32) error

	// Decrements a statsd count type.
	// stat is a string name for the metric.
	// value is the integer value.
	// rate is the sample rate (0.0 to 1.0).
	Dec(stat interface{}, value int64, rate float32) error

	// Submits/Updates a statsd gauge type.
	// stat is a string name for the metric.
	// value is the integer value.
	// rate is the sample rate (0.0 to 1.0).
	Gauge(stat interface{}, value int64, rate float32) error

	// Submits a delta to a statsd gauge.
	// stat is the string name for the metric.
	// value is the (positive or negative) change.
	// rate is the sample rate (0.0 to 1.0).
	GaugeDelta(stat interface{}, value int64, rate float32) error

	// Submits a statsd timing type.
	// stat is a string name for the metric.
	// value is the integer value.
	// rate is the sample rate (0.0 to 1.0).
	Timing(stat interface{}, delta int64, rate float32) error

	// Emit duration in milliseconds
	TimingMs(stat interface{}, tm time.Duration, rate float32) error

	// Submits a stats set type, where value is the unique string
	// rate is the sample rate (0.0 to 1.0).
	UniqueString(stat interface{}, value string, rate float32) error

	// Submits a stats set type
	// rate is the sample rate (0.0 to 1.0).
	UniqueInt64(stat interface{}, value int64, rate float32) error

	// Reports runtime metrics
	ReportRuntimeMetrics(prefix string, rate float32) error

	// Sets/Updates the statsd client prefix
	SetPrefix(prefix string)

	Metric(p ...string) Metric
}

// StatsdOptions allows tuning client for efficiency
type Options struct {
	// UseBuffering turns on buffering of metrics what reduces amount of UDP packets
	UseBuffering bool
	// FlushBytes will trigger the packet send whenever accumulated buffer size will reach this value, default is 1440
	FlushBytes int
	// FlushPeriod will trigger periodic flushes in case of inactivity to avoid metric loss
	FlushPeriod time.Duration
}

func NewWithOptions(addr, prefix string, opts Options) (Client, error) {
	s, err := newUDPSender(addr)
	if err != nil {
		return nil, err
	}

	if opts.UseBuffering {
		if opts.FlushBytes == 0 {
			opts.FlushBytes = 1440 // 1500(Ethernet MTU) - 60(Max UDP header size)
		}
		if opts.FlushBytes < 128 {
			return nil, fmt.Errorf("Too small flush bytes value, min is 128")
		}
		if opts.FlushPeriod == 0 {
			opts.FlushPeriod = 100 * time.Millisecond
		}
		// 60 bytes for UDP max header length
		b, err := newBufSender(s, opts.FlushBytes, opts.FlushPeriod)
		if err != nil {
			return nil, err
		}
		s = b
	}

	client := &client{
		s:         s,
		prefix:    prefix,
		mtx:       &sync.Mutex{},
		prevNumGC: -1,
	}

	return client, nil
}

// New returns a new statsd Client, and an error.
// addr is a string of the format "hostname:port", and must be parsable by net.ResolveUDPAddr.
// prefix is the statsd client prefix. Can be "" if no prefix is desired.
func New(addr, prefix string) (Client, error) {
	return NewWithOptions(addr, prefix, Options{})
}

type client struct {
	s sender

	// prefix for statsd name
	prefix string

	// To report memory stats correctly
	mtx *sync.Mutex

	// Previosly reported garbage collection number
	prevNumGC int32
	// Last garbage collection time
	lastGC uint64
}

func (s *client) Close() error {
	return s.s.Close()
}

func (s *client) Metric(p ...string) Metric {
	return NewMetric(s.prefix, p...)
}

func (s *client) Inc(stat interface{}, value int64, rate float32) error {
	return s.submit("c", stat, value, false, "", rate)
}

func (s *client) Dec(stat interface{}, value int64, rate float32) error {
	return s.Inc(stat, -value, rate)
}

func (s *client) Gauge(stat interface{}, value int64, rate float32) error {
	return s.submit("g", stat, value, false, "", rate)
}

func (s *client) GaugeDelta(stat interface{}, value int64, rate float32) error {
	return s.submit("g", stat, value, true, "", rate)
}

func (s *client) Timing(stat interface{}, delta int64, rate float32) error {
	return s.submit("ms", stat, delta, false, "", rate)
}

func (s *client) TimingMs(stat interface{}, d time.Duration, rate float32) error {
	return s.Timing(stat, int64(d/time.Millisecond), rate)
}

func (s *client) UniqueString(stat interface{}, value string, rate float32) error {
	return s.submit("s", stat, 0, false, value, rate)
}

func (s *client) UniqueInt64(stat interface{}, value int64, rate float32) error {
	return s.submit("s", stat, value, false, "", rate)
}

func (s *client) ReportRuntimeMetrics(prefix string, rate float32) error {
	stats := &runtime.MemStats{}
	runtime.ReadMemStats(stats)

	s.mtx.Lock()
	defer s.mtx.Unlock()

	s.Gauge(s.Metric(prefix, "runtime", "goroutines"), int64(runtime.NumGoroutine()), rate)

	s.Gauge(s.Metric(prefix, "runtime", "mem", "alloc"), int64(stats.Alloc), rate)
	s.Gauge(s.Metric(prefix, "runtime", "mem", "sys"), int64(stats.Sys), rate)
	s.Gauge(s.Metric(prefix, "runtime", "mem", "lookups"), int64(stats.Lookups), rate)
	s.Gauge(s.Metric(prefix, "runtime", "mem", "mallocs"), int64(stats.Mallocs), rate)
	s.Gauge(s.Metric(prefix, "runtime", "mem", "frees"), int64(stats.Frees), rate)

	s.Gauge(s.Metric(prefix, "runtime", "mem", "heap", "alloc"), int64(stats.HeapAlloc), rate)
	s.Gauge(s.Metric(prefix, "runtime", "mem", "heap", "sys"), int64(stats.HeapSys), rate)
	s.Gauge(s.Metric(prefix, "runtime", "mem", "heap", "idle"), int64(stats.HeapIdle), rate)
	s.Gauge(s.Metric(prefix, "runtime", "mem", "heap", "inuse"), int64(stats.HeapInuse), rate)
	s.Gauge(s.Metric(prefix, "runtime", "mem", "heap", "released"), int64(stats.HeapReleased), rate)
	s.Gauge(s.Metric(prefix, "runtime", "mem", "heap", "objects"), int64(stats.HeapObjects), rate)

	prevNumGC := s.prevNumGC
	lastGC := s.lastGC

	s.prevNumGC = int32(stats.NumGC)
	s.lastGC = stats.LastGC
	if prevNumGC == -1 {
		return nil
	}

	countGC := int32(stats.NumGC) - prevNumGC
	if countGC < 0 {
		return fmt.Errorf("Invalid number of garbage collections: %d", countGC)
	}

	// Nothing changed since last call, nothing to report
	if countGC == 0 {
		return nil
	}

	// We have missed some reportings and overwrote the data
	if countGC > 256 {
		countGC = 256
	}

	s.Timing(s.Metric(prefix, "runtime", "gc", "periodns"), int64(stats.LastGC-lastGC), rate)

	for i := int32(0); i < countGC; i += 1 {
		idx := int((stats.NumGC-uint32(i))+255) % 256
		s.Timing(s.Metric(prefix, "runtime", "gc", "pausens"), int64(stats.PauseNs[idx]), rate)
	}

	return nil
}

func (s *client) SetPrefix(prefix string) {
	s.prefix = prefix
}

// submit formats the statsd event data, handles sampling, and prepares it,
// and sends it to the server.
func (s *client) submit(metricType string, stat interface{}, value int64, sign bool, sval string, rate float32) error {
	if rate < 1 && rand.Float32() > rate {
		return nil
	}

	var buf *bytes.Buffer
	switch m := stat.(type) {
	case string:
		buf = &bytes.Buffer{}
		if s.prefix != "" {
			buf.WriteString(s.prefix)
			buf.WriteString(".")
		}
		buf.WriteString(escape(m))
	case *metric:
		buf = m.b
	default:
		return fmt.Errorf("Unexpected argument type: %T", stat)
	}

	buf.WriteByte(':')
	if sval != "" {
		buf.WriteString(escape(sval))
	} else {
		if sign {
			if value >= 0 {
				buf.WriteByte('+')
			}
		}
		buf.WriteString(strconv.FormatInt(value, 10))
	}

	buf.WriteByte('|')
	buf.WriteString(metricType)

	if rate < 1 {
		buf.WriteString("|@")
		buf.WriteString(strconv.FormatFloat(float64(rate), 'f', -1, 32))
	}

	_, err := buf.WriteTo(s.s)
	return err
}
