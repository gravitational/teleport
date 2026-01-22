// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package resources

import (
	"context"
	"io"

	"github.com/gravitational/trace"

	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
)

type managedResourceCollection struct {
	resources []*identitycenterv1.ManagedResource
}

func (c *managedResourceCollection) Resources() []types.Resource {
	// ManagedResource doesn't implement types.Resource yet
	// Return empty slice for now
	return nil
}

func (c *managedResourceCollection) WriteText(w io.Writer, verbose bool) error {
	rows := make([][]string, 0, len(c.resources))
	for _, mr := range c.resources {
		subKind := mr.GetSubKind()
		if subKind == "" {
			subKind = "unknown"
		}
		rows = append(rows, []string{
			mr.GetMetadata().GetName(),
			subKind,
			mr.GetSpec().GetArn(),
		})
	}
	headers := []string{"Name", "Type", "ARN"}
	t := asciitable.MakeTable(headers, rows...)
	t.SortRowsBy([]int{0}, true)
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func managedResourceHandler() Handler {
	return Handler{
		getHandler:  getManagedResource,
		description: "AWS Identity Center Managed Resources (RDS, S3, EC2, etc.)",
	}
}

// getManagedResource implements `tctl get managed_resource` command.
func getManagedResource(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	if ref.Name == "" {
		// List all resources
		resources, _, err := client.ListIdentityCenterManagedResources(ctx, 0, "")
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &managedResourceCollection{resources: resources}, nil
	}
	// Get specific resource
	resource, err := client.GetIdentityCenterManagedResource(ctx, ref.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &managedResourceCollection{resources: []*identitycenterv1.ManagedResource{resource}}, nil
}
