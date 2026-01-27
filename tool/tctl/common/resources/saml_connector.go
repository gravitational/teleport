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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
)

type samlConnectorCollection struct {
	connectors []types.SAMLConnector
}

func (c *samlConnectorCollection) Resources() (r []types.Resource) {
	for _, resource := range c.connectors {
		r = append(r, resource)
	}
	return r
}

func (c *samlConnectorCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "SSO URL"})
	for _, conn := range c.connectors {
		t.AddRow([]string{conn.GetName(), conn.GetSSO()})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func samlConnectorHandler() Handler {
	return Handler{
		getHandler:    getSAMLConnector,
		createHandler: createSAMLConnector,
		updateHandler: updateSAMLConnector,
		deleteHandler: deleteSAMLConnector,
		singleton:     false,
		mfaRequired:   true,
		description:   "Configures how users can connect using a SAML Identity Provider.",
	}
}

func getSAMLConnector(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	if ref.Name == "" {
		// TODO(okraport): DELETE IN v21.0.0, remove GetSAMLConnectors
		connectors, err := clientutils.CollectWithFallback(ctx,
			func(ctx context.Context, limit int, start string) ([]types.SAMLConnector, string, error) {
				return client.ListSAMLConnectorsWithOptions(ctx, limit, start, opts.WithSecrets)
			},
			func(ctx context.Context) ([]types.SAMLConnector, error) {
				//nolint:staticcheck // support older backends during migration
				return client.GetSAMLConnectors(ctx, opts.WithSecrets)
			},
		)

		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &samlConnectorCollection{connectors: connectors}, nil
	}
	connector, err := client.GetSAMLConnector(ctx, ref.Name, opts.WithSecrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &samlConnectorCollection{[]types.SAMLConnector{connector}}, nil
}

func createSAMLConnector(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	// Create services.SAMLConnector from raw YAML to extract the connector name.
	conn, err := services.UnmarshalSAMLConnector(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	connectorName := conn.GetName()
	foundConn, err := client.GetSAMLConnector(ctx, connectorName, true)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	exists := (err == nil)
	if !opts.Force && exists {
		return trace.AlreadyExists("connector %q already exists, use -f flag to override", connectorName)
	}

	// If the connector being pushed to the backend does not have a signing key
	// in it and an existing connector was found in the backend, extract the
	// signing key from the found connector and inject it into the connector
	// being injected into the backend.
	if conn.GetSigningKeyPair() == nil && exists {
		conn.SetSigningKeyPair(foundConn.GetSigningKeyPair())
	}

	if _, err = client.UpsertSAMLConnector(ctx, conn); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("authentication connector %q has been %s\n", connectorName, upsertVerb(exists, opts.Force))
	return nil
}

func updateSAMLConnector(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	conn, err := services.UnmarshalSAMLConnector(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if _, err = client.UpdateSAMLConnector(ctx, conn); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("authentication connector %q has been updated\n", conn.GetName())
	return nil
}

func deleteSAMLConnector(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.DeleteSAMLConnector(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("SAML connector %v has been deleted\n", ref.Name)
	return nil
}
