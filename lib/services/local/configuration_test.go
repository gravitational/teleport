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

package local

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
)

type configContext struct {
	bk backend.Backend
}

func setupConfigContext(ctx context.Context, t *testing.T) *configContext {
	var tt configContext
	t.Cleanup(func() { tt.Close() })

	clock := clockwork.NewFakeClock()

	var err error
	tt.bk, err = memory.New(memory.Config{
		Context: context.Background(),
		Clock:   clock,
	})
	require.NoError(t, err)

	return &tt
}

func (tt *configContext) Close() error {
	return tt.bk.Close()
}

func TestAuthPreference(t *testing.T) {
	tt := setupConfigContext(context.Background(), t)

	clusterConfig, err := NewClusterConfigurationService(tt.bk)
	require.NoError(t, err)

	suite := &ServicesTestSuite{
		ConfigS: clusterConfig,
	}
	suite.AuthPreference(t)
}

func TestAccessGraphSettings(t *testing.T) {
	tt := setupConfigContext(context.Background(), t)

	clusterConfig, err := NewClusterConfigurationService(tt.bk)
	require.NoError(t, err)

	suite := &ServicesTestSuite{
		ConfigS: clusterConfig,
	}
	suite.AccessGraphSettings(t)
}

func TestClusterName(t *testing.T) {
	tt := setupConfigContext(context.Background(), t)

	clusterConfig, err := NewClusterConfigurationService(tt.bk)
	require.NoError(t, err)

	suite := &ServicesTestSuite{
		ConfigS:      clusterConfig,
		LocalConfigS: clusterConfig,
	}
	suite.ClusterName(t)
}

func TestClusterNetworkingConfig(t *testing.T) {
	tt := setupConfigContext(context.Background(), t)

	clusterConfig, err := NewClusterConfigurationService(tt.bk)
	require.NoError(t, err)

	suite := &ServicesTestSuite{
		ConfigS: clusterConfig,
	}
	suite.ClusterNetworkingConfig(t)
}

func TestClusterAuditConfig(t *testing.T) {
	tt := setupConfigContext(context.Background(), t)

	clusterConfig, err := NewClusterConfigurationService(tt.bk)
	require.NoError(t, err)

	suite := &ServicesTestSuite{
		ConfigS: clusterConfig,
	}
	suite.ClusterNetworkingConfig(t)
}

func TestSessionRecordingConfig(t *testing.T) {
	tt := setupConfigContext(context.Background(), t)

	clusterConfig, err := NewClusterConfigurationService(tt.bk)
	require.NoError(t, err)

	suite := &ServicesTestSuite{
		ConfigS: clusterConfig,
	}
	suite.SessionRecordingConfig(t)
}

func TestStaticTokens(t *testing.T) {
	tt := setupConfigContext(context.Background(), t)

	clusterConfig, err := NewClusterConfigurationService(tt.bk)
	require.NoError(t, err)

	suite := &ServicesTestSuite{
		ConfigS: clusterConfig,
	}
	suite.StaticTokens(t)
}

func TestSessionRecording(t *testing.T) {
	// don't allow invalid session recording values
	_, err := types.NewSessionRecordingConfigFromConfigFile(types.SessionRecordingConfigSpecV2{
		Mode: "foo",
	})
	require.Error(t, err)

	// default is to record at the node
	recConfig, err := types.NewSessionRecordingConfigFromConfigFile(types.SessionRecordingConfigSpecV2{})
	require.NoError(t, err)
	require.Equal(t, types.RecordAtNode, recConfig.GetMode())

	// update sessions to be recorded at the proxy and check again
	recConfig.SetMode(types.RecordAtProxy)
	require.Equal(t, types.RecordAtProxy, recConfig.GetMode())
}

func TestAuditConfig(t *testing.T) {
	testCases := []struct {
		spec   types.ClusterAuditConfigSpecV2
		config string
	}{
		{
			spec: types.ClusterAuditConfigSpecV2{
				Region:           "us-west-1",
				Type:             "dynamodb",
				AuditSessionsURI: "file:///home/log",
				AuditEventsURI:   []string{"dynamodb://audit_table_name", "file:///home/log"},
			},
			config: `
region: 'us-west-1'
type: 'dynamodb'
audit_sessions_uri: file:///home/log
audit_events_uri: ['dynamodb://audit_table_name', 'file:///home/log']
`,
		},
		{
			spec: types.ClusterAuditConfigSpecV2{
				Region:           "us-west-1",
				Type:             "dir",
				AuditSessionsURI: "file:///home/log",
				AuditEventsURI:   []string{"dynamodb://audit_table_name"},
			},
			config: `
region: 'us-west-1'
type: 'dir'
audit_sessions_uri: file:///home/log
audit_events_uri: 'dynamodb://audit_table_name'
`,
		},
	}

	for _, tc := range testCases {
		in, err := types.NewClusterAuditConfig(tc.spec)
		require.NoError(t, err)

		var data map[string]interface{}
		err = yaml.Unmarshal([]byte(tc.config), &data)
		require.NoError(t, err)

		configSpec, err := services.ClusterAuditConfigSpecFromObject(data)
		require.NoError(t, err)

		out, err := types.NewClusterAuditConfig(*configSpec)
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(out, in))
	}
}

