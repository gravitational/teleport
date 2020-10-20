/*
Copyright 2017 Gravitational, Inc.
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
	// Threads is amount of concurrent execution threads to run
	Threads int
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
	// Histogram is a duration histogram
	Histogram *hdrhistogram.Histogram
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
	workerCtx, cancelWorkers := context.WithCancel(context.Background())
	defer cancelWorkers()

	requestC := make(chan benchMeasure)
	responseC := make(chan benchMeasure, c.Threads)

	for i := 0; i < c.Threads; i++ {
		thread := &benchmarkThread{
			id:          i,
			ctx:         ctx,
			client:      tc,
			command:     c.Command,
			interactive: c.Interactive,
			receiveC:    requestC,
			sendC:       responseC,
		}
		go thread.run()
	}

	go produceMeasures(ctx, c.Rate, requestC)

	var result Result
	// from one millisecond to 60000 milliseconds (minute) with 3 digits precision, refer to constants
	result.Histogram = hdrhistogram.New(MinValue, MaxValue, SignificantFigures)
	results := make([]benchMeasure, 0, c.MinimumMeasurements)
	statusTicker := time.NewTicker(1 * time.Second)
	timeElapsed := false
	start := time.Now()

	for {
		if c.MinimumWindow <= time.Since(start) {
			timeElapsed = true
		}
		select {
		case measure := <-responseC:
			result.Histogram.RecordValue(int64(measure.End.Sub(measure.Start) / time.Millisecond))
			results = append(results, measure)
			if timeElapsed && len(results) >= c.MinimumMeasurements {
				go cancelWorkers()
			}
			if measure.Error != nil {
				result.RequestsFailed++
				result.LastError = measure.Error
			}
			result.RequestsOriginated++
		case <-workerCtx.Done():
			result.Duration = time.Since(start)
			return result, nil
		case <-statusTicker.C:
			log.Printf("working... observations: %d", len(results))
		}
	}
}

func produceMeasures(ctx context.Context, rate int, c chan<- benchMeasure) {
	interval := time.Duration(float64(1) / float64(rate) * float64(time.Second))
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:

			measure := benchMeasure{
				Start: time.Now(),
			}
			select {
			case c <- measure:
			case <-ctx.Done():
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

type benchMeasure struct {
	Start           time.Time
	End             time.Time
	ThreadCompleted bool
	ThreadID        int
	Error           error
}

type benchmarkThread struct {
	id          int
	ctx         context.Context
	client      *client.TeleportClient
	command     []string
	interactive bool
	receiveC    chan benchMeasure
	sendC       chan benchMeasure
}

func (b *benchmarkThread) execute(measure benchMeasure) {
	if !b.interactive {
		// do not use parent context that will cancel in flight requests
		// because we give test some time to gracefully wrap up
		// the in-flight connections to avoid extra errors
		measure.Error = b.client.SSH(context.TODO(), nil, false)
		measure.End = time.Now()
		b.sendMeasure(measure)
		return
	}
	config := b.client.Config
	client, err := client.NewClient(&config)
	reader, writer := io.Pipe()
	client.Stdin = reader
	out := &bytes.Buffer{}
	client.Stdout = out
	client.Stderr = out
	if err != nil {
		measure.Error = err
		measure.End = time.Now()
		b.sendMeasure(measure)
		return
	}
	done := make(chan bool)
	go func() {
		measure.Error = b.client.SSH(b.ctx, nil, false)
		measure.End = time.Now()
		b.sendMeasure(measure)
		close(done)
	}()
	writer.Write([]byte(strings.Join(b.command, " ") + "\r\nexit\r\n"))
	<-done
}

func (b *benchmarkThread) sendMeasure(measure benchMeasure) {
	measure.ThreadID = b.id
	select {
	case b.sendC <- measure:
	default:
		logrus.Warningf("blocked on measure send\n")
	}
}

func (b *benchmarkThread) run() {
	defer func() {
		if r := recover(); r != nil {
			logrus.Warningf("recover from panic: %v", r)
			b.sendMeasure(benchMeasure{ThreadCompleted: true})
		}
	}()

	for {
		select {
		case measure := <-b.receiveC:
			b.execute(measure)
		case <-b.ctx.Done():
			b.sendMeasure(benchMeasure{
				ThreadCompleted: true,
			})
			return
		}
	}
}
