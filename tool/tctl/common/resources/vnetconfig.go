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
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/vnet/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
)

func getVnetConfig(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	if ref.Name != "" {
		return nil, trace.BadParameter("only simple `tctl get %v` can be used", types.KindVnetConfig)
	}
	vnetConfig, err := client.GetVnetConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &vnetConfigCollection{vnetConfig}, nil
}

func createVnetConfig(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	vnetConfig, err := services.UnmarshalProtoResource[*vnet.VnetConfig](raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if opts.Force {
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

func updateVnetConfig(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
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

type vnetConfigCollection struct {
	vnetConfig *vnet.VnetConfig
}

func (c *vnetConfigCollection) Resources() []types.Resource {
	return []types.Resource{types.Resource153ToLegacy(c.vnetConfig)}
}

func (c *vnetConfigCollection) WriteText(w io.Writer, verbose bool) error {
	var dnsZoneSuffixes []string
	for _, dnsZone := range c.vnetConfig.Spec.CustomDnsZones {
		dnsZoneSuffixes = append(dnsZoneSuffixes, dnsZone.Suffix)
	}
	t := asciitable.MakeTable([]string{"IPv4 CIDR range", "Custom DNS Zones"})
	t.AddRow([]string{
		c.vnetConfig.GetSpec().GetIpv4CidrRange(),
		strings.Join(dnsZoneSuffixes, ", "),
	})
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func vnetConfigHandler() Handler {
	return Handler{
		getHandler:    getVnetConfig,
		createHandler: createVnetConfig,
		updateHandler: updateVnetConfig,
		singleton:     true,
		mfaRequired:   false,
		description:   "Configures the VNet settings",
	}
}
