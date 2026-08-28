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

package gcp

import (
	"context"
	"testing"

	"cloud.google.com/go/alloydb/apiv1beta/alloydbpb"
	"github.com/googleapis/gax-go/v2"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	gcputils "github.com/gravitational/teleport/api/utils/gcp"
)

type fakeAlloyDBAdminAPIClient struct {
	alloyDBAdminAPIClient
	getConnectionInfoResponse *alloydbpb.ConnectionInfo
	getConnectionInfoRequest  *alloydbpb.GetConnectionInfoRequest
}

func (f *fakeAlloyDBAdminAPIClient) GetConnectionInfo(ctx context.Context, request *alloydbpb.GetConnectionInfoRequest, option ...gax.CallOption) (*alloydbpb.ConnectionInfo, error) {
	if request.String() != f.getConnectionInfoRequest.String() {
		return nil, trace.BadParameter("mismatched GetConnectionInfoRequest %v and %v", request, f.getConnectionInfoRequest)
	}
	return f.getConnectionInfoResponse, nil
}

func TestAlloyDBGetEndpointAddress(t *testing.T) {
	instance := gcputils.AlloyDBFullInstanceName{
		ProjectID:  "my-project-123456",
		Location:   "europe-west1",
		ClusterID:  "my-cluster",
		InstanceID: "my-instance",
	}

	client := &gcpAlloyDBAdminClient{
		apiClient: &fakeAlloyDBAdminAPIClient{
			getConnectionInfoRequest: &alloydbpb.GetConnectionInfoRequest{
				Parent: "projects/my-project-123456/locations/europe-west1/clusters/my-cluster/instances/my-instance",
			},
			getConnectionInfoResponse: &alloydbpb.ConnectionInfo{
				IpAddress:       "11.22.33.44",
				PublicIpAddress: "22.33.44.44",
				PscDnsName:      "dsc.internal.example.com",
			},
		},
	}

	addrInfo := map[gcputils.AlloyDBEndpointType]string{
		gcputils.AlloyDBEndpointTypePrivate: "11.22.33.44",
		gcputils.AlloyDBEndpointTypePublic:  "22.33.44.44",
		gcputils.AlloyDBEndpointTypePSC:     "dsc.internal.example.com",
	}

	for endpointType, wantAddr := range addrInfo {
		t.Run(string(endpointType), func(t *testing.T) {
			resp, err := client.GetEndpointAddress(context.Background(), instance, string(endpointType))
			require.NoError(t, err)
			require.Equal(t, wantAddr, resp)
		})
	}
}
