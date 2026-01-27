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

package resources

import (
	"context"
	"fmt"
	"io"
	"strconv"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
)

type networkingConfigCollection struct {
	netConfig types.ClusterNetworkingConfig
}

func (c *networkingConfigCollection) Resources() (r []types.Resource) {
	return []types.Resource{c.netConfig}
}

func (c *networkingConfigCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Client Idle Timeout", "Keep Alive Interval", "Keep Alive Count Max", "Session Control Timeout"})
	t.AddRow([]string{
		c.netConfig.GetClientIdleTimeout().String(),
		c.netConfig.GetKeepAliveInterval().String(),
		strconv.FormatInt(c.netConfig.GetKeepAliveCountMax(), 10),
		c.netConfig.GetSessionControlTimeout().String(),
	})
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func networkingConfigHandler() Handler {
	return Handler{
		getHandler:    getNetworkingConfig,
		createHandler: createNetworkingConfig,
		updateHandler: updateNetworkingConfig,
		deleteHandler: deleteNetworkingConfig,
		singleton:     true,
		mfaRequired:   false,
		description:   "Configures the cluster networking. Can only be used when networking is not configured in teleport.yaml.",
	}
}

func getNetworkingConfig(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	if ref.Name != "" {
		return nil, trace.BadParameter("only simple `tctl get %v` can be used", types.KindClusterNetworkingConfig)
	}
	netConfig, err := client.GetClusterNetworkingConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &networkingConfigCollection{netConfig}, nil
}

func createNetworkingConfig(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	newNetConfig, err := services.UnmarshalClusterNetworkingConfig(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	storedNetConfig, err := client.GetClusterNetworkingConfig(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := checkCreateResourceWithOrigin(storedNetConfig, "cluster networking configuration", opts.Force, opts.Confirm); err != nil {
		return trace.Wrap(err)
	}

	if _, err := client.UpsertClusterNetworkingConfig(ctx, newNetConfig); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("cluster networking configuration has been updated\n")
	return nil
}

func updateNetworkingConfig(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	newNetConfig, err := services.UnmarshalClusterNetworkingConfig(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	storedNetConfig, err := client.GetClusterNetworkingConfig(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := checkUpdateResourceWithOrigin(storedNetConfig, "cluster networking configuration", opts.Confirm); err != nil {
		return trace.Wrap(err)
	}

	if _, err := client.UpdateClusterNetworkingConfig(ctx, newNetConfig); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("cluster networking configuration has been updated\n")
	return nil
}

func deleteNetworkingConfig(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	storedNetConfig, err := client.GetClusterNetworkingConfig(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	managedByStaticConfig := storedNetConfig.Origin() == types.OriginConfigFile
	if managedByStaticConfig {
		return trace.BadParameter("%s", managedByStaticDeleteMsg)
	}

	if err := client.ResetClusterNetworkingConfig(ctx); err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("cluster networking configuration has been reset to defaults\n")
	return nil
}
