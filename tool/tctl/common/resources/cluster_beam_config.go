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

	"github.com/gravitational/trace"

	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
)

type clusterBeamConfigCollection struct {
	config *beamsv1.ClusterBeamConfig
}

func (c *clusterBeamConfigCollection) Resources() []types.Resource {
	return []types.Resource{types.Resource153ToLegacy(c.config)}
}

func (c *clusterBeamConfigCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"OpenAI App", "Anthropic App"})
	openai := c.config.GetSpec().GetLlm().GetOpenai().GetApp()
	anthropic := c.config.GetSpec().GetLlm().GetAnthropic().GetApp()
	t.AddRow([]string{openai, anthropic})
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func clusterBeamConfigHandler() Handler {
	return Handler{
		getHandler:    getClusterBeamConfig,
		createHandler: upsertClusterBeamConfig,
		updateHandler: updateClusterBeamConfig,
		deleteHandler: deleteClusterBeamConfig,
		singleton:     true,
		mfaRequired:   true,
		description:   "Configures cluster-wide beam settings.",
	}
}

func getClusterBeamConfig(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	cfg, err := client.ClusterBeamConfigServiceClient().GetClusterBeamConfig(ctx, &beamsv1.GetClusterBeamConfigRequest{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &clusterBeamConfigCollection{config: cfg}, nil
}

func upsertClusterBeamConfig(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	cfg, err := services.UnmarshalProtoResource[*beamsv1.ClusterBeamConfig](raw.Raw)
	if err != nil {
		return trace.Wrap(err)
	}

	if _, err := client.ClusterBeamConfigServiceClient().CreateClusterBeamConfig(ctx, &beamsv1.CreateClusterBeamConfigRequest{Config: cfg}); err != nil {
		if !trace.IsAlreadyExists(err) || !opts.Force {
			return trace.Wrap(err)
		}
		if _, err := client.ClusterBeamConfigServiceClient().UpdateClusterBeamConfig(ctx, &beamsv1.UpdateClusterBeamConfigRequest{Config: cfg}); err != nil {
			return trace.Wrap(err)
		}
	}

	fmt.Println("cluster_beam_config has been upserted")
	return nil
}

func updateClusterBeamConfig(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	cfg, err := services.UnmarshalProtoResource[*beamsv1.ClusterBeamConfig](raw.Raw)
	if err != nil {
		return trace.Wrap(err)
	}

	if _, err := client.ClusterBeamConfigServiceClient().UpdateClusterBeamConfig(ctx, &beamsv1.UpdateClusterBeamConfigRequest{Config: cfg}); err != nil {
		return trace.Wrap(err)
	}
	fmt.Println("cluster_beam_config has been updated")
	return nil
}

func deleteClusterBeamConfig(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if _, err := client.ClusterBeamConfigServiceClient().DeleteClusterBeamConfig(ctx, &beamsv1.DeleteClusterBeamConfigRequest{}); err != nil {
		return trace.Wrap(err)
	}
	fmt.Println("cluster_beam_config has been deleted")
	return nil
}
