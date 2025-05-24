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
	"encoding/json"
	"fmt"
	"os"

	"github.com/gravitational/trace"
	"golang.org/x/oauth2"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/utils/prompt"
	"github.com/gravitational/teleport/lib/client/sso"
	"github.com/gravitational/teleport/lib/utils"
)

// ssoRedirectorConfig returns a standard configured sso redirector for login.
// A display name for the SSO connector can optionally be provided for minor UI improvements.
func (tc *TeleportClient) ssoRedirectorConfig(ctx context.Context, connectorDisplayName string) (sso.RedirectorConfig, error) {
	if tc.CallbackAddr != "" && !utils.AsBool(os.Getenv("TELEPORT_LOGIN_SKIP_REMOTE_HOST_WARNING")) {
		const callbackPrompt = "Logging in from a remote host means that credentials will be stored on " +
			"the remote host. Make sure that you trust the provided callback host " +
			"(%v) and that it resolves to the provided bind addr (%v). Continue?"
		ok, err := prompt.Confirmation(ctx, os.Stderr, prompt.NewContextReader(os.Stdin),
			fmt.Sprintf(callbackPrompt, tc.CallbackAddr, tc.BindAddr),
		)
		if err != nil {
			return sso.RedirectorConfig{}, trace.Wrap(err)
		}
		if !ok {
			return sso.RedirectorConfig{}, trace.BadParameter("Login canceled.")
		}
	}

	return sso.RedirectorConfig{
		ProxyAddr:            tc.WebProxyAddr,
		BindAddr:             tc.BindAddr,
		CallbackAddr:         tc.CallbackAddr,
		Browser:              tc.Browser,
		PrivateKeyPolicy:     tc.PrivateKeyPolicy,
		ConnectorDisplayName: connectorDisplayName,
	}, nil
}

func (tc *TeleportClient) ssoLoginInitFn(keyRing *KeyRing, connectorID, connectorType string) sso.CeremonyInit {
	return func(ctx context.Context, clientCallbackURL string) (redirectURL string, err error) {
		redirectURL, _ /* postform */, err = tc.loginInitFn(ctx, keyRing, clientCallbackURL, connectorID, connectorType)
		if err != nil {
			return "", trace.Wrap(err)
		}
		return redirectURL, nil
	}
}

func (tc *TeleportClient) loginInitFn(ctx context.Context, keyRing *KeyRing, clientCallbackURL string, connectorID, connectorType string) (redirectURL string, postForm string, err error) {
	sshLogin, err := tc.NewSSHLogin(keyRing)
	if err != nil {
		return "", "", trace.Wrap(err)
	}

	codeVerifier := oauth2.GenerateVerifier()

	// initiate SSO login through the Proxy.
	req := SSOLoginConsoleReq{
		RedirectURL: clientCallbackURL,
		UserPublicKeys: UserPublicKeys{
			SSHPubKey:               sshLogin.SSHPubKey,
			TLSPubKey:               sshLogin.TLSPubKey,
			SSHAttestationStatement: sshLogin.SSHAttestationStatement,
			TLSAttestationStatement: sshLogin.TLSAttestationStatement,
		},
		CertTTL:           sshLogin.TTL,
		ConnectorID:       connectorID,
		Compatibility:     sshLogin.Compatibility,
		RouteToCluster:    sshLogin.RouteToCluster,
		KubernetesCluster: sshLogin.KubernetesCluster,
		PKCEVerifier:      codeVerifier,
		ClientVersion:     teleport.Version,
	}

	clt, _, err := initClient(sshLogin.ProxyAddr, sshLogin.Insecure, sshLogin.Pool, sshLogin.ExtraHeaders)
	if err != nil {
		return "", "", trace.Wrap(err)
	}

	out, err := clt.PostJSON(ctx, clt.Endpoint("webapi", connectorType, "login", "console"), req)
	if err != nil {
		return "", "", trace.Wrap(err)
	}

	var re SSOLoginConsoleResponse
	if err := json.Unmarshal(out.Bytes(), &re); err != nil {
		return "", "", trace.Wrap(err)
	}

	return re.RedirectURL, re.PostForm, nil
}

func (tc *TeleportClient) samlSSOLoginInitFn(keyRing *KeyRing, connectorID, connectorType string) sso.SAMLCeremonyInit {
	return func(ctx context.Context, clientCallbackURL string) (redirectURL string, postForm string, err error) {
		return tc.loginInitFn(ctx, keyRing, clientCallbackURL, connectorID, connectorType)
	}
}
