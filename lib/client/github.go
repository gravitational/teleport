/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package client

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client/sso"
)

// ReissueWithGitHubOAuth starts a GitHub OAuth flow for an logged-in user.
// The flow does not use regular SSO connectors for logins. Instead, a temporary
// connector will be created from the GitHub integration associated with the Git
// server of the provided GitHub organization.
// TODO(greedy52) preserve access request IDs throughout this flow.
func (tc *TeleportClient) ReissueWithGitHubOAuth(ctx context.Context, githubOrg string) error {
	keyRing, err := tc.localAgent.GetKeyRing(tc.SiteName, WithSSHCerts{})
	if err != nil {
		return trace.Wrap(err)
	}

	rdConfig, err := tc.ssoRedirectorConfig(ctx, "" /* display name is optional */)
	if err != nil {
		return trace.Wrap(err)
	}

	rd, err := sso.NewRedirector(rdConfig)
	if err != nil {
		return trace.Wrap(err)
	}
	defer rd.Close()

	ssoCeremony := sso.NewCLICeremony(rd, tc.loggedInUserGitHubOAuthInitFunc(keyRing, githubOrg))
	resp, err := ssoCeremony.Run(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	// Treat it as reissuing an SSH cert.
	keyRing.ClusterName = tc.SiteName
	keyRing.Cert = resp.Cert
	return trace.Wrap(tc.localAgent.AddKeyRing(keyRing))
}

func (tc *TeleportClient) loggedInUserGitHubOAuthInitFunc(keyRing *KeyRing, org string) sso.CeremonyInit {
	return func(ctx context.Context, clientCallbackURL string) (redirectURL string, err error) {
		request, err := tc.makeGitHubAuthRequest(keyRing, clientCallbackURL)
		if err != nil {
			return "", trace.Wrap(err)
		}

		clusterClient, err := tc.ConnectToCluster(ctx)
		if err != nil {
			return "", trace.Wrap(err)
		}
		defer clusterClient.Close()

		rootClient, err := clusterClient.ConnectToRootCluster(ctx)
		if err != nil {
			return "", trace.Wrap(err)
		}
		defer rootClient.Close()

		resp, err := rootClient.GitServerClient().CreateGitHubAuthRequest(ctx, request, org)
		if err != nil {
			return "", trace.Wrap(err)
		}
		return resp.RedirectURL, nil
	}
}

func (tc *TeleportClient) makeGitHubAuthRequest(keyRing *KeyRing, clientCallbackURL string) (*types.GithubAuthRequest, error) {
	tlsPub, err := keyRing.TLSPrivateKey.MarshalTLSPublicKey()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &types.GithubAuthRequest{
		ClientRedirectURL:       clientCallbackURL,
		KubernetesCluster:       tc.KubernetesCluster,
		SshPublicKey:            keyRing.SSHPrivateKey.MarshalSSHPublicKey(),
		TlsPublicKey:            tlsPub,
		SshAttestationStatement: keyRing.SSHPrivateKey.GetAttestationStatement().ToProto(),
		TlsAttestationStatement: keyRing.TLSPrivateKey.GetAttestationStatement().ToProto(),
		Compatibility:           tc.CertificateFormat,
	}, nil
}
