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

	crownjewelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/crownjewel/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/tool/common"
	clusterconfigrec "github.com/gravitational/teleport/tool/tctl/common/clusterconfig"
)

func NewAccessGraphSettingsCollection(settings *clusterconfigrec.AccessGraphSettings) ResourceCollection {
	return &accessGraphSettings{
		accessGraphSettings: settings,
	}
}

type accessGraphSettings struct {
	accessGraphSettings *clusterconfigrec.AccessGraphSettings
}

func (c *accessGraphSettings) Resources() []types.Resource {
	return []types.Resource{c.accessGraphSettings}
}

func (c *accessGraphSettings) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"SSH Keys Scan"})
	t.AddRow([]string{
		c.accessGraphSettings.Spec.SecretsScanConfig,
	})
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func NewCrownJewelCollection(jewels []*crownjewelv1.CrownJewel) ResourceCollection {
	return &crownJewelCollection{
		items: jewels,
	}
}

type crownJewelCollection struct {
	items []*crownjewelv1.CrownJewel
}

func (c *crownJewelCollection) Resources() []types.Resource {
	r := make([]types.Resource, 0, len(c.items))
	for _, resource := range c.items {
		r = append(r, types.Resource153ToLegacy(resource))
	}
	return r
}

// writeText formats the crown jewels into a table and writes them into w.
// If verbose is disabled, labels column can be truncated to fit into the console.
func (c *crownJewelCollection) WriteText(w io.Writer, verbose bool) error {
	var rows [][]string
	for _, item := range c.items {
		labels := common.FormatLabels(item.GetMetadata().GetLabels(), verbose)
		rows = append(rows, []string{item.Metadata.GetName(), item.GetSpec().String(), labels})
	}
	headers := []string{"Name", "Spec", "Labels"}
	var t asciitable.Table
	if verbose {
		t = asciitable.MakeTable(headers, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(headers, rows, "Labels")
	}
	// stable sort by name.
	t.SortRowsBy([]int{0}, true)
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}
