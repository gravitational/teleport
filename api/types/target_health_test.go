/*
Copyright 2025 Gravitational, Inc.

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

package types

import (
	"fmt"
	"testing"

	"math/rand/v2"

	"github.com/stretchr/testify/require"
)

func TestGroupByTargetHealth(t *testing.T) {
	wantGroups := [][]TargetHealthStatus{
		{TargetHealthStatusHealthy},
		{TargetHealthStatusUnknown},
		{TargetHealthStatusUnhealthy},
	}

	var servers []DatabaseServer
	for _, group := range wantGroups {
		for _, status := range group {
			name := fmt.Sprintf("db-%d", len(servers))
			server, err := NewDatabaseServerV3(Metadata{
				Name: name,
			}, DatabaseServerSpecV3{
				HostID:   "_",
				Hostname: "_",
				Database: &DatabaseV3{
					Metadata: Metadata{
						Name: name,
					},
					Spec: DatabaseSpecV3{
						Protocol: "_",
						URI:      "_",
						AWS: AWS{
							Redshift: Redshift{
								ClusterID: "_",
							},
						},
					},
				},
			})
			require.NoError(t, err)
			server.SetTargetHealth(TargetHealth{Status: string(status)})
			servers = append(servers, server)
		}
	}
	rand.Shuffle(len(servers), func(i, j int) {
		servers[i], servers[j] = servers[j], servers[i]
	})
	groups := GroupByTargetHealth(servers)
	require.Len(t, groups, len(wantGroups))
	for i, wantGroup := range wantGroups {
		for _, server := range groups[i] {
			require.Contains(t,
				wantGroup,
				server.GetTargetHealth().Status,
				"server %s is in the wrong group", server.GetName(),
			)
		}
	}
}
