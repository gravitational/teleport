/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

// Package benchmark package provides tools to run progressive or independent benchmarks against teleport services.
package benchmark

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/HdrHistogram/hdrhistogram-go"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/observability/tracing"
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

// Service is a the Teleport service to benchmark.
type Service string

const (
	// SSHService is the SSH service
	SSHService Service = "ssh"
	// KubernetesService is the Kubernetes service
	KubernetesService Service = "kube"
)

// Config specifies benchmark requests to run
type Config struct {
	// Rate is requests per second origination rate
	Rate int
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
func Run(ctx context.Context, lg *Linear, host, login, proxy string, suite Suite) ([]Result, error) {
	lg.config = &Config{}
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
		slog.DebugContext(ctx, "terminating benchmark due to signal", "signal", sig)
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
		result, err := benchmarkC.Benchmark(ctx, tc, suite)
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
func ExportLatencyProfile(ctx context.Context, path string, h *hdrhistogram.Histogram, ticks int32, valueScale float64) (string, error) {
	timeStamp := time.Now().Format("2006-01-02_15:04:05")
	suffix := fmt.Sprintf("latency_profile_%s.txt", timeStamp)
	if path != "." {
		if err := os.MkdirAll(path, 0o700); err != nil {
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
			slog.WarnContext(ctx, "failed to close latency profile file", "error", err)
		}
		return "", trace.Wrap(err)
	}

	if err := fo.Close(); err != nil {
		return "", trace.Wrap(err)
	}
	return fo.Name(), nil
}

// WorkloadFunc is a function that executes a single benchmark call.
type WorkloadFunc func(context.Context) error

// Suite is an interface that defines a benchmark suite.
type Suite interface {
	// BenchBuilder returns a function that executes a single benchmark call.
	// The returned function is called in a loop until the context is canceled.
	BenchBuilder(context.Context, *client.TeleportClient) (WorkloadFunc, error)
}

// configOverrider is implemented by a [Suite] that automatically
// overrides some configuration parameters.
type configOverrider interface {
	ConfigOverride(ctx context.Context, tc *client.TeleportClient, cfg *Config) error
}

// Benchmark connects to remote server and executes requests in parallel according
// to benchmark spec. It returns a benchmark result when completed.
// This is a blocking function that can be canceled via context argument.
func (c *Config) Benchmark(ctx context.Context, tc *client.TeleportClient, suite Suite) (Result, error) {
	if suite == nil {
		return Result{}, trace.BadParameter("missing benchmark suite")
	}

	if cfg, ok := suite.(configOverrider); ok {
		if err := cfg.ConfigOverride(ctx, tc, c); err != nil {
			return Result{}, trace.Wrap(err)
		}
	}

	tc.Stdout = io.Discard
	tc.Stderr = io.Discard
	tc.Stdin = &bytes.Buffer{}

	workload, err := suite.BenchBuilder(ctx, tc)
	if err != nil {
		return Result{}, trace.Wrap(err)
	}

	var delay time.Duration
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	requestsC := make(chan benchMeasure)
	resultC := make(chan benchMeasure)

	var wg sync.WaitGroup
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
				}

				wg.Add(1)
				go func() {
					defer wg.Done()
					work(ctx, measure, resultC, workload)
				}()
			case <-ctx.Done():
				close(requestsC)
				return
			}
		}
	}()

	defer wg.Wait()

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
			slog.InfoContext(ctx, "working...", "current_observation_count", result.RequestsOriginated)
		}
	}
}

type benchMeasure struct {
	ResponseStart time.Time
	End           time.Time
	Error         error
}

func work(ctx context.Context, m benchMeasure, send chan<- benchMeasure, workload WorkloadFunc) {
	m.Error = workload(ctx)
	m.End = time.Now()
	select {
	case send <- m:
	case <-ctx.Done():
		return
	}
}

// makeTeleportClient creates an instance of a teleport client
func makeTeleportClient(host, login, proxy string) (*client.TeleportClient, error) {
	c := client.Config{
		Host:        host,
		Tracer:      tracing.NoopProvider().Tracer("test"),
		ClientStore: client.NewFSClientStore(""),
	}

	if login != "" {
		c.HostLogin = login
		c.Username = login
	}
	if proxy != "" {
		c.SSHProxyAddr = proxy
	}

	if err := c.LoadProfile(proxy); err != nil {
		return nil, trace.Wrap(err)
	}
	tc, err := client.NewClient(&c)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return tc, nil
}
