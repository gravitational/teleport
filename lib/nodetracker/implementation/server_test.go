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
	"os"
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/nodetracker"
	"github.com/gravitational/teleport/lib/nodetracker/api"

	"github.com/bojand/ghz/printer"
	"github.com/bojand/ghz/runner"
	"github.com/golang/protobuf/proto"
	"github.com/jhump/protoreflect/desc"
	"github.com/pborman/uuid"
	"github.com/stretchr/testify/require"
)

// BenchmarkServer benchmarks and load tests the grpc server
func BenchmarkServer(b *testing.B) {
	listener, err := net.Listen("tcp", ":0")
	require.Nil(b, err)

	NewServer(listener, 10*time.Minute)
	go func() {
		err := nodetracker.GetServer().Start()
		require.Nil(b, err)
	}()

	report, err := runner.Run(
		"api.NodeTrackerService.AddNode",
		listener.Addr().String(),
		runner.WithProtoFile("../api/nodetracker.proto", []string{"../../../vendor/github.com/gogo/protobuf"}),
		runner.WithBinaryDataFunc(
			func(mtd *desc.MethodDescriptor, cd *runner.CallData) []byte {
				msg := &api.AddNodeRequest{
					NodeID:      uuid.New(),
					ProxyID:     uuid.New(),
					ClusterName: "cluster",
					Addr:        "proxy:3080",
				}
				binData, _ := proto.Marshal(msg)
				return binData
			},
		),
		runner.WithTotalRequests(1000000),
		runner.WithInsecure(true),
	)
	require.Nil(b, err)

	printer := printer.ReportPrinter{
		Out:    os.Stdout,
		Report: report,
	}

	printer.Print("summary")
	nodetracker.GetServer().Stop()
}
