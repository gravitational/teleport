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
	"log/slog"

	"github.com/crewjam/saml/samlsp"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
)

type samlIdPServiceProviderCollection struct {
	serviceProviders []types.SAMLIdPServiceProvider
}

// Resources returns collection of SAML IdP service provider resource.
func (c *samlIdPServiceProviderCollection) Resources() []types.Resource {
	r := make([]types.Resource, len(c.serviceProviders))
	for i, resource := range c.serviceProviders {
		r[i] = resource
	}
	return r
}

// WriteText writes collection of SAML IdP service provider resource to [w].
func (c *samlIdPServiceProviderCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name"})
	for _, serviceProvider := range c.serviceProviders {
		t.AddRow([]string{serviceProvider.GetName()})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func samlIdPServiceProviderHandler() Handler {
	return Handler{
		getHandler:    getSAMLIdPServiceProvider,
		createHandler: createSAMLIdPServiceProvider,
		updateHandler: updateSAMLIdPServiceProvider,
		deleteHandler: deleteSAMLIdPServiceProvider,

		singleton: false,
		// MFA not enforced in Auth.
		mfaRequired: false,
		description: "Configure service provider for the Teleport SAML IdP",
	}
}

func getSAMLIdPServiceProvider(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	if ref.Name != "" {
		serviceProvider, err := client.GetSAMLIdPServiceProvider(ctx, ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &samlIdPServiceProviderCollection{serviceProviders: []types.SAMLIdPServiceProvider{serviceProvider}}, nil
	}

	resources, err := stream.Collect(clientutils.Resources(ctx, client.ListSAMLIdPServiceProviders))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &samlIdPServiceProviderCollection{serviceProviders: resources}, nil
}

func createSAMLIdPServiceProvider(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	// Create services.SAMLIdPServiceProvider from raw YAML to extract the service provider name.
	sp, err := services.UnmarshalSAMLIdPServiceProvider(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if sp.GetEntityDescriptor() != "" {
		// verify that entity descriptor parses
		ed, err := samlsp.ParseMetadata([]byte(sp.GetEntityDescriptor()))
		if err != nil {
			return trace.BadParameter("invalid entity descriptor for SAML IdP Service Provider %q: %v", sp.GetEntityID(), err)
		}

		// issue warning about unsupported ACS bindings.
		if err := services.FilterSAMLEntityDescriptor(ed, false /* quiet */); err != nil {
			slog.WarnContext(ctx, "Entity descriptor for SAML IdP service provider contains unsupported ACS bindings",
				"entity_id", sp.GetEntityID(),
				"error", err,
			)
		}
	}

	exists := false
	if err = client.CreateSAMLIdPServiceProvider(ctx, sp); err != nil {
		if trace.IsAlreadyExists(err) {
			exists = true
			err = client.UpdateSAMLIdPServiceProvider(ctx, sp)
		}

		if err != nil {
			return trace.Wrap(err)
		}
	}
	fmt.Printf("SAML IdP service provider %q has been %s\n", sp.GetName(), upsertVerb(exists, opts.Force))
	return nil
}

func updateSAMLIdPServiceProvider(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	// Create services.SAMLIdPServiceProvider from raw YAML to extract the service provider name.
	sp, err := services.UnmarshalSAMLIdPServiceProvider(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if sp.GetEntityDescriptor() != "" {
		// verify that entity descriptor parses
		ed, err := samlsp.ParseMetadata([]byte(sp.GetEntityDescriptor()))
		if err != nil {
			return trace.BadParameter("invalid entity descriptor for SAML IdP Service Provider %q: %v", sp.GetEntityID(), err)
		}

		// issue warning about unsupported ACS bindings.
		if err := services.FilterSAMLEntityDescriptor(ed, false /* quiet */); err != nil {
			slog.WarnContext(ctx, "Entity descriptor for SAML IdP service provider contains unsupported ACS bindings",
				"entity_id", sp.GetEntityID(),
				"error", err,
			)
		}
	}

	if err = client.UpdateSAMLIdPServiceProvider(ctx, sp); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("SAML IdP service provider %q has been updated\n", sp.GetName())
	return nil
}

func deleteSAMLIdPServiceProvider(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.DeleteSAMLIdPServiceProvider(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("SAML IdP service provider %q has been deleted\n", ref.Name)
	return nil
}