func TestAuditConfigMarshal(t *testing.T) {
	// single audit_events uri value
	auditConfig, err := types.NewClusterAuditConfig(types.ClusterAuditConfigSpecV2{
		Region:           "us-west-1",
		Type:             "dynamodb",
		AuditSessionsURI: "file:///home/log",
		AuditEventsURI:   []string{"dynamodb://audit_table_name"},
	})
	require.NoError(t, err)

	data, err := services.MarshalClusterAuditConfig(auditConfig)
	require.NoError(t, err)

	out, err := services.UnmarshalClusterAuditConfig(data)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(auditConfig, out))

	// multiple events uri values
	auditConfig, err = types.NewClusterAuditConfig(types.ClusterAuditConfigSpecV2{
		Region:           "us-west-1",
		Type:             "dynamodb",
		AuditSessionsURI: "file:///home/log",
		AuditEventsURI:   []string{"dynamodb://audit_table_name", "file:///home/test/log"},
	})
	require.NoError(t, err)

	data, err = services.MarshalClusterAuditConfig(auditConfig)
	require.NoError(t, err)

	out, err = services.UnmarshalClusterAuditConfig(data)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(auditConfig, out))
}

func TestInstallerPagination(t *testing.T) {
	ctx := t.Context()

	tt := setupConfigContext(context.Background(), t)
	s, err := NewClusterConfigurationService(tt.bk)
	require.NoError(t, err)

	newResource := func(name string) types.Installer {
		contents := "#!/bin/bash some script stuff"
		inst, err := types.NewInstallerV1(name, contents)
		require.NoError(t, err)
		return inst
	}

	pageSize := 2
	totalItems := pageSize*2 + (pageSize / 2)
	var want []types.Installer

	for i := range totalItems {
		inst := newResource(fmt.Sprintf("installer-%d", i))
		require.NoError(t, s.SetInstaller(ctx, inst))
		want = append(want, inst)
	}

	t.Cleanup(func() {
		for _, conn := range want {
			s.DeleteInstaller(ctx, conn.GetName())
		}
	})

	t.Run("GetInstallers", func(t *testing.T) {
		// Verify legacy getters still work as expected.
		conns, err := s.GetInstallers(ctx)
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(want, conns, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
	})

	t.Run("ListInstallers", func(t *testing.T) {
		// no limits
		conns, next, err := s.ListInstallers(ctx, 0, "")
		require.NoError(t, err)
		require.Empty(t, next)
		require.Empty(t, cmp.Diff(want, conns, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

		// page limit
		page1, page2Start, err := s.ListInstallers(ctx, pageSize, "")
		require.NoError(t, err)
		require.NotEmpty(t, page2Start)
		require.Empty(t, cmp.Diff(want[:pageSize], page1, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

		// rest with start
		page2, next, err := s.ListInstallers(ctx, 0, page2Start)
		require.NoError(t, err)
		require.Empty(t, next)
		require.Empty(t, cmp.Diff(want[pageSize:], page2, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

		require.Empty(t, cmp.Diff(want, append(page1, page2...), cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
	})

	t.Run("RangeInstallers", func(t *testing.T) {
		// full range
		conns, err := stream.Collect(s.RangeInstallers(ctx, "", ""))
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(want, conns, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

		// with start
		conns, err = stream.Collect(s.RangeInstallers(ctx, want[pageSize].GetName(), ""))
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(want[pageSize:], conns, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

		// with end
		conns, err = stream.Collect(s.RangeInstallers(ctx, "", want[pageSize*2].GetName()))
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(want[:pageSize*2], conns, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

		// with start and end
		conns, err = stream.Collect(s.RangeInstallers(ctx, want[pageSize].GetName(), want[pageSize*2].GetName()))
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(want[pageSize:pageSize*2], conns, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
	})

}
