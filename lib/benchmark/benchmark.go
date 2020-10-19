package benchmark

import (
	"context"
	"log"
	"strings"

	"github.com/gravitational/teleport/lib/client"
	logrus "github.com/sirupsen/logrus"
)

// Generator provides a standardized way to get successive benchmarks from a consistent interface
type Generator interface {
	Generator() bool
	GetBenchmark() (context.Context, Config, error)
	Benchmark([]string, error, *client.TeleportClient)
}

// Run is used to run the benchmarks
func Run(cnf interface{}, cmd, host string) ([]*Result, error) {
	var results []*Result
	tc, err := makeTeleportClient(host)
	if err != nil {
		log.Fatalf("unable to make teleport client: %v", err)
	}
	logrus.SetLevel(logrus.ErrorLevel)
	command := strings.Split(cmd, " ")

	// Using type introspection here even though it's not relevant now, 
	// but it will be needed when I add more generators to the benchmark package
	if l, ok := cnf.(*Linear); ok {
		results, err = l.Benchmark(command, tc)
		if err != nil {
			return results, err
		}
	}
	return results, nil
}

// makeTeleportClient creates an instance of a teleport client
func makeTeleportClient(host string) (*client.TeleportClient, error) {
	c := client.Config{}
	path := client.FullProfilePath("")
	c.Host = host
	if err := c.LoadProfile(path, ""); err != nil {
		return nil, err
	}

	tc, err := client.NewClient(&c)
	if err != nil {
		return nil, err
	}
	return tc, err
}
