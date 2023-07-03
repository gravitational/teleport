/*
Copyright 2023 Gravitational, Inc.
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
	"math/rand"

	"github.com/gravitational/trace"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
)

// SSHBenchmark is a benchmark suite that connects to the configured
// target hosts and executes the provided command.
type SSHBenchmark struct {
	// Command to execute on the host.
	Command []string
	// Random whether to connect to a random host or not
	Random bool
}

// BenchBuilder returns a WorkloadFunc for the given benchmark suite.
func (s SSHBenchmark) BenchBuilder(ctx context.Context, tc *client.TeleportClient) (WorkloadFunc, error) {
	var resources []types.Server
	if s.Random {
		if tc.Host != "all" {
			return nil, trace.BadParameter("random ssh bench commands must use the format <user>@all <command>")
		}

		clt, err := tc.ConnectToCluster(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		defer clt.Close()

		resources, err = apiclient.GetAllResources[types.Server](ctx, clt.AuthClient, tc.ResourceFilter(types.KindNode))
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if len(resources) == 0 {
			return nil, trace.BadParameter("no target hosts available")
		}
	}

	return func(ctx context.Context) error {
		var opts []func(*client.SSHOptions)
		if len(resources) > 0 {
			opts = append(opts, client.WithHostAddress(chooseRandomHost(resources)))
		}

		return tc.SSH(ctx, s.Command, false, opts...)
	}, nil
}

// chooseRandomHost returns a random hostport from the given slice.
func chooseRandomHost(hosts []types.Server) string {
	switch len(hosts) {
	case 0:
		return ""
	case 1:
		name := hosts[0].GetName()
		return name + ":0"
	default:
		name := hosts[rand.Intn(len(hosts))].GetName()
		return name + ":0"
	}
}
