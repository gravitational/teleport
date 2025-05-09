// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package cache

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	clusterconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

func TestClusterName(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	p := newPackForAuth(t)
	t.Cleanup(p.Close)

	clusterName, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: "example.com",
	})
	require.NoError(t, err)
	err = p.clusterConfigS.SetClusterName(clusterName)
	require.NoError(t, err)

	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
		require.Equal(t, types.KindClusterName, event.Event.Resource.GetKind())
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	outName, err := p.cache.GetClusterName(ctx)
	require.NoError(t, err)

	require.Empty(t, cmp.Diff(outName, clusterName, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
}

func TestClusterAuditConfig(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	p := newPackForAuth(t)
	t.Cleanup(p.Close)

	auditConfig, err := types.NewClusterAuditConfig(types.ClusterAuditConfigSpecV2{
		AuditEventsURI: []string{"dynamodb://audit_table_name", "file:///home/log"},
	})
	require.NoError(t, err)
	err = p.clusterConfigS.SetClusterAuditConfig(ctx, auditConfig)
	require.NoError(t, err)

	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
		require.Equal(t, types.KindClusterAuditConfig, event.Event.Resource.GetKind())
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	outAuditConfig, err := p.cache.GetClusterAuditConfig(ctx)
	require.NoError(t, err)

	require.Empty(t, cmp.Diff(outAuditConfig, auditConfig, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
}

func TestClusterNetworkingConfig(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	p := newPackForAuth(t)
	t.Cleanup(p.Close)

	netConfig, err := types.NewClusterNetworkingConfigFromConfigFile(types.ClusterNetworkingConfigSpecV2{
		ClientIdleTimeout:        types.Duration(time.Minute),
		ClientIdleTimeoutMessage: "test idle timeout message",
	})
	require.NoError(t, err)
	_, err = p.clusterConfigS.UpsertClusterNetworkingConfig(ctx, netConfig)
	require.NoError(t, err)

	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
		require.Equal(t, types.KindClusterNetworkingConfig, event.Event.Resource.GetKind())
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	outNetConfig, err := p.cache.GetClusterNetworkingConfig(ctx)
	require.NoError(t, err)

	require.Empty(t, cmp.Diff(outNetConfig, netConfig, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
}

func TestSessionRecordingConfig(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	p := newPackForAuth(t)
	t.Cleanup(p.Close)

	recConfig, err := types.NewSessionRecordingConfigFromConfigFile(types.SessionRecordingConfigSpecV2{
		Mode:                types.RecordAtProxySync,
		ProxyChecksHostKeys: types.NewBoolOption(true),
	})
	require.NoError(t, err)
	_, err = p.clusterConfigS.UpsertSessionRecordingConfig(ctx, recConfig)
	require.NoError(t, err)

	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
		require.Equal(t, types.KindSessionRecordingConfig, event.Event.Resource.GetKind())
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	outRecConfig, err := p.cache.GetSessionRecordingConfig(ctx)
	require.NoError(t, err)

	require.Empty(t, cmp.Diff(outRecConfig, recConfig, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
}

func TestAuthPreference(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	p := newPackForAuth(t)
	t.Cleanup(p.Close)

	authPref, err := types.NewAuthPreferenceFromConfigFile(types.AuthPreferenceSpecV2{
		AllowLocalAuth:  types.NewBoolOption(true),
		MessageOfTheDay: "test MOTD",
	})
	require.NoError(t, err)
	authPref, err = p.clusterConfigS.UpsertAuthPreference(ctx, authPref)
	require.NoError(t, err)

	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
		require.Equal(t, types.KindClusterAuthPreference, event.Event.Resource.GetKind())
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	outAuthPref, err := p.cache.GetAuthPreference(ctx)
	require.NoError(t, err)

	require.Empty(t, cmp.Diff(outAuthPref, authPref, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
}

func TestAccessGraphSettings(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	testResources153(t, p, testFuncs153[*clusterconfigv1.AccessGraphSettings]{
		newResource: func(name string) (*clusterconfigv1.AccessGraphSettings, error) {
			return newAccessGraphSettings(t), nil
		},
		create: func(ctx context.Context, item *clusterconfigv1.AccessGraphSettings) error {
			_, err := p.clusterConfigS.UpsertAccessGraphSettings(ctx, item)
			return trace.Wrap(err)
		},
		list: func(ctx context.Context) ([]*clusterconfigv1.AccessGraphSettings, error) {
			item, err := p.clusterConfigS.GetAccessGraphSettings(ctx)
			if trace.IsNotFound(err) {
				return []*clusterconfigv1.AccessGraphSettings{}, nil
			}
			return []*clusterconfigv1.AccessGraphSettings{item}, trace.Wrap(err)
		},
		cacheList: func(ctx context.Context) ([]*clusterconfigv1.AccessGraphSettings, error) {
			item, err := p.cache.GetAccessGraphSettings(ctx)
			if trace.IsNotFound(err) {
				return []*clusterconfigv1.AccessGraphSettings{}, nil
			}
			return []*clusterconfigv1.AccessGraphSettings{item}, trace.Wrap(err)
		},
		deleteAll: func(ctx context.Context) error {
			return trace.Wrap(p.clusterConfigS.DeleteAccessGraphSettings(ctx))
		},
	})
}
