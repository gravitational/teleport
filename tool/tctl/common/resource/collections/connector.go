package collections

import (
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/trace"
	"io"
	"strings"
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

func NewConnectorsCollection(oidcColl, samlColl, githubColl ResourceCollection) (ResourceCollection, error) {
	oidc, ok := oidcColl.(*oidcCollection)
	if !ok {
		return nil, trace.BadParameter("expected oidc collection, got %T", oidcColl)
	}
	saml, ok := samlColl.(*samlCollection)
	if !ok {
		return nil, trace.BadParameter("expected saml collection, got %T", samlColl)
	}
	github, ok := githubColl.(*githubCollection)
	if !ok {
		return nil, trace.BadParameter("expected github collection, got %T", githubColl)
	}
	return &connectorsCollection{
		oidc:   oidc,
		saml:   saml,
		github: github,
	}, nil
}

type connectorsCollection struct {
	oidc   *oidcCollection
	saml   *samlCollection
	github *githubCollection
}

func (c *connectorsCollection) Resources() (r []types.Resource) {
	if c.oidc != nil {
		for _, resource := range c.oidc.Resources() {
			r = append(r, resource)
		}
	}
	if c.saml != nil {
		for _, resource := range c.saml.Resources() {
			r = append(r, resource)
		}
	}
	if c.github != nil {
		for _, resource := range c.github.Resources() {
			r = append(r, resource)
		}
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
