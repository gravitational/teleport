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

package resources

import (
	"context"
	"fmt"
	"io"
	"strconv"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
)

type trustedClusterCollection struct {
	trustedClusters []types.TrustedCluster
}

func (c *trustedClusterCollection) Resources() (r []types.Resource) {
	for _, resource := range c.trustedClusters {
		r = append(r, resource)
	}
	return r
}

func (c *trustedClusterCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{
		"Name", "Enabled", "Token", "Proxy Address", "Reverse Tunnel Address", "Role Map",
	})
	for _, tc := range c.trustedClusters {
		t.AddRow([]string{
			tc.GetName(),
			strconv.FormatBool(tc.GetEnabled()),
			tc.GetToken(),
			tc.GetProxyAddress(),
			tc.GetReverseTunnelAddress(),
			fmt.Sprintf("%v", tc.CombinedMapping()),
		})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func trustedClusterHandler() Handler {
	return Handler{
		getHandler:    getTrustedCluster,
		createHandler: createTrustedCluster,
		deleteHandler: deleteTrustedCluster,
		singleton:     false,
		mfaRequired:   true,
		description:   "Configures the current cluster (Leaf) to trust certificates emitted by another cluster (Root).",
	}
}

func getTrustedCluster(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	if ref.Name == "" {
		// TODO(okraport): DELETE IN v21.0.0, replace with regular Collect
		trustedClusters, err := clientutils.CollectWithFallback(
			ctx,
			client.ListTrustedClusters,
			client.GetTrustedClusters,
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return &trustedClusterCollection{trustedClusters: trustedClusters}, nil
	}
	trustedCluster, err := client.GetTrustedCluster(ctx, ref.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &trustedClusterCollection{trustedClusters: []types.TrustedCluster{trustedCluster}}, nil
}

func createTrustedCluster(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	tc, err := services.UnmarshalTrustedCluster(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	// check if such cluster already exists:
	name := tc.GetName()
	_, err = client.GetTrustedCluster(ctx, name)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	exists := err == nil
	if !opts.Force && exists {
		return trace.AlreadyExists("trusted cluster %q already exists", name)
	}

	//nolint:staticcheck // SA1019. UpsertTrustedCluster is deprecated but will
	// continue being supported for tctl clients.
	// TODO(bernardjkim) consider using UpsertTrustedClusterV2 in VX.0.0
	out, err := client.UpsertTrustedCluster(ctx, tc)
	if err != nil {
		return trace.Wrap(err)
	}

	if out.GetName() != tc.GetName() {
		fmt.Printf("WARNING: trusted cluster resource %q has been renamed to match root cluster name %q. this will become an error in future teleport versions, please update your configuration to use the correct name.\n", name, out.GetName())
	}
	fmt.Printf("trusted cluster %q has been %v\n", out.GetName(), upsertVerb(exists, opts.Force))
	return nil
}

func deleteTrustedCluster(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.DeleteTrustedCluster(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("trusted cluster %q has been deleted\n", ref.Name)
	return nil
}
