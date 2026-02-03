/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package common

import (
	"io"

	"github.com/gravitational/trace"

	workloadclusterv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadcluster/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
)

type workloadClusterCollection struct {
	workloadClusters []*workloadclusterv1pb.WorkloadCluster
}

func (c *workloadClusterCollection) resources() []types.Resource {
	resources := make([]types.Resource, 0, len(c.workloadClusters))

	for _, cc := range c.workloadClusters {
		resources = append(resources, types.ProtoResource153ToLegacy(cc))
	}

	return resources
}

func (c *workloadClusterCollection) writeText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name"})
	for _, cc := range c.workloadClusters {
		t.AddRow([]string{
			cc.GetMetadata().GetName(),
		})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}
