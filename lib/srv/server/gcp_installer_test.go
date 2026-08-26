/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package server

import (
	"context"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/gcp"
	gcpimds "github.com/gravitational/teleport/lib/cloud/imds/gcp"
	"github.com/gravitational/teleport/lib/cryptosuites"
)

func TestGCPInstaller(t *testing.T) {
	t.Parallel()

	newVMFn := func(id string) *gcpimds.Instance {
		return &gcpimds.Instance{
			ProjectID: "test-project",
			Zone:      "test-zone",
			Name:      "vm" + id,
		}
	}

	const totalInstances = 1000
	instances := make([]*gcpimds.Instance, totalInstances)
	for i := range totalInstances {
		instances[i] = newVMFn(strconv.Itoa(i))
	}

	mockRunner := &mockGCPRunClient{}
	installer := &GCPInstaller{}
	err := installer.Run(t.Context(), GCPRunRequest{
		InstallerParams: &types.InstallerParams{
			PublicProxyAddr: "localhost:8080",
		},
		Zone:       "useast-1",
		ProjectID:  "test",
		Instances:  instances,
		Client:     mockRunner,
		SSHKeyAlgo: cryptosuites.ECDSAP256,
	})
	require.Error(t, err)

	// Each instance should have been attempted, even when they all fail.
	require.Equal(t, int32(totalInstances), mockRunner.getInstancesCallCounter.Load())
}

type mockGCPRunClient struct {
	gcp.InstancesClient
	getInstancesCallCounter atomic.Int32
}

func (c *mockGCPRunClient) GetInstance(ctx context.Context, req *gcpimds.InstanceRequest) (*gcpimds.Instance, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	c.getInstancesCallCounter.Add(1)
	return nil, trace.BadParameter("invalid something")
}
