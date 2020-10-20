package benchmark

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	// Generator advances the Generator to the next generation.
	// It returns false when the generator no longer has configurations to run.
	Generator() bool
	// GetBenchmark returns the benchmark config for the current generation.
	// If called after Generate() returns false, this will result in an error.
	GetBenchmark() (context.Context, Config, error)
	// Benchmark runs the benchmark of receiver type
	// return an array of Results that contain information about the generations
	Benchmark([]string, *client.TeleportClient) ([]*Result, error)
}

// Run is used to run the benchmarks, it is given a generator, command to run,
// a host, host login, and proxy. If host login or proxy is an empty string, it will
// use the default login
func Run(cnf interface{}, cmd, host, login, proxy string) ([]Result, error) {
	var results []Result
	tc, err := makeTeleportClient(host, login, proxy)
	if err != nil {
		return nil, err
	}
	logrus.SetLevel(logrus.ErrorLevel)
	command := strings.Split(cmd, " ")

	switch c := cnf.(type) {
	case *Linear:
		c.config.Command = command
		results, err = c.Benchmark(context.Background(), command, tc)
		if err != nil {
			return results, err
		}
		return results, nil
	}
	return nil, errors.New("Invalid generator, not of type Linear")
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
