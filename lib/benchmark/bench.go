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
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"time"

	"log"

	"github.com/HdrHistogram/hdrhistogram-go"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/trace"
	logrus "github.com/sirupsen/logrus"
	yaml "gopkg.in/yaml.v2"
)

// Config specifies benchmark requests to run
type Config struct {
	// Threads is amount of concurrent execution threads to run
	Threads int
	// Rate is requests per second origination rate
	Rate int
	// Duration is the test duration, used to run original benchmark
	Duration time.Duration
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

// ConfigureAndRun configures the type of benchmark and runs it
func ConfigureAndRun(ctx context.Context, benchConfig Config, tc *client.TeleportClient, configPath string, isProgressive bool) (*Result, error) {
	var linearConfig *Linear
	var result *Result
	var err error
	var results []*Result
	var total int

	if configPath != "" {
		linearConfig, err = parseConfig(configPath)
		if err != nil {
			log.Fatalf("Unable to parse config file %v", err)
		}

		// Generate increases the RPS based in config rate until upper bound is exceeded
		for linearConfig.Generate() {
			c, benchmarkC, err := linearConfig.GetBenchmark()
			if err != nil {
				break
			}
			benchmarkC.Threads = 1
			benchmarkC.Command = benchConfig.Command
			result, err = ProgressiveBenchmark(c, benchmarkC, tc)
			total = result.RequestsOriginated
			results = append(results, result)
			fmt.Printf("current generation requests: %v, duration: %v", total, result.Duration)
			time.Sleep(5 * time.Second)
		}
		return result, err
	}

	result, err = Benchmark(ctx, benchConfig, tc)
	return result, err
}

// ProgressiveBenchmark ...
func ProgressiveBenchmark(ctx context.Context, benchConfig Config, tc *client.TeleportClient) (*Result, error) {
	tc.Stdout = ioutil.Discard
	tc.Stderr = ioutil.Discard
	tc.Stdin = &bytes.Buffer{}
	workerCtx, cancelWorkers := context.WithCancel(context.Background())
	defer cancelWorkers()

	minMeasures := benchConfig.MinimumMeasurements

	resultC := make(chan *benchMeasure)
	responseC := make(chan *benchMeasure, benchConfig.Threads)

	for i := 0; i < benchConfig.Threads; i++ {
		thread := &benchmarkThread{
			id:          i,
			ctx:         ctx,
			client:      tc,
			command:     benchConfig.Command,
			interactive: benchConfig.Interactive,
			receiveC:    resultC,
			sendC:       responseC,
		}
		go thread.run()
	}

	go func() {
		interval := time.Duration(float64(1) / float64(benchConfig.Rate) * float64(time.Second))
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				measure := &benchMeasure{
					Start: time.Now(),
				}
				resultC <- measure
			case <-ctx.Done():
				return
			}
		}
	}()

	var result Result
	var timeoutC <-chan time.Time

	result.Histogram = hdrhistogram.New(1, 60000, 3)
	results := make([]*benchMeasure, 0, minMeasures)
	statusTicker := time.NewTicker(1 * time.Second)
	timeElapsed := false
	start := time.Now()

	for {
		if benchConfig.MinimumWindow <= time.Since(start) {
			timeElapsed = true
		}
		select {
		case <-timeoutC:
			result.LastError = trace.BadParameter("several requests hang: timeout waiting for threads to finish")
			return &Result{RequestsOriginated: len(results), Duration: time.Since(start), Histogram: result.Histogram, LastError: result.LastError}, nil
		case measure := <-responseC:
			err := result.Histogram.RecordValue(int64(measure.End.Sub(measure.Start) / time.Millisecond))
			if err != nil {
				log.Println(err)
			}
			results = append(results, measure)
			if timeElapsed && len(results) >= benchConfig.MinimumMeasurements {
				go cancelWorkers()
			}
			if measure.Error != nil {
				result.RequestsFailed++
				result.LastError = measure.Error
			}
			result.RequestsOriginated++
		case <-workerCtx.Done():
			workerCtx = nil
			waitTime := time.Duration(result.Histogram.Max()) * time.Millisecond
			// going to wait latency + buffer to give requests in flight to wrap up
			waitTime = time.Duration(1.2 * float64(waitTime))
			timeoutC = time.After(waitTime)
			return &Result{RequestsOriginated: len(results), Duration: time.Since(start), Histogram: result.Histogram, LastError: result.LastError}, nil
		case <-statusTicker.C:
			log.Printf("working... observations: %d", len(results))
		}
	}
}

