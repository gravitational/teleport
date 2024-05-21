/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package tester

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

// SSOTestCommand implements common.CLICommand interface
type SSOTestCommand struct {
	config *servicecfg.Config

	ssoTestCmd *kingpin.CmdClause

	// connectorFileName points at file name with sso connector definition
	connectorFileName string

	// Handlers is a mapping between auth kind and appropriate handling function
	Handlers map[string]func(c *authclient.Client, connBytes []byte) (*AuthRequestInfo, error)
	// GetDiagInfoFields provides auth kind-specific diagnostic info fields.
	GetDiagInfoFields map[string]func(diag *types.SSODiagnosticInfo, debug bool) []string
	// Browser to use in login flow.
	Browser string
}

// Initialize allows a caller-defined command to plug itself into CLI
// argument parsing
func (cmd *SSOTestCommand) Initialize(app *kingpin.Application, cfg *servicecfg.Config) {
	cmd.config = cfg

	sso := app.GetCommand("sso")
	cmd.ssoTestCmd = sso.Command("test", "Perform end-to-end test of SSO flow using provided auth connector definition.")
	cmd.ssoTestCmd.Flag("browser", "Set to 'none' to suppress browser opening on login.").StringVar(&cmd.Browser)
	cmd.ssoTestCmd.Arg("filename", "Connector resource definition filename. Empty for stdin.").StringVar(&cmd.connectorFileName)
	cmd.ssoTestCmd.Alias(`
Examples:

  Test the auth connector from connector.yaml:

  > tctl sso test connector.yaml

  The command is designed to be used in conjunction with "tctl sso configure" family of commands:

  > tctl sso configure github ... | tctl sso test

  The pipeline may also utilize "tee" to capture the connector generated with "tctl sso configure".

  > tctl sso configure github ... | tee connector.yaml | tctl sso test`)

	cmd.Handlers = map[string]func(c *authclient.Client, connBytes []byte) (*AuthRequestInfo, error){
		types.KindGithubConnector: handleGithubConnector,
	}

	cmd.GetDiagInfoFields = map[string]func(diag *types.SSODiagnosticInfo, debug bool) []string{
		types.KindGithubConnector: getGithubDiagInfoFields,
	}
}

func (cmd *SSOTestCommand) getSupportedKinds() []string {
	var kinds []string

	for kind := range cmd.Handlers {
		kinds = append(kinds, kind)
	}

	return kinds
}

func (cmd *SSOTestCommand) ssoTestCommand(ctx context.Context, c *authclient.Client) error {
	reader := os.Stdin
	if cmd.connectorFileName != "" {
		f, err := utils.OpenFileAllowingUnsafeLinks(cmd.connectorFileName)
		if err != nil {
			return trace.Wrap(err, "could not open connector spec file %v", cmd.connectorFileName)
		}
		defer f.Close()
		reader = f
	}

	decoder := kyaml.NewYAMLOrJSONDecoder(reader, defaults.LookaheadBufSize)
	for {
		var raw services.UnknownResource
		err := decoder.Decode(&raw)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return trace.Wrap(err, "Unable to load resource. Make sure the file is in correct format.")
		}

		handler, ok := cmd.Handlers[raw.Kind]
		if !ok {
			return trace.BadParameter("Resources of type %q are not supported. Supported kinds: %v", raw.Kind, cmd.getSupportedKinds())
		}

		requestInfo, err := handler(c, raw.Raw)
		if err != nil {
			return trace.Wrap(err)
		}

		// note: loginErr is processed further down.
		loginResponse, loginErr := cmd.runSSOLoginFlow(ctx, raw.Kind, c, requestInfo.Config)

		if requestInfo.RequestCreateErr != nil {
			return trace.BadParameter("Failed to create auth request. Check the auth connector definition for errors. Error: %v", requestInfo.RequestCreateErr)
		}

		info, infoErr := c.GetSSODiagnosticInfo(ctx, raw.Kind, requestInfo.RequestID)

		err = cmd.reportLoginResult(raw.Kind, info, infoErr, loginResponse, loginErr)
		if err != nil {
			return trace.Wrap(err)
		}
	}
}

// TryRun is executed after the CLI parsing is done. The command must
// determine if selectedCommand belongs to it and return match=true
func (cmd *SSOTestCommand) TryRun(ctx context.Context, selectedCommand string, c *authclient.Client) (match bool, err error) {
	if selectedCommand == cmd.ssoTestCmd.FullCommand() {
		return true, cmd.ssoTestCommand(ctx, c)
	}
	return false, nil
}

