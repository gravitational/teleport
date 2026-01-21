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

package grpcclientconfig

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"

	grpcv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/grpcclientconfig/v1"
)

func TestServiceConfigJSON(t *testing.T) {
	value := `{"loadBalancingConfig":[{"teleport_pick_healthy":{"mode":"MODE_RECONNECT"}}], "healthCheckConfig":{"serviceName":""}}`
	empty := ""
	want := &grpcv1.ServiceConfig{
		LoadBalancingConfig: []*grpcv1.LoadBalancerConfig{{
			Config: &grpcv1.LoadBalancerConfig_TeleportPickHealthy{TeleportPickHealthy: &grpcv1.TeleportPickHealthyConfig{
				Mode: grpcv1.Mode_MODE_RECONNECT,
			}},
		}},
		HealthCheckConfig: &grpcv1.HealthCheckConfig{ServiceName: &empty},
	}
	got := &grpcv1.ServiceConfig{}
	err := protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}.Unmarshal([]byte(value), got)
	require.NoError(t, err)
	require.EqualExportedValues(t, want, got)
}
