// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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
// along with this program.  If not, see <http://www.gnu.org/licenses/>

package resources

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/gravitational/trace"

	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
)

type spiffeFederationCollection struct {
	items []*machineidv1pb.SPIFFEFederation
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

func spiffeFederationHandler() Handler {
	return Handler{
		getHandler:    getSPIFFEFederation,
		createHandler: createSPIFFEFederation,
		deleteHandler: deleteSPIFFEFederation,
		singleton:     false,
		mfaRequired:   false,
		description:   "Manages SPIFFE federation relationships between this Teleport cluster and other trust domains.",
	}
}

func getSPIFFEFederation(
	ctx context.Context,
	client *authclient.Client,
	ref services.Ref,
	opts GetOpts,
) (Collection, error) {
	c := client.SPIFFEFederationServiceClient()
	if ref.Name != "" {
		resource, err := c.GetSPIFFEFederation(ctx, &machineidv1pb.GetSPIFFEFederationRequest{
			Name: ref.Name,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &spiffeFederationCollection{items: []*machineidv1pb.SPIFFEFederation{resource}}, nil
	}

	var resources []*machineidv1pb.SPIFFEFederation
	pageToken := ""
	for {
		resp, err := c.ListSPIFFEFederations(ctx, &machineidv1pb.ListSPIFFEFederationsRequest{
			PageToken: pageToken,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		resources = append(resources, resp.SpiffeFederations...)

		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}

	return &spiffeFederationCollection{items: resources}, nil
}

func createSPIFFEFederation(
	ctx context.Context,
	client *authclient.Client,
	raw services.UnknownResource,
	opts CreateOpts,
) error {
	in, err := services.UnmarshalSPIFFEFederation(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	c := client.SPIFFEFederationServiceClient()
	_, err = c.CreateSPIFFEFederation(ctx, &machineidv1pb.CreateSPIFFEFederationRequest{
		SpiffeFederation: in,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("SPIFFE Federation %q has been created\n", in.GetMetadata().GetName())
	return nil
}

func deleteSPIFFEFederation(
	ctx context.Context,
	client *authclient.Client,
	ref services.Ref,
) error {
	c := client.SPIFFEFederationServiceClient()
	_, err := c.DeleteSPIFFEFederation(
		ctx, &machineidv1pb.DeleteSPIFFEFederationRequest{
			Name: ref.Name,
		})
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("SPIFFE Federation %q has been deleted\n", ref.Name)
	return nil
}
