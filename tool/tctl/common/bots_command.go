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

	"github.com/alecthomas/kingpin/v2"
	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/utils"
)

type BotsCommand struct {
	format string

	lockExpires string
	lockTTL     time.Duration

	botName  string
	botRoles string
	tokenID  string
	tokenTTL time.Duration

	allowedLogins []string

	botsList   *kingpin.CmdClause
	botsAdd    *kingpin.CmdClause
	botsRemove *kingpin.CmdClause
	botsLock   *kingpin.CmdClause
}

// Initialize sets up the "tctl bots" command.
func (c *BotsCommand) Initialize(app *kingpin.Application, config *service.Config) {
	bots := app.Command("bots", "Operate on certificate renewal bots registered with the cluster.")

	c.botsList = bots.Command("ls", "List all certificate renewal bots registered with the cluster.")
	c.botsList.Flag("format", "Output format, 'text' or 'json'").Hidden().Default(teleport.Text).EnumVar(&c.format, teleport.Text, teleport.JSON)

	c.botsAdd = bots.Command("add", "Add a new certificate renewal bot to the cluster.")
	c.botsAdd.Arg("name", "A name to uniquely identify this bot in the cluster.").Required().StringVar(&c.botName)
	c.botsAdd.Flag("roles", "Roles the bot is able to assume.").Required().StringVar(&c.botRoles)
	c.botsAdd.Flag("ttl", "TTL for the bot join token.").DurationVar(&c.tokenTTL)
	c.botsAdd.Flag("token", "Name of an existing token to use.").StringVar(&c.tokenID)
	c.botsAdd.Flag("format", "Output format, 'text' or 'json'").Hidden().Default(teleport.Text).EnumVar(&c.format, teleport.Text, teleport.JSON)
	c.botsAdd.Flag("logins", "List of allowed SSH logins for the bot user").StringsVar(&c.allowedLogins)

	c.botsRemove = bots.Command("rm", "Permanently remove a certificate renewal bot from the cluster.")
	c.botsRemove.Arg("name", "Name of an existing bot to remove.").Required().StringVar(&c.botName)

	c.botsLock = bots.Command("lock", "Prevent a bot from renewing its certificates.")
	c.botsLock.Arg("name", "Name of an existing bot to lock.").Required().StringVar(&c.botName)
	c.botsLock.Flag("expires", "Time point (RFC3339) when the lock expires.").StringVar(&c.lockExpires)
	c.botsLock.Flag("ttl", "Time duration after which the lock expires.").DurationVar(&c.lockTTL)
	c.botsLock.Hidden()
}

// TryRun attempts to run subcommands.
func (c *BotsCommand) TryRun(ctx context.Context, cmd string, client auth.ClientI) (match bool, err error) {
	switch cmd {
	case c.botsList.FullCommand():
		err = c.ListBots(ctx, client)
	case c.botsAdd.FullCommand():
		err = c.AddBot(ctx, client)
	case c.botsRemove.FullCommand():
		err = c.RemoveBot(ctx, client)
	case c.botsLock.FullCommand():
		err = c.LockBot(ctx, client)
	default:
		return false, nil
	}

	return true, trace.Wrap(err)
}

// ListBots writes a listing of the cluster's certificate renewal bots
// to standard out.
func (c *BotsCommand) ListBots(ctx context.Context, client auth.ClientI) error {
	// TODO: consider adding a custom column for impersonator roles, locks, ??
	users, err := client.GetBotUsers(ctx)
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
   --destination-dir=./tbot-user \
   --bot-user=tbot \
   --reader-user=alice

... where "tbot" is the username of the bot's UNIX user, and "alice" is the
UNIX user that will be making use of the certificates.

Then, run this {{ bold "as the bot user" }} to begin continuously fetching
certificates:

> tbot start \
   --destination-dir=./tbot-user \
   --token={{.token}} \
   --auth-server={{.addr}}{{if .join_method}} \
   --join-method={{.join_method}}{{end}}

Please note:

  - The ./tbot-user destination directory can be changed as desired.
  - /var/lib/teleport/bot must be accessible to the bot user, or --data-dir
    must point to another accessible directory to store internal bot data.
  - This invitation token will expire in {{.minutes}} minutes
  - {{.addr}} must be reachable from the new node
`))

// AddBot adds a new certificate renewal bot to the cluster.
func (c *BotsCommand) AddBot(ctx context.Context, client auth.ClientI) error {
	roles := splitRoles(c.botRoles)
	if len(roles) == 0 {
		return trace.BadParameter("at least one role must be specified with --roles")
	}

	traits := map[string][]string{
		constants.TraitLogins: flattenSlice(c.allowedLogins),
	}

	response, err := client.CreateBot(ctx, &proto.CreateBotRequest{
		Name:    c.botName,
		TTL:     proto.Duration(c.tokenTTL),
		Roles:   roles,
		TokenID: c.tokenID,
		Traits:  traits,
	})
	if err != nil {
		return trace.WrapWithMessage(err, "error while creating bot")
	}

	if c.format == teleport.JSON {
		out, err := json.MarshalIndent(response, "", "  ")
		if err != nil {
			return trace.Wrap(err, "failed to marshal CreateBot response")
		}

		fmt.Println(string(out))
		return nil
	}

	proxies, err := client.GetProxies()
	if err != nil {
		return trace.Wrap(err)
	}
	if len(proxies) == 0 {
		return trace.Errorf("This cluster does not have any proxy servers running.")
	}
	addr := proxies[0].GetPublicAddr()
	if addr == "" {
		addr = proxies[0].GetAddr()
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
		"addr":        addr,
		"join_method": joinMethod,
	})
}

func (c *BotsCommand) RemoveBot(ctx context.Context, client auth.ClientI) error {
	if err := client.DeleteBot(ctx, c.botName); err != nil {
		return trace.WrapWithMessage(err, "error deleting bot")
	}

	fmt.Printf("Bot %q deleted successfully.\n", c.botName)

	return nil
}

func (c *BotsCommand) LockBot(ctx context.Context, client auth.ClientI) error {
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

	if err := client.UpsertLock(ctx, lock); err != nil {
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
