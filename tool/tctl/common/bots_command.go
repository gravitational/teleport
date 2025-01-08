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
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/machineid/machineidv1"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

type BotsCommand struct {
	format string

	lockExpires string
	lockTTL     time.Duration

	botName    string
	botRoles   string
	tokenID    string
	tokenTTL   time.Duration
	addRoles   string
	instanceID string

	allowedLogins []string
	addLogins     string
	setLogins     string

	botsList          *kingpin.CmdClause
	botsAdd           *kingpin.CmdClause
	botsRemove        *kingpin.CmdClause
	botsLock          *kingpin.CmdClause
	botsUpdate        *kingpin.CmdClause
	botsInstances     *kingpin.CmdClause
	botsInstancesShow *kingpin.CmdClause
	botsInstancesList *kingpin.CmdClause
	botsInstancesAdd  *kingpin.CmdClause

	stdout io.Writer
}

// Initialize sets up the "tctl bots" command.
func (c *BotsCommand) Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, config *servicecfg.Config) {
	bots := app.Command("bots", "Manage Machine ID bots on the cluster.").Alias("bot")

	c.botsList = bots.Command("ls", "List all certificate renewal bots registered with the cluster.")
	c.botsList.Flag("format", "Output format, 'text' or 'json'").Hidden().Default(teleport.Text).EnumVar(&c.format, teleport.Text, teleport.JSON)

	c.botsAdd = bots.Command("add", "Add a new certificate renewal bot to the cluster.")
	c.botsAdd.Arg("name", "A name to uniquely identify this bot in the cluster.").Required().StringVar(&c.botName)
	c.botsAdd.Flag("roles", "Roles the bot is able to assume.").StringVar(&c.botRoles)
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

	c.botsUpdate = bots.Command("update", "Update an existing bot.")
	c.botsUpdate.Arg("name", "Name of an existing bot to update.").Required().StringVar(&c.botName)
	c.botsUpdate.Flag("set-roles", "Sets the bot's roles to the given comma-separated list, replacing any existing roles.").StringVar(&c.botRoles)
	c.botsUpdate.Flag("add-roles", "Adds a comma-separated list of roles to an existing bot.").StringVar(&c.addRoles)
	c.botsUpdate.Flag("set-logins", "Sets the bot's logins to the given comma-separated list, replacing any existing logins.").StringVar(&c.setLogins)
	c.botsUpdate.Flag("add-logins", "Adds a comma-separated list of logins to an existing bot.").StringVar(&c.addLogins)

	c.botsInstances = bots.Command("instances", "Manage bot instances.").Alias("instance")

	c.botsInstancesShow = c.botsInstances.Command("show", "Shows information about a specific bot instance.").Alias("get").Alias("describe")
	c.botsInstancesShow.Arg("id", "The full ID of the bot instance, in the form of [bot name]/[uuid]").Required().StringVar(&c.instanceID)

	c.botsInstancesList = c.botsInstances.Command("list", "List bot instances.").Alias("ls")
	c.botsInstancesList.Arg("name", "The name of the bot from which to list instances. If unset, lists instances from all bots.").StringVar(&c.botName)

	c.botsInstancesAdd = c.botsInstances.Command("add", "Join a new instance onto an existing bot.").Alias("join")
	c.botsInstancesAdd.Arg("name", "The name of the existing bot for which to add a new instance.").Required().StringVar(&c.botName)
	c.botsInstancesAdd.Flag("token", "The token to use, if any. If unset, a new one-time-use token will be created.").StringVar(&c.tokenID)
	c.botsInstancesAdd.Flag("format", "Output format, one of: text, json").Default(teleport.Text).EnumVar(&c.format, teleport.Text, teleport.JSON)

	if c.stdout == nil {
		c.stdout = os.Stdout
	}
}

