package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/kingpin"
	"github.com/gravitational/trace"

	"github.com/ghodss/yaml"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"
)

// SSOTestCommand implements common.CLICommand interface
type SSOTestCommand struct {
	config *service.Config

	ssoTestCmd *kingpin.CmdClause

	// connectorFileName points at file name with sso connector definition
	connectorFileName string
}

// Initialize allows a caller-defined command to plug itself into CLI
// argument parsing
func (cmd *SSOTestCommand) Initialize(app *kingpin.Application, cfg *service.Config) {
	cmd.config = cfg

	sso := app.Command("sso", "SSO operations")
	cmd.ssoTestCmd = sso.Command("test", "Test SSO auth connector")
	cmd.ssoTestCmd.Arg("filename", "Connector resource definition filename, empty for stdin").StringVar(&cmd.connectorFileName)
}

func (cmd *SSOTestCommand) ssoTestCommand(c auth.ClientI) error {
	reader := os.Stdin
	if cmd.connectorFileName != "" {
		f, err := utils.OpenFile(cmd.connectorFileName)
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
			if err == io.EOF {
				return nil
			}
			return trace.Wrap(err, "Unable to load resource. Make sure the file is in correct format.")
		}

		var requestInfo *authRequestInfo

		switch raw.Kind {
		case types.KindSAMLConnector:
			conn, err := services.UnmarshalSAMLConnector(raw.Raw)
			if err != nil {
				return trace.Wrap(err, "Unable to load SAML connector. Correct the definition and try again.")
			}
			requestInfo, err = cmd.samlTest(c, conn)
			if err != nil {
				return trace.Wrap(err)
			}

		case types.KindOIDCConnector:
			return trace.NotImplemented("not yet supported: %v", raw.Kind)

		case types.KindGithubConnector:
			return trace.NotImplemented("not yet supported: %v", raw.Kind)

		default:
			return trace.BadParameter("Resources of type %q are not supported. Supported kinds: %q, %q, %q.", raw.Kind, types.KindGithubConnector, types.KindSAMLConnector, types.KindOIDCConnector)
		}

		// note: loginErr is processed further down.
		loginResponse, loginErr := cmd.runSSOLoginFlow(context.TODO(), raw.Kind, c, requestInfo.config)

		if requestInfo.requestCreateErr != nil {
			return trace.BadParameter("Failed to create auth request. Check the auth connector definition for errors. Error: %v", requestInfo.requestCreateErr)
		}

		info, infoErr := c.GetSSODiagnosticInfo(context.TODO(), raw.Kind, requestInfo.getRequestID())

		err = cmd.reportLoginResult(info, infoErr, loginResponse, loginErr)
		if err != nil {
			return trace.Wrap(err)
		}
	}
}

// TryRun is executed after the CLI parsing is done. The command must
// determine if selectedCommand belongs to it and return match=true
func (cmd *SSOTestCommand) TryRun(selectedCommand string, c auth.ClientI) (match bool, err error) {
	if selectedCommand == cmd.ssoTestCmd.FullCommand() {
		return true, cmd.ssoTestCommand(c)
	}
	return false, nil
}

type authRequestInfo struct {
	config           *client.RedirectorConfig
	SAMLRequest      *services.SAMLAuthRequest
	requestCreateErr error
}

func (info *authRequestInfo) getRequestID() string {
	if info.SAMLRequest != nil {
		return info.SAMLRequest.ID
	}

	return ""
}

func (cmd *SSOTestCommand) samlTest(c auth.ClientI, samlConnector types.SAMLConnector) (*authRequestInfo, error) {
	// get connector spec
	var spec types.SAMLConnectorSpecV2
	switch samlConnector := samlConnector.(type) {
	case *types.SAMLConnectorV2:
		spec = samlConnector.Spec
	default:
		return nil, trace.BadParameter("Unrecognized SAML connector version: %T. Provide supported connector version.", samlConnector)
	}

	requestInfo := &authRequestInfo{}

	makeRequest := func(req client.SSOLoginConsoleReq) (*client.SSOLoginConsoleResponse, error) {
		samlRequest := services.SAMLAuthRequest{
			ConnectorID:       req.ConnectorID + "-" + samlConnector.GetName(),
			Type:              constants.SAML,
			CheckUser:         false,
			PublicKey:         req.PublicKey,
			CertTTL:           defaults.SAMLAuthRequestTTL,
			CreateWebSession:  false,
			ClientRedirectURL: req.RedirectURL,
			RouteToCluster:    req.RouteToCluster,
			SSOTestFlow:       true,
			ConnectorSpec:     &spec,
		}

		request, err := c.CreateSAMLAuthRequest(samlRequest)

		requestInfo.SAMLRequest = request
		requestInfo.requestCreateErr = err

		if err != nil {
			return nil, trace.Wrap(err)
		}

		return &client.SSOLoginConsoleResponse{RedirectURL: request.RedirectURL}, nil
	}

	requestInfo.config = &client.RedirectorConfig{SSOLoginConsoleRequestFn: makeRequest}
	return requestInfo, nil
}

