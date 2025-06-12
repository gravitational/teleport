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

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/vnet/v1"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/tctl/common/resource/collections"
)

var vnetConfig = resource{
	getHandler:    getVnetConfig,
	createHandler: createVnetConfig,
	updateHandler: updateVnetConfig,
	singleton:     true,
}

func createVnetConfig(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
	vnetConfig, err := services.UnmarshalProtoResource[*vnet.VnetConfig](raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if opts.force {
		_, err = client.VnetConfigServiceClient().UpsertVnetConfig(ctx, &vnet.UpsertVnetConfigRequest{VnetConfig: vnetConfig})
	} else {
		_, err = client.VnetConfigServiceClient().CreateVnetConfig(ctx, &vnet.CreateVnetConfigRequest{VnetConfig: vnetConfig})
	}
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Println("vnet_config has been created")
	return nil
}

func updateVnetConfig(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
	vnetConfig, err := services.UnmarshalProtoResource[*vnet.VnetConfig](raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	if _, err := client.VnetConfigServiceClient().UpdateVnetConfig(ctx, &vnet.UpdateVnetConfigRequest{VnetConfig: vnetConfig}); err != nil {
		return trace.Wrap(err)
	}
	fmt.Println("vnet_config has been updated")
	return nil
}

func getVnetConfig(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	vnetConfig, err := client.VnetConfigServiceClient().GetVnetConfig(ctx, &vnet.GetVnetConfigRequest{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return collections.NewVnetConfigCollection(vnetConfig), nil
}
