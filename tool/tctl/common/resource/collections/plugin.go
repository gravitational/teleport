package collections

import (
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/trace"
	"io"
)

type pluginCollection struct {
	plugins []types.Plugin
}

func NewPluginCollection(plugins []types.Plugin) ResourceCollection {
	return &pluginCollection{plugins: plugins}
}

func (c *pluginCollection) Resources() []types.Resource {
	r := make([]types.Resource, len(c.plugins))
	for i, resource := range c.plugins {
		r[i] = resource
	}
	return r
}

func (c *pluginCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Status"})
	for _, plugin := range c.plugins {
		t.AddRow([]string{
			plugin.GetName(),
			plugin.GetStatus().GetCode().String(),
		})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}
