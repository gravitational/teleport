package metrics

import (
	"bytes"
	"net"
	"reflect"
	"testing"
	"time"

	. "gopkg.in/check.v1"
)

func TestStatsdClient(t *testing.T) { TestingT(t) }

type CSuite struct {
}

var _ = Suite(&CSuite{})

func (s *CSuite) TestClientPrefix(c *C) {
	tests := []packet{
		{"Gauge", "gauge", 1, 1.0, "test.gauge:1|g", false},
		{"Inc", "count", 1, 0.999999, "test.count:1|c|@0.999999", false},
		{"Inc", "count", 1, 1.0, "test.count:1|c", false},
		{"Dec", "count", 1, 1.0, "test.count:-1|c", false},
		{"Timing", "timing", 1, 1.0, "test.timing:1|ms", false},
	}

	newClient := func(addr string) Client {
		cl, err := New(addr, "test")
		c.Assert(err, IsNil)
		return cl
	}

	testClient(c, newClient, tests)
}

func (s *CSuite) TestClientEmptyPrefix(c *C) {
	tests := []packet{
		{"Inc", "count", 1, 1.0, "count:1|c", false},
		{"GaugeDelta", "gauge", 1, 1.0, "gauge:+1|g", false},
		{"GaugeDelta", "gauge", -1, 1.0, "gauge:-1|g", false},
	}

	newClient := func(addr string) Client {
		cl, err := New(addr, "")
		c.Assert(err, IsNil)
		return cl
	}

	testClient(c, newClient, tests)
}

func (s *CSuite) TestEscapedMetrics(c *C) {
	tests := []packet{
		{"Gauge", "gauge:b", 1, 1.0, "test.gauge_b:1|g", false},
		{"Inc", "count b", 1, 0.999999, "test.count_b:1|c|@0.999999", false},
		{"Inc", "count<>b", 1, 1.0, "test.count__b:1|c", false},
		{"Dec", "count~12", 1, 1.0, "test.count_12:-1|c", false},
	}

	newClient := func(addr string) Client {
		cl, err := New(addr, "test")
		c.Assert(err, IsNil)
		return cl
	}

	testClient(c, newClient, tests)
}

func (s *CSuite) TestBufferedClient(c *C) {
	tests := []packet{
		{"Gauge", "gauge", 1, 1.0, "", true},
		{"Inc", "count", 1, 0.999999, "", true},
		{"Inc", "count", 1, 1.0, "", true},
		{"Dec", "count", 1, 1.0, "", true},
		{"Timing", "timing", 1, 1.0, "", true},
		{"Inc", "counter", 1, 1.0, "", true},
		{"GaugeDelta", "gauge", 1, 1.0, "", true},
		{"GaugeDelta", "gauge", -1, 1.0, ``, true},
		{"Inc", "flush", 1, 0.999999, `prefix.gauge:1|g
prefix.count:1|c|@0.999999
prefix.count:1|c
prefix.count:-1|c
prefix.timing:1|ms
prefix.counter:1|c
prefix.gauge:+1|g
prefix.gauge:-1|g
`, false},
		{"Gauge", "gauge", 1, 1.0, "", true},
		{"Inc", "count", 1, 0.999999, "", true},
		{"Inc", "count", 1, 1.0, "", true},
		{"Dec", "count", 1, 1.0, "", true},
		{"Timing", "timing", 1, 1.0, "", true},
		{"Inc", "counter", 1, 1.0, "", true},
		{"GaugeDelta", "gauge", 1, 1.0, "", true},
		{"GaugeDelta", "gauge", -1, 1.0, ``, true},
		{"Inc", "flush", 1, 0.999999, `prefix.flush:1|c|@0.999999
prefix.gauge:1|g
prefix.count:1|c|@0.999999
prefix.count:1|c
prefix.count:-1|c
prefix.timing:1|ms
prefix.counter:1|c
`, false},
	}

	newClient := func(addr string) Client {
		cl, err := NewWithOptions(addr, "prefix", Options{UseBuffering: true, FlushBytes: 160, FlushPeriod: 100 * time.Second})
		c.Assert(err, IsNil)
		return cl
	}

	testClient(c, newClient, tests)
}

func (s *CSuite) TestBufferedClientFlush(c *C) {
	tests := []packet{
		{"Gauge", "gauge", 1, 1.0, "", true},
		{"Gauge", "gauge", 1, 1.0, "prefix.gauge:1|g\nprefix.gauge:1|g\n", false},
	}

	newClient := func(addr string) Client {
		cl, err := NewWithOptions(addr, "prefix", Options{UseBuffering: true, FlushPeriod: 100 * time.Millisecond})
		c.Assert(err, IsNil)
		return cl
	}

	testClient(c, newClient, tests)
}

func testClient(c *C, newClient newClient, tests []packet) {
	l, err := newUDPListener("127.0.0.1:0")
	if err != nil {
		c.Fatal(err)
	}
	defer l.Close()
	metrics := make(chan []byte)
	go func() {
		for {
			data := make([]byte, 1024)
			_, _, err = l.ReadFrom(data)
			if err != nil {
				e, ok := err.(net.Error)
				if ok && e.Timeout() {
					continue
				}
				return
			}
			metrics <- data
		}
	}()
	cl := newClient(l.LocalAddr().String())
	defer cl.Close()
	for _, tt := range tests {
		method := reflect.ValueOf(cl).MethodByName(tt.Method)
		e := method.Call([]reflect.Value{
			reflect.ValueOf(tt.Stat),
			reflect.ValueOf(tt.Value),
			reflect.ValueOf(tt.Rate)})[0]
		errInter := e.Interface()
		if errInter != nil {
			c.Fatal(errInter.(error))
		}
		if tt.Skip {
			continue
		}
		var data []byte
		select {
		case data = <-metrics:
		case <-time.After(time.Second):
			c.Fatal("Timeout wairing for a metric")
		}

		data = bytes.TrimRight(data, "\x00")
		if bytes.Equal(data, []byte(tt.Expected)) != true {
			cl.Close()
			c.Fatalf("%s got\n'%#v'\nexpected\n'%#v'\n", tt.Method, string(data), string(tt.Expected))
		}

	}
}

func (s *CSuite) TestReportSystemMetrics(c *C) {
	l, err := newUDPListener("127.0.0.1:0")
	c.Assert(err, IsNil)
	defer l.Close()
	cl, err := New(l.LocalAddr().String(), "runtime")
	for i := 0; i < 1000; i += 1 {
		c.Assert(err, IsNil)
		cl.ReportRuntimeMetrics("runtime.metrics", 1)
	}
}

func (s *CSuite) TestMetric(c *C) {
	m := NewMetric("a", "b")
	c.Assert(m.String(), Equals, "a.b")

	c.Assert(m.Metric("c").String(), Equals, "a.b.c")
	c.Assert(m.Metric("d").String(), Equals, "a.b.d")
	c.Assert(m.String(), Equals, "a.b")
}

func newUDPListener(addr string) (*net.UDPConn, error) {
	l, err := net.ListenPacket("udp", addr)
	if err != nil {
		return nil, err
	}
	l.SetDeadline(time.Now().Add(time.Second))
	l.SetReadDeadline(time.Now().Add(time.Second))
	l.SetWriteDeadline(time.Now().Add(time.Second))
	return l.(*net.UDPConn), nil
}

type packet struct {
	Method   string
	Stat     string
	Value    int64
	Rate     float32
	Expected string
	Skip     bool // skip waiting and don't expect to receive anything
}

type newClient func(addr string) Client
