/*
Copyright 2020 Gravitational, Inc.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package benchmark

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"strings"
	"sync"
	"time"

	"log"

	"github.com/HdrHistogram/hdrhistogram-go"
	"github.com/gravitational/teleport/lib/client"
	logrus "github.com/sirupsen/logrus"
)

const (
	// MinValue is the min millisecond recorded for histogram
	MinValue = 1
	// MaxValue is the max millisecond recorded for histogram
	MaxValue = 60000
	// SignificantFigures is the precision of the values
	SignificantFigures = 3
)

// Config specifies benchmark requests to run
type Config struct {
	// Rate is requests per second origination rate
	Rate int
	// Command is a command to run
	Command []string
	// Interactive turns on interactive sessions
	Interactive bool
	//MinimumWindow is the min duration
	MinimumWindow time.Duration
	//MinimumMeasurments is the min amount of requests
	MinimumMeasurements int
}

// Result is a result of the benchmark
type Result struct {
	// RequestsOriginated is amount of reuqests originated
	RequestsOriginated int
	// RequestsFailed is amount of requests failed
	RequestsFailed int
	// ResponseHistogram is a duration actual request histogram
	ResponseHistogram *hdrhistogram.Histogram
	//ServiceHistogram is the duration of service histogram
	ServiceHistogram *hdrhistogram.Histogram
	// LastError contains last recorded error
	LastError error
	// Duration it takes for the whole benchmark to run
	Duration time.Duration
}

// Benchmark connects to remote server and executes requests in parallel according
// to benchmark spec. It returns benchmark result when completed.
// This is a blocking function that can be cancelled via context argument.
func (c *Config) Benchmark(ctx context.Context, tc *client.TeleportClient) (Result, error) {
	tc.Stdout = ioutil.Discard
	tc.Stderr = ioutil.Discard
	tc.Stdin = &bytes.Buffer{}
	var delay time.Duration = 0

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	requestsC := make(chan *benchMeasure)
	resultC := make(chan *benchMeasure)

	go func() {
		interval := time.Duration(float64(1) / float64(c.Rate) * float64(time.Second))
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		start := time.Now()
		for {
			select {
			case <-ticker.C:
				delay = delay + interval
				t := start.Add(delay)
				measure := benchMeasure{
					ResponseStart: t,
					command:       c.Command,
					client:        tc,
					ctx:           ctx,
					interactive:   c.Interactive,
				}
				requestsC <- &measure
			case <-ctx.Done():
				close(requestsC)
				return
			}
		}
	}()

	go run(ctx, requestsC, resultC)

	var result Result
	result.ResponseHistogram = hdrhistogram.New(MinValue, MaxValue, SignificantFigures)
	result.ServiceHistogram = hdrhistogram.New(MinValue, MaxValue, SignificantFigures)
	results := make([]*benchMeasure, 0, c.MinimumMeasurements)
	statusTicker := time.NewTicker(1 * time.Second)
	timeElapsed := false
	start := time.Now()
	for {
		if c.MinimumWindow <= time.Since(start) {
			timeElapsed = true
		}
		select {
		case measure := <-resultC:
			results = append(results, measure)
			if timeElapsed && len(results) >= c.MinimumMeasurements {
				go cancel()
			}
			if measure.Error != nil {
				result.RequestsFailed++
				result.LastError = measure.Error
			}
			result.ResponseHistogram.RecordValue(int64(measure.End.Sub(measure.ResponseStart) / time.Millisecond))
			result.ServiceHistogram.RecordValue(int64(measure.End.Sub(measure.ServiceStart) / time.Millisecond))
			result.RequestsOriginated++
		case <-ctx.Done():
			result.Duration = time.Since(start)
			return result, nil
		case <-statusTicker.C:
			log.Printf("working... current observation count: %d", len(results))
		}

	}
}

type benchMeasure struct {
	ResponseStart time.Time
	ServiceStart  time.Time
	End           time.Time
	Error         error
	ctx           context.Context
	client        *client.TeleportClient
	command       []string
	interactive   bool
}

func run(ctx context.Context, request <-chan *benchMeasure, done chan<- *benchMeasure) {
	defer func() {
		if r := recover(); r != nil {
			logrus.Warningf("recover from panic: %v", r)
		}
	}()
	for {
		select {
		case m, ok := <-request:
			if !ok {
				return
			}
			go worker(ctx, m, done)
		case <-ctx.Done():
			return
		}
	}
}

func worker(ctx context.Context, m *benchMeasure, send chan<- *benchMeasure) {
	m.ServiceStart = time.Now()
	execute(m)
	m.End = time.Now()
	select {
	case send <- m:
	case <-ctx.Done():
		return
	}
}

func execute(m *benchMeasure) {
	if !m.interactive {
		// do not use parent context that will cancel in flight requests
		// because we give test some time to gracefully wrap up
		// the in-flight connections to avoid extra errors
		m.Error = m.client.SSH(context.TODO(), nil, false)
		m.End = time.Now()
		return
	}
	config := m.client.Config
	client, err := client.NewClient(&config)
	reader, writer := io.Pipe()
	client.Stdin = reader
	out := sync.Pool{New: func() interface{} {
		return new(bytes.Buffer)
	}}
	buffer := out.Get().(*bytes.Buffer)
	client.Stdout = buffer
	client.Stderr = buffer
	if err != nil {
		m.Error = err
		m.End = time.Now()
		return
	}
	done := make(chan bool)
	go func() {
		m.Error = m.client.SSH(m.ctx, nil, false)
		m.End = time.Now()
		close(done)
	}()
	writer.Write([]byte(strings.Join(m.command, " ") + "\r\nexit\r\n"))
	<-done
}
