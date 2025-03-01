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

package plugin

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/fatih/color"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api"
	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/prompt"
	"github.com/gravitational/teleport/lib/defaults"
)

var (
	boldGreen          = color.New(color.Bold, color.FgGreen).SprintFunc()
	netIQStep1Template = bold("Step 1: Configure IDM OSP address") + `

Please provide the IDM OSP address to configure the integration.
For example, 'https://idm.example.com/osp' or 'https://osp.idm.example.com'.
`
	netIQStep2Template = bold("Step 2: Configure IDM API address") + `
Please provide the IDM API (IDMProv or IDM Identity Applications) address to configure the integration.
For example, 'https://idm.example.com/IDMProv' or 'https://idmapps.idm.example.com'.
`
	netIQStep3Template = bold("Step 3: Configure IDM OSP OAuth Client") + `

The easiest way to register a new OAuth client with OSP (IDM Authorization Server) is to edit the ` + bold("ism-configuration.properties") + ` file by hand.
The file is located in the ` + bold("osp-path/tomcat/conf/") + ` directory.
To use an OAuth client with OSP, you must configure the following properties:
` + boldGreen(`		com.example.<client-id>.clientID = <client-id>
		com.example.<client-id>.clientPass = <secret>
`) + `

If you prefer to store the client secret in a secure way, you can run the following command:

		` + boldGreen(`java -jar /opt/netiq/idm/apps/tomcat/lib/obscurity-*jar <secret>`) + `

Then, copy the output and paste it into the ` + boldGreen(`com.example.<client-id>.clientPass`) + ` property as shown below:
` + boldGreen(`		com.example.<client-id>.clientID = <client-id>
		com.example.<client-id>.clientPass._attr_obscurity = ENCRYPT
		com.example.<client-id>.clientPass = <encrypted-secret>
`) + `

After configuring the OAuth client, please restart OSP to apply the changes and type 'continue' to proceed.

`
	netIQStep4Template = bold("Step 4: Input Client ID and Client Secret") + `

With the values used in Step 2, please copy and paste the following information:
`
	netIQStep5Template = bold("Step 5: Input IDM User and Password") + `

Please provide the IDM user to configure the integration. 
This user must have permissions to access the IDM API and retrieve users, groups, roles 
and resources. Use the following format: ` + bold("cn=uaadmin,ou=sa,o=data") + `.
`
)

type netIQArgs struct {
	cmd                *kingpin.CmdClause
	insecureSkipVerify bool
}

func (p *PluginsCommand) initInstallNetIQ(parent *kingpin.CmdClause) {
	p.install.netIQ.cmd = parent.Command("netiq", "Install an Access Graph NetIQ integration.")
	cmd := p.install.netIQ.cmd
	cmd.Flag("insecure-skip-verify", "Skip verification of the NetIQ server's SSL certificate.").BoolVar(&p.install.netIQ.insecureSkipVerify)
}

type netIQSettings struct {
	apiURL                string
	ospURL                string
	identityVaultUser     string
	identityVaultPassword string
	oAuthClientID         string
	oAuthClientSecret     string
	insecureSkipVerify    bool
}

