/*
Copyright 2021 Gravitational, Inc.

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

package implementation

import (
	"fmt"
	"net"
	"os/exec"
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/nodetracker"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

const (
	requests    = 1000000
	connections = 10
	data        = `{"node_id":"{{newUUID}}", "proxy_id":"{{newUUID}}", "cluster_name":"cluster", "addr":"proxy:3080"}`
)

// BenchmarkServer benchmarks and load tests the grpc server
// This does not use the golang benchmarking capabilities but instead uses ghz
// For more info visit https://ghz.sh
func BenchmarkServer(b *testing.B) {
	// TODO(NajiObeid): update this test when the grpc server supports mtls
	listener, err := net.Listen("tcp", ":0")
	require.Nil(b, err)

	NewServer(listener, 10*time.Minute)
	go func() {
		err := nodetracker.GetServer().Start()
		require.Nil(b, err)
	}()
	defer nodetracker.GetServer().Stop()

	// It is unfortunate ghz.sh does not play well with Teleport dependencies
	// And I don't think this test warrants dealing with all the dependency
	// updates.
	//
	// So instead of using ghz as a library, we will be using the CLI if it exists
	// on the system.
	//
	// Here's what the original code looked like
	/*
		report, err := runner.Run(
			"api.NodeTrackerService.AddNode",
			listener.Addr().String(),
			runner.WithProtoFile("../api/nodetracker.proto", []string{"../../../vendor/github.com/gogo/protobuf"}),
			runner.WithDataFromJSON(data),
			runner.WithTotalRequests(requests),
			runner.WithConnections(connections),
			runner.WithInsecure(true),
		)
		require.Nil(b, err)
		printer := printer.ReportPrinter{
			Out:    os.Stdout,
			Report: report,
		}
		printer.Print("summary")
	*/

	ghzCLI, err := exec.LookPath("ghz")
	if err != nil {
		log.Warn("Could not find ghz cli: skipping benchmark")
		return
	}

	args := []string{
		"--insecure",
		"--proto=../api/nodetracker.proto",
		"--import-paths=../../../vendor/github.com/gogo/protobuf",
		"--call=api.NodeTrackerService.AddNode",
		"--data=" + data,
		fmt.Sprintf("--total=%d", requests),
		fmt.Sprintf("--connections=%d", connections),
		listener.Addr().String(),
	}

	cmd := exec.Command(ghzCLI, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Warnf("Error running ghz benchmark: %+v\n%s", err, output)
		return
	}

	log.Infof("%s", output)
}
