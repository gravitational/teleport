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

package common

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"slices"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	oidcclient "github.com/zitadel/oidc/v3/pkg/client"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	libclient "github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/teleportassets"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

var mdmTokenAddTemplate = template.Must(
	template.New("mdmTokenAdd").Parse(`The invite token: {{.token}}
This token will expire in {{.minutes}} minutes.

Use this token to add an MDM service to Teleport.

> teleport start \
   --token={{.token}} \{{range .ca_pins}}
   --ca-pin={{.}} \{{end}}
   --config=/path/to/teleport.yaml

`))

// TokensCommand implements `tctl tokens` group of commands
type TokensCommand struct {
	config *servicecfg.Config

	withSecrets bool

	// format is the output format, e.g. text or json
	format string

	// tokenType is the type of token. For example, "trusted_cluster".
	tokenType string

	// Value is the value of the token. Can be used to either act on a
	// token (for example, delete a token) or used to create a token with a
	// specific value.
	value string

	// appName is the name of the application to add.
	appName string

	// appURI is the URI (target address) of the application to add.
	appURI string

	// dbName is the database name to add.
	dbName string
	// dbProtocol is the database protocol.
	dbProtocol string
	// dbURI is the address the database is reachable at.
	dbURI string

	// serviceAccountName is the Kubernetes Service Account the token should allow joining with.
	serviceAccountName string
	// namespace is the Kubernetes namespace the token should allow joining from
	namespace string
	// kubeContext is the kubectl context used to discover the Kubernetes cluster.
	kubeContext string

	kubeName string

	tokenName string
	// botName is the name of the bot the token allow joining as.
	botName string
	// outputPath is the path of the output file containing the Helm values
	outputPath string
	// updateGroup is the name of the update group for version detection and the generated values.yaml
	updateGroup string
	force       bool

	// ttl is how long the token will live for.
	ttl time.Duration

	// labels is optional token labels
	labels string

	// tokenAdd is used to add a token.
	tokenAdd *kingpin.CmdClause

	// tokenDel is used to delete a token.
	tokenDel *kingpin.CmdClause

	// tokenList is used to view all tokens that Teleport knows about.
	tokenList *kingpin.CmdClause

	// tokenKubeOIDC is used to discover the OIDC provider of a Kube cluster and create the corresponding join token.
	tokenKubeOIDC *kingpin.CmdClause

	// stdout allows to switch the standard output source. Used in tests.
	stdout io.Writer
}

