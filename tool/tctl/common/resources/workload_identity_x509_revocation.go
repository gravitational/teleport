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
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package resources

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/gravitational/trace"

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
)

type workloadIdentityX509RevocationCollection struct {
	items []*workloadidentityv1pb.WorkloadIdentityX509Revocation
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

func workloadIdentityX509RevocationHandler() Handler {
	return Handler{
		getHandler:    getWorkloadIdentityX509Revocation,
		deleteHandler: deleteWorkloadIdentityX509Revocation,
		description:   "Controls the revocation of issued X.509 SVIDs",
		mfaRequired:   false,
		singleton:     false,
	}
}

func getWorkloadIdentityX509Revocation(
	ctx context.Context,
	client *authclient.Client,
	ref services.Ref,
	opts GetOpts,
) (Collection, error) {
	c := client.WorkloadIdentityRevocationServiceClient()
	if ref.Name != "" {
		resource, err := c.GetWorkloadIdentityX509Revocation(ctx, &workloadidentityv1pb.GetWorkloadIdentityX509RevocationRequest{
			Name: ref.Name,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &workloadIdentityX509RevocationCollection{items: []*workloadidentityv1pb.WorkloadIdentityX509Revocation{resource}}, nil
	}

	resources, err := stream.Collect(clientutils.Resources(ctx, func(ctx context.Context, limit int, pageToken string) ([]*workloadidentityv1pb.WorkloadIdentityX509Revocation, string, error) {
		resp, err := c.ListWorkloadIdentityX509Revocations(ctx, &workloadidentityv1pb.ListWorkloadIdentityX509RevocationsRequest{
			PageSize:  int32(limit),
			PageToken: pageToken,
		})

		return resp.GetWorkloadIdentityX509Revocations(), resp.GetNextPageToken(), trace.Wrap(err)
	}))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &workloadIdentityX509RevocationCollection{items: resources}, nil
}

func deleteWorkloadIdentityX509Revocation(
	ctx context.Context,
	client *authclient.Client,
	ref services.Ref,
) error {
	c := client.WorkloadIdentityRevocationServiceClient()
	_, err := c.DeleteWorkloadIdentityX509Revocation(
		ctx, &workloadidentityv1pb.DeleteWorkloadIdentityX509RevocationRequest{
			Name: ref.Name,
		})
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Workload identity X509 revocation %q has been deleted\n", ref.Name)
	return nil
}
