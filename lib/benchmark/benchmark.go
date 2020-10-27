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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/HdrHistogram/hdrhistogram-go"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/trace"
	logrus "github.com/sirupsen/logrus"
)

const (
	// PauseTimeBetweenBenchmarks is the time to pause between each benchmark
	PauseTimeBetweenBenchmarks = time.Second * 5
)

// Generator provides a standardized way to get successive benchmarks from a consistent interface
type Generator interface {
	// Generate advances the Generator to the next generation.
	// It returns false when the generator no longer has configurations to run.
	Generate() bool
	// GetBenchmark returns the benchmark config for the current generation.
	// If called after Generate() returns false, this will result in an error.
	GetBenchmark() (Config, error)
	// Benchmark runs the benchmark of receiver type
	// return an array of Results that contain information about the generations
	Benchmark(context.Context, []string, *client.TeleportClient) ([]*Result, error)
}

// Run is used to run the benchmarks, it is given a generator, command to run,
// a host, host login, and proxy. If host login or proxy is an empty string, it will
// use the default login
func Run(ctx context.Context, cnf interface{}, cmd, host, login, proxy string) ([]Result, error) {
	tc, err := makeTeleportClient(host, login, proxy)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	logrus.SetLevel(logrus.ErrorLevel)
	switch c := cnf.(type) {
	case *Linear:
		return c.Benchmark(ctx, cmd, tc)

	}
	return nil, trace.BadParameter("invalid generator, not of type linear")
}

// makeTeleportClient creates an instance of a teleport client
func makeTeleportClient(host, login, proxy string) (*client.TeleportClient, error) {
	c := client.Config{}
	path := client.FullProfilePath("")

	c.Host = host
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

// ExportLatencyProfile exports the latency profile and returns the path as a string if no errors
func ExportLatencyProfile(path string, h *hdrhistogram.Histogram, ticks int32, valueScale float64) (string, error) {
	var fullPath string
	timeStamp := time.Now().Format("2006-01-02_15:04:05")
	suffix := fmt.Sprintf("latency_profile_%s.txt", timeStamp)

	if path != "." {
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				if err = os.MkdirAll(path, 0700); err != nil {
					return "", trace.Wrap(err)
				}
			} else {
				return "", trace.Wrap(err)
			}
		}
	}
	fullPath = filepath.Join(path, suffix)
	fo, err := os.Create(fullPath)
	if err != nil {
		return "", trace.Wrap(err)
	}

	if _, err := h.PercentilesPrint(fo, ticks, valueScale); err != nil {
		err := fo.Close()
		if err != nil {
			logrus.WithError(err).Warningf("Failed to close file")
		}
		return "", trace.Wrap(err)
	}

	if err := fo.Close(); err != nil {
		return "", trace.Wrap(err)
	}
	return fo.Name(), nil
}
