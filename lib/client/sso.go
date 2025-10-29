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
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/gravitational/trace"
	"golang.org/x/oauth2"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/utils/prompt"
	"github.com/gravitational/teleport/lib/auth/authclient"
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

func (tc *TeleportClient) oidcDirectPKCELoginInitFn(keyRing *KeyRing, connectorID string) sso.CeremonyInit {
	return func(ctx context.Context, clientCallbackURL string) (redirectURL string, err error) {
		sshLogin, err := tc.NewSSHLogin(keyRing)
		if err != nil {
			return "", trace.Wrap(err)
		}

		codeVerifier := oauth2.GenerateVerifier()

		callbackURL, err := url.Parse(clientCallbackURL)
		if err != nil {
			return "", trace.Wrap(err)
		}
		query := callbackURL.Query()
		query.Del("secret_key")
		callbackURL.RawQuery = query.Encode()
		cleanCallbackURL := callbackURL.String()

		clt, _, err := initClient(sshLogin.ProxyAddr, sshLogin.Insecure, sshLogin.Pool, sshLogin.ExtraHeaders)
		if err != nil {
			return "", trace.Wrap(err)
		}

		configReq := authclient.OIDCConnectorConfigRequest{
			ConnectorID:   connectorID,
			RedirectURL:   cleanCallbackURL,
			ClientLoginIP: "",
			ClientVersion: teleport.Version,
		}

		out, err := clt.PostJSON(ctx, clt.Endpoint("webapi", "oidc", "config"), configReq)
		if err != nil {
			return "", trace.Wrap(err)
		}

		var configResp authclient.OIDCConnectorConfigResponse
		if err := json.Unmarshal(out.Bytes(), &configResp); err != nil {
			return "", trace.Wrap(err)
		}

		tc.storeOIDCDirectFlowState(configResp.StateToken, &oidcDirectFlowState{
			StateToken:   configResp.StateToken,
			CodeVerifier: codeVerifier,
			ConnectorID:  connectorID,
			SSHLogin:     &sshLogin,
		})

		authURL, err := buildOIDCAuthURL(configResp, codeVerifier)
		if err != nil {
			return "", trace.Wrap(err)
		}

		return authURL, nil
	}
}

func buildOIDCAuthURL(config authclient.OIDCConnectorConfigResponse, codeVerifier string) (string, error) {
	authURL, err := url.Parse(config.AuthorizationEndpoint)
	if err != nil {
		return "", trace.Wrap(err)
	}

	codeChallenge := oauth2.S256ChallengeFromVerifier(codeVerifier)

	params := url.Values{
		"client_id":             {config.ClientID},
		"redirect_uri":          {config.RedirectURL},
		"response_type":         {"code"},
		"scope":                 {strings.Join(config.Scopes, " ")},
		"state":                 {config.StateToken},
		"code_challenge":        {codeChallenge},
		"code_challenge_method": {"S256"},
	}

	if config.Prompt != "" {
		params.Set("prompt", config.Prompt)
	}

	if config.ACRValues != "" {
		params.Set("acr_values", config.ACRValues)
	}

	if config.MaxAge > 0 {
		params.Set("max_age", strconv.FormatInt(config.MaxAge, 10))
	}

	authURL.RawQuery = params.Encode()
	return authURL.String(), nil
}

type oidcDirectFlowState struct {
	StateToken   string
	CodeVerifier string
	ConnectorID  string
	SSHLogin     *SSHLogin
}

func (tc *TeleportClient) storeOIDCDirectFlowState(stateToken string, state *oidcDirectFlowState) {
	tc.oidcStatesMu.Lock()
	defer tc.oidcStatesMu.Unlock()
	tc.oidcDirectFlowStates[stateToken] = state
}

func (tc *TeleportClient) getOIDCDirectFlowState(stateToken string) (*oidcDirectFlowState, error) {
	tc.oidcStatesMu.Lock()
	defer tc.oidcStatesMu.Unlock()
	state, ok := tc.oidcDirectFlowStates[stateToken]
	if !ok {
		return nil, trace.NotFound("OIDC direct flow state not found")
	}
	return state, nil
}

func (tc *TeleportClient) deleteOIDCDirectFlowState(stateToken string) {
	tc.oidcStatesMu.Lock()
	defer tc.oidcStatesMu.Unlock()
	delete(tc.oidcDirectFlowStates, stateToken)
}

// oidcDirectPKCECallbackFn returns a callback function for direct PKCE flow
// that validates the authorization code with the proxy.
func (tc *TeleportClient) oidcDirectPKCECallbackFn(keyRing *KeyRing) func(ctx context.Context, params url.Values) (*authclient.SSHLoginResponse, error) {
	return func(ctx context.Context, params url.Values) (*authclient.SSHLoginResponse, error) {
		code := params.Get("code")
		if code == "" {
			return nil, trace.BadParameter("missing authorization code from IdP")
		}

		stateToken := params.Get("state")
		if stateToken == "" {
			return nil, trace.BadParameter("missing state token from IdP")
		}

		if errParam := params.Get("error"); errParam != "" {
			errDesc := params.Get("error_description")
			return nil, trace.Errorf("IdP returned error: %v [%v]", errDesc, errParam)
		}

		flowState, err := tc.getOIDCDirectFlowState(stateToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		defer tc.deleteOIDCDirectFlowState(stateToken)

		clt, _, err := initClient(flowState.SSHLogin.ProxyAddr, flowState.SSHLogin.Insecure, flowState.SSHLogin.Pool, flowState.SSHLogin.ExtraHeaders)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		validateReq := authclient.OIDCAuthCodeRequest{
			StateToken:              stateToken,
			Code:                    code,
			CodeVerifier:            flowState.CodeVerifier,
			ConnectorID:             flowState.ConnectorID,
			SSHPubKey:               flowState.SSHLogin.SSHPubKey,
			TLSPubKey:               flowState.SSHLogin.TLSPubKey,
			SSHAttestationStatement: flowState.SSHLogin.SSHAttestationStatement,
			TLSAttestationStatement: flowState.SSHLogin.TLSAttestationStatement,
			CertTTL:                 flowState.SSHLogin.TTL,
			Compatibility:           flowState.SSHLogin.Compatibility,
			RouteToCluster:          flowState.SSHLogin.RouteToCluster,
			KubernetesCluster:       flowState.SSHLogin.KubernetesCluster,
			ClientVersion:           teleport.Version,
		}

		out, err := clt.PostJSON(ctx, clt.Endpoint("webapi", "oidc", "validate"), validateReq)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		var loginResp authclient.SSHLoginResponse
		if err := json.Unmarshal(out.Bytes(), &loginResp); err != nil {
			return nil, trace.Wrap(err)
		}

		return &loginResp, nil
	}
}
