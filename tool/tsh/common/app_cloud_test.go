// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package common

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/tlsca"
)

func Test_filterApps(t *testing.T) {
	tests := []struct {
		name            string
		matchRouteToApp func(tlsca.RouteToApp) bool
		apps            []tlsca.RouteToApp
		want            []tlsca.RouteToApp
	}{
		{
			name:            "aws",
			matchRouteToApp: matchAWSApp,
			apps: []tlsca.RouteToApp{
				{Name: "none"},
				{Name: "aws1", AWSRoleARN: "dummy"},
				{Name: "aws2", AWSRoleARN: "dummy"},
				{Name: "aws3", AWSRoleARN: "dummy"},
				{Name: "azure", AzureIdentity: "dummy"},
				{Name: "gcp", GCPServiceAccount: "dummy"},
			},
			want: []tlsca.RouteToApp{
				{Name: "aws1", AWSRoleARN: "dummy"},
				{Name: "aws2", AWSRoleARN: "dummy"},
				{Name: "aws3", AWSRoleARN: "dummy"},
			},
		},
		{
			name:            "azure",
			matchRouteToApp: matchAzureApp,
			apps: []tlsca.RouteToApp{
				{Name: "none"},
				{Name: "aws", AWSRoleARN: "dummy"},
				{Name: "azure1", AzureIdentity: "dummy"},
				{Name: "azure2", AzureIdentity: "dummy"},
				{Name: "azure3", AzureIdentity: "dummy"},
				{Name: "gcp", GCPServiceAccount: "dummy"},
			},
			want: []tlsca.RouteToApp{
				{Name: "azure1", AzureIdentity: "dummy"},
				{Name: "azure2", AzureIdentity: "dummy"},
				{Name: "azure3", AzureIdentity: "dummy"},
			},
		},
		{
			name:            "gcp",
			matchRouteToApp: matchGCPApp,
			apps: []tlsca.RouteToApp{
				{Name: "none"},
				{Name: "aws", AWSRoleARN: "dummy"},
				{Name: "azure", AzureIdentity: "dummy"},
				{Name: "gcp1", GCPServiceAccount: "dummy"},
				{Name: "gcp2", GCPServiceAccount: "dummy"},
				{Name: "gcp3", GCPServiceAccount: "dummy"},
			},
			want: []tlsca.RouteToApp{
				{Name: "gcp1", GCPServiceAccount: "dummy"},
				{Name: "gcp2", GCPServiceAccount: "dummy"},
				{Name: "gcp3", GCPServiceAccount: "dummy"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, filterApps(tt.matchRouteToApp, tt.apps))
		})
	}
}
