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

package common

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/tlsca"
)

func Test_filterApps(t *testing.T) {
	t.Parallel()

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
