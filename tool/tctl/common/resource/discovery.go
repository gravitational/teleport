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

	"github.com/gravitational/teleport/api/types/discoveryconfig"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/tctl/common/resource/collections"
)

var discoveryConfig = resource{
	getHandler:    getDiscoveryConfig,
	createHandler: createDiscoveryConfig,
	deleteHandler: deleteDiscoveryConfig,
}

func createDiscoveryConfig(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
	discoveryConfig, err := services.UnmarshalDiscoveryConfig(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	remote := client.DiscoveryConfigClient()

	if opts.force {
		if _, err := remote.UpsertDiscoveryConfig(ctx, discoveryConfig); err != nil {
			return trace.Wrap(err)
		}

		fmt.Printf("DiscoveryConfig %q has been written\n", discoveryConfig.GetName())
		return nil
	}

	if _, err := remote.CreateDiscoveryConfig(ctx, discoveryConfig); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("DiscoveryConfig %q has been created\n", discoveryConfig.GetName())

	return nil
}

func getDiscoveryConfig(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	remote := client.DiscoveryConfigClient()
	if ref.Name != "" {
		dc, err := remote.GetDiscoveryConfig(ctx, ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return collections.NewDiscoveryConfigCollection([]*discoveryconfig.DiscoveryConfig{dc}), nil
	}

	var resources []*discoveryconfig.DiscoveryConfig
	var dcs []*discoveryconfig.DiscoveryConfig
	var err error
	var nextKey string
	for {
		dcs, nextKey, err = remote.ListDiscoveryConfigs(ctx, 0, nextKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		resources = append(resources, dcs...)
		if nextKey == "" {
			break
		}
	}

	return collections.NewDiscoveryConfigCollection(resources), nil
}

func deleteDiscoveryConfig(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	remote := client.DiscoveryConfigClient()
	if err := remote.DeleteDiscoveryConfig(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("DiscoveryConfig %q removed\n", ref.Name)
	return nil
}
