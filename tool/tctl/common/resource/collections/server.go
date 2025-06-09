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
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/tool/common"
)

type serverCollection struct {
	servers []types.Server
}

func NewServerCollection(servers []types.Server) ResourceCollection {
	return &serverCollection{servers: servers}
}

func (s *serverCollection) Resources() (r []types.Resource) {
	for _, resource := range s.servers {
		r = append(r, resource)
	}
	return r
}

func (s *serverCollection) WriteText(w io.Writer, verbose bool) error {
	var rows [][]string
	for _, se := range s.servers {
		labels := common.FormatLabels(se.GetAllLabels(), verbose)
		rows = append(rows, []string{
			se.GetHostname(), se.GetName(), se.GetAddr(), labels, se.GetTeleportVersion(),
		})
	}
	headers := []string{"Host", "UUID", "Public Address", "Labels", "Version"}
	var t asciitable.Table
	if verbose {
		t = asciitable.MakeTable(headers, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(headers, rows, "Labels")
	}

	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

type serverInfoCollection struct {
	serverInfos []types.ServerInfo
}

func NewServerInfoCollection(serverInfos []types.ServerInfo) ResourceCollection {
	return &serverInfoCollection{serverInfos: serverInfos}
}

func (c *serverInfoCollection) Resources() []types.Resource {
	r := make([]types.Resource, len(c.serverInfos))
	for i, resource := range c.serverInfos {
		r[i] = resource
	}
	return r
}

func (c *serverInfoCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Labels"})
	for _, si := range c.serverInfos {
		t.AddRow([]string{si.GetName(), PrintMetadataLabels(si.GetNewLabels())})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}
