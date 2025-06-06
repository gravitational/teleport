package collections

import (
	"io"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	"github.com/gravitational/teleport/lib/asciitable"
)

type discoveryConfigCollection struct {
	discoveryConfigs []*discoveryconfig.DiscoveryConfig
}

func NewDiscoveryConfigCollection(discoveryConfigs []*discoveryconfig.DiscoveryConfig) ResourceCollection {
	return &discoveryConfigCollection{discoveryConfigs: discoveryConfigs}
}

func (c *discoveryConfigCollection) Resources() []types.Resource {
	resources := make([]types.Resource, len(c.discoveryConfigs))
	for i, dc := range c.discoveryConfigs {
		resources[i] = dc
	}
	return resources
}

func (c *discoveryConfigCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Discovery Group"})
	for _, dc := range c.discoveryConfigs {
		t.AddRow([]string{
			dc.GetName(),
			dc.GetDiscoveryGroup(),
		})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}
