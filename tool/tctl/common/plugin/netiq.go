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

	"github.com/alecthomas/kingpin/v2"
	"github.com/fatih/color"
	"github.com/gravitational/trace"

	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
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
This user must have the necessary permissions to access the IDM API and retrieve users, groups, roles 
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

func (p *PluginsCommand) netIQSetupGuide() (netIQSettings, error) {
	settings := netIQSettings{}
	var err error

	settings.ospURL, err = promptForURL(os.Stdout, netIQStep1Template, "Please enter the IDM OSP address", p.install.netIQ.insecureSkipVerify, checkNetIQOSPAddress)
	if err != nil {
		return netIQSettings{}, trace.Wrap(err)
	}

	settings.apiURL, err = promptForURL(os.Stdout, netIQStep2Template, "Please enter the IDM API address: ", p.install.netIQ.insecureSkipVerify, checkNetIQAPIAddress)
	if err != nil {
		return netIQSettings{}, trace.Wrap(err)
	}

	if err := promptForContinue(os.Stdout, netIQStep3Template); err != nil {
		return netIQSettings{}, err
	}

	settings.oAuthClientID, err = promptForInput(os.Stdout, netIQStep4Template, "Enter the OAuth ClientID: ")
	if err != nil {
		return netIQSettings{}, trace.Wrap(err)
	}

	settings.oAuthClientSecret, err = promptForInput(os.Stdout, "", "Enter the OAuth Client Secret: ")
	if err != nil {
		return netIQSettings{}, trace.Wrap(err)
	}

	settings.identityVaultUser, err = promptForInput(os.Stdout, netIQStep5Template, "Enter the IDM User: ")
	if err != nil {
		return netIQSettings{}, trace.Wrap(err)
	}

	settings.identityVaultPassword, err = promptForInput(os.Stdout, "", "Enter the IDM User Password: ")
	if err != nil {
		return netIQSettings{}, trace.Wrap(err)
	}

	settings.insecureSkipVerify = p.install.netIQ.insecureSkipVerify
	return settings, nil
}

func (p *PluginsCommand) InstallNetIQ(ctx context.Context, args installPluginArgs) error {
	settings, err := p.netIQSetupGuide()
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
			"netiq/org": settings.apiURL,
		},
	}

	if _, err = args.plugins.CreatePlugin(ctx, createPluginRequest); err != nil {
		return trace.Wrap(err, "failed to create NetIQ plugin")
	}

	fmt.Println("NetIQ plugin has been successfully installed.")

	return nil
}

func promptForURL(out io.Writer, template, prompt string, insecureSkipVerify bool, checkFunc func(string, bool) error) (string, error) {
	fmt.Fprint(out, template)
	return readData(os.Stdin, out, prompt, func(input string) bool {
		_, err := url.Parse(input)
		if err != nil {
			fmt.Fprintf(out, "Invalid URL: %v\n", err)
			return false
		}
		if err := checkFunc(input, insecureSkipVerify); err != nil {
			fmt.Fprintf(out, "Invalid address: %v\n", err)
			return false
		}
		return true
	}, "Invalid input.")
}

func promptForContinue(out io.Writer, template string) error {
	fmt.Fprint(out, template)
	op, err := readData(os.Stdin, out, "Type 'continue' to proceed, 'exit' to quit: ", func(input string) bool {
		return input == "continue" || input == "exit"
	}, "Invalid input. Please enter 'continue' or 'exit'.")
	if err != nil {
		return trace.Wrap(err)
	}
	if op == "exit" {
		return errCancel
	}
	return nil
}

func promptForInput(out io.Writer, template, prompt string) (string, error) {
	if template != "" {
		fmt.Fprint(out, template)
	}
	return readData(os.Stdin, out, prompt, func(s string) bool {
		return s != ""
	}, "Invalid input.")
}

func checkNetIQOSPAddress(ospURL string, insecureSkipVerify bool) error {
	return checkNetIQAddress(ospURL, "/a/idm/auth/oauth2/.well-known/openid-configuration", insecureSkipVerify)
}

func checkNetIQAPIAddress(apiAddr string, insecureSkipVerify bool) error {
	return checkNetIQAddress(apiAddr, "rest/access/info/version", insecureSkipVerify)
}

func checkNetIQAddress(addr, path string, insecureSkipVerify bool) error {
	u, err := url.Parse(addr)
	if err != nil {
		return trace.Wrap(err, "invalid address")
	}
	u = u.JoinPath(path)
	rsp, err := doRequest(u.String(), http.MethodGet, insecureSkipVerify)
	if err != nil {
		return trace.Wrap(err, "failed to check address")
	}
	defer rsp.Body.Close()
	if path == "rest/access/info/version" && rsp.StatusCode != http.StatusUnauthorized {
		return trace.BadParameter("invalid API address")
	}
	if path != "rest/access/info/version" {
		type openIDConfig struct {
			Token string `json:"token_endpoint"`
		}
		out := openIDConfig{}
		if err := json.NewDecoder(rsp.Body).Decode(&out); err != nil || out.Token == "" {
			return trace.BadParameter("invalid OSP address")
		}
	}
	return nil
}

func doRequest(url, method string, insecureSkipVerify bool) (*http.Response, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Teleport")
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: insecureSkipVerify,
			},
		},
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

func buildOAuthCredentials(clientID, clientSecret string) *types.PluginStaticCredentialsV1 {
	return &types.PluginStaticCredentialsV1{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name:   types.PluginTypeNetIQ + "-oauth",
				Labels: map[string]string{"netiq/purpose": "oauth-client-id-secret"},
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
				Name:   types.PluginTypeNetIQ + "-basic-auth",
				Labels: map[string]string{"netiq/purpose": "oauth-client-id-secret"},
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
