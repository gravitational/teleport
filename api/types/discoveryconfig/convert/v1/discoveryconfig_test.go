/*
Copyright 2023 Gravitational, Inc.

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

package v1

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	discoveryconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryconfig/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	"github.com/gravitational/teleport/api/types/header"
)

func TestRoundtrip(t *testing.T) {
	discoveryConfig := newDiscoveryConfig(t, "discovery-config-01")

	converted, err := FromProto(ToProto(discoveryConfig))
	require.NoError(t, err)

	require.Empty(t, cmp.Diff(discoveryConfig, converted))
}

// Make sure that we don't panic if any of the message fields are missing.
func TestFromProtoNils(t *testing.T) {
	// Spec is nil
	discoveryConfig := ToProto(newDiscoveryConfig(t, "discovery-config-01"))
	discoveryConfig.Spec = nil

	_, err := FromProto(discoveryConfig)
	require.Error(t, err)
}

func newDiscoveryConfig(t *testing.T, name string) *discoveryconfig.DiscoveryConfig {
	t.Helper()

	discoveryConfig, err := discoveryconfig.NewDiscoveryConfig(
		header.Metadata{
			Name: name,
		},
		discoveryconfig.Spec{
			DiscoveryGroup: "discovery-group-01",
			AWS: []types.AWSMatcher{
				{
					Types:   []string{"rds"},
					Regions: []string{"us-west-2"},
				},
				{
					Types:   []string{"ec2"},
					Regions: []string{"eu-west-2"},
				},
			},
		},
	)
	require.NoError(t, err)
	discoveryConfig.Status.State = discoveryconfigv1.DiscoveryConfigState_name[int32(discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_RUNNING)]
	discoveryConfig.Status.DiscoveredResources = 42
	now := time.Now()
	discoveryConfig.Status.LastSyncTime = now
	errMsg := "error message"
	discoveryConfig.Status.ErrorMessage = &errMsg
	return discoveryConfig
}
