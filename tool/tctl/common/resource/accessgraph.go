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

package resource

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"

	clusterconfigpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
	crownjewelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/crownjewel/v1"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	clusterconfigrec "github.com/gravitational/teleport/tool/tctl/common/clusterconfig"
	"github.com/gravitational/teleport/tool/tctl/common/resource/collections"
)

var accessGraphSettings = resource{
	getHandler:    getAccessGraphSettings,
	createHandler: upsertAccessGraphSettings,
	updateHandler: updateAccessGraphSettings,
	singleton:     true,
}

func getAccessGraphSettings(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	settings, err := client.ClusterConfigClient().GetAccessGraphSettings(ctx, &clusterconfigpb.GetAccessGraphSettingsRequest{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	rec, err := clusterconfigrec.ProtoToResource(settings)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return collections.NewAccessGraphSettingsCollection(rec), nil
}

func upsertAccessGraphSettings(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
	settings, err := clusterconfigrec.UnmarshalAccessGraphSettings(raw.Raw)
	if err != nil {
		return trace.Wrap(err)
	}

	if _, err = client.ClusterConfigClient().UpsertAccessGraphSettings(ctx, &clusterconfigpb.UpsertAccessGraphSettingsRequest{AccessGraphSettings: settings}); err != nil {
		return trace.Wrap(err)
	}

	fmt.Println("access_graph_settings has been upserted")
	return nil
}

func updateAccessGraphSettings(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
	settings, err := clusterconfigrec.UnmarshalAccessGraphSettings(raw.Raw)
	if err != nil {
		return trace.Wrap(err)
	}

	if _, err = client.ClusterConfigClient().UpdateAccessGraphSettings(ctx, &clusterconfigpb.UpdateAccessGraphSettingsRequest{AccessGraphSettings: settings}); err != nil {
		return trace.Wrap(err)
	}
	fmt.Println("access_graph_settings has been updated")
	return nil
}

var crownJewel = resource{
	getHandler:    getCrownJewel,
	createHandler: createCrownJewel,
	updateHandler: updateCrownJewel,
	deleteHandler: deleteCrownJewel,
	singleton:     false,
	description:   "",
}

// Note(hugoShaka): This getter does not seem to support fetching a single resource,
// but the resource does not look like a singleton. This is sketchy, is this intentional?
func getCrownJewel(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	cjClient := client.CrownJewelsClient()
	var rules []*crownjewelv1.CrownJewel
	nextToken := ""
	for {
		resp, token, err := cjClient.ListCrownJewels(ctx, 0 /* default size */, nextToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		rules = append(rules, resp...)

		if token == "" {
			break
		}
		nextToken = token
	}
	return collections.NewCrownJewelCollection(rules), nil
}

func createCrownJewel(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
	crownJewel, err := services.UnmarshalCrownJewel(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	c := client.CrownJewelsClient()
	if opts.force {
		if _, err := c.UpsertCrownJewel(ctx, crownJewel); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("crown jewel %q has been updated\n", crownJewel.GetMetadata().GetName())
	} else {
		if _, err := c.CreateCrownJewel(ctx, crownJewel); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("crown jewel %q has been created\n", crownJewel.GetMetadata().GetName())
	}

	return nil
}

func updateCrownJewel(ctx context.Context, client *authclient.Client, resource services.UnknownResource, opts createOpts) error {
	in, err := services.UnmarshalCrownJewel(resource.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	if _, err := client.CrownJewelsClient().UpdateCrownJewel(ctx, in); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("crown jewel %q has been updated\n", in.GetMetadata().GetName())
	return nil
}

func deleteCrownJewel(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.CrownJewelsClient().DeleteCrownJewel(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("crown_jewel %q has been deleted\n", ref.Name)
	return nil
}
