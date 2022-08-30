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

// Package benchmark package provides tools to run progressive or independent benchmarks against teleport services.
package benchmark

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/observability/tracing"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/HdrHistogram/hdrhistogram-go"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

const (
	// minValue is the min millisecond recorded for histogram
	minValue = 1
	// maxValue is the max millisecond recorded for histogram
	maxValue = 60000
	// significantFigures is the precision of the values
	significantFigures = 3
	// pauseTimeBetweenBenchmarks is the time to pause between each benchmark
	pauseTimeBetweenBenchmarks = time.Second * 5
)

// Config specifies benchmark requests to run
type Config struct {
	// Rate is requests per second origination rate
	Rate int
	// Command is a command to run
	Command []string
	// Interactive turns on interactive sessions
	Interactive bool
	// MinimumWindow is the min duration
	MinimumWindow time.Duration
	// MinimumMeasurments is the min amount of requests
	MinimumMeasurements int
}

// Result is a result of the benchmark
type Result struct {
	// RequestsOriginated is amount of requests originated
	RequestsOriginated int
	// RequestsFailed is amount of requests failed
	RequestsFailed int
	// Histogram holds the response duration values
	Histogram *hdrhistogram.Histogram
	// LastError contains last recorded error
	LastError error
	// Duration it takes for the whole benchmark to run
	Duration time.Duration
}

// Run is used to run the benchmarks, it is given a generator, command to run,
// a host, host login, and proxy. If host login or proxy is an empty string, it will
// use the default login
func Run(ctx context.Context, lg *Linear, cmd, host, login, proxy string) ([]Result, error) {
	c := strings.Split(cmd, " ")
	lg.config = &Config{Command: c}
	if err := validateConfig(lg); err != nil {
		return nil, trace.Wrap(err)
	}
	tc, err := makeTeleportClient(host, login, proxy)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		exitSignals := make(chan os.Signal, 1)
		signal.Notify(exitSignals, syscall.SIGTERM, syscall.SIGINT)
		defer signal.Stop(exitSignals)
		sig := <-exitSignals
		logrus.Debugf("signal: %v", sig)
		cancel()
	}()
	var results []Result
	sleep := false
	for {
		if sleep {
			select {
			case <-time.After(pauseTimeBetweenBenchmarks):
			case <-ctx.Done():
				return results, trace.ConnectionProblem(ctx.Err(), "context canceled or timed out")
			}
		}
		benchmarkC := lg.GetBenchmark()
		if benchmarkC == nil {
			break
		}
		result, err := benchmarkC.Benchmark(ctx, tc)
		if err != nil {
			return results, trace.Wrap(err)
		}
		results = append(results, result)
		fmt.Printf("current generation requests: %v, duration: %v\n", result.RequestsOriginated, result.Duration)
		sleep = true
	}
	return results, nil
}

// ExportLatencyProfile exports the latency profile and returns the path as a string if no errors
func ExportLatencyProfile(path string, h *hdrhistogram.Histogram, ticks int32, valueScale float64) (string, error) {
	timeStamp := time.Now().Format("2006-01-02_15:04:05")
	suffix := fmt.Sprintf("latency_profile_%s.txt", timeStamp)
	if path != "." {
		if err := os.MkdirAll(path, 0700); err != nil {
			return "", trace.Wrap(err)
		}
	}
	fullPath := filepath.Join(path, suffix)
	fo, err := os.Create(fullPath)
	if err != nil {
		return "", trace.Wrap(err)
	}

	if _, err := h.PercentilesPrint(fo, ticks, valueScale); err != nil {
		if err := fo.Close(); err != nil {

			logrus.WithError(err).Warningf("failed to close file")
		}
		return "", trace.Wrap(err)
	}

	if err := fo.Close(); err != nil {
		return "", trace.Wrap(err)
	}
	return fo.Name(), nil
}

// Benchmark connects to remote server and executes requests in parallel according
// to benchmark spec. It returns benchmark result when completed.
// This is a blocking function that can be canceled via context argument.
func (c *Config) Benchmark(ctx context.Context, tc *client.TeleportClient) (Result, error) {
	tc.Stdout = io.Discard
	tc.Stderr = io.Discard
	tc.Stdin = &bytes.Buffer{}
	var delay time.Duration
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	requestsC := make(chan benchMeasure)
	resultC := make(chan benchMeasure)

	go func() {
		interval := time.Duration(1 / float64(c.Rate) * float64(time.Second))
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		start := time.Now()
		for {
			select {
			case <-ticker.C:
				// ticker makes its first tick after the given duration, not immediately
				// this sets the send measure ResponseStart time accurately
				delay = delay + interval
				t := start.Add(delay)
				measure := benchMeasure{
					ResponseStart: t,
					command:       c.Command,
					client:        tc,
					interactive:   c.Interactive,
				}
				go work(ctx, measure, resultC)
			case <-ctx.Done():
				close(requestsC)
				return
			}
		}
	}()

	var result Result
	result.Histogram = hdrhistogram.New(minValue, maxValue, significantFigures)
	statusTicker := time.NewTicker(1 * time.Second)
	timeElapsed := false
	start := time.Now()
	for {
		if c.MinimumWindow <= time.Since(start) {
			timeElapsed = true
		}
		select {
		case measure := <-resultC:
			result.Histogram.RecordValue(int64(measure.End.Sub(measure.ResponseStart) / time.Millisecond))
			result.RequestsOriginated++
			if timeElapsed && result.RequestsOriginated >= c.MinimumMeasurements {
				cancel()
			}
			if measure.Error != nil {
				result.RequestsFailed++
				result.LastError = measure.Error
			}
		case <-ctx.Done():
			result.Duration = time.Since(start)
			return result, nil
		case <-statusTicker.C:
			logrus.Infof("working... current observation count: %d", result.RequestsOriginated)
		}

	}
}

type benchMeasure struct {
	ResponseStart time.Time
	End           time.Time
	Error         error
	client        *client.TeleportClient
	command       []string
	interactive   bool
}

func work(ctx context.Context, m benchMeasure, send chan<- benchMeasure) {
	m.Error = execute(m)
	m.End = time.Now()
	select {
	case send <- m:
	case <-ctx.Done():
		return
	}
}

func execute(m benchMeasure) error {
	if !m.interactive {
		// do not use parent context that will cancel in flight requests
		// because we give test some time to gracefully wrap up
		// the in-flight connections to avoid extra errors
		return m.client.SSH(context.TODO(), m.command, false)
	}
	config := m.client.Config
	client, err := client.NewClient(&config)
	if err != nil {
		return err
	}
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	client.Stdin = reader
	out := &utils.SyncBuffer{}
	client.Stdout = out
	client.Stderr = out
	err = m.client.SSH(context.TODO(), nil, false)
	if err != nil {
		return err
	}
	writer.Write([]byte(strings.Join(m.command, " ") + "\r\nexit\r\n"))
	return nil
}

// makeTeleportClient creates an instance of a teleport client
func makeTeleportClient(host, login, proxy string) (*client.TeleportClient, error) {
	c := client.Config{
		Host:   host,
		Tracer: tracing.NoopProvider().Tracer("test"),
	}
	path := profile.FullProfilePath("")
	if login != "" {
		c.HostLogin = login
		c.Username = login
	}
	if proxy != "" {
		c.SSHProxyAddr = proxy
	}
	if err := c.LoadProfile(path, proxy); err != nil {
		return nil, trace.Wrap(err)
	}
	tc, err := client.NewClient(&c)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return tc, nil
}
