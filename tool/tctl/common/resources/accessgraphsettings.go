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

	"github.com/gravitational/trace"

	clusterconfigpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	clusterconfigrec "github.com/gravitational/teleport/tool/tctl/common/clusterconfig"
)

type accessGraphSettingsCollection struct {
	accessGraphSettings *clusterconfigrec.AccessGraphSettings
}

func (c *accessGraphSettingsCollection) Resources() []types.Resource {
	return []types.Resource{c.accessGraphSettings}
}

func (c *accessGraphSettingsCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"SSH Keys Scan"})
	t.AddRow([]string{
		c.accessGraphSettings.Spec.SecretsScanConfig,
	})
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func accessGraphSettingsHandler() Handler {
	return Handler{
		getHandler:    getAccessGraphSettings,
		createHandler: upsertAccessGraphSettings,
		updateHandler: updateAccessGraphSettings,
		singleton:     true,
		mfaRequired:   false,
		description:   "Configures Access Graph settings for the cluster.",
	}
}

func getAccessGraphSettings(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	settings, err := client.ClusterConfigClient().GetAccessGraphSettings(ctx, &clusterconfigpb.GetAccessGraphSettingsRequest{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	rec, err := clusterconfigrec.ProtoToResource(settings)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &accessGraphSettingsCollection{accessGraphSettings: rec}, nil
}

func upsertAccessGraphSettings(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
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

func updateAccessGraphSettings(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
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
