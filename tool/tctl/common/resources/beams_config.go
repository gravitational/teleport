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

func getBeamsConfig(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	resp, err := client.BeamsConfigServiceClient().GetBeamsConfig(ctx, beamsv1.GetBeamsConfigRequest_builder{}.Build())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &beamsConfigCollection{resp.GetBeamsConfig()}, nil
}

func createBeamsConfig(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	if opts.Force {
		return trace.Wrap(updateBeamsConfig(ctx, client, raw, opts))
	}

	config, err := services.UnmarshalProtoResource[*beamsv1.BeamsConfig](raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	if _, err := client.BeamsConfigServiceClient().CreateBeamsConfig(ctx, beamsv1.CreateBeamsConfigRequest_builder{
		BeamsConfig: config,
	}.Build()); err != nil {
		return trace.Wrap(err)
	}
	fmt.Println("beams_config has been created")
	return nil
}

func updateBeamsConfig(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	config, err := services.UnmarshalProtoResource[*beamsv1.BeamsConfig](raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	existing, err := client.BeamsConfigServiceClient().GetBeamsConfig(ctx, beamsv1.GetBeamsConfigRequest_builder{}.Build())
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	if existing.GetBeamsConfig().GetMetadata().GetRevision() == "" {
		if _, err := client.BeamsConfigServiceClient().CreateBeamsConfig(ctx, beamsv1.CreateBeamsConfigRequest_builder{
			BeamsConfig: config,
		}.Build()); err != nil {
			return trace.Wrap(err)
		}
		fmt.Println("beams_config has been created")
		return nil
	}

	if opts.Force {
		config.GetMetadata().Revision = existing.GetBeamsConfig().GetMetadata().GetRevision()
	}
	if _, err := client.BeamsConfigServiceClient().UpdateBeamsConfig(ctx, beamsv1.UpdateBeamsConfigRequest_builder{
		BeamsConfig: config,
	}.Build()); err != nil {
		return trace.Wrap(err)
	}
	fmt.Println("beams_config has been updated")
	return nil
}

func deleteBeamsConfig(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if _, err := client.BeamsConfigServiceClient().DeleteBeamsConfig(ctx, beamsv1.DeleteBeamsConfigRequest_builder{}.Build()); err != nil {
		return trace.Wrap(err)
	}
	fmt.Println("beams_config has been deleted")
	return nil
}

type beamsConfigCollection struct {
	config *beamsv1.BeamsConfig
}

func (c *beamsConfigCollection) Resources() []types.Resource {
	return []types.Resource{types.ProtoResource153ToLegacy(c.config)}
}

func (c *beamsConfigCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Anthropic App", "OpenAI App"})
	t.AddRow([]string{
		c.config.GetSpec().GetLlm().GetAnthropic().GetAppName(),
		c.config.GetSpec().GetLlm().GetOpenai().GetAppName(),
	})
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func beamsConfigHandler() Handler {
	return Handler{
		getHandler:    getBeamsConfig,
		createHandler: createBeamsConfig,
		updateHandler: updateBeamsConfig,
		deleteHandler: deleteBeamsConfig,
		singleton:     true,
		mfaRequired:   true,
		description:   "Configures the Beams settings",
	}
}