// Initialize allows TokenCommand to plug itself into the CLI parser
func (c *TokensCommand) Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, config *servicecfg.Config) {
	c.config = config

	tokens := app.Command("tokens", "List or revoke invitation tokens")

	formats := []string{teleport.Text, teleport.JSON, teleport.YAML}

	// tctl tokens add ..."
	c.tokenAdd = tokens.Command("add", "Create a invitation token.")
	c.tokenAdd.Flag("type", "Type(s) of token to add, e.g. --type=node,app,db,proxy,etc").Required().StringVar(&c.tokenType)
	c.tokenAdd.Flag("value", "Override the default random generated token with a specified value").StringVar(&c.value)
	c.tokenAdd.Flag("labels", "Set token labels, e.g. env=prod,region=us-west").StringVar(&c.labels)
	c.tokenAdd.Flag("ttl", fmt.Sprintf("Set expiration time for token, default is %v minutes",
		int(defaults.ProvisioningTokenTTL/time.Minute))).
		Default(fmt.Sprintf("%v", defaults.ProvisioningTokenTTL)).
		DurationVar(&c.ttl)
	c.tokenAdd.Flag("app-name", "Name of the application to add").Default("example-app").StringVar(&c.appName)
	c.tokenAdd.Flag("app-uri", "URI of the application to add").Default("http://localhost:8080").StringVar(&c.appURI)
	c.tokenAdd.Flag("db-name", "Name of the database to add").StringVar(&c.dbName)
	c.tokenAdd.Flag("db-protocol", fmt.Sprintf("Database protocol to use. Supported are: %v", defaults.DatabaseProtocols)).StringVar(&c.dbProtocol)
	c.tokenAdd.Flag("db-uri", "Address the database is reachable at").StringVar(&c.dbURI)
	c.tokenAdd.Flag("format", "Output format, 'text', 'json', or 'yaml'").EnumVar(&c.format, formats...)

	// "tctl tokens rm ..."
	c.tokenDel = tokens.Command("rm", "Delete/revoke an invitation token.").Alias("del")
	c.tokenDel.Arg("token", "Token to delete").StringVar(&c.value)

	// "tctl tokens ls"
	c.tokenList = tokens.Command("ls", "List node and user invitation tokens.")
	c.tokenList.Flag("format", "Output format, 'text', 'json' or 'yaml'").EnumVar(&c.format, formats...)
	c.tokenList.Flag("with-secrets", "Do not redact join tokens").BoolVar(&c.withSecrets)
	c.tokenList.Flag("labels", labelHelp).StringVar(&c.labels)

	// "tctl tokens configure-kube-oidc ..."
	c.tokenKubeOIDC = tokens.Command("configure-kube-oidc", "Makes Teleport trust the OIDC provider of a Kube cluster, allowing kube workload to join.")
	c.tokenKubeOIDC.Flag("out", "Path of the output file.").Short('o').Default("./values.yaml").StringVar(&c.outputPath)
	c.tokenKubeOIDC.Flag("context", "Kubernetes context to use. When not set, defaults to the active context.").StringVar(&c.kubeContext)
	c.tokenKubeOIDC.Flag("cluster-name", "Name of the Kubernetes cluster. When not set, defaults to the context name.").StringVar(&c.kubeName)
	c.tokenKubeOIDC.Flag("token-name", "Optional name of the created join token. When not set, default to '<CLUSTER_NAME>(-<BOT_NAME>)'").StringVar(&c.tokenName)
	c.tokenKubeOIDC.Flag("bot", "Name of the bot that will use this token. When set, creates a bot token. Overrides --type").StringVar(&c.botName)
	c.tokenKubeOIDC.Flag("type", "Type(s) of token to add, e.g. --type=kube,app,db,discovery,proxy,etc").Default("kube,app,discovery").StringVar(&c.tokenType)
	c.tokenKubeOIDC.Flag("service-account", "Name of the Kubernetes Service Account using the token. For 'teleport-kube-agent' and 'tbot' Helm charts, this is the release name.").Short('s').Required().StringVar(&c.serviceAccountName)
	c.tokenKubeOIDC.Flag("namespace", "Namespace of the Kubernetes Service Account using the token. For 'teleport-kube-agent' and 'tbot' Helm charts, this is release namespace.").Short('n').Default("teleport").StringVar(&c.namespace)
	c.tokenKubeOIDC.Flag("update-group", "Optional update group used for version detection and agent updater configuration").StringVar(&c.updateGroup)
	c.tokenKubeOIDC.Flag("force", "Force the token creation, even if the token already exists").Default("false").Short('f').BoolVar(&c.force)

	if c.stdout == nil {
		c.stdout = os.Stdout
	}
}

// TryRun takes the CLI command as an argument (like "nodes ls") and executes it.
func (c *TokensCommand) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (match bool, err error) {
	var commandFunc func(ctx context.Context, client *authclient.Client) error
	switch cmd {
	case c.tokenAdd.FullCommand():
		commandFunc = c.Add
	case c.tokenDel.FullCommand():
		commandFunc = c.Del
	case c.tokenList.FullCommand():
		commandFunc = c.List
	case c.tokenKubeOIDC.FullCommand():
		commandFunc = c.KubeOIDC
	default:
		return false, nil
	}
	client, closeFn, err := clientFunc(ctx)
	if err != nil {
		return false, trace.Wrap(err)
	}
	err = commandFunc(ctx, client)
	closeFn(ctx)

	return true, trace.Wrap(err)
}

