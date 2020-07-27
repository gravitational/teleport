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
	"sort"
	"strings"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"

	"github.com/gravitational/kingpin"
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

	// ttl is how long the token will live for.
	ttl time.Duration

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
	c.tokenAdd.Flag("ttl", fmt.Sprintf("Set expiration time for token, default is %v hour, maximum is %v hours",
		int(defaults.SignupTokenTTL/time.Hour), int(defaults.MaxSignupTokenTTL/time.Hour))).
		Default(fmt.Sprintf("%v", defaults.SignupTokenTTL)).DurationVar(&c.ttl)

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
	roles, err := teleport.ParseRoles(c.tokenType)
	if err != nil {
		return trace.Wrap(err)
	}

	// Generate token.
	token, err := client.GenerateToken(context.TODO(), auth.GenerateTokenRequest{
		Roles: roles,
		TTL:   c.ttl,
		Token: c.value,
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
		return trace.Errorf("this cluster has no auth servers")
	}

	// Print signup message.
	switch {
	case roles.Include(teleport.RoleTrustedCluster), roles.Include(teleport.LegacyClusterTokenType):
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
	if c.value == "" {
		return trace.Errorf("Need an argument: token")
	}
	if err := client.DeleteToken(c.value); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Token %s has been deleted\n", c.value)
	return nil
}

// List is called to execute "tokens ls" command.
func (c *TokenCommand) List(client auth.ClientI) error {
	tokens, err := client.GetTokens()
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
			table := asciitable.MakeTable([]string{"Token", "Type", "Expiry Time (UTC)"})
			now := time.Now()
			for _, t := range tokens {
				expiry := "never"
				if t.Expiry().Unix() > 0 {
					exptime := t.Expiry().Format(time.RFC822)
					expdur := t.Expiry().Sub(now).Round(time.Second)
					expiry = fmt.Sprintf("%s (%s)", exptime, expdur.String())
				}
				table.AddRow([]string{t.GetName(), t.GetRoles().String(), expiry})
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
