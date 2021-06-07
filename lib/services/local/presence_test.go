/*
Copyright 2017 Gravitational, Inc.

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

package local

import (
	"context"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/pborman/uuid"
	"github.com/stretchr/testify/require"
	"gopkg.in/check.v1"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/defaults"

	"github.com/gravitational/trace"
)

type PresenceSuite struct {
	bk backend.Backend
}

var _ = check.Suite(&PresenceSuite{})

func (s *PresenceSuite) SetUpTest(c *check.C) {
	var err error

	s.bk, err = lite.New(context.TODO(), backend.Params{"path": c.MkDir()})
	c.Assert(err, check.IsNil)
}

func (s *PresenceSuite) TearDownTest(c *check.C) {
	c.Assert(s.bk.Close(), check.IsNil)
}

func (s *PresenceSuite) TestTrustedClusterCRUD(c *check.C) {
	ctx := context.Background()
	presenceBackend := NewPresenceService(s.bk)

	tc, err := types.NewTrustedCluster("foo", types.TrustedClusterSpecV2{
		Enabled:              true,
		Roles:                []string{"bar", "baz"},
		Token:                "qux",
		ProxyAddress:         "quux",
		ReverseTunnelAddress: "quuz",
	})
	c.Assert(err, check.IsNil)

	// we just insert this one for get all
	stc, err := types.NewTrustedCluster("bar", types.TrustedClusterSpecV2{
		Enabled:              false,
		Roles:                []string{"baz", "aux"},
		Token:                "quux",
		ProxyAddress:         "quuz",
		ReverseTunnelAddress: "corge",
	})
	c.Assert(err, check.IsNil)

	// create trusted clusters
	_, err = presenceBackend.UpsertTrustedCluster(ctx, tc)
	c.Assert(err, check.IsNil)
	_, err = presenceBackend.UpsertTrustedCluster(ctx, stc)
	c.Assert(err, check.IsNil)

	// get trusted cluster make sure it's correct
	gotTC, err := presenceBackend.GetTrustedCluster(ctx, "foo")
	c.Assert(err, check.IsNil)
	c.Assert(gotTC.GetName(), check.Equals, "foo")
	c.Assert(gotTC.GetEnabled(), check.Equals, true)
	c.Assert(gotTC.GetRoles(), check.DeepEquals, []string{"bar", "baz"})
	c.Assert(gotTC.GetToken(), check.Equals, "qux")
	c.Assert(gotTC.GetProxyAddress(), check.Equals, "quux")
	c.Assert(gotTC.GetReverseTunnelAddress(), check.Equals, "quuz")

	// get all clusters
	allTC, err := presenceBackend.GetTrustedClusters(ctx)
	c.Assert(err, check.IsNil)
	c.Assert(allTC, check.HasLen, 2)

	// delete cluster
	err = presenceBackend.DeleteTrustedCluster(ctx, "foo")
	c.Assert(err, check.IsNil)

	// make sure it's really gone
	_, err = presenceBackend.GetTrustedCluster(ctx, "foo")
	c.Assert(err, check.NotNil)
	c.Assert(trace.IsNotFound(err), check.Equals, true)
}

func TestDatabaseServersCRUD(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	backend, err := lite.NewWithConfig(ctx, lite.Config{
		Path:  t.TempDir(),
		Clock: clock,
	})
	require.NoError(t, err)

	presence := NewPresenceService(backend)

	// Create a database server.
	server := types.NewDatabaseServerV3("foo", nil,
		types.DatabaseServerSpecV3{
			Protocol: defaults.ProtocolPostgres,
			URI:      "localhost:5432",
			Hostname: "localhost",
			HostID:   uuid.New(),
		})

	// Initially expect not to be returned any servers.
	out, err := presence.GetDatabaseServers(ctx, defaults.Namespace)
	require.NoError(t, err)
	require.Equal(t, 0, len(out))

	// Upsert server.
	lease, err := presence.UpsertDatabaseServer(ctx, server)
	require.NoError(t, err)
	require.Equal(t, &types.KeepAlive{}, lease)

	// Check again, expect a single server to be found.
	out, err = presence.GetDatabaseServers(ctx, server.GetNamespace())
	require.NoError(t, err)
	server.SetResourceID(out[0].GetResourceID())
	require.EqualValues(t, []types.DatabaseServer{server}, out)

	// Make sure can't delete with empty namespace or host ID or name.
	err = presence.DeleteDatabaseServer(ctx, server.GetNamespace(), server.GetHostID(), "")
	require.Error(t, err)
	require.IsType(t, trace.BadParameter(""), err)
	err = presence.DeleteDatabaseServer(ctx, server.GetNamespace(), "", server.GetName())
	require.Error(t, err)
	require.IsType(t, trace.BadParameter(""), err)
	err = presence.DeleteDatabaseServer(ctx, "", server.GetHostID(), server.GetName())
	require.Error(t, err)
	require.IsType(t, trace.BadParameter(""), err)

	// Remove the server.
	err = presence.DeleteDatabaseServer(ctx, server.GetNamespace(), server.GetHostID(), server.GetName())
	require.NoError(t, err)

	// Now expect no servers to be returned.
	out, err = presence.GetDatabaseServers(ctx, defaults.Namespace)
	require.NoError(t, err)
	require.Equal(t, 0, len(out))

	// Upsert server with TTL.
	server.SetExpiry(clock.Now().UTC().Add(time.Hour))
	lease, err = presence.UpsertDatabaseServer(ctx, server)
	require.NoError(t, err)
	require.Equal(t, &types.KeepAlive{
		Type:      types.KeepAlive_DATABASE,
		LeaseID:   lease.LeaseID,
		Name:      server.GetName(),
		Namespace: server.GetNamespace(),
		HostID:    server.GetHostID(),
		Expires:   server.Expiry(),
	}, lease)

	// Make sure can't delete all with empty namespace.
	err = presence.DeleteAllDatabaseServers(ctx, "")
	require.Error(t, err)
	require.IsType(t, trace.BadParameter(""), err)

	// Delete all.
	err = presence.DeleteAllDatabaseServers(ctx, server.GetNamespace())
	require.NoError(t, err)

	// Now expect no servers to be returned.
	out, err = presence.GetDatabaseServers(ctx, defaults.Namespace)
	require.NoError(t, err)
	require.Equal(t, 0, len(out))
}
