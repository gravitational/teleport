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
	"path/filepath"
	"slices"
	"strings"
	"text/template"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/set"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

type BotsCommand struct {
	format string

	lockExpires string
	lockTTL     time.Duration

	botName       scopes.QualifiedName
	botRoles      string
	tokenID       string
	tokenTTL      time.Duration
	addRoles      string
	instanceID    string
	maxSessionTTL time.Duration

	allowedLogins []string
	addLogins     string
	setLogins     string

	search string
	query  string

	sortIndex string
	sortOrder string

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
	bots := app.Command("bots", "Manage Machine & Workload Identity bots on the cluster.").Alias("bot")

	c.botsList = bots.Command("ls", "List all certificate renewal bots registered with the cluster.")
	c.botsList.Flag("format", "Output format.").Hidden().Default(teleport.Text).EnumVar(&c.format, teleport.Text, teleport.JSON)

	c.botsAdd = bots.Command("add", "Add a new certificate renewal bot to the cluster.")
	c.botsAdd.Arg("name", "A name to uniquely identify this bot in the cluster.").Required().SetValue(&c.botName)
	c.botsAdd.Flag("roles", "Roles the bot is able to assume.").StringVar(&c.botRoles)
	c.botsAdd.Flag("ttl", "TTL for the bot join token.").DurationVar(&c.tokenTTL)
	c.botsAdd.Flag("token", "Name of an existing token to use.").StringVar(&c.tokenID)
	c.botsAdd.Flag("format", "Output format, 'text' or 'json'").Hidden().Default(teleport.Text).EnumVar(&c.format, teleport.Text, teleport.JSON)
	c.botsAdd.Flag("logins", "List of allowed SSH logins for the bot user").StringsVar(&c.allowedLogins)
	c.botsAdd.Flag("max-session-ttl", "Set a max session TTL for the bot's internal identity. 12h default, 168h maximum.").DurationVar(&c.maxSessionTTL)

	c.botsRemove = bots.Command("rm", "Permanently remove a certificate renewal bot from the cluster.")
	c.botsRemove.Arg("name", "Name of an existing bot to remove. For a scoped bot, provide a scope-qualified name of the form [scope]::[name].").Required().SetValue(&c.botName)

	c.botsLock = bots.Command("lock", "Prevent a bot from renewing its certificates.")
	c.botsLock.Arg("name", "Name of an existing bot to lock. For a scoped bot, provide a scope-qualified name of the form [scope]::[name].").Required().SetValue(&c.botName)
	c.botsLock.Flag("expires", "Time point (RFC3339) when the lock expires.").StringVar(&c.lockExpires)
	c.botsLock.Flag("ttl", "Time duration after which the lock expires.").DurationVar(&c.lockTTL)
	c.botsLock.Hidden()

	c.botsUpdate = bots.Command("update", "Update an existing bot.")
	c.botsUpdate.Arg("name", "Name of an existing bot to update.").Required().SetValue(&c.botName)
	c.botsUpdate.Flag("set-roles", "Sets the bot's roles to the given comma-separated list, replacing any existing roles.").StringVar(&c.botRoles)
	c.botsUpdate.Flag("add-roles", "Adds a comma-separated list of roles to an existing bot.").StringVar(&c.addRoles)
	c.botsUpdate.Flag("set-logins", "Sets the bot's logins to the given comma-separated list, replacing any existing logins.").StringVar(&c.setLogins)
	c.botsUpdate.Flag("add-logins", "Adds a comma-separated list of logins to an existing bot.").StringVar(&c.addLogins)
	c.botsUpdate.Flag("set-max-session-ttl", "Sets the max session TTL. 168h maximum.").DurationVar(&c.maxSessionTTL)

	c.botsInstances = bots.Command("instances", "Manage bot instances.").Alias("instance")

	c.botsInstancesShow = c.botsInstances.Command("show", "Shows information about a specific bot instance.").Alias("get").Alias("describe")
	c.botsInstancesShow.Arg("id", "The full ID of the bot instance, in the form of [bot name]/[uuid]. For an instance of a scoped bot, prefix the ID with the bot's scope: [scope]::[bot name]/[uuid].").Required().StringVar(&c.instanceID)

	c.botsInstancesList = c.botsInstances.Command("list", "List bot instances.").Alias("ls")
	c.botsInstancesList.Arg("name", "The name of the bot from which to list instances. For a scoped bot, provide a scope-qualified name of the form [scope]::[name]. If unset, lists instances from all bots.").SetValue(&c.botName)
	c.botsInstancesList.Flag("format", "Output format.").Default(teleport.Text).EnumVar(&c.format, teleport.Text, teleport.JSON)
	c.botsInstancesList.Flag("search", "Fuzzy search query used to filter bot instances").StringVar(&c.search)
	c.botsInstancesList.Flag("query", "An expression in the Teleport predicate language used to filter bot instances").StringVar(&c.query)
	c.botsInstancesList.Flag("sort-index", "Request sort index, 'bot_name', 'active_at_latest', 'version_latest' or 'host_name_latest'").Default("bot_name").StringVar(&c.sortIndex)
	c.botsInstancesList.Flag("sort-order", "Request sort order, 'ascending' or 'descending'").Default("ascending").StringVar(&c.sortOrder)

	c.botsInstancesAdd = c.botsInstances.Command("add", "Join a new instance onto an existing bot.").Alias("join")
	c.botsInstancesAdd.Arg("name", "The name of the existing bot for which to add a new instance.").Required().SetValue(&c.botName)
	c.botsInstancesAdd.Flag("token", "The token to use, if any. If unset, a new one-time-use token will be created.").StringVar(&c.tokenID)
	c.botsInstancesAdd.Flag("format", "Output format, one of: text, json").Default(teleport.Text).EnumVar(&c.format, teleport.Text, teleport.JSON)

	if c.stdout == nil {
		c.stdout = os.Stdout
	}
}

