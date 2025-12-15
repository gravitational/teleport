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

func NewAutoUpdateVersionCollection(version *autoupdatev1pb.AutoUpdateVersion) Collection {
	return &autoUpdateVersionCollection{
		version: version,
	}
}

type autoUpdateVersionCollection struct {
	version *autoupdatev1pb.AutoUpdateVersion
}

func (c *autoUpdateVersionCollection) Resources() []types.Resource {
	return []types.Resource{types.ProtoResource153ToLegacy(c.version)}
}

func (c *autoUpdateVersionCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Tools AutoUpdate Version"})
	t.AddRow([]string{
		c.version.GetMetadata().GetName(),
		fmt.Sprintf("%v", c.version.GetSpec().GetTools().TargetVersion),
	})
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func autoUpdateVersionHandler() Handler {
	return Handler{
		getHandler:    getAutoUpdateVersion,
		createHandler: createAutoUpdateVersion,
		updateHandler: updateAutoUpdateVersion,
		deleteHandler: deleteAutoUpdateVersion,
		singleton:     true,
		mfaRequired:   false,
		description:   "Controls which version agents and tools will update to. Cannot be changed in Teleport cloud.",
	}
}

func getAutoUpdateVersion(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	config, err := client.GetAutoUpdateVersion(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &autoUpdateVersionCollection{config}, nil
}

func createAutoUpdateVersion(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	version, err := services.UnmarshalProtoResource[*autoupdatev1pb.AutoUpdateVersion](raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if version.GetMetadata() == nil {
		version.Metadata = &headerv1.Metadata{}
	}
	if version.GetMetadata().GetName() == "" {
		version.Metadata.Name = types.MetaNameAutoUpdateVersion
	}

	if opts.Force {
		_, err = client.UpsertAutoUpdateVersion(ctx, version)
	} else {
		_, err = client.CreateAutoUpdateVersion(ctx, version)
	}
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Println("autoupdate_version has been created")
	return nil
}

func updateAutoUpdateVersion(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	version, err := services.UnmarshalProtoResource[*autoupdatev1pb.AutoUpdateVersion](raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if version.GetMetadata() == nil {
		version.Metadata = &headerv1.Metadata{}
	}
	if version.GetMetadata().GetName() == "" {
		version.Metadata.Name = types.MetaNameAutoUpdateVersion
	}

	if _, err := client.UpdateAutoUpdateVersion(ctx, version); err != nil {
		return trace.Wrap(err)
	}
	fmt.Println("autoupdate_version has been updated")
	return nil
}

func deleteAutoUpdateVersion(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.DeleteAutoUpdateVersion(ctx); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("AutoUpdateVersion has been deleted\n")
	return nil
}