// Add is called to execute "tokens add ..." command.
func (c *TokensCommand) Add(ctx context.Context, client *authclient.Client) error {
	// Parse string to see if it's a type of role that Teleport supports.
	roles, err := types.ParseTeleportRoles(c.tokenType)
	if err != nil {
		return trace.Wrap(err)
	}

	// If it's Kube, then enable App and Discovery roles automatically so users
	// don't have problems with running Kubernetes App Discovery by default.
	if len(roles) == 1 && roles[0] == types.RoleKube {
		roles = append(roles, types.RoleApp, types.RoleDiscovery)
	}

	token := c.value
	if c.value == "" {
		token, err = utils.CryptoRandomHex(defaults.TokenLenBytes)
		if err != nil {
			return trace.Wrap(err, "generating token value")
		}
	}

	expires := time.Now().UTC().Add(c.ttl)
	pt, err := types.NewProvisionToken(token, roles, expires)
	if err != nil {
		return trace.Wrap(err)
	}

	if c.labels != "" {
		labels, err := libclient.ParseLabelSpec(c.labels)
		if err != nil {
			return trace.Wrap(err)
		}
		meta := pt.GetMetadata()
		meta.Labels = labels
		pt.SetMetadata(meta)
	}

	if err := client.CreateToken(ctx, pt); err != nil {
		if trace.IsAlreadyExists(err) {
			return trace.AlreadyExists(
				"failed to create token (%q already exists), please use another name",
				pt.GetName(),
			)
		}
		return trace.Wrap(err, "creating token")
	}

	// Print token information formatted with JSON, YAML, or just print the raw token.
	switch c.format {
	case teleport.JSON, teleport.YAML:
		expires := time.Now().Add(c.ttl)
		tokenInfo := map[string]any{
			"token":   token,
			"roles":   roles,
			"expires": expires,
		}

		var (
			data []byte
			err  error
		)
		if c.format == teleport.JSON {
			data, err = json.MarshalIndent(tokenInfo, "", " ")
		} else {
			data, err = yaml.Marshal(tokenInfo)
		}
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Fprint(c.stdout, string(data))

		return nil
	case teleport.Text:
		fmt.Fprintln(c.stdout, token)
		return nil
	}

	// Calculate the CA pins for this cluster. The CA pins are used by the
	// client to verify the identity of the Auth Server.
	localCAResponse, err := client.GetClusterCACert(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	caPins, err := tlsca.CalculatePins(localCAResponse.TLSCA)
	if err != nil {
		return trace.Wrap(err)
	}

	// Get list of auth servers. Used to print friendly signup message.
	authServers, err := client.GetAuthServers()
	if err != nil {
		return trace.Wrap(err)
	}
	if len(authServers) == 0 {
		return trace.BadParameter("this cluster has no auth servers")
	}

	// Print signup message.
	switch {
	case roles.Include(types.RoleKube):
		proxies, err := client.GetProxies()
		if err != nil {
			return trace.Wrap(err)
		}
		if len(proxies) == 0 {
			return trace.NotFound("cluster has no proxies")
		}
		setRoles := strings.ToLower(strings.Join(roles.StringSlice(), "\\,"))
		return kubeMessageTemplate.Execute(c.stdout,
			map[string]any{
				"auth_server": proxies[0].GetPublicAddr(),
				"token":       token,
				"minutes":     c.ttl.Minutes(),
				"set_roles":   setRoles,
				"version":     proxies[0].GetTeleportVersion(),
			})
	case roles.Include(types.RoleApp):
		proxies, err := client.GetProxies()
		if err != nil {
			return trace.Wrap(err)
		}
		if len(proxies) == 0 {
			return trace.BadParameter("cluster has no proxies")
		}
		appPublicAddr := fmt.Sprintf("%v.%v", c.appName, proxies[0].GetPublicAddr())

		return appMessageTemplate.Execute(c.stdout,
			map[string]any{
				"token":           token,
				"minutes":         c.ttl.Minutes(),
				"ca_pins":         caPins,
				"auth_server":     proxies[0].GetPublicAddr(),
				"app_name":        c.appName,
				"app_uri":         c.appURI,
				"app_public_addr": appPublicAddr,
			})
	case roles.Include(types.RoleDatabase):
		proxies, err := client.GetProxies()
		if err != nil {
			return trace.Wrap(err)
		}
		if len(proxies) == 0 {
			return trace.NotFound("cluster has no proxies")
		}
		return dbMessageTemplate.Execute(c.stdout,
			map[string]any{
				"token":       token,
				"minutes":     c.ttl.Minutes(),
				"ca_pins":     caPins,
				"auth_server": proxies[0].GetPublicAddr(),
				"db_name":     c.dbName,
				"db_protocol": c.dbProtocol,
				"db_uri":      c.dbURI,
			})
	case roles.Include(types.RoleTrustedCluster):
		fmt.Fprintf(c.stdout, trustedClusterMessage,
			token,
			int(c.ttl.Minutes()))
	case roles.Include(types.RoleWindowsDesktop):
		return desktopMessageTemplate.Execute(c.stdout,
			map[string]any{
				"token":   token,
				"minutes": c.ttl.Minutes(),
			})
	case roles.Include(types.RoleMDM):
		return mdmTokenAddTemplate.Execute(c.stdout, map[string]any{
			"token":   token,
			"minutes": c.ttl.Minutes(),
			"ca_pins": caPins,
		})
	default:
		authServer := authServers[0].GetAddr()

		pingResponse, err := client.Ping(ctx)
		if err != nil {
			slog.DebugContext(ctx, "unable to ping auth client", "error", err)
		}

		if err == nil && pingResponse.GetServerFeatures().Cloud {
			proxies, err := client.GetProxies()
			if err != nil {
				return trace.Wrap(err)
			}

			if len(proxies) != 0 {
				authServer = proxies[0].GetPublicAddr()
			}
		}

		return nodeMessageTemplate.Execute(c.stdout, map[string]any{
			"token":       token,
			"roles":       strings.ToLower(roles.String()),
			"minutes":     int(c.ttl.Minutes()),
			"ca_pins":     caPins,
			"auth_server": authServer,
		})
	}

	return nil
}

// Del is called to execute "tokens del ..." command.
func (c *TokensCommand) Del(ctx context.Context, client *authclient.Client) error {
	if c.value == "" {
		return trace.Errorf("Need an argument: token")
	}
	if err := client.DeleteToken(ctx, c.value); err != nil {
		return trace.Wrap(err)
	}
	fmt.Fprintf(c.stdout, "Token %s has been deleted\n", c.value)
	return nil
}

// The caller MUST make sure the MFA ceremony has been performed and is stored in the context
// Else this function will cause several MFA prompts.
// The MFa ceremony cannot be done in this function because we don't know if
// the caller already attempted one (e.g. tctl get all)
func getAllTokens(ctx context.Context, clt *authclient.Client) ([]types.ProvisionToken, error) {
	// There are 3 tokens types:
	// - provision tokens
	// - static tokens
	// - user tokens
	// This endpoint returns all 3 for compatibility reasons.
	// Before, all 3 tokens were returned by the same "GetTokens" RPC, now we are using
	// separate RPCs, with pagination. However, we don't know if the auth we are talking
	// to supports the new RPCs. As the static token one got introduced last, we
	// try to use it.If it works, we consume the two other RPCs. If it doesn't,
	// we fallback to the legacy all-in-one RPC.
	var tokens []types.ProvisionToken

	// Trying to get static tokens
	staticTokens, err := clt.GetStaticTokens(ctx)
	if err != nil && !trace.IsNotImplemented(err) {
		return nil, trace.Wrap(err, "getting static tokens")
	}

	// TODO(hugoShaka): DELETE IN 19.0.0
	if trace.IsNotImplemented(err) {
		// We are connected to an old auth, that doesn't support the per-token type RPCs
		// so we fallback to the legacy all-in-one RPC.
		tokens, err := clt.GetTokens(ctx)
		return tokens, trace.Wrap(err, "getting all tokens through the legacy RPC")
	}

	// We are connected to a modern auth, we must collect all 3 tokens types.
	// Getting the provision tokens.
	provisionTokens, err := stream.Collect(clientutils.Resources(ctx,
		func(ctx context.Context, pageSize int, pageKey string) ([]types.ProvisionToken, string, error) {
			return clt.ListProvisionTokens(ctx, pageSize, pageKey, nil, "")
		},
	))
	if err != nil {
		return nil, trace.Wrap(err, "getting provision tokens")
	}
	tokens = append(staticTokens.GetStaticTokens(), provisionTokens...)

	// Getting the user tokens.
	userTokens, err := stream.Collect(clientutils.Resources(ctx, clt.ListResetPasswordTokens))
	if err != nil && !trace.IsNotImplemented(err) {
		return nil, trace.Wrap(err)
	}
	if err != nil {
		return nil, trace.Wrap(err, "getting user tokens")
	}
	// Converting the user tokens as provision tokens for presentation and
	// backward compatibility.
	for _, t := range userTokens {
		roles := types.SystemRoles{types.RoleSignup}
		tok, err := types.NewProvisionToken(t.GetName(), roles, t.Expiry())
		if err != nil {
			return nil, trace.Wrap(err, "converting user token as a provision token")
		}
		tokens = append(tokens, tok)
	}

	return tokens, nil
}

// List is called to execute "tokens ls" command.
func (c *TokensCommand) List(ctx context.Context, client *authclient.Client) error {
	labels, err := libclient.ParseLabelSpec(c.labels)
	if err != nil {
		return trace.Wrap(err)
	}

	// Because getAllTokens do up to 3 calls, we want to perform the MFA ceremony
	// once and put it in the context. Else the users will get 3 MFA prompts.
	if mfaResponse, err := mfa.PerformAdminActionMFACeremony(ctx, client.PerformMFACeremony, true /*allowReuse*/); err == nil {
		ctx = mfa.ContextWithMFAResponse(ctx, mfaResponse)
	} else if !errors.Is(err, &mfa.ErrMFANotRequired) && !errors.Is(err, &mfa.ErrMFANotSupported) {
		return trace.Wrap(err)
	}

	tokens, err := getAllTokens(ctx, client)
	if err != nil {
		return trace.Wrap(err)
	}

	tokens = slices.DeleteFunc(tokens, func(token types.ProvisionToken) bool {
		tokenLabels := token.GetMetadata().Labels
		for k, v := range labels {
			if tokenLabels[k] != v {
				return true
			}
		}
		return false
	})

	if len(tokens) == 0 && c.format == teleport.Text {
		fmt.Fprintln(c.stdout, "No active tokens found.")
		return nil
	}

	// Sort by expire time.
	sort.Slice(tokens, func(i, j int) bool { return tokens[i].Expiry().Unix() < tokens[j].Expiry().Unix() })

	nameFunc := (types.ProvisionToken).GetSafeName
	if c.withSecrets {
		nameFunc = (types.ProvisionToken).GetName
	}

	switch c.format {
	case teleport.JSON:
		err := utils.WriteJSONArray(c.stdout, tokens)
		if err != nil {
			return trace.Wrap(err, "failed to marshal tokens")
		}
	case teleport.YAML:
		err := utils.WriteYAML(c.stdout, tokens)
		if err != nil {
			return trace.Wrap(err, "failed to marshal tokens")
		}
	case teleport.Text:
		for _, token := range tokens {
			fmt.Fprintln(c.stdout, nameFunc(token))
		}
	default:
		tokensView := func() string {
			table := asciitable.MakeTable([]string{"Token", "Type", "Labels", "Expiry Time (UTC)"})
			now := time.Now()
			for _, t := range tokens {
				expiry := "never"
				if !t.Expiry().IsZero() && t.Expiry().Unix() != 0 {
					exptime := t.Expiry().Format(time.RFC822)
					expdur := t.Expiry().Sub(now).Round(time.Second)
					expiry = fmt.Sprintf("%s (%s)", exptime, expdur.String())
				}
				table.AddRow([]string{nameFunc(t), t.GetRoles().String(), printMetadataLabels(t.GetMetadata().Labels), expiry})
			}
			return table.AsBuffer().String()
		}
		fmt.Fprint(c.stdout, tokensView())
	}
	return nil
}

// TODO: check if we have types for this
type KubernetesOIDCResponse struct {
	Issuer                           string   `json:"issuer"`
	JwksUri                          string   `json:"jwks_uri"`
	ResponseTypesSupported           []string `json:"response_types_supported"`
	SubjectTypesSupported            []string `json:"subject_types_supported"`
	IdTokenSigningAlgValuesSupported []string `json:"id_token_signing_alg_values_supported"`
}

// KubeOIDC is called to execute "tctl tokens configure-kube-oidc ..." command
func (c *TokensCommand) KubeOIDC(ctx context.Context, client *authclient.Client) error {
	// preflight checks
	var roles types.SystemRoles
	var err error
	if c.botName != "" {
		roles = types.SystemRoles{types.RoleBot}
	} else {
		// Parse string to see if it's a type of role that Teleport supports.
		roles, err = types.ParseTeleportRoles(c.tokenType)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	fmt.Fprintln(os.Stderr, "📡 Looking up Teleport cluster settings")

	// detect proxy address
	authPong, err := client.Ping(ctx)
	if err != nil {
		return trace.Wrap(err, "failed to ping Teleport Auth")
	}
	proxyAddr := authPong.GetProxyPublicAddr()
	if proxyAddr == "" {
		return trace.BadParameter("failed to discover Teleport proxy address, make sure the Teleport Proxy service is running with `public_addr` set")
	}
	// detect autoupdate version
	proxyPong, err := webclient.Ping(&webclient.Config{
		Context:   ctx,
		ProxyAddr: proxyAddr,
		// TODO: handle insecure properly
		Insecure:    false,
		UpdateGroup: c.updateGroup,
	})
	if err != nil {
		return trace.Wrap(err, "failed to ping Teleport Proxy")
	}
	agentVersion := proxyPong.AutoUpdate.AgentVersion
	if agentVersion == "" {
		return trace.NotFound("failed to discover Teleport Agent version")
	}

	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return trace.Wrap(err, "failed to find kubectl")
	}

	fmt.Fprintln(os.Stderr, "📁 Finding local kubectl context")

	kubeContext := c.kubeContext
	// If kube context not set, we look it up
	if kubeContext == "" {
		var stdout, stderr bytes.Buffer
		cmd := exec.Cmd{
			Path:      kubectlPath,
			Args:      []string{kubectlPath, "config", "current-context"},
			Env:       os.Environ(),
			Stdout:    &stdout,
			Stderr:    &stderr,
			WaitDelay: 30 * time.Second,
		}
		if err := cmd.Run(); err != nil {
			return trace.Wrap(err, "failed to detect current kubectl context")
		}
		kubeContext = strings.TrimSpace(stdout.String())
		if kubeContext == "" {
			return trace.BadParameter("context not set. and no active kubectl context found")
		}
	}

	kubeName := c.kubeName
	if kubeName == "" {
		kubeName = kubeContext
	}

	tokenName := c.tokenName
	if tokenName == "" {
		tokenName = kubeName
		if c.botName != "" {
			tokenName = tokenName + "-" + c.botName
		}
	}

	fmt.Fprintf(os.Stderr, "🔎 Detecting OIDC provider for Kubernetes cluster %q\n", kubeName)

	kubectlArgs := []string{kubectlPath, "--context", kubeContext}

	var kubectlStdout, kubectlStderr bytes.Buffer
	var oidcResponse KubernetesOIDCResponse
	kubectlArgs = append(kubectlArgs, "get", "--raw=/.well-known/openid-configuration")
	// ping kube cluster OIDC endpoint
	kubectl := exec.Cmd{
		Path:      kubectlPath,
		Args:      kubectlArgs,
		Env:       os.Environ(),
		Stdout:    &kubectlStdout,
		Stderr:    &kubectlStderr,
		WaitDelay: 30 * time.Second,
	}

	err = kubectl.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, kubectlStderr.String())
		return trace.Wrap(err, "failed to run kubectl")
	}
	err = json.Unmarshal(kubectlStdout.Bytes(), &oidcResponse)
	if err != nil {
		// TODO: log the response
		return trace.Wrap(err, "failed to parse oidc response")
	}

	if oidcResponse.Issuer == "" {
		return trace.BadParameter("failed to discover OIDC issuer")
	}

	oidcDiscoverCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	// hit OIDC provider
	dc, err := oidcclient.Discover(oidcDiscoverCtx, oidcResponse.Issuer, otelhttp.DefaultClient)
	if err != nil {
		// TODO: explain that the cluster might ot have OIDC enabled
		return trace.Wrap(err, "failed to discover OIDC issuer")
	}

	fmt.Fprintf(os.Stderr, "⚙️ Configuring trust by creating token %q\n", tokenName)

	// craft token for cluster
	tokenSpec := types.ProvisionTokenSpecV2{
		Roles:      roles,
		Allow:      nil,
		AWSIIDTTL:  0,
		JoinMethod: types.JoinMethodKubernetes,
		BotName:    c.botName,
		Kubernetes: &types.ProvisionTokenSpecV2Kubernetes{
			Allow: []*types.ProvisionTokenSpecV2Kubernetes_Rule{
				{ServiceAccount: fmt.Sprintf("%s:%s", c.namespace, c.serviceAccountName)},
			},
			Type: types.KubernetesJoinTypeOIDC,
			OIDC: &types.ProvisionTokenSpecV2Kubernetes_OIDCConfig{
				Issuer:                  dc.Issuer,
				InsecureAllowHTTPIssuer: false,
			},
		},
	}
	token, err := types.NewProvisionTokenFromSpec(tokenName, time.Time{}, tokenSpec)
	if err != nil {
		return trace.Wrap(err, "error crafting provision token for the Kube cluster")
	}
	// apply token
	if c.force {
		err := client.UpsertToken(ctx, token)
		if err != nil {
			return trace.Wrap(err, "failed to upsert provision token")
		}
	} else {
		err = client.CreateToken(ctx, token)
		if err != nil {
			if trace.IsAlreadyExists(err) {
				return trace.AlreadyExists("token %q already exists, you can pick a different token name with --token-name, or overwrite the existing token with --force", tokenName)
			}
			return trace.Wrap(err, "failed to create provision token")
		}
	}

	chartName := "teleport/teleport-kube-agent"
	if c.botName != "" {
		chartName = "teleport/tbot"
	}

	fmt.Fprintf(os.Stderr, "📝 Writing %s Helm values to: %q\n", chartName, c.outputPath)

	var valueGenerator valueGeneratorFunc
	if c.botName != "" {
		valueGenerator = generateTbotValues
	} else {
		valueGenerator = generateAgentValues
	}

	values, err := valueGenerator(valueGeneratorInput{
		botName:             c.botName,
		roles:               roles,
		proxyAddr:           proxyAddr,
		enterprise:          proxyPong.Edition == modules.BuildEnterprise,
		updateGroup:         c.updateGroup,
		tokenName:           tokenName,
		teleportClusterName: authPong.GetClusterName(),
		kubeClusterName:     kubeName,
	})
	if err != nil {
		return trace.Wrap(err, "error generating chart values")
	}

	if err := os.WriteFile(c.outputPath, values, 0644); err != nil {
		return trace.Wrap(err, "error writing chart values to file %s", c.outputPath)
	}

	fmt.Fprintln(os.Stderr, "🚀 You can now deploy the Helm chart by running:\n")
	fmt.Println(craftHelmCommand(craftHelmCommandInput{
		releaseName: c.serviceAccountName,
		chartName:   chartName,
		namespace:   c.namespace,
		version:     proxyPong.AutoUpdate.AgentVersion,
		valuesPath:  c.outputPath,
		kubeContext: kubeContext,
	}))
	return nil
}

