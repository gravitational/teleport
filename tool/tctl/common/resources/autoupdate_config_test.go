/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package resources

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"

	autoupdatev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/autoupdate"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

// autoUpdateConfigBrokenCollection is an intentionally broken version of the
// autoUpdateConfigCollection that is not marshaling resources properly because
// it's doing json marshaling instead of protojson marshaling.
type autoUpdateConfigBrokenCollection struct {
	autoUpdateConfigCollection
}

func (c *autoUpdateConfigBrokenCollection) Resources() []types.Resource {
	// We use Resource153ToLegacy instead of ProtoResource153ToLegacy.
	return []types.Resource{types.Resource153ToLegacy(c.config)}
}

// This test makes sure we marshal and unmarshal proto-based Resource153 properly.
// We had a bug where types.Resource153 implemented by protobuf structs were not
// marshaled properly (they should be marshaled using protojson). This test
// checks we can do a round-trip with one of those proto-struct resource.
func TestRoundTripProtoResource153(t *testing.T) {
	// Test setup: generate fixture.
	initial, err := autoupdate.NewAutoUpdateConfig(&autoupdatev1pb.AutoUpdateConfigSpec{
		Agents: &autoupdatev1pb.AutoUpdateConfigSpecAgents{
			Mode:                      autoupdate.AgentsUpdateModeEnabled,
			Strategy:                  autoupdate.AgentsStrategyTimeBased,
			MaintenanceWindowDuration: durationpb.New(1 * time.Hour),
			Schedules: &autoupdatev1pb.AgentAutoUpdateSchedules{
				Regular: []*autoupdatev1pb.AgentAutoUpdateGroup{
					{
						Name: "group1",
						Days: []string{types.Wildcard},
					},
				},
			},
		},
	})
	require.NoError(t, err)

	// Test execution: dump the resource into a YAML manifest.
	collection := &autoUpdateConfigCollection{config: initial}
	buf := &bytes.Buffer{}
	require.NoError(t, utils.WriteYAML(buf, collection.Resources()))

	// Test execution: load the YAML manifest back.
	decoder := kyaml.NewYAMLOrJSONDecoder(buf, defaults.LookaheadBufSize)
	var raw services.UnknownResource
	require.NoError(t, decoder.Decode(&raw))
	result, err := services.UnmarshalProtoResource[*autoupdatev1pb.AutoUpdateConfig](raw.Raw)
	require.NoError(t, err)

	// Test validation: check that the loaded content matches what we had before.
	require.Equal(t, result, initial)

	// Test execution: now dump the resource into a YAML manifest with a
	// collection using types.Resource153ToLegacy instead of types.ProtoResource153ToLegacy
	brokenCollection := &autoUpdateConfigBrokenCollection{autoUpdateConfigCollection{initial}}
	buf = &bytes.Buffer{}
	require.NoError(t, utils.WriteYAML(buf, brokenCollection.Resources()))

	// Test execution: load the YAML manifest back and see that we can't unmarshal it.
	decoder = kyaml.NewYAMLOrJSONDecoder(buf, defaults.LookaheadBufSize)
	require.NoError(t, decoder.Decode(&raw))
	_, err = services.UnmarshalProtoResource[*autoupdatev1pb.AutoUpdateConfig](raw.Raw)
	require.Error(t, err)
}

func TestNormalizeAutoUpdateConfigDurations(t *testing.T) {
	// base is a minimal valid autoupdate_config JSON document. Tests that need
	// a specific maintenance_window_duration value embed it here.
	const base = `{"kind":"autoupdate_config","version":"v1","metadata":{"name":"autoupdate-config"},"spec":{"agents":{"mode":"enabled","strategy":"time-based","maintenance_window_duration":%q}}}`

	tests := []struct {
		name     string
		input    string
		wantDur  string // expected value of maintenance_window_duration in output
		passthru bool   // true when input should be returned unchanged
	}{
		{
			name:    "hour string is converted",
			input:   fmt.Sprintf(base, "1h"),
			wantDur: "3600s",
		},
		{
			name:    "minute string is converted",
			input:   fmt.Sprintf(base, "30m"),
			wantDur: "1800s",
		},
		{
			name:    "compound duration is converted",
			input:   fmt.Sprintf(base, "1h30m"),
			wantDur: "5400s",
		},
		{
			name:    "proto seconds format is preserved",
			input:   fmt.Sprintf(base, "3600s"),
			wantDur: "3600s",
		},
		{
			name:     "invalid duration is passed through unchanged",
			input:    fmt.Sprintf(base, "notaduration"),
			wantDur:  "notaduration",
			passthru: true,
		},
		{
			name:     "no agents field is a no-op",
			input:    `{"kind":"autoupdate_config","version":"v1","spec":{"tools":{"mode":"enabled"}}}`,
			passthru: true,
		},
		{
			name:     "no duration field is a no-op",
			input:    `{"kind":"autoupdate_config","version":"v1","spec":{"agents":{"mode":"enabled"}}}`,
			passthru: true,
		},
		{
			name:     "malformed JSON is passed through unchanged",
			input:    `not valid json`,
			passthru: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := normalizeAutoUpdateConfigDurations([]byte(tt.input))
			if tt.passthru {
				require.Equal(t, tt.input, string(out))
				return
			}
			require.Contains(t, string(out), fmt.Sprintf(`"maintenance_window_duration":%q`, tt.wantDur))
		})
	}
}
