/*
Copyright 2021 Gravitational, Inc.

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
	"fmt"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/kingpin"
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/trace"
)

const BOT_LABEL = "teleport-bot"

type BotsCommand struct {
	format string

	lockExpires string
	lockTTL     time.Duration

	botName  string
	botRoles string

	botsList   *kingpin.CmdClause
	botsAdd    *kingpin.CmdClause
	botsRemove *kingpin.CmdClause
	botsLock   *kingpin.CmdClause
	botsUnlock *kingpin.CmdClause
}

// Initialize sets up the "tctl bots" command.
func (c *BotsCommand) Initialize(app *kingpin.Application, config *service.Config) {
	bots := app.Command("bots", "Operate on certificate renewal bots registered with the cluster.")

	c.botsList = bots.Command("ls", "List all certificate renewal bots registered with the cluster.")
	c.botsList.Flag("format", "Output format, 'text', 'json', or 'yaml'").Default("text").StringVar(&c.format)

	c.botsAdd = bots.Command("add", "Add a new certificate renewal bot to the cluster.")
	c.botsAdd.Flag("name", "A name to uniquely identify this bot in the cluster.").Required().StringVar(&c.botName)
	c.botsAdd.Flag("roles", "Roles the bot is able to assume.").Required().StringVar(&c.botRoles)
	// TODO: --token for optionally specifying the join token to use?
	// TODO: --ttl for setting a ttl on the join token

	c.botsRemove = bots.Command("rm", "Permanently remove a certificate renewal bot from the cluster.")
	c.botsRemove.Arg("name", "Name of an existing bot to remove.").Required().StringVar(&c.botName)

	c.botsLock = bots.Command("lock", "Prevent a bot from renewing its certificates.")
	c.botsLock.Flag("expires", "Time point (RFC3339) when the lock expires.").StringVar(&c.lockExpires)
	c.botsLock.Flag("ttl", "Time duration after which the lock expires.").DurationVar(&c.lockTTL)
	c.botsLock.Hidden() // TODO
	// TODO: id/name flag or arg instead? what do other commands do?

	c.botsUnlock = bots.Command("unlock", "Unlock a locked bot, allowing it to resume renewing certificates.")
	c.botsUnlock.Hidden() // TODO
}

// TryRun attemps to run subcommands.
func (c *BotsCommand) TryRun(cmd string, client auth.ClientI) (match bool, err error) {
	// TODO: create a smaller interface - we don't need all of ClientI

	switch cmd {
	case c.botsList.FullCommand():
		err = c.ListBots(client)
	case c.botsAdd.FullCommand():
		err = c.AddBot(client)
	case c.botsRemove.FullCommand():
		err = c.RemoveBot(client)
	case c.botsLock.FullCommand():
		err = c.LockBot(client)
	case c.botsUnlock.FullCommand():
		err = c.UnlockBot(client)
	default:
		return false, nil
	}

	return true, trace.Wrap(err)
}

// TODO: define a smaller interface than auth.ClientI for the CLI commands
// (we only use a small subset of the giant ClientI interface)

// ListBots writes a listing of the cluster's certificate renewal bots
// to standard out.
func (c *BotsCommand) ListBots(client auth.ClientI) error {
	// TODO: replace with a user query
	// bots, err := client.GetBots(context.TODO(), apidefaults.Namespace)
	// if err != nil {
	// 	return trace.Wrap(err)
	// }

	// TODO: handle format (JSON, etc)
	// TODO: collection is going to also need locks so it can write that status
	// err = (&botCollection{bots: bots}).writeText(os.Stdout)
	// if err != nil {
	// 	return trace.Wrap(err)
	// }

	return trace.NotImplemented("bots ls not implemented")
}

var startMessageTemplate = template.Must(template.New("node").Parse(`The bot token: {{.token}}
This token will expire in {{.minutes}} minutes.

Run this on the new bot node to join the cluster:

> tbot start \
   --data-dir=./tbotdata \
   --token={{.token}} \{{range .ca_pins}}
   --ca-pin={{.}} \{{end}}
   --auth-server={{.auth_server}}

Please note:

  - This invitation token will expire in {{.minutes}} minutes
  - {{.auth_server}} must be reachable from the new node
`))

// AddBot adds a new certificate renewal bot to the cluster.
func (c *BotsCommand) AddBot(client auth.ClientI) error {
	// At this point, we don't know whether the bot will be used to generate
	// user certs, host certs, or both. We create a user and a host join token
	// so the bot will just work in either mode.
	userName := "bot-" + strings.ReplaceAll(c.botName, " ", "-")

	_, err := client.GetRole(context.Background(), userName)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if roleExists := (err == nil); roleExists {
		return trace.AlreadyExists("cannot add bot: role %q already exists", userName)
	}

	if err := c.addBotRole(client, c.botName, userName); err != nil {
		return trace.Wrap(err)
	}

	user, err := types.NewUser(userName)
	if err != nil {
		return trace.Wrap(err)
	}

	roles := []string{userName}
	roles = append(roles, splitRoles(c.botRoles)...)
	user.SetRoles(roles)

	metadata := user.GetMetadata()
	metadata.Labels = map[string]string{
		BOT_LABEL: c.botName,
	}
	user.SetMetadata(metadata)

	user.SetTraits(map[string][]string{
		teleport.TraitLogins:     {},
		teleport.TraitKubeUsers:  {},
		teleport.TraitKubeGroups: {},
	})

	if err := client.CreateUser(context.TODO(), user); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Created bot user: %s\n", userName)

	// TODO: make this user configurable via CLI?
	ttl := time.Hour * 24 * 7

	// TODO: we create a User for the bot. CreateBotJoinToken authorizes for
	// Update/Bot, even though we then create a token for the associated User.
	// Is this sane?

	// Create the user token, used by the bot to generate user SSH certificates.
	userToken, err := client.CreateBotJoinToken(context.TODO(), auth.CreateUserTokenRequest{
		Name: userName,
		TTL:  time.Hour,
		Type: auth.UserTokenTypeBot,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// Create the node join token, used by the bot to join the cluster and fetch
	// host certificates.
	// To ease the UX, we'll now create a host token that explicitly re-uses
	// the user token.
	// TODO: can the bot join as only one type (auto-expiring the other), or
	// should it always join as both?
	token, err := client.GenerateToken(context.Background(), auth.GenerateTokenRequest{
		Roles:  types.SystemRoles{types.RoleProvisionToken, types.RoleNode},
		TTL:    ttl,
		Token:  userToken.GetName(),
		Labels: map[string]string{"bot": c.botName},
	})
	if err != nil {
		return trace.Wrap(err)
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

	return startMessageTemplate.Execute(os.Stdout, map[string]interface{}{
		"token":       token,
		"minutes":     int(ttl.Minutes()),
		"ca_pins":     caPins,
		"auth_server": addr,
	})
}

func (c *BotsCommand) addBotRole(client auth.ClientI, botName, userName string) error {
	return client.UpsertRole(context.Background(), &types.RoleV4{
		Kind:    types.KindRole,
		Version: types.V4,
		Metadata: types.Metadata{
			Name:        userName,
			Description: fmt.Sprintf("Automatically generated role for bot %s", c.botName),
			Labels: map[string]string{
				BOT_LABEL: botName,
			},
		},
		Spec: types.RoleSpecV4{
			Options: types.RoleOptions{
				// TODO: inherit TTLs from cert length?
				MaxSessionTTL: types.Duration(12 * time.Hour),
			},
			Allow: types.RoleConditions{
				Rules: []types.Rule{
					// read certificate authorities to watch for CA rotations
					types.NewRule(types.KindCertAuthority, []string{types.VerbReadNoSecrets}),
				},
			},
		},
	})
}

func (c *BotsCommand) addImpersonatorRole(client auth.ClientI, userName, roleName string) error {
	return client.UpsertRole(context.Background(), &types.RoleV4{
		Kind:    types.KindRole,
		Version: types.V4,
		Metadata: types.Metadata{
			Name:        roleName,
			Description: fmt.Sprintf("Automatically generated impersonator role for certificate renewal bot %s", c.botName),
		},
		Spec: types.RoleSpecV4{
			Options: types.RoleOptions{
				MaxSessionTTL: types.Duration(12 * time.Hour),
			},
			Allow: types.RoleConditions{
				Rules: []types.Rule{
					// read certificate authorities to watch for CA rotations
					types.NewRule(types.KindCertAuthority, []string{types.VerbReadNoSecrets}),
				},
				Impersonate: &types.ImpersonateConditions{
					Roles: splitRoles(c.botRoles),
					Users: []string{userName},
				},
			},
		},
	})
}

func (c *BotsCommand) RemoveBot(client auth.ClientI) error {
	// TODO:
	// remove the bot's associated impersonator role
	// remove any locks for the bot's impersonator role?
	// remove the bot's user
	userName := "bot-" + strings.ReplaceAll(c.botName, " ", "-")

	user, userErr := client.GetUser(userName, false)
	if userErr != nil {
		userErr = trace.WrapWithMessage(userErr, "could not fetch expected bot user %s", userName)
	} else {
		label, ok := user.GetMetadata().Labels[BOT_LABEL]
		if !ok {
			userErr = trace.Errorf("will not delete user %s that is missing label %s", userName, BOT_LABEL)
		} else if label != c.botName {
			userErr = trace.Errorf("will not delete user %s with mismatched label %s = %s", userName, BOT_LABEL, label)
		} else if userErr = client.DeleteUser(context.Background(), userName); userErr == nil {
			fmt.Printf("Removed bot user %s.\n", userName)
		}
	}

	role, roleErr := client.GetRole(context.Background(), userName)
	if roleErr != nil {
		roleErr = trace.WrapWithMessage(roleErr, "could not fetch expected bot role %s", userName)
	} else {
		label, ok := role.GetMetadata().Labels[BOT_LABEL]
		if !ok {
			roleErr = trace.Errorf("will not delete role %s that is missing label %s", userName, BOT_LABEL)
		} else if label != c.botName {
			roleErr = trace.Errorf("will not delete role %s with mismatched label %s = %s", userName, BOT_LABEL, label)
		} else if roleErr = client.DeleteRole(context.Background(), userName); roleErr == nil {
			fmt.Printf("Removed bot role %s.\n", userName)
		}
	}

	// TODO: locks, tokens?

	return trace.NewAggregate(userErr, roleErr)
}

func (c *BotsCommand) LockBot(client auth.ClientI) error {
	lockExpiry, err := computeLockExpiry(c.lockExpires, c.lockTTL)
	if err != nil {
		return trace.Wrap(err)
	}

	lock, err := types.NewLock(uuid.New().String(), types.LockSpecV2{
		Target:  types.LockTarget{}, // TODO: fill in role for impersonator
		Expires: lockExpiry,
		Message: "The certificate renewal bot associated with this role has been locked.",
	})
	if err != nil {
		return trace.Wrap(err)
	}

	if err := client.UpsertLock(context.Background(), lock); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (c *BotsCommand) UnlockBot(client auth.ClientI) error {
	// find the lock with a target role corresponding to this bot and remove it
	return trace.NotImplemented("")
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
