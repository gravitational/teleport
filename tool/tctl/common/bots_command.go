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
	"errors"
	"fmt"
	"maps"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
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
)

type BotsCommand struct {
	format string

	lockExpires string
	lockTTL     time.Duration

	botName  string
	botRoles string
	tokenID  string
	tokenTTL time.Duration
	addRoles string

	allowedLogins []string
	addLogins     string
	setLogins     string

	botsList   *kingpin.CmdClause
	botsAdd    *kingpin.CmdClause
	botsRemove *kingpin.CmdClause
	botsLock   *kingpin.CmdClause
	botsUpdate *kingpin.CmdClause
}

// Initialize sets up the "tctl bots" command.
func (c *BotsCommand) Initialize(app *kingpin.Application, config *servicecfg.Config) {
	bots := app.Command("bots", "Operate on certificate renewal bots registered with the cluster.")

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
}

// TryRun attempts to run subcommands.
func (c *BotsCommand) TryRun(ctx context.Context, cmd string, client *authclient.Client) (match bool, err error) {
	switch cmd {
	case c.botsList.FullCommand():
		err = c.ListBots(ctx, client)
	case c.botsAdd.FullCommand():
		err = c.AddBot(ctx, client)
	case c.botsRemove.FullCommand():
		err = c.RemoveBot(ctx, client)
	case c.botsLock.FullCommand():
		err = c.LockBot(ctx, client)
	case c.botsUpdate.FullCommand():
		err = c.UpdateBot(ctx, client)
	default:
		return false, nil
	}

	return true, trace.Wrap(err)
}

// TODO(noah): DELETE IN 16.0.0
func (c *BotsCommand) listBotsLegacy(ctx context.Context, client *authclient.Client) error {
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
		err := utils.WriteJSONArray(os.Stdout, users)
		if err != nil {
			return trace.Wrap(err, "failed to marshal users")
		}
	}
	return nil
}