func (cmd *SSOTestCommand) runSSOLoginFlow(ctx context.Context, protocol string, c auth.ClientI, config *client.RedirectorConfig) (*auth.SSHLoginResponse, error) {
	key, err := client.NewKey()
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

	tc, err := client.NewClient(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return client.SSHAgentSSOLogin(ctx, client.SSHLoginSSO{
		SSHLogin: client.SSHLogin{
			ProxyAddr:         tc.WebProxyAddr,
			PubKey:            key.Pub,
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

func formatString(description string, msg string) string {
	return fmt.Sprintf("%v:\n%v\n", description, msg)
}

func formatYAML(description string, object interface{}) string {
	output, err := yaml.Marshal(object)
	if err != nil {
		return formatError(description, err)
	}
	return fmt.Sprintf("%v:\n%v", description, string(output))
}

func formatJSON(description string, object interface{}) string {
	output, err := json.MarshalIndent(object, "", "    ")
	if err != nil {
		return formatError(description, err)
	}
	return fmt.Sprintf("%v:\n%v\n", description, string(output))
}

func formatUserDetails(description string, info *types.CreateUserParams) string {
	if info == nil {
		return ""
	}

	// Skip fields: connector_name, session_ttl
	info.ConnectorName = ""
	info.SessionTTL = 0

	output, err := yaml.Marshal(info)
	if err != nil {
		return formatError(description, err)
	}
	return fmt.Sprintf("%v:\n%v", description, indent(string(output), "   "))
}

func formatSSOWarnings(description string, info *types.SSOWarnings) string {
	if info == nil {
		return ""
	}

	if len(info.Warnings) > 0 {
		return fmt.Sprintf("%v: %v. Warnings:\n%v\n", description, info.Message, indent(strings.Join(info.Warnings, "\n"), "- "))
	}

	return fmt.Sprintf("%v: %v\n", description, info.Message)
}

func formatError(fieldDesc string, err error) string {
	return fmt.Sprintf("%v: error rendering field: %v\n", fieldDesc, err)
}

func (cmd *SSOTestCommand) reportLoginResult(diag *types.SSODiagnosticInfo, infoErr error, loginResponse *auth.SSHLoginResponse, loginErr error) (errResult error) {
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

	type field struct {
		present bool
		show    bool
		msg     string
	}

	fields := []field{
		{
			present: diag.Error != "",
			show:    cmd.config.Debug || loginErr == nil,
			msg:     formatString("Original error", diag.Error),
		},
		{
			present: diag.CreateUserParams != nil,
			show:    true,
			msg:     formatUserDetails("Authentication details", diag.CreateUserParams),
		},
		{
			present: diag.SAMLAttributesToRoles != nil,
			show:    true,
			msg:     formatYAML("[SAML] Attributes to roles", diag.SAMLAttributesToRoles),
		},
		{
			present: diag.SAMLAttributesToRolesWarnings != nil,
			show:    true,
			msg:     formatSSOWarnings("[SAML] Attributes mapping warning", diag.SAMLAttributesToRolesWarnings),
		},
		{
			present: diag.SAMLAttributeStatements != nil,
			show:    true,
			msg:     formatYAML("[SAML] Attributes statements", diag.SAMLAttributeStatements),
		},
		{
			present: diag.SAMLAssertionInfo != nil,
			show:    cmd.config.Debug,
			msg:     formatJSON("[SAML] Assertion info", diag.SAMLAssertionInfo),
		},
		{
			present: diag.SAMLTraitsFromAssertions != nil,
			show:    cmd.config.Debug,
			msg:     formatJSON("[SAML] Calculated user traits", diag.SAMLTraitsFromAssertions),
		},
		{
			present: diag.SAMLConnectorTraitMapping != nil,
			show:    cmd.config.Debug,
			msg:     formatYAML("[SAML] Connector trait mapping", diag.SAMLConnectorTraitMapping),
		},
		{
			present: true,
			show:    cmd.config.Debug,
			msg:     formatJSON("Raw data", diag),
		},
	}

	const termWidth = 80

	for _, f := range fields {
		if f.present && f.show {
			fmt.Println(strings.Repeat("-", termWidth))
			fmt.Println(f.msg)
		}
	}

	if !cmd.config.Debug {
		fmt.Println(strings.Repeat("-", termWidth))
		fmt.Println("For more details repeat the command with --debug flag.")
	}

	fmt.Println()

	return errResult
}
