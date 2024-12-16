// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package config

import "testing"

func TestWorkloadIdentityOutput(t *testing.T) {
	t.Parallel()

	dest := &DestinationMemory{}
	tests := []testYAMLCase[WorkloadIdentityOutput]{
		{
			name: "full",
			in: WorkloadIdentityOutput{
				Destination: dest,
				WorkloadIdentity: WorkloadIdentitySelector{
					Name: "my-workload-identity",
				},
				IncludeFederatedTrustBundles: true,
				JWTs: []JWTSVID{
					{
						Audience: "example.com",
						FileName: "foo",
					},
					{
						Audience: "2.example.com",
						FileName: "bar",
					},
				},
			},
		},
		{
			name: "minimal",
			in: WorkloadIdentityOutput{
				Destination: dest,
				WorkloadIdentity: WorkloadIdentitySelector{
					Name: "my-workload-identity",
				},
			},
		},
	}
	testYAML(t, tests)
}