// Benchmark connects to remote server and executes requests in parallel according
// to benchmark spec. It returns benchmark result when completed.
// This is a blocking function that can be cancelled via context argument.
func Benchmark(ctx context.Context, benchConfig Config, tc *client.TeleportClient) (*Result, error) {
	tc.Stdout = ioutil.Discard
	tc.Stderr = ioutil.Discard
	tc.Stdin = &bytes.Buffer{}
	ctx, cancel := context.WithTimeout(ctx, benchConfig.Duration)

	defer cancel()

	requestC := make(chan *benchMeasure)
	responseC := make(chan *benchMeasure, benchConfig.Threads)

	// create goroutines for concurrency
	for i := 0; i < benchConfig.Threads; i++ {
		thread := &benchmarkThread{
			id:          i,
			ctx:         ctx,
			client:      tc,
			command:     benchConfig.Command,
			interactive: benchConfig.Interactive,
			receiveC:    requestC,
			sendC:       responseC,
		}
		go thread.run()
	}

	// producer goroutine
	go func() {
		interval := time.Duration(float64(1) / float64(benchConfig.Rate) * float64(time.Second))
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				// notice how we start the timer regardless of whether any goroutine can process it
				// this is to account for coordinated omission,
				// http://psy-lob-saw.blogspot.com/2015/03/fixing-ycsb-coordinated-omission.html
				measure := &benchMeasure{
					Start: time.Now(),
				}
				select {
				case requestC <- measure:
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	var result Result
	// from one millisecond to 60000 milliseconds (minute) with 3 digits precision
	result.Histogram = hdrhistogram.New(1, 60000, 3)

	var doneThreads int
	var timeoutC <-chan time.Time
	doneC := ctx.Done()
	for {
		select {
		case <-timeoutC:
			result.LastError = trace.BadParameter("several requests hang: timeout waiting for %v threads to finish", benchConfig.Threads-doneThreads)
			return &result, nil
		case <-doneC:
			// give it a couple of seconds to wrap up the goroutines,
			// set up the timer that will fire up if the all goroutines were not finished
			doneC = nil
			waitTime := time.Duration(result.Histogram.Max()) * time.Millisecond
			// going to wait latency + buffer to give requests in flight to wrap up
			waitTime = time.Duration(1.2 * float64(waitTime))
			timeoutC = time.After(waitTime)
		case measure := <-responseC:
			if measure.ThreadCompleted {
				doneThreads++
				if doneThreads == benchConfig.Threads {
					return &result, nil
				}
			} else {
				if measure.Error != nil {
					result.RequestsFailed++
					result.LastError = measure.Error
				}
				result.RequestsOriginated++
				result.Histogram.RecordValue(int64(measure.End.Sub(measure.Start) / time.Millisecond))
			}
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
	receiveC    chan *benchMeasure
	sendC       chan *benchMeasure
}

func (b *benchmarkThread) execute(measure *benchMeasure) {
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

func (b *benchmarkThread) sendMeasure(measure *benchMeasure) {
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
			b.sendMeasure(&benchMeasure{ThreadCompleted: true})
		}
	}()

	for {
		select {
		case measure := <-b.receiveC:
			b.execute(measure)
		case <-b.ctx.Done():
			b.sendMeasure(&benchMeasure{
				ThreadCompleted: true,
			})
			return
		}
	}
}

func defaultConfig() (*Linear) {
	defaultDuration := 30 * time.Second
	return &Linear{
		LowerBound: 10, 
		UpperBound: 50,
		Step: 10, 
		MinimumMeasurements: 1000, 
		MinimumWindow: defaultDuration,
	}
}

func parseConfig(path string) (*Linear, error) {
	linearConfig := &Linear{}
	yamlFile, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(yamlFile, linearConfig)
	if err != nil {
		return nil, err
	}
	return linearConfig, nil
}