// TryRun attempts to run subcommands.
func (c *BotsCommand) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (bool, error) {
	var commandFunc func(ctx context.Context, client botsCommandClient) error
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

type botsCommandClient interface {
	BotServiceClient() machineidv1pb.BotServiceClient
	BotInstanceServiceClient() machineidv1pb.BotInstanceServiceClient

	GetToken(ctx context.Context, name string) (types.ProvisionToken, error)
	UpsertToken(ctx context.Context, token types.ProvisionToken) error
	GetUser(ctx context.Context, name string, withSecrets bool) (types.User, error)
	GetRole(context.Context, string) (types.Role, error)
	UpsertLock(ctx context.Context, lock types.Lock) error
	// Deprecated: Prefer paginated variant [ListProxyServers].
	//
	// TODO(kiosion): DELETE IN 21.0.0
	GetProxies() ([]types.Server, error)
	ListProxyServers(ctx context.Context, pageSize int, pageToken string) ([]types.Server, string, error)
	PerformMFACeremony(ctx context.Context, in *proto.CreateAuthenticateChallengeRequest, promptOpts ...mfa.PromptOpt) (*proto.MFAAuthenticateResponse, error)
}

// ListBots writes a listing of the cluster's certificate renewal bots
// to standard out.
func (c *BotsCommand) ListBots(ctx context.Context, client botsCommandClient) error {
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
			// Same-named bots in different scopes would otherwise be identical rows.
			name := scopes.QualifiedName{Scope: u.GetScope(), Name: u.GetMetadata().GetName()}.String()
			t.AddRow([]string{
				name, u.Status.UserName, strings.Join(u.Spec.GetRoles(), ","),
			})
		}
		fmt.Fprintln(c.stdout, t.AsBuffer().String())

		executableFileName := filepath.Base(os.Args[0])
		fmt.Fprintf(c.stdout, "\nTo view active instances of a bot, run:\n\n> %s bots instances list [name]\n", executableFileName)
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
func (c *BotsCommand) AddBot(ctx context.Context, client botsCommandClient) error {
	if c.botName.Scope != "" {
		return trace.Wrap(errScopedBotTokenUnsupported(c.botName, "creating"))
	}

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
			BotName:    c.botName.Name,
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
		if err := checkTokenBot(c.tokenID, token, c.botName); err != nil {
			return trace.Wrap(err)
		}
	}

	var maxSessionTTL *durationpb.Duration
	if c.maxSessionTTL > 0 {
		maxSessionTTL = durationpb.New(c.maxSessionTTL)
	}

	bot := &machineidv1pb.Bot{
		Kind:    types.KindBot,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: c.botName.Name,
		},
		Spec: &machineidv1pb.BotSpec{
			Roles: roles,
			Traits: []*machineidv1pb.Trait{
				{
					Name:   constants.TraitLogins,
					Values: flattenSlice(c.allowedLogins),
				},
			},
			MaxSessionTtl: maxSessionTTL,
		},
	}

	bot, err = client.BotServiceClient().CreateBot(ctx, &machineidv1pb.CreateBotRequest{
		Bot: bot,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(outputToken(ctx, c.stdout, c.format, client, bot, token))
}

func (c *BotsCommand) RemoveBot(ctx context.Context, client botsCommandClient) error {
	_, err := client.BotServiceClient().DeleteBot(ctx, &machineidv1pb.DeleteBotRequest{
		BotName: c.botName.Name,
		Scope:   c.botName.Scope,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Fprintf(c.stdout, "Bot %q deleted successfully.\n", c.botName)

	return nil
}

func (c *BotsCommand) LockBot(ctx context.Context, client botsCommandClient) error {
	lockExpiry, err := computeLockExpiry(c.lockExpires, c.lockTTL)
	if err != nil {
		return trace.Wrap(err)
	}

	resourceName, err := services.BotResourceName(c.botName)
	if err != nil {
		return trace.Wrap(err, "building bot resource name")
	}

	user, err := client.GetUser(ctx, resourceName, false)
	if err != nil {
		return trace.Wrap(err)
	}

	meta := user.GetMetadata()
	botName, ok := meta.Labels[types.BotLabel]
	if !ok {
		return trace.BadParameter("User %q is not a bot user; use `tctl lock` directly to lock this user", user.GetName())
	}

	// The name label alone is ambiguous now the same name may exist in many scopes.
	found := scopes.QualifiedName{Scope: meta.Labels[types.BotScopeLabel], Name: botName}
	if found != c.botName {
		return trace.BadParameter("User %q is not associated with expected bot %q (expected %q); use `tctl lock` directly to lock this user", user.GetName(), c.botName, found)
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

	currentLogins := set.New[string]()
	if logins, exists := traits[constants.TraitLogins]; exists {
		currentLogins.Add(logins...)
	}

	var desiredLogins set.Set[string]
	if c.setLogins != "" {
		desiredLogins = set.New[string](splitEntries(c.setLogins)...)
	} else {
		desiredLogins = currentLogins.Clone()
	}

	addLogins := splitEntries(c.addLogins)
	if len(addLogins) > 0 {
		desiredLogins.Add(addLogins...)
	}

	desiredLoginsArray := desiredLogins.Elements()

	if maps.Equal(currentLogins, desiredLogins) {
		slog.InfoContext(ctx, "Logins will be left unchanged", "logins", desiredLoginsArray)
		return nil
	}

	slog.InfoContext(ctx, "Desired logins for bot", "bot", c.botName, "logins", desiredLoginsArray)

	if desiredLogins.Len() == 0 {
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

// updateBotRoles applies updates from CLI arguments to a bot's roles, updating
// the field mask as necessary if any updates were made.
func (c *BotsCommand) updateBotRoles(ctx context.Context, client botsCommandClient, bot *machineidv1pb.Bot, mask *fieldmaskpb.FieldMask) error {
	currentRoles := set.New[string](bot.Spec.Roles...)

	var desiredRoles set.Set[string]
	if c.botRoles != "" {
		desiredRoles = set.New(splitEntries(c.botRoles)...)
	} else {
		desiredRoles = currentRoles.Clone()
	}

	if c.addRoles != "" {
		desiredRoles.Add(splitEntries(c.addRoles)...)
	}

	desiredRolesArray := desiredRoles.Elements()

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
func (c *BotsCommand) UpdateBot(ctx context.Context, client botsCommandClient) error {
	if c.botName.Scope != "" {
		// Nothing this command can set is settable on a scoped bot, so the RPC
		// would refuse the request anyway.
		return trace.BadParameter(
			"cannot update scoped bot %q: scoped bots have no updatable fields "+
				"(roles are granted with scoped role assignments)", c.botName,
		)
	}

	bot, err := client.BotServiceClient().GetBot(ctx, &machineidv1pb.GetBotRequest{
		BotName: c.botName.Name,
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

	if c.maxSessionTTL > 0 {
		bot.Spec.MaxSessionTtl = durationpb.New(c.maxSessionTTL)
		if err := fieldMask.Append(&machineidv1pb.Bot{}, "spec.max_session_ttl"); err != nil {
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

	slog.InfoContext(ctx, "Bot has been updated, roles will take effect on its next renewal", "bot", c.botName.Name)

	return nil
}

// ListBotInstances lists bot instances, possibly filtering for a specific bot
func (c *BotsCommand) ListBotInstances(ctx context.Context, client botsCommandClient) error {
	botName, botScope := c.botName.Name, c.botName.Scope

	// Exhaustive view, per the scope_filter field docs.
	var scopeFilter *scopesv1.Filter
	if botName == "" {
		scopeFilter = &scopesv1.Filter{Mode: scopesv1.Mode_MODE_ALL}
	}

	pageFunc := func(ctx context.Context, pageSize int, pageToken string) ([]*machineidv1pb.BotInstance, string, error) {
		resp, err := client.BotInstanceServiceClient().ListBotInstancesV2(ctx, &machineidv1pb.ListBotInstancesV2Request{
			PageSize:  int32(pageSize),
			PageToken: pageToken,
			SortField: c.sortIndex,
			SortDesc:  c.sortOrder == "descending",
			Filter: &machineidv1pb.ListBotInstancesV2Request_Filters{
				BotName:     botName,
				BotScope:    botScope,
				SearchTerm:  c.search,
				Query:       c.query,
				ScopeFilter: scopeFilter,
			},
		})
		return resp.GetBotInstances(), resp.GetNextPageToken(), trace.Wrap(err)
	}

	fallbackFunc := func(ctx context.Context) ([]*machineidv1pb.BotInstance, error) {
		if c.query != "" {
			return nil, trace.NotImplemented("fallback not supported for requests with a query")
		}
		if botScope != "" {
			return nil, trace.NotImplemented("fallback not supported for requests with a bot scope")
		}
		fallbackPageFunc := func(ctx context.Context, pageSize int, pageToken string) ([]*machineidv1pb.BotInstance, string, error) {
			// Needed for backwards compatibility
			//nolint:staticcheck // SA1019
			resp, err := client.BotInstanceServiceClient().ListBotInstances(ctx, &machineidv1pb.ListBotInstancesRequest{
				FilterBotName:    botName,
				PageSize:         int32(pageSize),
				PageToken:        pageToken,
				FilterSearchTerm: c.search,
				Sort: &types.SortBy{
					Field:  c.sortIndex,
					IsDesc: c.sortOrder == "descending",
				},
			})
			return resp.GetBotInstances(), resp.GetNextPageToken(), trace.Wrap(err)
		}
		return stream.Collect(clientutils.Resources(ctx, fallbackPageFunc))
	}

	instances, err := clientutils.CollectWithFallback(ctx, pageFunc, fallbackFunc)
	if err != nil {
		return trace.Wrap(err)
	}

	if c.format == teleport.JSON {
		// Wrap resource type so the correct protojson marshaling is used for
		// timestamp fields.
		wrappedInstances := make([]types.Resource, 0, len(instances))
		for _, instance := range instances {
			wrappedInstances = append(
				wrappedInstances, types.ProtoResource153ToLegacy(instance),
			)
		}
		err := utils.WriteJSONArray(c.stdout, wrappedInstances)
		if err != nil {
			return trace.Wrap(err, "failed to marshal bot instances")
		}

		return nil
	}

	if len(instances) == 0 {
		if c.botName.Name == "" {
			fmt.Fprintln(c.stdout, "No bot instances found.")
		} else {
			fmt.Fprintf(c.stdout, "No bot instances found with name %q.\n", c.botName)
		}
		return nil
	}

	t := asciitable.MakeTable([]string{"ID", "Join Method", "Version", "Hostname", "Status", "Last Seen"})
	for _, i := range instances {
		var (
			joinMethod string
			hostname   string
			version    string
		)

		initialJoinMethod := cmp.Or(
			i.GetStatus().GetInitialAuthentication().GetJoinAttrs().GetMeta().GetJoinMethod(),
			i.GetStatus().GetInitialAuthentication().GetJoinMethod(),
		)

		lastSeen := i.GetStatus().GetInitialAuthentication().GetAuthenticatedAt().AsTime()

		if len(i.GetStatus().GetLatestAuthentications()) > 0 {
			auth := i.GetStatus().GetLatestAuthentications()[len(i.GetStatus().GetLatestAuthentications())-1]

			authJM := cmp.Or(
				auth.GetJoinAttrs().GetMeta().GetJoinMethod(),
				auth.GetJoinMethod(),
			)
			if authJM == initialJoinMethod {
				joinMethod = authJM
			} else {
				// If the join method changed, show the original method and latest
				joinMethod = fmt.Sprintf("%s (%s)", auth.GetJoinMethod(), initialJoinMethod)
			}

			if auth.GetAuthenticatedAt().AsTime().After(lastSeen) {
				lastSeen = auth.GetAuthenticatedAt().AsTime()
			}
		}

		if len(i.GetStatus().GetLatestHeartbeats()) == 0 {
			hostname = "-"
			version = "-"
		} else {
			hb := i.GetStatus().GetLatestHeartbeats()[len(i.GetStatus().GetLatestHeartbeats())-1]

			hostname = hb.GetHostname()
			version = hb.GetVersion()

			if hb.GetRecordedAt().AsTime().After(lastSeen) {
				lastSeen = hb.GetRecordedAt().AsTime()
			}
		}

		healthStatus := "-"
		if hasStatus, status := aggregateServiceHealth(i.GetStatus().GetServiceHealth()); hasStatus {
			healthStatus = formatStatus(status, false) // Disable color, it messes with the table layout
		}

		// Instances of scoped bots are identified by the bot's scope-qualified
		// name; instances of unscoped bots by the bot's bare name.
		id := scopes.QualifiedName{
			Scope: i.GetScope(),
			Name:  i.GetSpec().GetBotName() + "/" + i.GetSpec().GetInstanceId(),
		}.String()

		t.AddRow([]string{
			id, joinMethod,
			version, hostname, healthStatus, lastSeen.Format(time.RFC3339),
		})
	}
	fmt.Fprintln(c.stdout, t.AsBuffer().String())

	executableFileName := filepath.Base(os.Args[0])
	fmt.Fprintf(c.stdout, "\nTo view more information on a particular instance, run:\n\n> %s bots instances show [id]\n", executableFileName)

	// 'bots instances add' refuses scoped bots, so don't advertise it for one.
	if c.botName.Name != "" && c.botName.Scope == "" {
		fmt.Fprintf(c.stdout, "\nTo onboard a new instance for this bot, run:\n\n> %s bots instances add %s\n", executableFileName, c.botName)
	}

	return nil
}

// AddBotInstance begins onboarding a new instance of an existing bot.
func (c *BotsCommand) AddBotInstance(ctx context.Context, client botsCommandClient) error {
	// A bit of a misnomer but makes the terminology a bit more consistent. This
	// doesn't directly create a bot instance, but creates token that allows a
	// bot to join, which creates a new instance.
	if c.botName.Scope != "" {
		return trace.Wrap(errScopedBotTokenUnsupported(c.botName, "onboarding an instance of"))
	}

	bot, err := client.BotServiceClient().GetBot(ctx, &machineidv1pb.GetBotRequest{
		BotName: c.botName.Name,
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
			BotName:    c.botName.Name,
		}
		token, err = types.NewProvisionTokenFromSpec(tokenName, time.Now().Add(ttl), tokenSpec)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := client.UpsertToken(ctx, token); err != nil {
			return trace.Wrap(err)
		}

		return trace.Wrap(outputToken(ctx, c.stdout, c.format, client, bot, token))
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
	if err := checkTokenBot(c.tokenID, token, c.botName); err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(outputToken(ctx, c.stdout, c.format, client, bot, token))
}

// checkTokenBot verifies that the join token references the bot the caller
// named, scope included.
func checkTokenBot(tokenID string, token types.ProvisionToken, ref scopes.QualifiedName) error {
	name, scope := token.GetBot()
	if tokenBot := (scopes.QualifiedName{Scope: scope, Name: name}); tokenBot != ref {
		return trace.BadParameter("token %q is valid for bot %q, not %q",
			tokenID, tokenBot, ref)
	}
	return nil
}

// errScopedBotTokenUnsupported explains that the token-minting `tctl bots`
// subcommands cannot serve a scoped bot: classic provision tokens can only
// reference unscoped bots, and this command does not create scoped_tokens.
func errScopedBotTokenUnsupported(ref scopes.QualifiedName, verb string) error {
	return trace.BadParameter(
		"%s a scoped bot is not supported by this command (got %q)\n"+
			"hint: a scoped bot joins with a scoped token, which this command cannot create.\n"+
			"  Apply one with 'tctl create -f': a scoped_token in scope %q with spec.bot: %q,\n"+
			"  spec.usage_mode: bot, spec.roles: [Bot] and spec.join_method: bound_keypair.\n"+
			"  The bot also needs a scoped role assignment applicable to its scope before it can join.",
		verb, ref, ref.Scope, ref,
	)
}

var showMessageTemplate = template.Must(template.New("show").Funcs(template.FuncMap{
	"bold": bold,
}).Parse(`Bot:    {{.instance.Spec.BotName}}
{{if .scope}}Scope:  {{.scope}}
{{end}}ID:     {{.instance.Spec.InstanceId}}
Status: {{.health_status}}

Initial Authentication: {{.initial_authentication_table}}

Latest Authentication: {{.latest_authentication_table}}

Latest Heartbeat: {{.heartbeat_table}}

Services:
{{.services_table}}

To view a full, machine-readable record including past heartbeats and
authentication records, run:

> {{.executable}} get {{.get_ref}}
{{if .can_add_instance}}
To onboard a new instance for this bot, run:

> {{.executable}} bots instances add {{.instance.Spec.BotName}}
{{end}}`))

func (c *BotsCommand) ShowBotInstance(ctx context.Context, client botsCommandClient) error {
	botScope, botName, instanceID, err := parseInstanceID(c.instanceID)
	if err != nil {
		return trace.Wrap(err)
	}

	instance, err := client.BotInstanceServiceClient().GetBotInstance(ctx, &machineidv1pb.GetBotInstanceRequest{
		BotName:    botName,
		InstanceId: instanceID,
		BotScope:   botScope,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	initialAuthenticationTable := formatBotInstanceAuthentication(instance.GetStatus().GetInitialAuthentication())

	var latestAuthenticationTable string
	if latestAuthentications := instance.GetStatus().GetLatestAuthentications(); len(latestAuthentications) > 0 {
		latest := latestAuthentications[len(latestAuthentications)-1]
		latestAuthenticationTable = formatBotInstanceAuthentication(latest)
	} else {
		latestAuthenticationTable = "No authentication records."
	}

	var heartbeatTable string
	if latestHeartbeats := instance.GetStatus().GetLatestHeartbeats(); len(latestHeartbeats) > 0 {
		latest := latestHeartbeats[len(latestHeartbeats)-1]
		heartbeatTable = formatBotInstanceHeartbeat(latest)
	} else {
		heartbeatTable = "No heartbeat records."
	}

	healthStatus := "-"
	if hasStatus, status := aggregateServiceHealth(instance.GetStatus().GetServiceHealth()); hasStatus {
		healthStatus = formatStatus(status, true)
	}

	servicesTable := "  No reported services."
	if instance.GetStatus().GetServiceHealth() != nil {
		servicesTable = formatServices(instance.GetStatus().GetServiceHealth())
	}

	// Only the two-argument form of 'tctl get' can carry a scope, so use it for
	// scoped and unscoped alike rather than changing shape between them.
	instanceRef := instance.GetSpec().GetBotName() + "/" + instance.GetSpec().GetInstanceId()
	if scope := instance.GetScope(); scope != "" {
		instanceRef = scopes.QualifiedName{Scope: scope, Name: instanceRef}.String()
	}
	getRef := types.KindBotInstance + " " + instanceRef

	templateData := map[string]any{
		"executable": os.Args[0],
		"instance":   instance,
		"scope":      instance.GetScope(),
		"get_ref":    getRef,
		// A bare name in the 'instances add' hint would target the same-named
		// unscoped bot, and that command can't onboard a scoped bot anyway.
		"can_add_instance":             instance.GetScope() == "",
		"initial_authentication_table": initialAuthenticationTable,
		"latest_authentication_table":  latestAuthenticationTable,
		"heartbeat_table":              heartbeatTable,
		"health_status":                healthStatus,
		"services_table":               servicesTable,
	}

	return trace.Wrap(showMessageTemplate.Execute(c.stdout, templateData))
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
func outputToken(ctx context.Context, wr io.Writer, format string, client botsCommandClient, bot *machineidv1pb.Bot, token types.ProvisionToken) error {
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

	proxies, err := clientutils.CollectWithFallback(ctx, client.ListProxyServers, func(context.Context) ([]types.Server, error) {
		//nolint:staticcheck // TODO(kiosion) DELETE IN 21.0.0
		return client.GetProxies()
	})
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
	table.AddRow([]string{"Authenticated At:", record.GetAuthenticatedAt().AsTime().Format(time.RFC3339)})
	table.AddRow([]string{"Join Method:", cmp.Or(record.GetJoinAttrs().GetMeta().GetJoinMethod(), record.GetJoinMethod())})
	table.AddRow([]string{"Join Token:", cmp.Or(record.GetJoinAttrs().GetMeta().GetJoinTokenName(), record.GetJoinToken())})
	var meta fmt.Stringer = record.GetMetadata()
	if attrs := record.GetJoinAttrs(); attrs != nil {
		meta = attrs
	}
	table.AddRow([]string{"Join Metadata:", meta.String()})
	table.AddRow([]string{"Generation:", fmt.Sprint(record.GetGeneration())})
	table.AddRow([]string{"Public Key:", fmt.Sprintf("<%d bytes>", len(record.GetPublicKey()))})

	return "\n" + indentString(table.AsBuffer().String(), "  ")
}

// formatBotInstanceHeartbeat returns a multiline, indented string containing
// a textual representation of a bot heartbeat.
func formatBotInstanceHeartbeat(record *machineidv1pb.BotInstanceStatusHeartbeat) string {
	table := asciitable.MakeHeadlessTable(2)
	table.AddRow([]string{"Recorded At:", record.GetRecordedAt().AsTime().Format(time.RFC3339)})
	table.AddRow([]string{"Is Startup:", fmt.Sprint(record.GetIsStartup())})
	table.AddRow([]string{"Version:", record.GetVersion()})
	table.AddRow([]string{"Hostname:", record.GetHostname()})
	table.AddRow([]string{"Uptime:", record.GetUptime().AsDuration().String()})
	table.AddRow([]string{"Join Method:", record.GetJoinMethod()})
	table.AddRow([]string{"One Shot:", fmt.Sprint(record.GetOneShot())})
	table.AddRow([]string{"Architecture:", record.GetArchitecture()})
	table.AddRow([]string{"OS:", record.GetOs()})

	return "\n" + indentString(table.AsBuffer().String(), "  ")
}

// formatServices returns a string containing a tabular representation of a
// bot's services.
func formatServices(services []*machineidv1pb.BotInstanceServiceHealth) string {
	all := strings.Builder{}

	sortedServices := slices.SortedFunc(slices.Values(services), func(a, b *machineidv1pb.BotInstanceServiceHealth) int {
		return cmp.Compare(a.GetService().GetName(), b.GetService().GetName())
	})
	for _, service := range sortedServices {
		all.WriteString("Name:        " + service.GetService().GetName())
		all.WriteString("\n")
		all.WriteString("Type:        " + service.GetService().GetType())
		all.WriteString("\n")
		all.WriteString("Status:      " + formatStatus(service.GetStatus(), true))
		all.WriteString("\n")

		if service.GetReason() != "" {
			all.WriteString("Reason:      " + service.GetReason())
			all.WriteString("\n")
		}

		all.WriteString("Reported at: " + service.GetUpdatedAt().AsTime().Format(time.RFC3339))
		all.WriteString("\n\n")
	}

	return indentString(all.String(), "  ")
}

// formatStatus returns an human-readable representation of a service status.
// Optionally, it can include a colored dot.
func formatStatus(status machineidv1pb.BotInstanceHealthStatus, useColor bool) string {
	var (
		greenDot  = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
		redDot    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
		whiteDot  = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
		yellowDot = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	)

	switch status {
	case machineidv1pb.BotInstanceHealthStatus_BOT_INSTANCE_HEALTH_STATUS_HEALTHY:
		if useColor {
			return greenDot.Render("\u25CF") + " Healthy"
		}
		return "Healthy"
	case machineidv1pb.BotInstanceHealthStatus_BOT_INSTANCE_HEALTH_STATUS_UNHEALTHY:
		if useColor {
			return redDot.Render("\u25CF") + " Unhealthy"
		}
		return "Unhealthy"
	case machineidv1pb.BotInstanceHealthStatus_BOT_INSTANCE_HEALTH_STATUS_INITIALIZING:
		if useColor {
			return whiteDot.Render("\u25CF") + " Initializing"
		}
		return "Initializing"
	default:
		if useColor {
			return yellowDot.Render("\u25CF") + " Unknown"
		}
		return "Unknown"
	}
}

// parseInstanceID converts an instance ID string in the form of
// [scope::][bot name]/[uuid] into its component parts. The scope prefix
// addresses an instance of a scoped bot; scope is empty when the prefix is
// absent.
func parseInstanceID(s string) (scope string, name string, uuid string, err error) {
	if before, after, ok := strings.Cut(s, scopes.QualifiedNameSeparator); ok {
		if err := scopes.StrongValidate(before); err != nil {
			return "", "", "", trace.Wrap(err)
		}
		scope, s = before, after
	} else if scopes.MaybeSQN(s) {
		return "", "", "", trace.BadParameter("invalid bot instance syntax, must be: [scope::][bot name]/[uuid]")
	}

	name, uuid, ok := strings.Cut(s, "/")
	if !ok {
		return "", "", "", trace.BadParameter("invalid bot instance syntax, must be: [scope::][bot name]/[uuid]")
	}

	return scope, name, uuid, nil
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

// aggregateServiceHealth returns the least healthy status from the list of
// services provided. Priority; unhealthy, unspecified, initializing, healthy
func aggregateServiceHealth(services []*machineidv1pb.BotInstanceServiceHealth) (bool, machineidv1pb.BotInstanceHealthStatus) {
	if len(services) == 0 {
		return false, 0
	}

	hasUnhealthy := slices.ContainsFunc(services, func(service *machineidv1pb.BotInstanceServiceHealth) bool {
		return service.GetStatus() == machineidv1pb.BotInstanceHealthStatus_BOT_INSTANCE_HEALTH_STATUS_UNHEALTHY
	})
	if hasUnhealthy {
		return true, machineidv1pb.BotInstanceHealthStatus_BOT_INSTANCE_HEALTH_STATUS_UNHEALTHY
	}

	hasUnknown := slices.ContainsFunc(services, func(service *machineidv1pb.BotInstanceServiceHealth) bool {
		return service.GetStatus() == machineidv1pb.BotInstanceHealthStatus_BOT_INSTANCE_HEALTH_STATUS_UNSPECIFIED
	})
	if hasUnknown {
		return true, machineidv1pb.BotInstanceHealthStatus_BOT_INSTANCE_HEALTH_STATUS_UNSPECIFIED
	}

	hasInitializing := slices.ContainsFunc(services, func(service *machineidv1pb.BotInstanceServiceHealth) bool {
		return service.GetStatus() == machineidv1pb.BotInstanceHealthStatus_BOT_INSTANCE_HEALTH_STATUS_INITIALIZING
	})
	if hasInitializing {
		return true, machineidv1pb.BotInstanceHealthStatus_BOT_INSTANCE_HEALTH_STATUS_INITIALIZING
	}

	return true, machineidv1pb.BotInstanceHealthStatus_BOT_INSTANCE_HEALTH_STATUS_HEALTHY
}