type craftHelmCommandInput struct {
	releaseName string
	chartName   string
	namespace   string
	version     string
	valuesPath  string
	kubeContext string
}

func craftHelmCommand(input craftHelmCommandInput) string {
	sb := &strings.Builder{}
	fmt.Fprintf(sb, "helm repo add teleport %q; \n", teleportassets.HelmRepoURL().String())
	sb.WriteString("helm repo update; \n")
	fmt.Fprintf(sb, "helm upgrade --install %q %q", input.releaseName, input.chartName)
	fmt.Fprintf(sb, "\\\n  --namespace %s --create-namespace ", input.namespace)
	fmt.Fprintf(sb, "\\\n  --version %s ", input.version)
	fmt.Fprintf(sb, "\\\n  --values %s ", input.valuesPath)
	fmt.Fprintf(sb, "\\\n  --kube-context %s\n", input.kubeContext)
	return sb.String()
}

type valueGeneratorInput struct {
	botName             string
	roles               types.SystemRoles
	proxyAddr           string
	enterprise          bool
	updateGroup         string
	tokenName           string
	teleportClusterName string
	kubeClusterName     string
}
type valueGeneratorFunc func(valueGeneratorInput) ([]byte, error)

type TbotValues struct {
	ClusterName          string `yaml:"clusterName"`
	TeleportProxyAddress string `yaml:"teleportProxyAddress"`
	DefaultOutput        struct {
		SecretName string `yaml:"secretName"`
	} `yaml:"defaultOutput"`
	Token string `yaml:"token"`
}