// AuthRequestInfo is helper type, useful for tying together test handlers of different auth types.
type AuthRequestInfo struct {
	// Config holds *client.RedirectorConfig used for SSO redirect.
	Config *client.RedirectorConfig
	// RequestID is ID of auth request created for SSO test.
	RequestID string
	// RequestCreateErr holds an error in case auth request creation failed.
	RequestCreateErr error
}

func (cmd *SSOTestCommand) runSSOLoginFlow(ctx context.Context, protocol string, c *authclient.Client, config *client.RedirectorConfig) (*authclient.SSHLoginResponse, error) {
	key, err := client.GenerateRSAKey()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	proxies, err := c.GetProxies()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(proxies) == 0 {
		return nil, trace.BadParameter("cluster has no proxies.")
	}

	cfg := client.MakeDefaultConfig()
	cfg.WebProxyAddr = proxies[0].GetPublicAddr()
	cfg.Browser = cmd.Browser

	tc, err := client.NewClient(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return client.SSHAgentSSOLogin(ctx, client.SSHLoginSSO{
		SSHLogin: client.SSHLogin{
			ProxyAddr:         tc.WebProxyAddr,
			PubKey:            key.MarshalSSHPublicKey(),
			TTL:               tc.KeyTTL,
			Insecure:          tc.InsecureSkipVerify,
			Pool:              nil,
			Compatibility:     tc.CertificateFormat,
			RouteToCluster:    tc.SiteName,
			KubernetesCluster: tc.KubernetesCluster,
		},
		ConnectorID: "-sso-test",
		Protocol:    protocol,
		BindAddr:    tc.BindAddr,
		Browser:     tc.Browser,
	}, config)
}

// GetDiagMessage is helper function for preparing message set to be shown to user.
func GetDiagMessage(present bool, show bool, msg string) string {
	if present && show {
		return msg
	}
	return ""
}

func (cmd *SSOTestCommand) reportLoginResult(authKind string, diag *types.SSODiagnosticInfo, infoErr error, loginResponse *authclient.SSHLoginResponse, loginErr error) (errResult error) {
	success := diag != nil && diag.Success

	// check for errors
	if loginErr != nil || infoErr != nil {
		success = false
	}

	if success {
		fmt.Printf("Success! Logged in as: %v\n", loginResponse.Username)
	} else {
		fmt.Printf("Failure!\n")
		errResult = trace.Errorf("SSO flow failed.")

		if infoErr != nil {
			fmt.Printf("No diagnostic info found. Most likely cause: the request timed out or callback configuration is incorrect. Ensure that user logs within alloted time and IdP configuration is correct.\n Error details: %v\n", trace.UserMessage(infoErr))
			errResult = trace.Wrap(infoErr, "SSO flow failed.")
		}

		if loginErr != nil {
			fmt.Printf("Login error: %v\n", trace.UserMessage(loginErr))
			errResult = trace.Wrap(loginErr, "SSO flow failed.")
		}
	}

	// finish early if there is no diag info to show.
	if diag == nil {
		return errResult
	}

	cmd.printDiagnosticInfo(authKind, diag, loginErr)

	return errResult
}

func (cmd *SSOTestCommand) printDiagnosticInfo(authKind string, diag *types.SSODiagnosticInfo, loginErr error) {
	fields := []string{
		// common fields across auth connector types.
		GetDiagMessage(
			diag.Error != "",
			cmd.config.Debug || loginErr == nil,
			FormatString("Original error", diag.Error)),
		GetDiagMessage(
			diag.CreateUserParams != nil,
			true,
			formatUserDetails("Authentication details", diag.CreateUserParams)),
	}

	// enrich the fields with auth-specific fields
	if getFields, ok := cmd.GetDiagInfoFields[authKind]; ok {
		fields = append(fields, getFields(diag, cmd.config.Debug)...)
	}

	// raw data - debug. we want this field last.
	fields = append(fields, GetDiagMessage(true, cmd.config.Debug, FormatJSON("Raw data", diag)))

	const termWidth = 80

	for _, msg := range fields {
		if msg != "" {
			fmt.Println(strings.Repeat("-", termWidth))
			fmt.Println(msg)
		}
	}

	if !cmd.config.Debug {
		fmt.Println(strings.Repeat("-", termWidth))
		fmt.Println("For more details repeat the command with --debug flag.")
	}

	fmt.Println()
}
