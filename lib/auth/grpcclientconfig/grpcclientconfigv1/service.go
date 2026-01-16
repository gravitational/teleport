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

package grpcclientconfigv1

import (
	"context"
	"os"

	grpcv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/grpcclientconfig/v1"
	"github.com/gravitational/teleport/api/types/grpcclientconfig"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/encoding/protojson"
)

func init() {
	// TELEPORT_UNSTABLE_GRPC_SERVICE_CONFIG must be the json representation
	// of a [grpcv1.ServiceConfig].
	serviceConfig, _ = os.LookupEnv("TELEPORT_UNSTABLE_GRPC_SERVICE_CONFIG")
}

var serviceConfig string

// NewService initializes a new grpcclientconfig [Service].
func NewService() (*Service, error) {
	config := &grpcv1.ServiceConfig{}
	if serviceConfig != "" {
		err := protojson.Unmarshal([]byte(serviceConfig), config)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		config = grpcclientconfig.DefaultServiceConfig()
	}

	return &Service{
		config: config,
	}, nil
}

// Service is an impementation of the [grpcv1.ServiceConfigDiscoveryServiceServer].
// It allows grpc clients to discover their client configuration at runtime.
type Service struct {
	grpcv1.UnimplementedServiceConfigDiscoveryServiceServer
	config *grpcv1.ServiceConfig
}

// GetServiceConfig handles requests for fetching the service config configured
// on this server.
func (s *Service) GetServiceConfig(ctx context.Context, req *grpcv1.GetServiceConfigRequest) (*grpcv1.GetServiceConfigResponse, error) {
	return &grpcv1.GetServiceConfigResponse{
		Config: s.config,
	}, nil
}
