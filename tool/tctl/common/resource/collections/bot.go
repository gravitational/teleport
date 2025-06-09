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
	"strings"

	"github.com/gravitational/trace"

	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
)

func NewBotCollection(bots []*machineidv1pb.Bot) ResourceCollection {
	return &botCollection{bots: bots}
}

type botCollection struct {
	bots []*machineidv1pb.Bot
}

func (c *botCollection) Resources() []types.Resource {
	resources := make([]types.Resource, len(c.bots))
	for i, b := range c.bots {
		resources[i] = types.Resource153ToLegacy(b)
	}
	return resources
}

func (c *botCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Roles"})
	for _, b := range c.bots {
		t.AddRow([]string{
			b.Metadata.Name,
			strings.Join(b.Spec.Roles, ", "),
		})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func NewBotInstanceCollection(bots []*machineidv1pb.BotInstance) ResourceCollection {
	return &botInstanceCollection{items: bots}
}

type botInstanceCollection struct {
	items []*machineidv1pb.BotInstance
}

func (c *botInstanceCollection) Resources() []types.Resource {
	r := make([]types.Resource, 0, len(c.items))
	for _, resource := range c.items {
		r = append(r, types.ProtoResource153ToLegacy(resource))
	}
	return r
}

func (c *botInstanceCollection) WriteText(w io.Writer, verbose bool) error {
	headers := []string{"Bot Name", "Instance ID"}

	// TODO: consider adding additional (possibly verbose) fields showing
	// last heartbeat, last auth, etc.
	var rows [][]string
	for _, item := range c.items {
		rows = append(rows, []string{item.Spec.BotName, item.Spec.InstanceId})
	}

	t := asciitable.MakeTable(headers, rows...)

	// stable sort by name.
	t.SortRowsBy([]int{0}, true)
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}
