/*
Copyright 2021-2022 Gravitational, Inc.

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
	"strings"
	"text/template"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/kingpin"
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

type BotsCommand struct {
	format string

	lockExpires string
	lockTTL     time.Duration

	botName  string
	botRoles string
	tokenID  string
	tokenTTL time.Duration

	botsList   *kingpin.CmdClause
	botsAdd    *kingpin.CmdClause
	botsRemove *kingpin.CmdClause
	botsLock   *kingpin.CmdClause
}

// Initialize sets up the "tctl bots" command.
func (c *BotsCommand) Initialize(app *kingpin.Application, config *service.Config) {
	bots := app.Command("bots", "Operate on certificate renewal bots registered with the cluster.")

	c.botsList = bots.Command("ls", "List all certificate renewal bots registered with the cluster.")
	c.botsList.Flag("format", "Output format, 'text' or 'json'").Hidden().Default(teleport.Text).StringVar(&c.format)

	c.botsAdd = bots.Command("add", "Add a new certificate renewal bot to the cluster.")
	c.botsAdd.Arg("name", "A name to uniquely identify this bot in the cluster.").Required().StringVar(&c.botName)
	c.botsAdd.Flag("roles", "Roles the bot is able to assume.").Required().StringVar(&c.botRoles)
	c.botsAdd.Flag("ttl", "TTL for the bot join token.").DurationVar(&c.tokenTTL)
	c.botsAdd.Flag("token", "Name of an existing token to use.").StringVar(&c.tokenID)
	// TODO: --ttl for setting a ttl on the join token

	c.botsRemove = bots.Command("rm", "Permanently remove a certificate renewal bot from the cluster.")
	c.botsRemove.Arg("name", "Name of an existing bot to remove.").Required().StringVar(&c.botName)

	c.botsLock = bots.Command("lock", "Prevent a bot from renewing its certificates.")
	c.botsLock.Arg("name", "Name of an existing bot to lock.").Required().StringVar(&c.botName)
	c.botsLock.Flag("expires", "Time point (RFC3339) when the lock expires.").StringVar(&c.lockExpires)
	c.botsLock.Flag("ttl", "Time duration after which the lock expires.").DurationVar(&c.lockTTL)
	c.botsLock.Hidden()
}

// TryRun attempts to run subcommands.
func (c *BotsCommand) TryRun(cmd string, client auth.ClientI) (match bool, err error) {
	switch cmd {
	case c.botsList.FullCommand():
		err = c.ListBots(client)
	case c.botsAdd.FullCommand():
		err = c.AddBot(client)
	case c.botsRemove.FullCommand():
		err = c.RemoveBot(client)
	case c.botsLock.FullCommand():
		err = c.LockBot(client)
	default:
		return false, nil
	}

	return true, trace.Wrap(err)
}

// ListBots writes a listing of the cluster's certificate renewal bots
// to standard out.
func (c *BotsCommand) ListBots(client auth.ClientI) error {
	// TODO: consider adding a custom column for impersonator roles, locks, ??
	users, err := client.GetBotUsers(context.Background())
	if err != nil {
		return trace.Wrap(err)
	}
	if c.format == teleport.Text {
		if len(users) == 0 {
			fmt.Println("No users found")
			return nil
		}
		t := asciitable.MakeTable([]string{"Bot", "User", "Roles"})
		for _, u := range users {
			var botName string
			meta := u.GetMetadata()
			if val, ok := meta.Labels[types.BotLabel]; ok {
				botName = val
			} else {
				// Should not be possible, but not worth failing over.
				botName = "-"
			}

			t.AddRow([]string{
				botName, u.GetName(), strings.Join(u.GetRoles(), ","),
			})
		}
		fmt.Println(t.AsBuffer().String())
	} else {
		out, err := json.MarshalIndent(users, "", "  ")
		if err != nil {
			return trace.Wrap(err, "failed to marshal users")
		}
		fmt.Print(string(out))
	}
	return nil
}

// bold wraps the given text in an ANSI escape to bold it
func bold(text string) string {
	return utils.Color(utils.Bold, text)
}

var startMessageTemplate = template.Must(template.New("node").Funcs(template.FuncMap{
	"bold": bold,
}).Parse(`The bot token: {{.token}}
This token will expire in {{.minutes}} minutes.

Optionally, if running the bot under an isolated user account, first initialize
the data directory by running the following command {{ bold "as root" }}:

> tbot init \
   --auth-server={{.auth_server}} \
   --destination-dir=./tbot-user \
   --bot-user=tbot \
   --reader-user=alice

... where "tbot" is the username of the bot's UNIX user, and "alice" is the
UNIX user that will be making use of the certificates.

Then, run this {{ bold "as the bot user" }} to begin continuously fetching
certificates:

> tbot start \
   --destination-dir=./tbot-user \
   --token={{.token}} \{{range .ca_pins}}
   --ca-pin={{.}} \{{end}}
   --auth-server={{.auth_server}}{{if .join_method}} \
   --join-method={{.join_method}}{{end}}

Please note:

  - The ./tbot-user destination directory can be changed as desired.
  - /var/lib/teleport/bot must be accessible to the bot user, or --data-dir
    must point to another accessible directory to store internal bot data.
  - This invitation token will expire in {{.minutes}} minutes
  - {{.auth_server}} must be reachable from the new node
`))

// AddBot adds a new certificate renewal bot to the cluster.
func (c *BotsCommand) AddBot(client auth.ClientI) error {
	response, err := client.CreateBot(context.Background(), &proto.CreateBotRequest{
		Name:    c.botName,
		TTL:     proto.Duration(c.tokenTTL),
		Roles:   splitRoles(c.botRoles),
		TokenID: c.tokenID,
	})
	if err != nil {
		return trace.WrapWithMessage(err, "error while creating bot")
	}

	// Calculate the CA pins for this cluster. The CA pins are used by the
	// client to verify the identity of the Auth Server.
	localCAResponse, err := client.GetClusterCACert()
	if err != nil {
		return trace.Wrap(err)
	}
	caPins, err := tlsca.CalculatePins(localCAResponse.TLSCA)
	if err != nil {
		return trace.Wrap(err)
	}

	authServers, err := client.GetAuthServers()
	if err != nil {
		return trace.Wrap(err)
	}
	if len(authServers) == 0 {
		return trace.Errorf("This cluster does not have any auth servers running.")
	}

	addr := authServers[0].GetPublicAddr()
	if addr == "" {
		addr = authServers[0].GetAddr()
	}

	joinMethod := response.JoinMethod
	// omit join method output for the token method
	switch joinMethod {
	case types.JoinMethodUnspecified, types.JoinMethodToken:
		// the template will omit an empty string
		joinMethod = ""
	default:
	}

	return startMessageTemplate.Execute(os.Stdout, map[string]interface{}{
		"token":       response.TokenID,
		"minutes":     int(time.Duration(response.TokenTTL).Minutes()),
		"ca_pins":     caPins,
		"auth_server": addr,
		"join_method": joinMethod,
	})
}

func (c *BotsCommand) RemoveBot(client auth.ClientI) error {
	if err := client.DeleteBot(context.Background(), c.botName); err != nil {
		return trace.WrapWithMessage(err, "error deleting bot")
	}

	fmt.Printf("Bot %q deleted successfully.\n", c.botName)

	return nil
}

func (c *BotsCommand) LockBot(client auth.ClientI) error {
	lockExpiry, err := computeLockExpiry(c.lockExpires, c.lockTTL)
	if err != nil {
		return trace.Wrap(err)
	}

	user, err := client.GetUser(auth.BotResourceName(c.botName), false)
	if err != nil {
		return trace.Wrap(err)
	}

	meta := user.GetMetadata()
	botName, ok := meta.Labels[types.BotLabel]
	if !ok {
		return trace.BadParameter("User %q is not a bot user; use `tctl lock` directly to lock this user", user.GetName())
	}

	if botName != c.botName {
		return trace.BadParameter("User %q is not associated with expected bot %q (expected %q); use `tctl lock` directly to lock this user", user.GetName(), c.botName, botName)
	}

	lock, err := types.NewLock(uuid.New().String(), types.LockSpecV2{
		Target: types.LockTarget{
			User: user.GetName(),
		},
		Expires: lockExpiry,
		Message: fmt.Sprintf("The bot user %q associated with bot %q has been locked.", user.GetName(), c.botName),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	if err := client.UpsertLock(context.Background(), lock); err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("Created a lock with name %q.\n", lock.GetName())

	return nil
}

func splitRoles(flag string) []string {
	var roles []string
	for _, s := range strings.Split(flag, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		roles = append(roles, s)
	}
	return roles
}
