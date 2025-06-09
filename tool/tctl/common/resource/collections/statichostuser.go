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
	"strconv"

	"github.com/gravitational/trace"

	userprovisioningpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userprovisioning/v2"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/label"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/tool/common"
)

type staticHostUserCollection struct {
	items []*userprovisioningpb.StaticHostUser
}

func NewStaticHostUserCollection(items []*userprovisioningpb.StaticHostUser) ResourceCollection {
	return &staticHostUserCollection{items: items}
}

func (c *staticHostUserCollection) Resources() []types.Resource {
	r := make([]types.Resource, 0, len(c.items))
	for _, resource := range c.items {
		r = append(r, types.Resource153ToLegacy(resource))
	}
	return r
}

func (c *staticHostUserCollection) WriteText(w io.Writer, verbose bool) error {
	var rows [][]string
	for _, item := range c.items {

		for _, matcher := range item.Spec.Matchers {
			labelMap := label.ToMap(matcher.NodeLabels)
			labelStringMap := make(map[string]string, len(labelMap))
			for k, vals := range labelMap {
				labelStringMap[k] = fmt.Sprintf("[%s]", printSortedStringSlice(vals))
			}
			var uid string
			if matcher.Uid != 0 {
				uid = strconv.Itoa(int(matcher.Uid))
			}
			var gid string
			if matcher.Gid != 0 {
				gid = strconv.Itoa(int(matcher.Gid))
			}
			rows = append(rows, []string{
				item.GetMetadata().Name,
				common.FormatLabels(labelStringMap, verbose),
				matcher.NodeLabelsExpression,
				printSortedStringSlice(matcher.Groups),
				uid,
				gid,
			})
		}
	}
	headers := []string{"Login", "Node Labels", "Node Expression", "Groups", "Uid", "Gid"}
	var t asciitable.Table
	if verbose {
		t = asciitable.MakeTable(headers, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(headers, rows, "Node Expression")
	}
	t.SortRowsBy([]int{0}, true)
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}