func (p *PluginsCommand) netIQSetupGuide(ctx context.Context) (netIQSettings, error) {
	settings := netIQSettings{}
	var err error

	settings.ospURL, err = promptForURL(
		ctx,
		os.Stdout,
		netIQStep1Template,
		"Please enter the IDM OSP address",
		p.install.netIQ.insecureSkipVerify,
		func(ctx context.Context, s *url.URL, b bool) error {
			_, err := checkNetIQOSPAddress(ctx, s.String(), b)
			return trace.Wrap(err)
		})
	if err != nil {
		return netIQSettings{}, trace.Wrap(err)
	}

	settings.apiURL, err = promptForURL(
		ctx,
		os.Stdout,
		netIQStep2Template,
		"Please enter the IDM API address",
		p.install.netIQ.insecureSkipVerify,
		func(ctx context.Context, u *url.URL, b bool) error {
			return checkUnauthenticatedNetIQAPIAddress(ctx, u.String(), b)
		})
	if err != nil {
		return netIQSettings{}, trace.Wrap(err)
	}

	if err := promptForContinue(ctx, os.Stdout, netIQStep3Template); err != nil {
		return netIQSettings{}, err
	}

	settings.oAuthClientID, err = promptForInput(ctx, os.Stdout, netIQStep4Template, "Enter the OAuth ClientID")
	if err != nil {
		return netIQSettings{}, trace.Wrap(err)
	}

	settings.oAuthClientSecret, err = promptForPassword(ctx, os.Stdout, "Enter the OAuth Client Secret")
	if err != nil {
		return netIQSettings{}, trace.Wrap(err)
	}

	settings.identityVaultUser, err = promptForInput(ctx, os.Stdout, netIQStep5Template, "Enter the IDM User")
	if err != nil {
		return netIQSettings{}, trace.Wrap(err)
	}

	settings.identityVaultPassword, err = promptForPassword(ctx, os.Stdout, "Enter the IDM User Password")
	if err != nil {
		return netIQSettings{}, trace.Wrap(err)
	}

	settings.insecureSkipVerify = p.install.netIQ.insecureSkipVerify

	// Validate the NetIQ settings.
	if err := checkAuthenticatedNetIQAPIAddress(ctx, settings); err != nil {
		return netIQSettings{}, trace.Wrap(err)
	}
	return settings, nil
}

func (p *PluginsCommand) InstallNetIQ(ctx context.Context, args installPluginArgs) error {
	settings, err := p.netIQSetupGuide(ctx)
	if err != nil {
		if errors.Is(err, errCancel) {
			return nil
		}
		return trace.Wrap(err)
	}

	plugin, err := createNetIQPlugin(&settings)
	if err != nil {
		return trace.Wrap(err, "failed to create NetIQ plugin")
	}

	creds, err := getNetIQPluginCredentials(settings)
	if err != nil {
		return trace.Wrap(err, "failed to get NetIQ plugin credentials")
	}

	createPluginRequest := &pluginspb.CreatePluginRequest{
		Plugin:                plugin,
		StaticCredentialsList: creds,
		CredentialLabels: map[string]string{
			netIQOrgURLLabel: settings.apiURL,
		},
	}

	if _, err = args.plugins.CreatePlugin(ctx, createPluginRequest); err != nil {
		return trace.Wrap(err, "failed to create NetIQ plugin")
	}

	fmt.Println("NetIQ plugin has been successfully installed.")

	return nil
}

func promptForURL(ctx context.Context, out io.Writer, template, promptT string, insecureSkipVerify bool, checkFunc func(context.Context, *url.URL, bool) error) (string, error) {
	fmt.Fprintf(out, "\n%s", template)

	return prompt.URL(
		ctx,
		out,
		prompt.Stdin(),
		promptT,
		prompt.WithURLValidator(
			func(u *url.URL) error {
				if err := checkFunc(ctx, u, insecureSkipVerify); err != nil {
					fmt.Fprintf(out, "Invalid address: %v\n", err)
					return trace.Wrap(err)
				}
				return nil
			},
		),
	)
}

func promptForContinue(ctx context.Context, out io.Writer, template string) error {
	fmt.Fprintf(out, "\n%s", template)
	op, err := prompt.PickOne(
		ctx,
		out,
		prompt.Stdin(),
		"Type 'continue' to proceed, 'exit' to quit",
		[]string{"continue", "exit"},
	)
	if err != nil {
		return trace.Wrap(err)
	}
	if op == "exit" {
		return errCancel
	}
	return nil
}

func promptForPassword(ctx context.Context, out io.Writer, promptMsg string) (string, error) {
	return prompt.Password(
		ctx, out, prompt.Stdin(),
		promptMsg,
	)
}

func promptForInput(ctx context.Context, out io.Writer, template, promptT string) (string, error) {
	if template != "" {
		fmt.Fprintf(out, "\n%s", template)
	}
	return prompt.Input(ctx, out, prompt.Stdin(), promptT)
}

