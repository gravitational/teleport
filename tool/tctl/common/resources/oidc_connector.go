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
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
)

type oidcConnectorCollection struct {
	connectors []types.OIDCConnector
}

func (c *oidcConnectorCollection) Resources() (r []types.Resource) {
	for _, resource := range c.connectors {
		r = append(r, resource)
	}
	return r
}

func (c *oidcConnectorCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Issuer URL", "Additional Scope"})
	for _, conn := range c.connectors {
		t.AddRow([]string{
			conn.GetName(), conn.GetIssuerURL(), strings.Join(conn.GetScope(), ","),
		})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func oidcConnectorHandler() Handler {
	return Handler{
		getHandler:    getOIDCConnector,
		createHandler: createOIDCConnector,
		updateHandler: updateOIDCConnector,
		deleteHandler: deleteOIDCConnector,
		singleton:     false,
		mfaRequired:   true,
		description:   "Configures how users can connect using a OIDC Identity Provider.",
	}
}

func getOIDCConnector(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	if ref.Name == "" {
		// TODO(okraport): DELETE IN v21.0.0, remove GetOIDCConnectors
		connectors, err := clientutils.CollectWithFallback(ctx,
			func(ctx context.Context, limit int, start string) ([]types.OIDCConnector, string, error) {
				return client.ListOIDCConnectors(ctx, limit, start, opts.WithSecrets)
			},
			func(ctx context.Context) ([]types.OIDCConnector, error) {
				return client.GetOIDCConnectors(ctx, opts.WithSecrets)
			},
		)

		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &oidcConnectorCollection{connectors: connectors}, nil
	}
	connector, err := client.GetOIDCConnector(ctx, ref.Name, opts.WithSecrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &oidcConnectorCollection{[]types.OIDCConnector{connector}}, nil
}

func createOIDCConnector(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	conn, err := services.UnmarshalOIDCConnector(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if opts.Force {
		upserted, err := client.UpsertOIDCConnector(ctx, conn)
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("authentication connector %q has been updated\n", upserted.GetName())
		return nil
	}

	created, err := client.CreateOIDCConnector(ctx, conn)
	if err != nil {
		if trace.IsAlreadyExists(err) {
			return trace.AlreadyExists("connector %q already exists, use -f flag to override", conn.GetName())
		}

		return trace.Wrap(err)
	}

	fmt.Printf("authentication connector %q has been created\n", created.GetName())
	return nil
}

func updateOIDCConnector(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	connector, err := services.UnmarshalOIDCConnector(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if _, err := client.UpdateOIDCConnector(ctx, connector); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("authentication connector %q has been updated\n", connector.GetName())
	return nil
}

func deleteOIDCConnector(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.DeleteOIDCConnector(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("OIDC connector %v has been deleted\n", ref.Name)
	return nil
}
