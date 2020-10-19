package benchmark

import (
	"context"
	"errors"
	"log"
	"strings"

	"github.com/gravitational/teleport/lib/client"
	logrus "github.com/sirupsen/logrus"
)

// Generator provides a standardized way to get successive benchmarks from a consistent interface
type Generator interface {
	Generator() bool
	GetBenchmark() (context.Context, Config, error)
	Benchmark([]string, *client.TeleportClient) ([]*Result, error)
}

// Run is used to run the benchmarks, it is given a generator, command to run,
// a host, host login, and proxy. If host login or proxy is an empty string, it will
// use the default login
func Run(cnf interface{}, cmd, host, login, proxy string) ([]*Result, error) {
	var results []*Result
	tc, err := makeTeleportClient(host, login, proxy)
	if err != nil {
		log.Fatalf("Unable to make teleport client: %v", err)
	}
	logrus.SetLevel(logrus.ErrorLevel)
	command := strings.Split(cmd, " ")

	// Using type introspection here even though it's not relevant now,
	// but it will be needed when I add more generators to the benchmark package
	if l, ok := cnf.(*Linear); ok {
		l.config.Command = command
		results, err = l.Benchmark(command, tc)
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
		return nil, err
	}

	tc, err := client.NewClient(&c)
	if err != nil {
		return nil, err
	}
	return tc, err
}