func checkNetIQOSPAddress(ctx context.Context, ospURL string, insecureSkipVerify bool) (string, error) {
	var tokenAddr string
	err := checkNetIQAddress(
		ctx,
		ospURL,
		"/a/idm/auth/oauth2/.well-known/openid-configuration",
		insecureSkipVerify,
		func(r *http.Response) error {
			type openIDConfig struct {
				Token string `json:"token_endpoint"`
			}
			out := openIDConfig{}
			if err := json.NewDecoder(r.Body).Decode(&out); err != nil || out.Token == "" {
				return trace.BadParameter("invalid OSP address")
			}
			tokenAddr = out.Token
			return nil
		},
	)
	return tokenAddr, trace.Wrap(err)
}

// checkUnauthenticatedNetIQAPIAddress checks if the NetIQ API address is reachable and serves the expected content.
func checkUnauthenticatedNetIQAPIAddress(ctx context.Context, apiAddr string, insecureSkipVerify bool) error {
	return checkNetIQAddress(
		ctx,
		apiAddr,
		"rest/access/info/version",
		insecureSkipVerify,
		// API endpoints are protected so we are ensuring that the endpoint
		// is reachable and doesn't return 404.
		func(r *http.Response) error {
			if r.StatusCode != http.StatusUnauthorized {
				return trace.BadParameter("invalid API address")
			}
			return nil
		},
	)
}

func checkAuthenticatedNetIQAPIAddress(ctx context.Context, data netIQSettings) error {
	tokenUrl, err := checkNetIQOSPAddress(ctx, data.ospURL, data.insecureSkipVerify)
	if err != nil {
		return trace.Wrap(err)
	}

	q := url.Values{}
	q.Add("grant_type", "password")
	q.Add("username", data.identityVaultUser)
	q.Add("password", data.identityVaultPassword)

	// validate API credentials
	rsp, err := doRequest(
		ctx,
		tokenUrl,
		http.MethodPost,
		data.insecureSkipVerify,
		withBody(strings.NewReader(q.Encode())),
		withBasicAuth(data.oAuthClientID, data.oAuthClientSecret),
		withHeader("Content-Type", "application/x-www-form-urlencoded"),
	)

	if err != nil {
		return trace.Wrap(err)
	}

	defer rsp.Body.Close()
	defer io.Copy(io.Discard, rsp.Body)

	// tokenResponse represents the response from the OSP(NetIQ authorization service) token endpoint
	// when requesting an access token.
	type tokenResponse struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
	}
	var token tokenResponse
	if err := json.NewDecoder(rsp.Body).Decode(&token); err != nil {
		return trace.Wrap(err, "failed to decode response")
	}

	return checkNetIQAddress(
		ctx,
		data.apiURL,
		"rest/access/info/version",
		data.insecureSkipVerify,
		// API endpoints are protected so we are ensuring that the endpoint
		// is reachable and doesn't return 404.
		func(r *http.Response) error {
			switch r.StatusCode {
			case http.StatusUnauthorized:
				return trace.BadParameter("invalid API credentials")
			case http.StatusOK:
				return nil
			default:
				return trace.BadParameter("invalid API address")
			}
		},
		withHeader("Authorization", fmt.Sprintf("%s %s", token.TokenType, token.AccessToken)),
	)
}

func checkNetIQAddress(ctx context.Context, addr, path string, insecureSkipVerify bool, validation func(*http.Response) error, opts ...doRequestOptions) error {
	u, err := url.Parse(addr)
	if err != nil {
		return trace.Wrap(err, "invalid address")
	}
	u = u.JoinPath(path)
	rsp, err := doRequest(ctx, u.String(), http.MethodGet, insecureSkipVerify, opts...)
	if err != nil {
		return trace.Wrap(err, "failed to check address")
	}
	defer rsp.Body.Close()
	defer io.Copy(io.Discard, rsp.Body)

	if err := validation(rsp); err != nil {
		return trace.Wrap(err, "failed to validate address")
	}

	return nil
}

type doRequestOptions func(*http.Request) error

