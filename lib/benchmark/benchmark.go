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
	GetBenchmarkResults(Result) error
	RunBenchmark(Config, error)
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

	if l, ok := cnf.(*Linear); ok {
		results, err = l.RunBenchmark(command, tc)
		if err != nil {
			log.Println(err)
		}

	}
	return results, nil
}

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
