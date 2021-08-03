/*
Copyright 2015-2017 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package common

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth"
	libclient "github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/kingpin"
	"github.com/gravitational/trace"
)

// TokenCommand implements `tctl token` group of commands
type TokenCommand struct {
	config *service.Config

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
}

// Initialize allows TokenCommand to plug itself into the CLI parser
func (c *TokenCommand) Initialize(app *kingpin.Application, config *service.Config) {
	c.config = config

	tokens := app.Command("tokens", "List or revoke invitation tokens")

	// tctl tokens add ..."
	c.tokenAdd = tokens.Command("add", "Create a invitation token")
	c.tokenAdd.Flag("type", "Type of token to add").Required().StringVar(&c.tokenType)
	c.tokenAdd.Flag("value", "Value of token to add").StringVar(&c.value)
	c.tokenAdd.Flag("labels", "Set token labels, e.g. env=prod,region=us-west").StringVar(&c.labels)
	c.tokenAdd.Flag("ttl", fmt.Sprintf("Set expiration time for token, default is %v hour, maximum is %v hours",
		int(defaults.SignupTokenTTL/time.Hour), int(defaults.MaxSignupTokenTTL/time.Hour))).
		Default(fmt.Sprintf("%v", defaults.SignupTokenTTL)).DurationVar(&c.ttl)
	c.tokenAdd.Flag("app-name", "Name of the application to add").Default("example-app").StringVar(&c.appName)
	c.tokenAdd.Flag("app-uri", "URI of the application to add").Default("http://localhost:8080").StringVar(&c.appURI)
	c.tokenAdd.Flag("db-name", "Name of the database to add").StringVar(&c.dbName)
	c.tokenAdd.Flag("db-protocol", fmt.Sprintf("Database protocol to use. Supported are: %v", defaults.DatabaseProtocols)).StringVar(&c.dbProtocol)
	c.tokenAdd.Flag("db-uri", "Address the database is reachable at").StringVar(&c.dbURI)

	// "tctl tokens rm ..."
	c.tokenDel = tokens.Command("rm", "Delete/revoke an invitation token").Alias("del")
	c.tokenDel.Arg("token", "Token to delete").StringVar(&c.value)

	// "tctl tokens ls"
	c.tokenList = tokens.Command("ls", "List node and user invitation tokens")
	c.tokenList.Flag("format", "Output format, 'text' or 'json'").Hidden().Default(teleport.Text).StringVar(&c.format)
}

// TryRun takes the CLI command as an argument (like "nodes ls") and executes it.
func (c *TokenCommand) TryRun(cmd string, client auth.ClientI) (match bool, err error) {
	switch cmd {
	case c.tokenAdd.FullCommand():
		err = c.Add(client)
	case c.tokenDel.FullCommand():
		err = c.Del(client)
	case c.tokenList.FullCommand():
		err = c.List(client)
	default:
		return false, nil
	}
	return true, trace.Wrap(err)
}

// Add is called to execute "tokens add ..." command.
func (c *TokenCommand) Add(client auth.ClientI) error {
	// Parse string to see if it's a type of role that Teleport supports.
	roles, err := types.ParseTeleportRoles(c.tokenType)
	if err != nil {
		return trace.Wrap(err)
	}

	var labels map[string]string
	if c.labels != "" {
		labels, err = libclient.ParseLabelSpec(c.labels)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	// Generate token.
	token, err := client.GenerateToken(context.TODO(), auth.GenerateTokenRequest{
		Roles:  roles,
		TTL:    c.ttl,
		Token:  c.value,
		Labels: labels,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// Calculate the CA pin for this cluster. The CA pin is used by the client
	// to verify the identity of the Auth Server.
	caPin, err := calculateCAPin(client)
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
	case roles.Include(types.RoleApp):
		proxies, err := client.GetProxies()
		if err != nil {
			return trace.Wrap(err)
		}
		if len(proxies) == 0 {
			return trace.BadParameter("cluster has no proxies")
		}
		appPublicAddr := fmt.Sprintf("%v.%v", c.appName, proxies[0].GetPublicAddr())

		fmt.Printf(appMessage,
			token,
			int(c.ttl.Minutes()),
			token,
			caPin,
			proxies[0].GetPublicAddr(),
			c.appName,
			c.appName,
			c.appURI,
			c.appURI,
			appPublicAddr,
			int(c.ttl.Minutes()),
			proxies[0].GetPublicAddr(),
			appPublicAddr,
			appPublicAddr)
	case roles.Include(types.RoleDatabase):
		proxies, err := client.GetProxies()
		if err != nil {
			return trace.Wrap(err)
		}
		if len(proxies) == 0 {
			return trace.NotFound("cluster has no proxies")
		}
		return dbMessageTemplate.Execute(os.Stdout,
			map[string]interface{}{
				"token":       token,
				"minutes":     c.ttl.Minutes(),
				"ca_pin":      caPin,
				"auth_server": proxies[0].GetPublicAddr(),
				"db_name":     c.dbName,
				"db_protocol": c.dbProtocol,
				"db_uri":      c.dbURI,
			})
	case roles.Include(types.RoleTrustedCluster), roles.Include(types.LegacyClusterTokenType):
		fmt.Printf(trustedClusterMessage,
			token,
			int(c.ttl.Minutes()))
	default:
		fmt.Printf(nodeMessage,
			token,
			int(c.ttl.Minutes()),
			strings.ToLower(roles.String()),
			token,
			caPin,
			authServers[0].GetAddr(),
			int(c.ttl.Minutes()),
			authServers[0].GetAddr())
	}

	return nil
}

// Del is called to execute "tokens del ..." command.
func (c *TokenCommand) Del(client auth.ClientI) error {
	ctx := context.TODO()
	if c.value == "" {
		return trace.Errorf("Need an argument: token")
	}
	if err := client.DeleteToken(ctx, c.value); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Token %s has been deleted\n", c.value)
	return nil
}

// List is called to execute "tokens ls" command.
func (c *TokenCommand) List(client auth.ClientI) error {
	ctx := context.TODO()
	tokens, err := client.GetTokens(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(tokens) == 0 {
		fmt.Println("No active tokens found.")
		return nil
	}

	// Sort by expire time.
	sort.Slice(tokens, func(i, j int) bool { return tokens[i].Expiry().Unix() < tokens[j].Expiry().Unix() })

	if c.format == teleport.Text {
		tokensView := func() string {
			table := asciitable.MakeTable([]string{"Token", "Type", "Labels", "Expiry Time (UTC)"})
			now := time.Now()
			for _, t := range tokens {
				expiry := "never"
				if t.Expiry().Unix() > 0 {
					exptime := t.Expiry().Format(time.RFC822)
					expdur := t.Expiry().Sub(now).Round(time.Second)
					expiry = fmt.Sprintf("%s (%s)", exptime, expdur.String())
				}
				table.AddRow([]string{t.GetName(), t.GetRoles().String(), printMetadataLabels(t.GetMetadata().Labels), expiry})
			}
			return table.AsBuffer().String()
		}
		fmt.Print(tokensView())
	} else {
		data, err := json.MarshalIndent(tokens, "", "  ")
		if err != nil {
			return trace.Wrap(err, "failed to marshal tokens")
		}
		fmt.Print(string(data))
	}
	return nil
}

// calculateCAPin returns the SPKI pin for the local cluster.
func calculateCAPin(client auth.ClientI) (string, error) {
	localCA, err := client.GetClusterCACert()
	if err != nil {
		return "", trace.Wrap(err)
	}
	tlsCA, err := tlsca.ParseCertificatePEM(localCA.TLSCA)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return utils.CalculateSPKI(tlsCA), nil
}