func withBasicAuth(user, password string) doRequestOptions {
	return func(req *http.Request) error {
		req.SetBasicAuth(user, password)
		return nil
	}
}

func withBody(body io.Reader) doRequestOptions {
	return func(req *http.Request) error {
		req.Body = io.NopCloser(body)
		return nil
	}
}

func withHeader(key, value string) doRequestOptions {
	return func(req *http.Request) error {
		req.Header.Set(key, value)
		return nil
	}
}

func doRequest(ctx context.Context, url, method string, insecureSkipVerify bool, opts ...doRequestOptions) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", fmt.Sprintf("%s/%s", teleport.ComponentTCTL, api.Version))

	for _, opt := range opts {
		if err := opt(req); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	transport, err := defaults.Transport()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if transport.TLSClientConfig == nil {
		transport.TLSClientConfig = &tls.Config{}
	}
	transport.TLSClientConfig.InsecureSkipVerify = insecureSkipVerify
	client := &http.Client{
		Transport: transport,
	}
	return client.Do(req)
}

func getNetIQPluginCredentials(req netIQSettings) ([]*types.PluginStaticCredentialsV1, error) {
	var out []*types.PluginStaticCredentialsV1
	if req.oAuthClientID != "" {
		out = append(out, buildOAuthCredentials(req.oAuthClientID, req.oAuthClientSecret))
	}
	if req.identityVaultUser != "" {
		out = append(out, buildBasicAuthCredentials(req.identityVaultUser, req.identityVaultPassword))
	}
	return out, nil
}

const (
	// netIQOrgURLLabel is the label for which NetIQ instance object belongs to.
	netIQOrgURLLabel = "netiq/org"
	// credPurposeLabel is the label for the purpose of the credential.
	credPurposeLabel = "netiq/purpose"
	// credPurposeNetIQOauth is the purpose for the NetIQ OAuth client ID and secret.
	credPurposeNetIQOauth = "oauth-client-id-secret"
	// credPurposeNetIQAuth is the purpose for the NetIQ Identity Vault user and password.
	credPurposeNetIQAuth = "netiq-auth"
)

func buildOAuthCredentials(clientID, clientSecret string) *types.PluginStaticCredentialsV1 {
	return &types.PluginStaticCredentialsV1{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name: types.PluginTypeNetIQ + "-oauth",
				Labels: map[string]string{
					credPurposeLabel: credPurposeNetIQOauth,
				},
			},
		},
		Spec: &types.PluginStaticCredentialsSpecV1{
			Credentials: &types.PluginStaticCredentialsSpecV1_OAuthClientSecret{
				OAuthClientSecret: &types.PluginStaticCredentialsOAuthClientSecret{
					ClientId:     clientID,
					ClientSecret: clientSecret,
				},
			},
		},
	}
}

func buildBasicAuthCredentials(user, password string) *types.PluginStaticCredentialsV1 {
	return &types.PluginStaticCredentialsV1{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name: types.PluginTypeNetIQ + "-basic-auth",
				Labels: map[string]string{
					credPurposeLabel: credPurposeNetIQAuth,
				},
			},
		},
		Spec: &types.PluginStaticCredentialsSpecV1{
			Credentials: &types.PluginStaticCredentialsSpecV1_BasicAuth{
				BasicAuth: &types.PluginStaticCredentialsBasicAuth{
					Username: user,
					Password: password,
				},
			},
		},
	}
}

func createNetIQPlugin(params *netIQSettings) (*types.PluginV1, error) {
	return &types.PluginV1{
		SubKind: types.PluginSubkindAccessGraph,
		Metadata: types.Metadata{
			Labels: map[string]string{
				types.TeleportNamespace + "/hosted-plugin": "true",
			},
			Name: types.PluginTypeNetIQ,
		},
		Spec: types.PluginSpecV1{
			Settings: &types.PluginSpecV1_NetIq{
				NetIq: &types.PluginNetIQSettings{
					OauthIssuerEndpoint: params.ospURL,
					ApiEndpoint:         params.apiURL,
					InsecureSkipVerify:  params.insecureSkipVerify,
				},
			},
		},
	}, nil
}
