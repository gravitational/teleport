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

package benchmark

import (
	"context"
	"math/rand/v2"

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

		return tc.SSH(ctx, s.Command, opts...)
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
		name := hosts[rand.N(len(hosts))].GetName()
		return name + ":0"
	}
}
