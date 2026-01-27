// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package resources

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/internalutils/stream"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
)

type serverInfoCollection struct {
	serverInfos []types.ServerInfo
}

func (c *serverInfoCollection) Resources() []types.Resource {
	r := make([]types.Resource, len(c.serverInfos))
	for i, resource := range c.serverInfos {
		r[i] = resource
	}
	return r
}

func (c *serverInfoCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Labels"})
	for _, si := range c.serverInfos {
		pairs := []string{}
		for key, value := range si.GetNewLabels() {
			pairs = append(pairs, fmt.Sprintf("%v=%v", key, value))
		}
		t.AddRow([]string{si.GetName(), strings.Join(pairs, ",")})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func serverInfoHandler() Handler {
	return Handler{
		getHandler:    getServerInfo,
		createHandler: createServerInfo,
		deleteHandler: deleteServerInfo,
		singleton:     false,
		mfaRequired:   false,
		description:   "Allows setting labels on a running Teleport SSH agent.",
	}
}

func getServerInfo(
	ctx context.Context,
	client *authclient.Client,
	ref services.Ref,
	opts GetOpts,
) (Collection, error) {
	if ref.Name != "" {
		si, err := client.GetServerInfo(ctx, ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &serverInfoCollection{serverInfos: []types.ServerInfo{si}}, nil
	}
	serverInfos, err := stream.Collect(client.GetServerInfos(ctx))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &serverInfoCollection{serverInfos: serverInfos}, nil
}

func createServerInfo(
	ctx context.Context,
	client *authclient.Client,
	raw services.UnknownResource,
	opts CreateOpts,
) error {
	si, err := services.UnmarshalServerInfo(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	// Check if the ServerInfo already exists.
	name := si.GetName()
	_, err = client.GetServerInfo(ctx, name)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	exists := (err == nil)
	if !opts.Force && exists {
		return trace.AlreadyExists("server info %q already exists", name)
	}

	err = client.UpsertServerInfo(ctx, si)
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Server info %q has been %s\n",
		name, upsertVerb(exists, opts.Force),
	)
	return nil
}

func deleteServerInfo(
	ctx context.Context,
	client *authclient.Client,
	ref services.Ref,
) error {
	if err := client.DeleteServerInfo(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Server info %q has been deleted\n", ref.Name)
	return nil
}
