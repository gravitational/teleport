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

package collections

import (
	"io"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
)

func NewOIDCCollection(connectors []types.OIDCConnector) ResourceCollection {
	return &oidcCollection{connectors: connectors}
}

type oidcCollection struct {
	connectors []types.OIDCConnector
}

func (c *oidcCollection) Resources() (r []types.Resource) {
	for _, resource := range c.connectors {
		r = append(r, resource)
	}
	return r
}

func (c *oidcCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Issuer URL", "Additional Scope"})
	for _, conn := range c.connectors {
		t.AddRow([]string{
			conn.GetName(), conn.GetIssuerURL(), strings.Join(conn.GetScope(), ","),
		})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func NewSAMLCollection(connectors []types.SAMLConnector) ResourceCollection {
	return &samlCollection{connectors: connectors}
}

type samlCollection struct {
	connectors []types.SAMLConnector
}

func (c *samlCollection) Resources() (r []types.Resource) {
	for _, resource := range c.connectors {
		r = append(r, resource)
	}
	return r
}

func (c *samlCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "SSO URL"})
	for _, conn := range c.connectors {
		t.AddRow([]string{conn.GetName(), conn.GetSSO()})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func NewGithubCollection(connectors []types.GithubConnector) ResourceCollection {
	return &githubCollection{connectors: connectors}
}

type githubCollection struct {
	connectors []types.GithubConnector
}

func (c *githubCollection) Resources() (r []types.Resource) {
	for _, resource := range c.connectors {
		r = append(r, resource)
	}
	return r
}

func (c *githubCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Teams To Logins"})
	for _, conn := range c.connectors {
		t.AddRow([]string{conn.GetName(), formatTeamsToLogins(
			conn.GetTeamsToLogins())})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func NewConnectorsCollection(oidc, saml, github ResourceCollection) (ResourceCollection, error) {
	return &connectorsCollection{
		oidc:   oidc,
		saml:   saml,
		github: github,
	}, nil
}

type connectorsCollection struct {
	oidc   ResourceCollection
	saml   ResourceCollection
	github ResourceCollection
}

func (c *connectorsCollection) Resources() (r []types.Resource) {
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

func (c *connectorsCollection) WriteText(w io.Writer, verbose bool) error {
	if c.oidc != nil && len(c.oidc.Resources()) > 0 {
		_, err := io.WriteString(w, "\nOIDC:\n")
		if err != nil {
			return trace.Wrap(err)
		}
		err = c.oidc.WriteText(w, verbose)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	if c.saml != nil && len(c.saml.Resources()) > 0 {
		_, err := io.WriteString(w, "\nSAML:\n")
		if err != nil {
			return trace.Wrap(err)
		}
		err = c.saml.WriteText(w, verbose)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	if c.github != nil && len(c.github.Resources()) > 0 {
		_, err := io.WriteString(w, "\nGitHub:\n")
		if err != nil {
			return trace.Wrap(err)
		}
		err = c.github.WriteText(w, verbose)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}
