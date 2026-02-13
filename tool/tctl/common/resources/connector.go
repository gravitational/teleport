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
	"io"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	authclient "github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
)

// NewConnectorCollection creates a new connector collection. Used in tests.
func NewConnectorCollection(oidc []types.OIDCConnector, saml []types.SAMLConnector, github []types.GithubConnector) Collection {
	return &connectorCollection{
		oidc:   &oidcConnectorCollection{oidc},
		saml:   &samlConnectorCollection{saml},
		github: &githubConnectorCollection{github},
	}
}

// connectorCollection is a meta-collection over the SAML, OIDC, and GitHub connectors.
type connectorCollection struct {
	oidc, saml, github Collection
}

func (c *connectorCollection) Resources() (r []types.Resource) {
	if c.oidc != nil {
		r = append(r, c.oidc.Resources()...)
	}
	if c.saml != nil {
		r = append(r, c.saml.Resources()...)
	}
	if c.github != nil {
		r = append(r, c.github.Resources()...)
	}
	return r
}

func (c *connectorCollection) WriteText(w io.Writer, verbose bool) error {
	if c.oidc != nil && len(c.oidc.Resources()) > 0 {
		_, err := io.WriteString(w, "\nOIDC:\n")
		if err != nil {
			return trace.Wrap(err)
		}
		if err := c.oidc.WriteText(w, verbose); err != nil {
			return trace.Wrap(err)
		}
	}

	if c.saml != nil && len(c.saml.Resources()) > 0 {
		_, err := io.WriteString(w, "\nSAML:\n")
		if err != nil {
			return trace.Wrap(err)
		}
		if err := c.saml.WriteText(w, verbose); err != nil {
			return trace.Wrap(err)
		}
	}

	if c.github != nil && len(c.github.Resources()) > 0 {
		_, err := io.WriteString(w, "\nGitHub:\n")
		if err != nil {
			return trace.Wrap(err)
		}
		if err := c.github.WriteText(w, verbose); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

func connectorsHandler() Handler {
	return Handler{
		getHandler:  getConnectors,
		singleton:   false,
		mfaRequired: true,
		description: "Meta resource listing GitHub, SAML, and OIDC connectors.",
	}
}

func getConnectors(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	oidc, oidcErr := getOIDCConnector(ctx, client, ref, opts)
	noOIDC := oidcErr != nil || len(oidc.Resources()) == 0

	saml, samlErr := getSAMLConnector(ctx, client, ref, opts)
	noSAML := samlErr != nil || len(saml.Resources()) == 0

	github, githubErr := getGithubConnector(ctx, client, ref, opts)
	noGithub := githubErr != nil || len(github.Resources()) == 0

	errs := []error{oidcErr, samlErr, githubErr}
	allEmpty := noOIDC && noSAML && noGithub

	var unexpectedErrs []error
	for _, err := range errs {
		if err != nil && !trace.IsNotFound(err) {
			unexpectedErrs = append(unexpectedErrs, err)
		}
	}

	var finalErr error
	if allEmpty || len(unexpectedErrs) > 0 {
		finalErr = trace.NewAggregate(errs...)
	}

	return &connectorCollection{
		saml:   saml,
		oidc:   oidc,
		github: github,
	}, finalErr
}
