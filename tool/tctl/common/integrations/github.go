/*
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

package integrations

import (
	"context"
	"fmt"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client/sso"
	"github.com/gravitational/teleport/lib/services"
)

type ssoDiagGetter interface {
	GetSSODiagnosticInfo(ctx context.Context, authKind string, authRequestID string) (*types.SSODiagnosticInfo, error)
}

type gitHubAuthRequestCreator interface {
	CreateGitHubAuthRequest(ctx context.Context, req *types.GithubAuthRequest, org string) (*types.GithubAuthRequest, error)
}

// ssoCeremony abstracts the SSO login ceremony for testing.
type ssoCeremony interface {
	Run(ctx context.Context) (*authclient.CLILoginResponse, error)
	StateToken() string
	Close()
}

type testGitHubConfig struct {
	diagGetter  ssoDiagGetter
	integration types.Integration
	ceremony    ssoCeremony
}

func (c *Command) testGitHub(ctx context.Context, cfg testGitHubConfig) (*testGitHubOutput, error) {
	spec := cfg.integration.GetGitHubIntegrationSpec()
	if spec == nil {
		return nil, trace.BadParameter("integration %q has no GitHub spec", cfg.integration.GetName())
	}
	if cfg.ceremony == nil {
		return nil, trace.BadParameter("ceremony is required")
	}
	defer cfg.ceremony.Close()

	if c.testArgs.isTextFormat() {
		fmt.Fprintf(c.Stdout, "Testing GitHub integration %q (org: %s).\n"+
			"No GitHub identity will be saved.\n\n",
			cfg.integration.GetName(), spec.Organization)
	}
	if _, err := cfg.ceremony.Run(ctx); err != nil {
		return nil, trace.Wrap(err, "OAuth flow failed")
	}

	output := &testGitHubOutput{
		Status:      "healthy",
		Integration: cfg.integration.GetName(),
	}

	diag, err := cfg.diagGetter.GetSSODiagnosticInfo(ctx, types.KindGithub, cfg.ceremony.StateToken())
	if err != nil {
		return nil, trace.Wrap(err, "retrieving SSO diagnostic info")
	}
	if diag.GithubClaims == nil || diag.GithubClaims.Username == "" {
		return nil, trace.NotFound("GitHub user info not found in SSO diagnostic info")
	}
	output.GitHubUsername = diag.GithubClaims.Username
	output.GitHubUserID = diag.GithubClaims.UserID

	return output, nil
}

// gitHubSSOCeremony is the production implementation of ssoCeremony.
type gitHubSSOCeremony struct {
	rd         *sso.Redirector
	ceremony   *sso.Ceremony
	stateToken string
}

func (g *gitHubSSOCeremony) Run(ctx context.Context) (*authclient.CLILoginResponse, error) {
	return g.ceremony.Run(ctx)
}

func (g *gitHubSSOCeremony) StateToken() string {
	return g.stateToken
}

func (g *gitHubSSOCeremony) Close() {
	g.rd.Close()
}

func (c *Command) newGitHubSSOCeremony(
	ctx context.Context,
	authRequestCreator gitHubAuthRequestCreator,
	proxyGetter services.ProxyGetter,
	ig types.Integration,
) (*gitHubSSOCeremony, error) {
	proxyAddr, err := getProxyAddr(ctx, proxyGetter)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rd, err := sso.NewRedirector(sso.RedirectorConfig{
		ProxyAddr: proxyAddr,
		Browser:   c.testArgs.browser,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	g := &gitHubSSOCeremony{
		rd: rd,
	}
	g.ceremony = sso.NewCLICeremony(rd, func(ctx context.Context, clientCallbackURL string) (string, error) {
		authReq, err := authRequestCreator.CreateGitHubAuthRequest(ctx, &types.GithubAuthRequest{
			SSOTestFlow:       true,
			ClientRedirectURL: clientCallbackURL,
			Integration:       ig.GetName(),
		}, "" /* org is not used for the test flow */)
		if err != nil {
			return "", trace.Wrap(err)
		}
		g.stateToken = authReq.StateToken
		return authReq.RedirectURL, nil
	})
	return g, nil
}

func getProxyAddr(ctx context.Context, proxyGetter services.ProxyGetter) (string, error) {
	proxies, err := clientutils.CollectWithFallback(ctx,
		proxyGetter.ListProxyServers,
		func(context.Context) ([]types.Server, error) {
			//nolint:staticcheck // TODO DELETE IN 21.0.0
			return proxyGetter.GetProxies()
		},
	)
	if err != nil {
		return "", trace.Wrap(err)
	}
	if len(proxies) == 0 {
		return "", trace.BadParameter("cluster has no proxies")
	}
	return proxies[0].GetPublicAddr(), nil
}

type testGitHubOutput struct {
	Status         string `json:"status"`
	Integration    string `json:"integration_name"`
	GitHubUsername string `json:"github_username,omitempty"`
	GitHubUserID   string `json:"github_user_id,omitempty"`
}

func (o *testGitHubOutput) String() string {
	var sb strings.Builder

	sb.WriteString("GitHub OAuth flow completed:\n\n")
	sb.WriteString("Integration Name:  ")
	sb.WriteString(o.Integration)
	sb.WriteString("\n")
	if o.GitHubUsername != "" {
		sb.WriteString("GitHub Username:   ")
		sb.WriteString(o.GitHubUsername)
		sb.WriteString("\n")
	}
	if o.GitHubUserID != "" {
		sb.WriteString("GitHub User ID:    ")
		sb.WriteString(o.GitHubUserID)
		sb.WriteString("\n")
	}
	sb.WriteString("\n")
	sb.WriteString(bold("GitHub integration is healthy."))
	sb.WriteString("\n")

	return sb.String()
}