// TryRun attempts to run subcommands.
func (c *BotsCommand) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (match bool, err error) {
	var commandFunc func(ctx context.Context, client *authclient.Client) error
	switch cmd {
	case c.botsList.FullCommand():
		commandFunc = c.ListBots
	case c.botsAdd.FullCommand():
		commandFunc = c.AddBot
	case c.botsRemove.FullCommand():
		commandFunc = c.RemoveBot
	case c.botsLock.FullCommand():
		commandFunc = c.LockBot
	case c.botsUpdate.FullCommand():
		commandFunc = c.UpdateBot
	case c.botsInstancesShow.FullCommand():
		commandFunc = c.ShowBotInstance
	case c.botsInstancesList.FullCommand():
		commandFunc = c.ListBotInstances
	case c.botsInstancesAdd.FullCommand():
		commandFunc = c.AddBotInstance
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

// ListBots writes a listing of the cluster's certificate renewal bots
// to standard out.
func (c *BotsCommand) ListBots(ctx context.Context, client *authclient.Client) error {
	var bots []*machineidv1pb.Bot
	req := &machineidv1pb.ListBotsRequest{}
	for {
		resp, err := client.BotServiceClient().ListBots(ctx, req)
		if err != nil {
			return trace.Wrap(err)
		}

		bots = append(bots, resp.Bots...)
		if resp.NextPageToken == "" {
			break
		}
		req.PageToken = resp.NextPageToken
	}

	if c.format == teleport.Text {
		if len(bots) == 0 {
			fmt.Fprintln(c.stdout, "No bots found")
			return nil
		}
		t := asciitable.MakeTable([]string{"Bot", "User", "Roles"})
		for _, u := range bots {
			t.AddRow([]string{
				u.Metadata.Name, u.Status.UserName, strings.Join(u.Spec.GetRoles(), ","),
			})
		}
		fmt.Fprintln(c.stdout, t.AsBuffer().String())

		fmt.Fprintf(c.stdout, "\nTo view active instances of a bot, run:\n\n> %s bots instances list [name]\n", os.Args[0])
	} else {
		err := utils.WriteJSONArray(c.stdout, bots)
		if err != nil {
			return trace.Wrap(err, "failed to marshal bots")
		}
	}
	return nil
}

// bold wraps the given text in an ANSI escape to bold it
func bold(text string) string {
	return utils.Color(utils.Bold, text)
}

var startMessageTemplate = template.Must(template.New("node").Funcs(template.FuncMap{
	"bold": bold,
}).Parse(`The bot token: {{.token}}{{if .minutes}}
This token will expire in {{.minutes}} minutes.{{end}}

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
   --proxy-server={{.addr}}{{if .join_method}} \
   --join-method={{.join_method}}{{end}}

Please note:

  - The ./tbot-user destination directory can be changed as desired.
  - /var/lib/teleport/bot must be accessible to the bot user, or --data-dir
    must point to another accessible directory to store internal bot data.
  - This invitation token will expire in {{.minutes}} minutes
  - {{.addr}} must be reachable from the new node{{if eq .join_method "token"}}
  - This is a single-token that will be consumed upon usage. For scalable
    alternatives, see our documentation on other supported join methods:
    https://goteleport.com/docs/enroll-resources/machine-id/deployment/{{end}}
`))

// AddBot adds a new certificate renewal bot to the cluster.
func (c *BotsCommand) AddBot(ctx context.Context, client *authclient.Client) error {
	// Prompt for admin action MFA if required, allowing reuse for UpsertToken and CreateBot.
	mfaResponse, err := mfa.PerformAdminActionMFACeremony(ctx, client.PerformMFACeremony, true /*allowReuse*/)
	if err == nil {
		ctx = mfa.ContextWithMFAResponse(ctx, mfaResponse)
	} else if !errors.Is(err, &mfa.ErrMFANotRequired) && !errors.Is(err, &mfa.ErrMFANotSupported) {
		return trace.Wrap(err)
	}

	roles := splitEntries(c.botRoles)
	if len(roles) == 0 {
		slog.WarnContext(ctx, "No roles specified - the bot will not be able to produce outputs until a role is added to the bot")
	}
	var token types.ProvisionToken
	if c.tokenID == "" {
		// If there's no token specified, generate one
		tokenName, err := utils.CryptoRandomHex(defaults.TokenLenBytes)
		if err != nil {
			return trace.Wrap(err)
		}
		ttl := c.tokenTTL
		if ttl == 0 {
			ttl = defaults.DefaultBotJoinTTL
		}
		tokenSpec := types.ProvisionTokenSpecV2{
			Roles:      types.SystemRoles{types.RoleBot},
			JoinMethod: types.JoinMethodToken,
			BotName:    c.botName,
		}
		token, err = types.NewProvisionTokenFromSpec(tokenName, time.Now().Add(ttl), tokenSpec)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := client.UpsertToken(ctx, token); err != nil {
			return trace.Wrap(err)
		}
	} else {
		// If there is, check the token matches the potential bot
		token, err = client.GetToken(ctx, c.tokenID)
		if err != nil {
			if trace.IsNotFound(err) {
				return trace.NotFound("token with name %q not found, create the token or do not set TokenName: %v",
					c.tokenID, err)
			}
			return trace.Wrap(err)
		}
		if !token.GetRoles().Include(types.RoleBot) {
			return trace.BadParameter("token %q is not valid for role %q",
				c.tokenID, types.RoleBot)
		}
		if token.GetBotName() != c.botName {
			return trace.BadParameter("token %q is valid for bot with name %q, not %q",
				c.tokenID, token.GetBotName(), c.botName)
		}
	}

	bot := &machineidv1pb.Bot{
		Metadata: &headerv1.Metadata{
			Name: c.botName,
		},
		Spec: &machineidv1pb.BotSpec{
			Roles: roles,
			Traits: []*machineidv1pb.Trait{
				{
					Name:   constants.TraitLogins,
					Values: flattenSlice(c.allowedLogins),
				},
			},
		},
	}

	bot, err = client.BotServiceClient().CreateBot(ctx, &machineidv1pb.CreateBotRequest{
		Bot: bot,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(outputToken(c.stdout, c.format, client, bot, token))
}

func (c *BotsCommand) RemoveBot(ctx context.Context, client *authclient.Client) error {
	_, err := client.BotServiceClient().DeleteBot(ctx, &machineidv1pb.DeleteBotRequest{
		BotName: c.botName,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Fprintf(c.stdout, "Bot %q deleted successfully.\n", c.botName)

	return nil
}

func (c *BotsCommand) LockBot(ctx context.Context, client *authclient.Client) error {
	lockExpiry, err := computeLockExpiry(c.lockExpires, c.lockTTL)
	if err != nil {
		return trace.Wrap(err)
	}

	user, err := client.GetUser(ctx, machineidv1.BotResourceName(c.botName), false)
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

	fmt.Fprintf(c.stdout, "Created a lock with name %q.\n", lock.GetName())

	return nil
}

// updateBotLogins applies updates from CLI arguments to a bot's logins trait,
// updating the field mask if any updates were made.
func (c *BotsCommand) updateBotLogins(ctx context.Context, bot *machineidv1pb.Bot, mask *fieldmaskpb.FieldMask) error {
	traits := map[string][]string{}
	for _, t := range bot.Spec.GetTraits() {
		traits[t.Name] = t.Values
	}

	currentLogins := make(map[string]struct{})
	if logins, exists := traits[constants.TraitLogins]; exists {
		for _, login := range logins {
			currentLogins[login] = struct{}{}
		}
	}

	var desiredLogins map[string]struct{}
	if c.setLogins != "" {
		desiredLogins = make(map[string]struct{})
		for _, login := range splitEntries(c.setLogins) {
			desiredLogins[login] = struct{}{}
		}
	} else {
		desiredLogins = maps.Clone(currentLogins)
	}

	addLogins := splitEntries(c.addLogins)
	if len(addLogins) > 0 {
		for _, login := range addLogins {
			desiredLogins[login] = struct{}{}
		}
	}

	desiredLoginsArray := utils.StringsSliceFromSet(desiredLogins)

	if maps.Equal(currentLogins, desiredLogins) {
		slog.InfoContext(ctx, "Logins will be left unchanged", "logins", desiredLoginsArray)
		return nil
	}

	slog.InfoContext(ctx, "Desired logins for bot", "bot", c.botName, "logins", desiredLoginsArray)

	if len(desiredLogins) == 0 {
		delete(traits, constants.TraitLogins)
		slog.InfoContext(ctx, "Removing logins trait from bot user")
	} else {
		traits[constants.TraitLogins] = desiredLoginsArray
	}

	traitsArray := []*machineidv1pb.Trait{}
	for k, v := range traits {
		traitsArray = append(traitsArray, &machineidv1pb.Trait{
			Name:   k,
			Values: v,
		})
	}

	bot.Spec.Traits = traitsArray

	return trace.Wrap(mask.Append(&machineidv1pb.Bot{}, "spec.traits"))
}

// clientRoleGetter is a minimal mockable interface for the client API
type clientRoleGetter interface {
	GetRole(context.Context, string) (types.Role, error)
}

// updateBotRoles applies updates from CLI arguments to a bot's roles, updating
// the field mask as necessary if any updates were made.
func (c *BotsCommand) updateBotRoles(ctx context.Context, client clientRoleGetter, bot *machineidv1pb.Bot, mask *fieldmaskpb.FieldMask) error {
	currentRoles := make(map[string]struct{})
	for _, role := range bot.Spec.Roles {
		currentRoles[role] = struct{}{}
	}

	var desiredRoles map[string]struct{}
	if c.botRoles != "" {
		desiredRoles = make(map[string]struct{})
		for _, role := range splitEntries(c.botRoles) {
			desiredRoles[role] = struct{}{}
		}
	} else {
		desiredRoles = maps.Clone(currentRoles)
	}

	if c.addRoles != "" {
		for _, role := range splitEntries(c.addRoles) {
			desiredRoles[role] = struct{}{}
		}
	}

	desiredRolesArray := utils.StringsSliceFromSet(desiredRoles)

	if maps.Equal(currentRoles, desiredRoles) {
		slog.InfoContext(ctx, "Roles will be left unchanged", "roles", desiredRolesArray)
		return nil
	}

	slog.InfoContext(ctx, "Desired roles for bot", "bot", c.botName, "roles", desiredRolesArray)

	// Validate roles (server does not do this yet).
	for roleName := range desiredRoles {
		if _, err := client.GetRole(ctx, roleName); err != nil {
			return trace.Wrap(err)
		}
	}

	bot.Spec.Roles = desiredRolesArray

	return trace.Wrap(mask.Append(&machineidv1pb.Bot{}, "spec.roles"))
}

// UpdateBot performs various updates to existing bot users and roles.
func (c *BotsCommand) UpdateBot(ctx context.Context, client *authclient.Client) error {
	bot, err := client.BotServiceClient().GetBot(ctx, &machineidv1pb.GetBotRequest{
		BotName: c.botName,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	fieldMask, err := fieldmaskpb.New(&machineidv1pb.Bot{})
	if err != nil {
		return trace.Wrap(err)
	}

	if c.setLogins != "" || c.addLogins != "" {
		if err := c.updateBotLogins(ctx, bot, fieldMask); err != nil {
			return trace.Wrap(err)
		}
	}

	if c.botRoles != "" || c.addRoles != "" {
		if err := c.updateBotRoles(ctx, client, bot, fieldMask); err != nil {
			return trace.Wrap(err)
		}
	}

	if len(fieldMask.Paths) == 0 {
		slog.InfoContext(ctx, "No changes requested, nothing to do")
		return nil
	}

	_, err = client.BotServiceClient().UpdateBot(ctx, &machineidv1pb.UpdateBotRequest{
		Bot:        bot,
		UpdateMask: fieldMask,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	slog.InfoContext(ctx, "Bot has been updated, roles will take effect on its next renewal", "bot", c.botName)

	return nil
}

// ListBotInstances lists bot instances, possibly filtering for a specific bot
func (c *BotsCommand) ListBotInstances(ctx context.Context, client *authclient.Client) error {
	var instances []*machineidv1pb.BotInstance
	req := &machineidv1pb.ListBotInstancesRequest{}

	if c.botName != "" {
		req.FilterBotName = c.botName
	}

	for {
		resp, err := client.BotInstanceServiceClient().ListBotInstances(ctx, req)
		if err != nil {
			return trace.Wrap(err)
		}

		instances = append(instances, resp.BotInstances...)
		if resp.NextPageToken == "" {
			break
		}
		req.PageToken = resp.NextPageToken
	}

	if c.format == teleport.JSON {
		err := utils.WriteJSONArray(c.stdout, instances)
		if err != nil {
			return trace.Wrap(err, "failed to marshal bot instances")
		}

		return nil
	}

	if len(instances) == 0 {
		if c.botName == "" {
			fmt.Fprintln(c.stdout, "No bot instances found.")
		} else {
			fmt.Fprintf(c.stdout, "No bot instances found with name %q.\n", c.botName)
		}
		return nil
	}

	t := asciitable.MakeTable([]string{"ID", "Join Method", "Hostname", "Joined", "Last Seen", "Generation"})
	for _, i := range instances {
		var (
			joinMethod string
			hostname   string
			generation string
		)

		joined := i.Status.InitialAuthentication.AuthenticatedAt.AsTime().Format(time.RFC3339)
		initialJoinMethod := cmp.Or(
			i.Status.InitialAuthentication.GetJoinAttrs().GetMeta().GetJoinMethod(),
			i.Status.InitialAuthentication.JoinMethod,
		)

		lastSeen := i.Status.InitialAuthentication.AuthenticatedAt.AsTime()

		if len(i.Status.LatestAuthentications) == 0 {
			generation = "n/a"
		} else {
			auth := i.Status.LatestAuthentications[len(i.Status.LatestAuthentications)-1]

			generation = fmt.Sprint(auth.Generation)

			authJM := cmp.Or(
				auth.GetJoinAttrs().GetMeta().GetJoinMethod(),
				auth.JoinMethod,
			)
			if authJM == initialJoinMethod {
				joinMethod = authJM
			} else {
				// If the join method changed, show the original method and latest
				joinMethod = fmt.Sprintf("%s (%s)", auth.JoinMethod, initialJoinMethod)
			}

			if auth.AuthenticatedAt.AsTime().After(lastSeen) {
				lastSeen = auth.AuthenticatedAt.AsTime()
			}
		}

		if len(i.Status.LatestHeartbeats) == 0 {
			hostname = "n/a"
		} else {
			hb := i.Status.LatestHeartbeats[len(i.Status.LatestHeartbeats)-1]

			hostname = hb.Hostname

			if hb.RecordedAt.AsTime().After(lastSeen) {
				lastSeen = hb.RecordedAt.AsTime()
			}
		}

		t.AddRow([]string{
			fmt.Sprintf("%s/%s", i.Spec.BotName, i.Spec.InstanceId), joinMethod,
			hostname, joined, lastSeen.Format(time.RFC3339), generation,
		})
	}
	fmt.Fprintln(c.stdout, t.AsBuffer().String())

	fmt.Fprintf(c.stdout, "\nTo view more information on a particular instance, run:\n\n> %s bots instances show [id]\n", os.Args[0])

	if c.botName != "" {
		fmt.Fprintf(c.stdout, "\nTo onboard a new instance for this bot, run:\n\n> %s bots instances add %s\n", os.Args[0], c.botName)
	}

	return nil
}

// AddBotInstance begins onboarding a new instance of an existing bot.
func (c *BotsCommand) AddBotInstance(ctx context.Context, client *authclient.Client) error {
	// A bit of a misnomer but makes the terminology a bit more consistent. This
	// doesn't directly create a bot instance, but creates token that allows a
	// bot to join, which creates a new instance.

	bot, err := client.BotServiceClient().GetBot(ctx, &machineidv1pb.GetBotRequest{
		BotName: c.botName,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	var token types.ProvisionToken

	if c.tokenID == "" {
		// If there's no token specified, generate one
		tokenName, err := utils.CryptoRandomHex(defaults.TokenLenBytes)
		if err != nil {
			return trace.Wrap(err)
		}
		ttl := cmp.Or(c.tokenTTL, defaults.DefaultBotJoinTTL)
		tokenSpec := types.ProvisionTokenSpecV2{
			Roles:      types.SystemRoles{types.RoleBot},
			JoinMethod: types.JoinMethodToken,
			BotName:    c.botName,
		}
		token, err = types.NewProvisionTokenFromSpec(tokenName, time.Now().Add(ttl), tokenSpec)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := client.UpsertToken(ctx, token); err != nil {
			return trace.Wrap(err)
		}

		return trace.Wrap(outputToken(c.stdout, c.format, client, bot, token))
	}

	// There's not much to do in this case, but we can validate the token.
	// The bot and token should already exist in this case, so we'll just
	// print joining instructions.

	// If there is, check the token matches the potential bot
	token, err = client.GetToken(ctx, c.tokenID)
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("token with name %q not found, create the token or do not set TokenName: %v",
				c.tokenID, err)
		}
		return trace.Wrap(err)
	}
	if !token.GetRoles().Include(types.RoleBot) {
		return trace.BadParameter("token %q is not valid for role %q",
			c.tokenID, types.RoleBot)
	}
	if token.GetBotName() != c.botName {
		return trace.BadParameter("token %q is valid for bot with name %q, not %q",
			c.tokenID, token.GetBotName(), c.botName)
	}

	return trace.Wrap(outputToken(c.stdout, c.format, client, bot, token))
}

var showMessageTemplate = template.Must(template.New("show").Funcs(template.FuncMap{
	"bold": bold,
}).Parse(`Bot: {{.instance.Spec.BotName}}
ID:  {{.instance.Spec.InstanceId}}

Initial Authentication: {{.initial_authentication_table}}

Latest Authentication: {{.latest_authentication_table}}

Latest Heartbeat: {{.heartbeat_table}}

To view a full, machine-readable record including past heartbeats and
authentication records, run:

> {{.executable}} get bot_instance/{{.instance.Spec.BotName}}/{{.instance.Spec.InstanceId}}

To onboard a new instance for this bot, run:

> {{.executable}} bots instances add {{.instance.Spec.BotName}}
`))

func (c *BotsCommand) ShowBotInstance(ctx context.Context, client *authclient.Client) error {
	botName, instanceID, err := parseInstanceID(c.instanceID)
	if err != nil {
		return trace.Wrap(err)
	}

	instance, err := client.BotInstanceServiceClient().GetBotInstance(ctx, &machineidv1pb.GetBotInstanceRequest{
		BotName:    botName,
		InstanceId: instanceID,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	initialAuthenticationTable := formatBotInstanceAuthentication(instance.Status.InitialAuthentication)

	var latestAuthenticationTable string
	if len(instance.Status.LatestAuthentications) > 0 {
		latest := instance.Status.LatestAuthentications[len(instance.Status.LatestAuthentications)-1]
		latestAuthenticationTable = formatBotInstanceAuthentication(latest)
	} else {
		latestAuthenticationTable = "No authentication records."
	}

	var heartbeatTable string
	if len(instance.Status.LatestHeartbeats) > 0 {
		latest := instance.Status.LatestHeartbeats[len(instance.Status.LatestHeartbeats)-1]
		heartbeatTable = formatBotInstanceHeartbeat(latest)
	} else {
		heartbeatTable = "No heartbeat records."
	}

	templateData := map[string]interface{}{
		"executable":                   os.Args[0],
		"instance":                     instance,
		"initial_authentication_table": initialAuthenticationTable,
		"latest_authentication_table":  latestAuthenticationTable,
		"heartbeat_table":              heartbeatTable,
	}

	return trace.Wrap(showMessageTemplate.Execute(os.Stdout, templateData))
}

// botJSONResponse is a response generated by the `tctl bots add` family of
// commands when the format is `json`
type botJSONResponse struct {
	UserName string        `json:"user_name"`
	RoleName string        `json:"role_name"`
	TokenID  string        `json:"token_id"`
	TokenTTL time.Duration `json:"token_ttl"`
}

// outputToken writes token information to stdout, depending on the token format.
func outputToken(wr io.Writer, format string, client *authclient.Client, bot *machineidv1pb.Bot, token types.ProvisionToken) error {
	if format == teleport.JSON {
		tokenTTL := time.Duration(0)
		if exp := token.Expiry(); !exp.IsZero() {
			tokenTTL = time.Until(exp)
		}
		// This struct is equivalent to a legacy bit of JSON we used to output
		// when we called an older RPC. We've preserved it here to avoid
		// breaking customer scripts.
		response := botJSONResponse{
			UserName: bot.Status.UserName,
			RoleName: bot.Status.RoleName,
			TokenID:  token.GetName(),
			TokenTTL: tokenTTL,
		}
		out, err := json.MarshalIndent(response, "", "  ")
		if err != nil {
			return trace.Wrap(err, "failed to marshal CreateBot response")
		}

		fmt.Fprintln(wr, string(out))
		return nil
	}

	proxies, err := client.GetProxies()
	if err != nil {
		return trace.Wrap(err)
	}
	if len(proxies) == 0 {
		return trace.Errorf("bot was created but this cluster does not have any proxy servers running so unable to display success message")
	}
	addr := cmp.Or(proxies[0].GetPublicAddr(), proxies[0].GetAddr())

	joinMethod := token.GetJoinMethod()
	if joinMethod == types.JoinMethodUnspecified {
		joinMethod = types.JoinMethodToken
	}

	templateData := map[string]interface{}{
		"token":       token.GetName(),
		"addr":        addr,
		"join_method": joinMethod,
	}
	if !token.Expiry().IsZero() {
		templateData["minutes"] = int(time.Until(token.Expiry()).Minutes())
	}
	return startMessageTemplate.Execute(wr, templateData)
}

// splitEntries splits a comma separated string into an array of entries,
// ignoring empty or whitespace-only elements.
func splitEntries(flag string) []string {
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

// formatBotInstanceAuthentication returns a multiline, indented string showing
// a textual representation of a bot authentication record.
func formatBotInstanceAuthentication(record *machineidv1pb.BotInstanceStatusAuthentication) string {
	table := asciitable.MakeHeadlessTable(2)
	table.AddRow([]string{"Authenticated At:", record.AuthenticatedAt.AsTime().Format(time.RFC3339)})
	table.AddRow([]string{"Join Method:", cmp.Or(record.GetJoinAttrs().GetMeta().GetJoinMethod(), record.JoinMethod)})
	table.AddRow([]string{"Join Token:", cmp.Or(record.GetJoinAttrs().GetMeta().GetJoinTokenName(), record.JoinToken)})
	var meta fmt.Stringer = record.Metadata
	if attrs := record.GetJoinAttrs(); attrs != nil {
		meta = attrs
	}
	table.AddRow([]string{"Join Metadata:", meta.String()})
	table.AddRow([]string{"Generation:", fmt.Sprint(record.Generation)})
	table.AddRow([]string{"Public Key:", fmt.Sprintf("<%d bytes>", len(record.PublicKey))})

	return "\n" + indentString(table.AsBuffer().String(), "  ")
}

// formatBotInstanceHeartbeat returns a multiline, indented string containing
// a textual representation of a bot heartbeat.
func formatBotInstanceHeartbeat(record *machineidv1pb.BotInstanceStatusHeartbeat) string {
	table := asciitable.MakeHeadlessTable(2)
	table.AddRow([]string{"Recorded At:", record.RecordedAt.AsTime().Format(time.RFC3339)})
	table.AddRow([]string{"Is Startup:", fmt.Sprint(record.IsStartup)})
	table.AddRow([]string{"Version:", record.Version})
	table.AddRow([]string{"Hostname:", record.Hostname})
	table.AddRow([]string{"Uptime:", record.Uptime.AsDuration().String()})
	table.AddRow([]string{"Join Method:", record.JoinMethod})
	table.AddRow([]string{"One Shot:", fmt.Sprint(record.OneShot)})
	table.AddRow([]string{"Architecture:", record.Architecture})
	table.AddRow([]string{"OS:", record.Os})

	return "\n" + indentString(table.AsBuffer().String(), "  ")
}

// parseInstanceID converts an instance ID string in the form of
// '[bot name]/[uuid]' to separate bot name and UUID strings.
func parseInstanceID(s string) (name string, uuid string, err error) {
	name, uuid, ok := strings.Cut(s, "/")
	if !ok {
		return "", "", trace.BadParameter("invalid bot instance syntax, must be: [bot name]/[uuid]")
	}

	return
}

// indentString prefixes each line (ending with \n) with the provided prefix.
func indentString(s string, indent string) string {
	buf := strings.Builder{}
	splits := strings.SplitAfter(s, "\n")

	for _, line := range splits {
		if line == "" {
			continue
		}

		fmt.Fprintf(&buf, "%s%s", indent, line)
	}

	return buf.String()
}
