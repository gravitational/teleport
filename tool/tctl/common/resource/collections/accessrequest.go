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
	"fmt"
	"io"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
)

func NewAccessRequestCollection(requests []types.AccessRequest) ResourceCollection {
	return &accessRequestCollection{accessRequests: requests}
}

type accessRequestCollection struct {
	accessRequests []types.AccessRequest
}

func (c *accessRequestCollection) Resources() []types.Resource {
	r := make([]types.Resource, len(c.accessRequests))
	for i, resource := range c.accessRequests {
		r[i] = resource
	}
	return r
}

func (c *accessRequestCollection) WriteText(w io.Writer, verbose bool) error {
	var t asciitable.Table
	var rows [][]string
	for _, al := range c.accessRequests {
		var annotations []string
		for k, v := range al.GetSystemAnnotations() {
			annotations = append(annotations, fmt.Sprintf("%s/%s", k, strings.Join(v, ",")))
		}
		rows = append(rows, []string{
			al.GetName(),
			al.GetUser(),
			strings.Join(al.GetRoles(), ", "),
			strings.Join(annotations, ", "),
		})
	}
	if verbose {
		t = asciitable.MakeTable([]string{"Name", "User", "Roles", "Annotations"}, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn([]string{"Name", "User", "Roles", "Annotations"}, rows, "Annotations")
	}

	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}
