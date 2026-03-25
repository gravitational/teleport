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

	autoupdatev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
)

func NewAutoUpdateConfigCollection(config *autoupdatev1pb.AutoUpdateConfig) Collection {
	return &autoUpdateConfigCollection{
		config: config,
	}
}

type autoUpdateConfigCollection struct {
	config *autoupdatev1pb.AutoUpdateConfig
}

func (c *autoUpdateConfigCollection) Resources() []types.Resource {
	return []types.Resource{types.ProtoResource153ToLegacy(c.config)}
}

func (c *autoUpdateConfigCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Tools AutoUpdate Enabled"})
	t.AddRow([]string{
		c.config.GetMetadata().GetName(),
		fmt.Sprintf("%v", c.config.GetSpec().GetTools().GetMode()),
	})
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func autoUpdateConfigHandler() Handler {
	return Handler{
		getHandler:    getAutoUpdateConfig,
		createHandler: createAutoUpdateConfig,
		updateHandler: updateAutoUpdateConfig,
		deleteHandler: deleteAutoUpdateConfig,
		singleton:     true,
		mfaRequired:   false,
		description:   "Configures if, when, and how managed updates happen.",
	}
}

func getAutoUpdateConfig(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	config, err := client.GetAutoUpdateConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &autoUpdateConfigCollection{config}, nil
}

func createAutoUpdateConfig(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	config, err := services.UnmarshalProtoResource[*autoupdatev1pb.AutoUpdateConfig](raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if config.GetMetadata() == nil {
		config.Metadata = &headerv1.Metadata{}
	}
	if config.GetMetadata().GetName() == "" {
		config.Metadata.Name = types.MetaNameAutoUpdateConfig
	}

	if opts.Force {
		_, err = client.UpsertAutoUpdateConfig(ctx, config)
	} else {
		_, err = client.CreateAutoUpdateConfig(ctx, config)
	}
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Println("autoupdate_config has been created")
	return nil
}

func updateAutoUpdateConfig(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	config, err := services.UnmarshalProtoResource[*autoupdatev1pb.AutoUpdateConfig](raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if config.GetMetadata() == nil {
		config.Metadata = &headerv1.Metadata{}
	}
	if config.GetMetadata().GetName() == "" {
		config.Metadata.Name = types.MetaNameAutoUpdateConfig
	}

	if _, err := client.UpdateAutoUpdateConfig(ctx, config); err != nil {
		return trace.Wrap(err)
	}
	fmt.Println("autoupdate_config has been updated")
	return nil
}

func deleteAutoUpdateConfig(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.DeleteAutoUpdateConfig(ctx); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("AutoUpdateConfig has been deleted\n")
	return nil
}
