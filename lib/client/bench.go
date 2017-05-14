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

package client

import (
	"context"
	"io/ioutil"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/codahale/hdrhistogram"
)

// Benchmark specifies benchmark requests to run
type Benchmark struct {
	// Threads is amount of concurrent execution threads to run
	Threads int
	// Rate is requests per second origination rate
	Rate int
	// Duration is test duration
	Duration time.Duration
	// Command is a command to run
	Command []string
}

// BenchmarkResult is a result of the benchmark
type BenchmarkResult struct {
	// RequestsOriginated is amount of reuqests originated
	RequestsOriginated int
	// RequestsFailed is amount of requests failed
	RequestsFailed int
	// Histogram is a duration histogram
	Histogram *hdrhistogram.Histogram
	// LastError contains last recorded error
	LastError error
}

// Benchmark connects to remote server and executes requests in parallel according
// to benchmark spec. It returns benchmark result when completed.
// This is a blocking function that can be cancelled via context argument.
func (tc *TeleportClient) Benchmark(ctx context.Context, bench Benchmark) (*BenchmarkResult, error) {
	tc.Stdout = ioutil.Discard
	tc.Stderr = ioutil.Discard

	ctx, cancel := context.WithTimeout(ctx, bench.Duration)
	defer cancel()

	requestC := make(chan *benchMeasure)
	responseC := make(chan *benchMeasure, bench.Threads)

	// create goroutines for concurrency
	for i := 0; i < bench.Threads; i++ {
		go benchmarkThread(i, ctx, tc, bench.Command, requestC, responseC)
	}

	// producer goroutine
	go func() {
		interval := time.Duration(float64(1) / float64(bench.Rate) * float64(time.Second))
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

	var result BenchmarkResult
	// from one millisecond to 60000 milliseconds (minute) with 3 digits precision
	result.Histogram = hdrhistogram.New(1, 60000, 3)

	var doneThreads int
	for {
		select {
		case <-ctx.Done():
			return &result, nil
		case measure := <-responseC:
			if measure.ThreadCompleted {
				doneThreads += 1
				if doneThreads == bench.Threads {
					return &result, nil
				}
			} else {
				if measure.Error != nil {
					result.RequestsFailed += 1
					result.LastError = measure.Error
				}
				result.RequestsOriginated += 1
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

func benchmarkThread(threadID int, ctx context.Context, tc *TeleportClient, command []string, receiveC chan *benchMeasure, sendC chan *benchMeasure) {
	sendMeasure := func(measure *benchMeasure) {
		measure.ThreadID = threadID
		select {
		case sendC <- measure:
		default:
			log.Warningf("blocked on measure send\n")
		}
	}
	defer func() {
		if r := recover(); r != nil {
			log.Warningf("recover from panic: %v", r)
			sendMeasure(&benchMeasure{ThreadCompleted: true})
		}
	}()

	for {
		select {
		case measure := <-receiveC:
			err := tc.SSH(ctx, command, false)
			measure.Error = err
			measure.End = time.Now()
			sendMeasure(measure)
		case <-ctx.Done():
			sendMeasure(&benchMeasure{
				ThreadCompleted: true,
			})
			return
		}
	}
}
