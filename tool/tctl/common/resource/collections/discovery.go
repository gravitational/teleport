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
