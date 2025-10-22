package resources

import (
	"bytes"
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
