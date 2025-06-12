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
	"time"

	"github.com/gravitational/trace"

	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
)

type spiffeFederationCollection struct {
	items []*machineidv1pb.SPIFFEFederation
}

func NewSpiffeFederationCollection(items []*machineidv1pb.SPIFFEFederation) ResourceCollection {
	return &spiffeFederationCollection{items: items}
}

func (c *spiffeFederationCollection) Resources() []types.Resource {
	r := make([]types.Resource, 0, len(c.items))
	for _, resource := range c.items {
		r = append(r, types.Resource153ToLegacy(resource))
	}
	return r
}

func (c *spiffeFederationCollection) WriteText(w io.Writer, verbose bool) error {
	headers := []string{"Name", "Last synced at"}

	var rows [][]string
	for _, item := range c.items {
		lastSynced := "never"
		if t := item.GetStatus().GetCurrentBundleSyncedAt().AsTime(); !t.IsZero() {
			lastSynced = t.Format(time.RFC3339)
		}
		rows = append(rows, []string{
			item.Metadata.Name,
			lastSynced,
		})
	}

	t := asciitable.MakeTable(headers, rows...)

	// stable sort by name.
	t.SortRowsBy([]int{0}, true)
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

type workloadIdentityCollection struct {
	items []*workloadidentityv1pb.WorkloadIdentity
}

func NewWorkloadIdentityCollection(items []*workloadidentityv1pb.WorkloadIdentity) ResourceCollection {
	return &workloadIdentityCollection{items: items}
}

func (c *workloadIdentityCollection) Resources() []types.Resource {
	r := make([]types.Resource, 0, len(c.items))
	for _, resource := range c.items {
		r = append(r, types.ProtoResource153ToLegacy(resource))
	}
	return r
}

func (c *workloadIdentityCollection) WriteText(w io.Writer, verbose bool) error {
	headers := []string{"Name", "SPIFFE ID"}

	var rows [][]string
	for _, item := range c.items {
		rows = append(rows, []string{
			item.Metadata.Name,
			item.GetSpec().GetSpiffe().GetId(),
		})
	}

	t := asciitable.MakeTable(headers, rows...)

	// stable sort by name.
	t.SortRowsBy([]int{0}, true)
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

type workloadIdentityX509RevocationCollection struct {
	items []*workloadidentityv1pb.WorkloadIdentityX509Revocation
}

func NewWorkloadIdentityX509RevocationCollection(items []*workloadidentityv1pb.WorkloadIdentityX509Revocation) ResourceCollection {
	return &workloadIdentityX509RevocationCollection{items: items}
}

func (c *workloadIdentityX509RevocationCollection) Resources() []types.Resource {
	r := make([]types.Resource, 0, len(c.items))
	for _, resource := range c.items {
		r = append(r, types.ProtoResource153ToLegacy(resource))
	}
	return r
}

func (c *workloadIdentityX509RevocationCollection) WriteText(w io.Writer, verbose bool) error {
	headers := []string{"Serial", "Revoked At", "Expires At", "Reason"}

	var rows [][]string
	for _, item := range c.items {
		expiryTime := item.GetMetadata().GetExpires().AsTime()
		revokeTime := item.GetSpec().GetRevokedAt().AsTime()

		rows = append(rows, []string{
			item.Metadata.Name,
			revokeTime.Format(time.RFC3339),
			expiryTime.Format(time.RFC3339),
			item.GetSpec().GetReason(),
		})
	}

	t := asciitable.MakeTable(headers, rows...)

	// stable sort by name.
	t.SortRowsBy([]int{0}, true)
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}