// ListBots writes a listing of the cluster's certificate renewal bots
// to standard out.
func (c *BotsCommand) ListBots(ctx context.Context, client *authclient.Client) error {
	var bots []*machineidv1pb.Bot
	req := &machineidv1pb.ListBotsRequest{}
	for {
		resp, err := client.BotServiceClient().ListBots(ctx, req)
		if err != nil {
			if trace.IsNotImplemented(err) {
				return trace.Wrap(c.listBotsLegacy(ctx, client))
			}
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
			fmt.Println("No bots found")
			return nil
		}
		t := asciitable.MakeTable([]string{"Bot", "User", "Roles"})
		for _, u := range bots {
			t.AddRow([]string{
				u.Metadata.Name, u.Status.UserName, strings.Join(u.Spec.GetRoles(), ","),
			})
		}
		fmt.Println(t.AsBuffer().String())
	} else {
		err := utils.WriteJSONArray(os.Stdout, bots)
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
  - {{.addr}} must be reachable from the new node
`))

// TODO(noah): DELETE IN 16.0.0
func (c *BotsCommand) addBotLegacy(ctx context.Context, client *authclient.Client) error {
	roles := splitEntries(c.botRoles)
	if len(roles) == 0 {
		log.Warning("No roles specified. The bot will not be able to produce outputs until a role is added to the bot.")
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
		return trace.Wrap(err, "creating bot")
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
		return trace.Errorf("bot was created but this cluster does not have any proxy servers running so unable to display success message")
	}
	addr := proxies[0].GetPublicAddr()
	if addr == "" {
		addr = proxies[0].GetAddr()
	}

	joinMethod := response.JoinMethod
	if joinMethod == types.JoinMethodUnspecified {
		joinMethod = types.JoinMethodToken
	}

	return startMessageTemplate.Execute(os.Stdout, map[string]interface{}{
		"token":       response.TokenID,
		"minutes":     int(time.Duration(response.TokenTTL).Minutes()),
		"addr":        addr,
		"join_method": joinMethod,
	})
}

// AddBot adds a new certificate renewal bot to the cluster.
func (c *BotsCommand) AddBot(ctx context.Context, client *authclient.Client) error {
	// Prompt for admin action MFA if required, allowing reuse for UpsertToken and CreateBot.
	mfaResponse, err := mfa.PerformAdminActionMFACeremony(ctx, client.PerformMFACeremony, true /*allowReuse*/)
	if err == nil {
		ctx = mfa.ContextWithMFAResponse(ctx, mfaResponse)
	} else if !errors.Is(err, &mfa.ErrMFANotRequired) && !errors.Is(err, &mfa.ErrMFANotSupported) {
		return trace.Wrap(err)
	}

	// Jankily call the endpoint invalidly. This lets us version check and use
	// the legacy version of this CLI tool if we are talking to an older
	// server.
	// DELETE IN 16.0
	{
		_, err := client.BotServiceClient().CreateBot(ctx, &machineidv1pb.CreateBotRequest{
			Bot: nil,
		})
		if trace.IsNotImplemented(err) {
			return trace.Wrap(c.addBotLegacy(ctx, client))
		}
	}

	roles := splitEntries(c.botRoles)
	if len(roles) == 0 {
		log.Warning("No roles specified. The bot will not be able to produce outputs until a role is added to the bot.")
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

	if c.format == teleport.JSON {
		tokenTTL := time.Duration(0)
		if exp := token.Expiry(); !exp.IsZero() {
			tokenTTL = time.Until(exp)
		}
		// This struct is equivalent to a legacy bit of JSON we used to output
		// when we called an older RPC. We've preserved it here to avoid
		// breaking customer scripts.
		response := struct {
			UserName string        `json:"user_name"`
			RoleName string        `json:"role_name"`
			TokenID  string        `json:"token_id"`
			TokenTTL time.Duration `json:"token_ttl"`
		}{
			UserName: bot.Status.UserName,
			RoleName: bot.Status.RoleName,
			TokenID:  token.GetName(),
			TokenTTL: tokenTTL,
		}
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
		return trace.Errorf("bot was created but this cluster does not have any proxy servers running so unable to display success message")
	}
	addr := proxies[0].GetPublicAddr()
	if addr == "" {
		addr = proxies[0].GetAddr()
	}

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
	return startMessageTemplate.Execute(os.Stdout, templateData)
}

func (c *BotsCommand) RemoveBot(ctx context.Context, client *authclient.Client) error {
	_, err := client.BotServiceClient().DeleteBot(ctx, &machineidv1pb.DeleteBotRequest{
		BotName: c.botName,
	})
	if err != nil {
		if trace.IsNotImplemented(err) {
			// This falls back to the deprecated RPC.
			// TODO(noah): DELETE IN 16.0.0
			if err := client.DeleteBot(ctx, c.botName); err != nil {
				return trace.Wrap(err, "error deleting bot")
			}
		} else {
			return trace.Wrap(err)
		}
	}

	fmt.Printf("Bot %q deleted successfully.\n", c.botName)

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

	fmt.Printf("Created a lock with name %q.\n", lock.GetName())

	return nil
}

// updateBotLogins applies updates from CLI arguments to a bot's logins trait,
// updating the field mask if any updates were made.
func (c *BotsCommand) updateBotLogins(bot *machineidv1pb.Bot, mask *fieldmaskpb.FieldMask) error {
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
		log.Infof("Logins will be left unchanged: %+v", desiredLoginsArray)
		return nil
	}

	log.Infof("Desired logins for bot %q: %+v", c.botName, desiredLoginsArray)

	if len(desiredLogins) == 0 {
		delete(traits, constants.TraitLogins)
		log.Infof("Removing logins trait from bot user")
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
		log.Infof("Roles will be left unchanged: %+v", desiredRolesArray)
		return nil
	}

	log.Infof("Desired roles for bot %q:  %+v", c.botName, desiredRolesArray)

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
		if err := c.updateBotLogins(bot, fieldMask); err != nil {
			return trace.Wrap(err)
		}
	}

	if c.botRoles != "" || c.addRoles != "" {
		if err := c.updateBotRoles(ctx, client, bot, fieldMask); err != nil {
			return trace.Wrap(err)
		}
	}

	if len(fieldMask.Paths) == 0 {
		log.Infof("No changes requested, nothing to do.")
		return nil
	}

	_, err = client.BotServiceClient().UpdateBot(ctx, &machineidv1pb.UpdateBotRequest{
		Bot:        bot,
		UpdateMask: fieldMask,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	log.Infof("Bot %q has been updated. Roles will take effect on its next renewal.", c.botName)

	return nil
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
