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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"slices"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/ghodss/yaml"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	libclient "github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
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
		tokenInfo := map[string]interface{}{
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
			map[string]interface{}{
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
			map[string]interface{}{
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
			map[string]interface{}{
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
			map[string]interface{}{
				"token":   token,
				"minutes": c.ttl.Minutes(),
			})
	case roles.Include(types.RoleMDM):
		return mdmTokenAddTemplate.Execute(c.stdout, map[string]interface{}{
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

		return nodeMessageTemplate.Execute(c.stdout, map[string]interface{}{
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

// List is called to execute "tokens ls" command.
func (c *TokensCommand) List(ctx context.Context, client *authclient.Client) error {
	labels, err := libclient.ParseLabelSpec(c.labels)
	if err != nil {
		return trace.Wrap(err)
	}

	tokens, err := client.GetTokens(ctx)
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