func generateTbotValues(input valueGeneratorInput) ([]byte, error) {
	tbotValues := TbotValues{
		ClusterName:          input.teleportClusterName,
		TeleportProxyAddress: input.proxyAddr,
		DefaultOutput: struct {
			SecretName string `yaml:"secretName"`
		}{
			SecretName: fmt.Sprintf("%s-output", input.botName),
		},
		Token: input.tokenName,
	}
	return yaml.Marshal(tbotValues)
}

type AgentValues struct {
	Roles      string `yaml:"roles"`
	ProxyAddr  string `yaml:"proxyAddr"`
	Enterprise bool   `yaml:"enterprise"`
	Updater    struct {
		Enabled bool   `yaml:"enabled"`
		Group   string `yaml:"group,omitempty"`
	} `yaml:"updater,omitempty"`
	JoinParams struct {
		Method    string `yaml:"method"`
		TokenName string `yaml:"tokenName"`
	} `yaml:"joinParams"`
	KubeClusterName     string `yaml:"kubeClusterName"`
	TeleportClusterName string `yaml:"teleportClusterName"`
}

func generateAgentValues(input valueGeneratorInput) ([]byte, error) {
	agentValues := AgentValues{
		Roles:      strings.ToLower(input.roles.String()),
		ProxyAddr:  input.proxyAddr,
		Enterprise: input.enterprise,
		Updater: struct {
			Enabled bool   `yaml:"enabled"`
			Group   string `yaml:"group,omitempty"`
		}{
			Enabled: true,
			Group:   input.updateGroup,
		},
		JoinParams: struct {
			Method    string `yaml:"method"`
			TokenName string `yaml:"tokenName"`
		}{
			Method:    string(types.JoinMethodKubernetes),
			TokenName: input.tokenName,
		},
		TeleportClusterName: input.teleportClusterName,
		KubeClusterName:     input.kubeClusterName,
	}

	return yaml.Marshal(agentValues)
}
