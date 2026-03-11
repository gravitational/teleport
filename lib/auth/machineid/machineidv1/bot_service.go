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

package machineidv1

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	userspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/services"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
	"github.com/gravitational/teleport/lib/utils/set"
)

// SupportedJoinMethods should match SupportedJoinMethods declared in
// lib/tbot/config
var SupportedJoinMethods = []types.JoinMethod{
	types.JoinMethodAzure,
	types.JoinMethodAzureDevops,
	types.JoinMethodCircleCI,
	types.JoinMethodEnv0,
	types.JoinMethodGCP,
	types.JoinMethodGitHub,
	types.JoinMethodGitLab,
	types.JoinMethodIAM,
	types.JoinMethodKubernetes,
	types.JoinMethodSpacelift,
	types.JoinMethodToken,
	types.JoinMethodTPM,
	types.JoinMethodTerraformCloud,
	types.JoinMethodBitbucket,
	types.JoinMethodOracle,
	types.JoinMethodBoundKeypair,
}

// BotResourceName returns the default name for resources associated with the
// given named bot.
func BotResourceName(botName string) string {
	return "bot-" + strings.ReplaceAll(botName, " ", "-")
}

// Cache is the subset of the cached resources that the Service queries.
type Cache interface {
	// GetUser returns a user by name.
	GetUser(ctx context.Context, user string, withSecrets bool) (types.User, error)
	// ListUsers lists users
	ListUsers(ctx context.Context, req *userspb.ListUsersRequest) (*userspb.ListUsersResponse, error)
	// GetRole returns a role by name.
	GetRole(ctx context.Context, name string) (types.Role, error)
}

// Backend is the subset of the backend resources that the Service modifies.
type Backend interface {
	// CreateUser creates user, only if the user entry does not exist
	CreateUser(ctx context.Context, user types.User) (types.User, error)
	// CreateRole creates role, only if the role entry does not exist
	CreateRole(ctx context.Context, role types.Role) (types.Role, error)
	// UpdateUser updates an existing user if revisions match.
	UpdateUser(ctx context.Context, user types.User) (types.User, error)
	// UpdateRole updates an existing role if revisions match.
	UpdateRole(ctx context.Context, role types.Role) (types.Role, error)
	// UpsertUser creates a new user or forcefully updates an existing user.
	UpsertUser(ctx context.Context, user types.User) (types.User, error)
	// UpsertRole creates a new role or forcefully updates an existing role.
	UpsertRole(ctx context.Context, role types.Role) (types.Role, error)
	// UpsertToken creates a new token or forcefully updates an existing token.
	UpsertToken(ctx context.Context, token types.ProvisionToken) error
	// DeleteRole deletes a role by name.
	DeleteRole(ctx context.Context, name string) error
	// DeleteUser deletes a user and all associated objects.
	DeleteUser(ctx context.Context, user string) error
	// GetUser returns a user by name.
	GetUser(ctx context.Context, name string, withSecrets bool) (types.User, error)
	// GetRole returns a role by name.
	GetRole(ctx context.Context, name string) (types.Role, error)
	// GetToken returns a token by name.
	GetToken(ctx context.Context, name string) (types.ProvisionToken, error)
}

// BotServiceConfig holds configuration options for
// the bots gRPC service.
type BotServiceConfig struct {
	ScopedAuthorizer authz.ScopedAuthorizer
	Cache            Cache
	Backend          Backend
	Logger           *slog.Logger
	Emitter          apievents.Emitter
	Reporter         usagereporter.UsageReporter
	Clock            clockwork.Clock
}

// NewBotService returns a new instance of the BotService.
func NewBotService(cfg BotServiceConfig) (*BotService, error) {
	switch {
	case cfg.Cache == nil:
		return nil, trace.BadParameter("cache service is required")
	case cfg.Backend == nil:
		return nil, trace.BadParameter("backend service is required")
	case cfg.ScopedAuthorizer == nil:
		return nil, trace.BadParameter("scoped authorizer is required")
	case cfg.Emitter == nil:
		return nil, trace.BadParameter("emitter is required")
	case cfg.Reporter == nil:
		return nil, trace.BadParameter("reporter is required")
	case cfg.Logger == nil:
		return nil, trace.BadParameter("logger is required")
	}

	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}

	return &BotService{
		logger:           cfg.Logger,
		scopedAuthorizer: cfg.ScopedAuthorizer,
		cache:            cfg.Cache,
		backend:          cfg.Backend,
		emitter:          cfg.Emitter,
		reporter:         cfg.Reporter,
		clock:            cfg.Clock,
	}, nil
}

// BotService implements the teleport.machineid.v1.BotService RPC service.
type BotService struct {
	pb.UnimplementedBotServiceServer

	cache            Cache
	backend          Backend
	scopedAuthorizer authz.ScopedAuthorizer
	logger           *slog.Logger
	emitter          apievents.Emitter
	reporter         usagereporter.UsageReporter
	clock            clockwork.Clock
}

// GetBot gets a bot by name. It will throw an error if the bot does not exist.
func (bs *BotService) GetBot(ctx context.Context, req *pb.GetBotRequest) (*pb.Bot, error) {
	authCtx, err := bs.scopedAuthorizer.AuthorizeScoped(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Check if it's feasible that user has access before hitting the database.
	ruleCtx := authCtx.RuleContext()
	if err := authCtx.CheckerContext.CheckMaybeHasAccessToRules(
		&ruleCtx, types.KindBot, types.VerbRead,
	); err != nil {
		return nil, trace.Wrap(err)
	}

	if req.BotName == "" {
		return nil, trace.BadParameter("bot_name: must be non-empty")
	}

	user, err := bs.cache.GetUser(ctx, BotResourceName(req.BotName), false)
	if err != nil {
		return nil, trace.Wrap(err, "fetching bot user")
	}
	if scope, _ := user.GetLabel(types.BotScopeLabel); scope != "" {
		bot, err := bs.getBotScoped(ctx, authCtx, user)
		return bot, trace.Wrap(err, "getting scoped bot")
	}

	role, err := bs.cache.GetRole(ctx, BotResourceName(req.BotName))
	if err != nil {
		return nil, trace.Wrap(err, "fetching bot role")
	}

	bot, err := botFromUserAndRole(user, role)
	if err != nil {
		return nil, trace.Wrap(err, "converting from resources")
	}

	ruleCtx.Resource153 = bot
	if err := authCtx.CheckerContext.Decision(
		ctx, scopes.Root, func(checker *services.SplitAccessChecker) error {
			return checker.Common().CheckAccessToRules(&ruleCtx, types.KindBot, types.VerbRead)
		},
	); err != nil {
		// Return NotFound rather than Forbidden to avoid leaking existence of
		// bot.
		return nil, trace.NotFound("bot %q not found", req.BotName)
	}

	return bot, nil
}

func (bs *BotService) getBotScoped(
	ctx context.Context, authCtx *authz.ScopedContext, user types.User,
) (*pb.Bot, error) {
	bot, err := scopedBotFromUser(user)
	if err != nil {
		return nil, trace.Wrap(err, "converting user to scoped bot")
	}

	ruleCtx := authCtx.RuleContext()
	ruleCtx.Resource153 = bot
	if err := authCtx.CheckerContext.Decision(
		ctx, cmp.Or(bot.Scope, scopes.Root), func(checker *services.SplitAccessChecker) error {
			return checker.Common().CheckAccessToRules(&ruleCtx, types.KindBot, types.VerbReadNoSecrets)
		},
	); err != nil {
		return nil, trace.NotFound("bot %q not found", bot.Metadata.Name)
	}

	return bot, nil
}

// ListBots lists all bots.
func (bs *BotService) ListBots(
	ctx context.Context, req *pb.ListBotsRequest,
) (*pb.ListBotsResponse, error) {
	authCtx, err := bs.scopedAuthorizer.AuthorizeScoped(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ruleCtx := authCtx.RuleContext()
	// Check generally if this user may have the ability to list bots - ignoring
	// where conditions.
	if err := authCtx.CheckerContext.CheckMaybeHasAccessToRules(
		&ruleCtx, types.KindBot, types.VerbList,
	); err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO(noah): We should add pre-hydrated Bot support to the cache to avoid
	// needing to iterate over all users here. This is currently a fairly
	// expensive implementation.
	bots := []*pb.Bot{}
	rsp, err := bs.cache.ListUsers(ctx, &userspb.ListUsersRequest{
		PageSize:  req.PageSize,
		PageToken: req.PageToken,
	})
	if err != nil {
		return nil, trace.Wrap(err, "listing users")
	}
	for _, u := range rsp.Users {
		botName, isBot := u.GetLabel(types.BotLabel)
		if !isBot {
			continue
		}

		scope, _ := u.GetLabel(types.BotScopeLabel)
		var bot *pb.Bot
		if scope == "" {
			// We only need to fetch the bot role for unscoped bots.
			role, err := bs.cache.GetRole(ctx, BotResourceName(botName))
			if err != nil {
				bs.logger.WarnContext(
					ctx,
					"Failed to fetch role for bot during ListBots. Bot will be omitted from results",
					"error", err,
					"bot_name", botName,
				)
				continue
			}

			bot, err = botFromUserAndRole(u, role)
			if err != nil {
				bs.logger.WarnContext(
					ctx,
					"Failed to convert bot during ListBots. Bot will be omitted from results",
					"error", err,
					"bot_name", botName,
				)
				continue
			}
		} else {
			bot, err = scopedBotFromUser(u)
			if err != nil {
				bs.logger.WarnContext(
					ctx,
					"Failed to convert scoped bot during ListBots. Bot will be omitted from results",
					"error", err,
					"bot_name", botName,
				)
				continue
			}
		}

		// Check if user can access this specific Bot.
		ruleCtx := authCtx.RuleContext()
		ruleCtx.Resource153 = bot
		if err := authCtx.CheckerContext.Decision(ctx, cmp.Or(bot.Scope, scopes.Root), func(checker *services.SplitAccessChecker) error {
			return checker.Common().CheckAccessToRules(&ruleCtx, types.KindBot, types.VerbList)
		}); err != nil {
			// Ignore resources the user cannot access.
			continue
		}

		bots = append(bots, bot)
	}

	return &pb.ListBotsResponse{
		Bots:          bots,
		NextPageToken: rsp.NextPageToken,
	}, nil
}

// CreateBot creates a new bot. It will throw an error if the bot already
// exists.
func (bs *BotService) CreateBot(
	ctx context.Context, req *pb.CreateBotRequest,
) (*pb.Bot, error) {
	if err := setKindAndVersion(req.Bot); err != nil {
		return nil, trace.Wrap(err, "setting kind and version")
	}
	authCtx, err := bs.scopedAuthorizer.AuthorizeScoped(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := StrongValidateBot(req.Bot); err != nil {
		return nil, trace.Wrap(err, "validating bot")
	}
	// Validation comes before authz checks so we know that scope etc is
	// well-formed.

	ruleCtx := authCtx.RuleContext()
	ruleCtx.Resource153 = req.Bot
	if err := authCtx.CheckerContext.Decision(ctx, cmp.Or(req.Bot.Scope, scopes.Root), func(checker *services.SplitAccessChecker) error {
		return checker.Common().CheckAccessToRules(&ruleCtx, types.KindBot, types.VerbCreate)
	}); err != nil {
		return nil, trace.Wrap(err)
	}
	if unscoped, ok := authCtx.UnscopedContext(); ok {
		// We can only perform MFA checks on unscoped identities.
		// TODO(noah): When scopes supports MFA, add check here :')
		if err := unscoped.AuthorizeAdminActionAllowReusedMFA(); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	var bot *pb.Bot
	if req.Bot.Scope != "" {
		bot, err = bs.createScopedBot(ctx, authCtx, req.Bot)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		bot, err = bs.createUnscopedBot(ctx, authCtx, req.Bot)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	bs.reporter.AnonymizeAndSubmit(&usagereporter.BotCreateEvent{
		UserName:    authz.ClientUsername(ctx),
		BotUserName: BotResourceName(bot.Metadata.Name),
		RoleName:    BotResourceName(bot.Metadata.Name),
		BotName:     bot.Metadata.Name,
		RoleCount:   int64(len(bot.Spec.Roles)),
	})
	if err := bs.emitter.EmitAuditEvent(ctx, &apievents.BotCreate{
		Metadata: apievents.Metadata{
			Type: events.BotCreateEvent,
			Code: events.BotCreateCode,
		},
		UserMetadata: authz.ClientUserMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: bot.Metadata.Name,
		},
	}); err != nil {
		bs.logger.WarnContext(
			ctx, "Failed to emit BotCreate audit event",
			"error", err,
		)
	}

	return bot, nil
}

func (bs *BotService) createScopedBot(
	ctx context.Context,
	authCtx *authz.ScopedContext,
	bot *pb.Bot,
) (*pb.Bot, error) {
	if err := scopes.AssertMWIFeatureEnabled(); err != nil {
		return nil, trace.Wrap(err)
	}

	user, err := scopedBotToUser(bot, bs.clock.Now(), authCtx.User.GetName())
	if err != nil {
		return nil, trace.Wrap(err, "converting to user resource")
	}

	user, err = bs.backend.CreateUser(ctx, user)
	if err != nil {
		return nil, trace.Wrap(err, "creating bot user")
	}

	bot, err = scopedBotFromUser(user)
	if err != nil {
		return nil, trace.Wrap(err, "converting from user resource")
	}

	return bot, nil
}

func (bs *BotService) createUnscopedBot(
	ctx context.Context,
	authCtx *authz.ScopedContext,
	bot *pb.Bot,
) (*pb.Bot, error) {
	user, role, err := botToUserAndRole(
		bot, bs.clock.Now(), authCtx.User.GetName(),
	)
	if err != nil {
		return nil, trace.Wrap(err, "converting to resources")
	}

	user, err = bs.backend.CreateUser(ctx, user)
	if err != nil {
		return nil, trace.Wrap(err, "creating bot user")
	}
	role, err = bs.backend.CreateRole(ctx, role)
	if err != nil {
		return nil, trace.Wrap(err, "creating bot role")
	}

	// Convert back from user and role, this ensures some consistency in what
	// we return (i.e if client has provided a bot config that does not
	// roundtrip).
	bot, err = botFromUserAndRole(user, role)
	if err != nil {
		return nil, trace.Wrap(err, "converting from resources")
	}
	return bot, nil
}

// UpsertBot creates a new bot or forcefully updates an existing bot. This is
// a function rather than a method so that it can be used by both the gRPC
// service and the auth server init code when dealing with resources to be
// applied at startup.
func UpsertBot(
	ctx context.Context,
	backend Backend,
	bot *pb.Bot,
	now time.Time,
	createdBy string,
) (*pb.Bot, error) {
	if bot.Scope != "" {
		if err := scopes.AssertMWIFeatureEnabled(); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if err := StrongValidateBot(bot); err != nil {
		return nil, trace.Wrap(err, "validating bot")
	}

	// Fetch pre-existing user, we'll use this to preserve generation and
	// check for scope transitions.
	existingUser, err := backend.GetUser(
		ctx, BotResourceName(bot.Metadata.Name), false,
	)
	if err != nil && !trace.IsNotFound(err) {
		// We'll happily ignore a not-found error, in this case, we have an
		// upsert for a non-existent bot. If we have any other kind of error,
		// we want to propagae this up.
		return nil, trace.Wrap(err, "fetching existing bot user")
	}
	if existingUser != nil {
		// If the bot already exists, we need to check that the upsert does not
		// cause a scope transition (i.e change of scope, including from/to
		// unscoped). This is because our RBAC does not account for this.
		// This restriction may be loosened in future if we evaluate pre-upsert
		// and post-upsert scope authz.
		existingScope, _ := existingUser.GetLabel(types.BotScopeLabel)
		if existingScope != bot.Scope {
			return nil, trace.BadParameter(
				"cannot change scope of existing bot from %q to %q",
				existingScope, bot.Scope,
			)
		}
	}

	// Create User (and maybe Role if unscoped) from the Bot.
	var user types.User
	var role types.Role
	if bot.Scope != "" {
		user, err = scopedBotToUser(bot, now, createdBy)
		if err != nil {
			return nil, trace.Wrap(err, "converting scoped bot to user resource")
		}
	} else {
		user, role, err = botToUserAndRole(bot, now, createdBy)
		if err != nil {
			return nil, trace.Wrap(err, "converting unscoped bot to resources")
		}
	}
	// If the bot already exists, we need to copy across the generation label.
	// TODO(noah): When we fully deprecate generation labels, we also need to
	// remove this - https://github.com/gravitational/teleport/issues/64484
	if existingUser != nil {
		if existingGeneration, ok := existingUser.GetLabel(types.BotGenerationLabel); ok {
			meta := user.GetMetadata()
			meta.Labels[types.BotGenerationLabel] = existingGeneration
			user.SetMetadata(meta)
		} else {
			return nil, trace.BadParameter("unable to determine existing generation for bot due to missing label")
		}
	}

	user, err = backend.UpsertUser(ctx, user)
	if err != nil {
		return nil, trace.Wrap(err, "upserting bot user")
	}
	if role != nil {
		// Bot role only exists for unscoped bots.
		role, err = backend.UpsertRole(ctx, role)
		if err != nil {
			return nil, trace.Wrap(err, "upserting bot role")
		}
	}

	if role != nil {
		bot, err = botFromUserAndRole(user, role)
		if err != nil {
			return nil, trace.Wrap(err, "converting unscoped bot from resources")
		}
	} else {
		bot, err = scopedBotFromUser(user)
		if err != nil {
			return nil, trace.Wrap(err, "converting scoped bot from user resource")
		}
	}

	return bot, nil
}

// UpsertBot creates a new bot or forcefully updates an existing bot.
func (bs *BotService) UpsertBot(ctx context.Context, req *pb.UpsertBotRequest) (*pb.Bot, error) {
	if err := setKindAndVersion(req.Bot); err != nil {
		return nil, trace.Wrap(err, "setting kind and version")
	}

	authCtx, err := bs.scopedAuthorizer.AuthorizeScoped(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := StrongValidateBot(req.Bot); err != nil {
		return nil, trace.Wrap(err, "validating bot")
	}
	// Validation comes before authz checks so that we know scope (if present)
	// is well-formed.

	ruleCtx := authCtx.RuleContext()
	ruleCtx.Resource153 = req.Bot
	if err := authCtx.CheckerContext.Decision(
		ctx,
		cmp.Or(req.Bot.Scope, scopes.Root),
		func(checker *services.SplitAccessChecker) error {
			return checker.Common().CheckAccessToRules(
				&ruleCtx, types.KindBot, types.VerbCreate, types.VerbUpdate,
			)
		},
	); err != nil {
		return nil, trace.Wrap(err)
	}
	if unscoped, ok := authCtx.UnscopedContext(); ok {
		// We can only perform MFA checks on unscoped identities.
		// TODO(noah): When scopes supports MFA, add check here :')
		// Allow re-use for bulk upserts.
		if err := unscoped.AuthorizeAdminActionAllowReusedMFA(); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	bot, err := UpsertBot(
		ctx, bs.backend, req.Bot, bs.clock.Now(), authCtx.User.GetName(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	bs.reporter.AnonymizeAndSubmit(&usagereporter.BotCreateEvent{
		UserName:    authz.ClientUsername(ctx),
		BotUserName: BotResourceName(bot.Metadata.Name),
		RoleName:    BotResourceName(bot.Metadata.Name),
		BotName:     bot.Metadata.Name,
		RoleCount:   int64(len(bot.Spec.Roles)),
	})
	if err := bs.emitter.EmitAuditEvent(ctx, &apievents.BotCreate{
		Metadata: apievents.Metadata{
			Type: events.BotCreateEvent,
			Code: events.BotCreateCode,
		},
		UserMetadata: authz.ClientUserMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: bot.Metadata.Name,
		},
	}); err != nil {
		bs.logger.WarnContext(
			ctx, "Failed to emit BotCreate audit event",
			"error", err,
		)
	}

	return bot, nil
}

// UpdateBot updates an existing bot. It will throw an error if the bot does
// not exist.
func (bs *BotService) UpdateBot(
	ctx context.Context, req *pb.UpdateBotRequest,
) (*pb.Bot, error) {
	if err := setKindAndVersion(req.Bot); err != nil {
		return nil, trace.Wrap(err, "setting kind and version")
	}

	scopedAuthCtx, err := bs.scopedAuthorizer.AuthorizeScoped(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Out of an abundance of caution, we're avoiding scoped identities/bots
	// and the update RPC for now. There's no meaningful fields a scoped
	// identity would need to interact with on a Bot or any identity on a
	// scoped Bot.
	authCtx, ok := scopedAuthCtx.UnscopedContext()
	if !ok {
		return nil, trace.AccessDenied("scoped identity cannot call update Bot RPC")
	}

	if err := authCtx.CheckAccessToResource153(
		req.Bot,
		types.VerbUpdate,
	); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	switch {
	case req.Bot == nil:
		return nil, trace.BadParameter("bot: must be non-nil")
	case req.Bot.Metadata == nil:
		return nil, trace.BadParameter("bot.metadata: must be non-nil")
	case req.Bot.Metadata.Name == "":
		return nil, trace.BadParameter("bot.metadata.name: must be non-empty")
	case req.Bot.Spec == nil:
		return nil, trace.BadParameter("bot.spec: must be non-nil")
	case req.UpdateMask == nil:
		return nil, trace.BadParameter("update_mask: must be non-nil")
	case len(req.UpdateMask.Paths) == 0:
		return nil, trace.BadParameter("update_mask.paths: must be non-empty")
	}

	user, err := bs.backend.GetUser(ctx, BotResourceName(req.Bot.Metadata.Name), false)
	if err != nil {
		return nil, trace.Wrap(err, "getting bot user")
	}
	if scope := user.GetMetadata().Labels[types.BotScopeLabel]; scope != "" {
		return nil, trace.BadParameter("cannot update scoped bot")
	}
	role, err := bs.backend.GetRole(ctx, BotResourceName(req.Bot.Metadata.Name))
	if err != nil {
		return nil, trace.Wrap(err, "getting bot role")
	}

	for _, path := range req.UpdateMask.Paths {
		switch path {
		case "spec.roles":
			if slices.Contains(req.Bot.Spec.Roles, "") {
				return nil, trace.BadParameter(
					"spec.roles: must not contain empty strings",
				)
			}
			role.SetImpersonateConditions(types.Allow, types.ImpersonateConditions{
				Roles: req.Bot.Spec.Roles,
			})
		case "spec.traits":
			traits := map[string][]string{}
			for _, t := range req.Bot.Spec.Traits {
				if len(t.Values) == 0 {
					continue
				}
				if traits[t.Name] == nil {
					traits[t.Name] = []string{}
				}
				traits[t.Name] = append(traits[t.Name], t.Values...)
			}
			user.SetTraits(traits)
		case "spec.max_session_ttl":
			opts := role.GetOptions()
			opts.MaxSessionTTL = types.Duration(req.Bot.Spec.MaxSessionTtl.AsDuration())
			role.SetOptions(opts)
		case "metadata.description":
			meta := user.GetMetadata()
			meta.Description = req.Bot.Metadata.Description
			user.SetMetadata(meta)
		default:
			return nil, trace.BadParameter("update_mask: unsupported path %q", path)
		}
	}

	user, err = bs.backend.UpdateUser(ctx, user)
	if err != nil {
		return nil, trace.Wrap(err, "updating bot user")
	}
	role, err = bs.backend.UpdateRole(ctx, role)
	if err != nil {
		return nil, trace.Wrap(err, "updating bot role")
	}

	if err := bs.emitter.EmitAuditEvent(ctx, &apievents.BotUpdate{
		Metadata: apievents.Metadata{
			Type: events.BotUpdateEvent,
			Code: events.BotUpdateCode,
		},
		UserMetadata: authz.ClientUserMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: req.Bot.Metadata.Name,
		},
	}); err != nil {
		bs.logger.WarnContext(
			ctx, "Failed to emit BotUpdate audit event",
			"error", err,
		)
	}

	bot, err := botFromUserAndRole(user, role)
	if err != nil {
		return nil, trace.Wrap(err, "converting from resources")
	}

	return bot, nil
}

func (bs *BotService) deleteBotUser(
	ctx context.Context, botName string, user types.User,
) error {
	if v := user.GetMetadata().Labels[types.BotLabel]; v != botName {
		return trace.BadParameter(
			"user missing bot label matching bot name; consider manually deleting user",
		)
	}
	return bs.backend.DeleteUser(ctx, user.GetName())
}

func (bs *BotService) deleteBotRole(ctx context.Context, botName string) error {
	// Check the role that's being deleted is linked to the bot.
	role, err := bs.backend.GetRole(ctx, BotResourceName(botName))
	if err != nil {
		return trace.Wrap(err, "fetching bot role")
	}
	if v := role.GetMetadata().Labels[types.BotLabel]; v != botName {
		return trace.BadParameter(
			"role missing bot label matching bot name; consider manually deleting role",
		)
	}
	return bs.backend.DeleteRole(ctx, role.GetName())
}

// dummyBotWithName returns a dummy bot with the given name. This is used
// for evaluating RBAC for the Delete RPC
func dummyBotWithName(name string) *pb.Bot {
	return &pb.Bot{
		Kind:    types.KindBot,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: name,
		},
	}
}

// DeleteBot deletes an existing bot. It will throw an error if the bot does
// not exist.
func (bs *BotService) DeleteBot(
	ctx context.Context, req *pb.DeleteBotRequest,
) (*emptypb.Empty, error) {
	// Note: this does not remove any locks for the bot's user / role. That
	// might be convenient in case of accidental bot locking but there doesn't
	// seem to be any automatic deletion of locks in teleport today (other
	// than expiration). Consistency around security controls seems important
	// but we can revisit this if desired.
	authCtx, err := bs.scopedAuthorizer.AuthorizeScoped(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if req.BotName == "" {
		return nil, trace.BadParameter("bot_name: must be non-empty")
	}

	// Perform maybe-check before we hit the backend.
	ruleCtx := authCtx.RuleContext()
	if err := authCtx.CheckerContext.CheckMaybeHasAccessToRules(
		&ruleCtx, types.KindBot, types.VerbDelete,
	); err != nil {
		return nil, trace.Wrap(err)
	}

	// Fetch user to determine if bot is scoped or unscoped.
	user, err := bs.backend.GetUser(
		ctx, BotResourceName(req.BotName), false,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	scope := user.GetMetadata().Labels[types.BotScopeLabel]

	ruleCtx.Resource153 = dummyBotWithName(req.BotName)
	if err := authCtx.CheckerContext.Decision(ctx, cmp.Or(scope, scopes.Root), func(checker *services.SplitAccessChecker) error {
		return checker.Common().CheckAccessToRules(&ruleCtx, types.KindBot, types.VerbDelete)
	}); err != nil {
		return nil, trace.Wrap(err)
	}
	// If identity is unscoped, perform admin action MFA.
	// TODO(noah): When scope identities support MFA, enforce it!
	if unscoped, ok := authCtx.UnscopedContext(); ok {
		if err := unscoped.AuthorizeAdminAction(); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	userErr := bs.deleteBotUser(ctx, req.BotName, user)
	var roleErr error
	if scope == "" {
		// Only unscoped bots have a Bot role for us to delete.
		roleErr = bs.deleteBotRole(ctx, req.BotName)
	}
	err = trace.NewAggregate(
		trace.Wrap(userErr, "deleting bot user"),
		trace.Wrap(roleErr, "deleting bot role"),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := bs.emitter.EmitAuditEvent(ctx, &apievents.BotDelete{
		Metadata: apievents.Metadata{
			Type: events.BotDeleteEvent,
			Code: events.BotDeleteCode,
		},
		UserMetadata: authz.ClientUserMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: req.BotName,
		},
	}); err != nil {
		bs.logger.WarnContext(
			ctx, "Failed to emit BotDelete audit event",
			"error", err,
		)
	}

	return &emptypb.Empty{}, nil
}

// setKindAndVersion patches for the fact that when this API was originally
// introduced, we did not enforce that the Kind/Version fields were set.
// This is largely not an issue since someone would need to invoke the API
// directly (as tctl will require that they are set). However, we do need these
// fields to be set correctly for authz to work properly.
//
// TODO(noah): In the future, we should commit to a breaking change to validate
// that these fields are set correctly.
func setKindAndVersion(b *pb.Bot) error {
	if b == nil {
		return trace.BadParameter("bot: must be non-nil")
	}
	if b.Kind == "" {
		b.Kind = types.KindBot
	}
	if b.Version == "" {
		b.Version = types.V1
	}
	return nil
}

// StrongValidateBot performs strong validation on scoped and unscoped bots,
// and is suitable for being called on write operations. This should not be
// called on read operations.
func StrongValidateBot(b *pb.Bot) error {
	if b == nil {
		return trace.BadParameter("must be non-nil")
	}
	if b.Metadata == nil {
		return trace.BadParameter("metadata: must be non-nil")
	}
	if b.Metadata.Name == "" {
		return trace.BadParameter("metadata.name: must be non-empty")
	}
	if b.Spec == nil {
		return trace.BadParameter("spec: must be non-nil")
	}
	if slices.Contains(b.Spec.Roles, "") {
		return trace.BadParameter("spec.roles: must not contain empty strings")
	}

	// Scoped bot only validation
	if b.Scope != "" {
		// Validate scope-specific fields
		if err := scopes.StrongValidate(b.Scope); err != nil {
			return trace.Wrap(err, "scope:")
		}

		// Validate unsupported fields aren't set.
		if len(b.Spec.Roles) > 0 {
			return trace.BadParameter("spec.roles: cannot be set on scoped bot")
		}
		if b.Spec.MaxSessionTtl.AsDuration() > 0 {
			return trace.BadParameter("spec.max_session_ttl: cannot be set on scoped bot")
		}
		if len(b.Spec.Traits) > 0 {
			return trace.BadParameter("spec.traits: cannot be set on scoped bot")
		}
	}

	return nil
}

// nonPropagatedLabels are labels that are not propagated from the User to the
// Bot when converting a User and Role to a Bot. Typically, these are internal
// labels that are managed by this service and exposing them to the end user
// would allow for misconfiguration.
var nonPropagatedLabels = set.New(
	types.BotLabel,
	types.BotGenerationLabel,
	types.BotScopeLabel,
)

// botFromUserAndRole converts a user and role to an unscoped bot. This should
// not be called on a scoped bot user.
//
// Typically, we treat the bot user as the "canonical" source of information
// where possible. The bot role should be used for information which cannot
// come from the bot user.
func botFromUserAndRole(user types.User, role types.Role) (*pb.Bot, error) {
	// User label is canonical source of bot name
	botName, ok := user.GetLabel(types.BotLabel)
	if !ok {
		return nil, trace.BadParameter("user missing bot label")
	}
	scope, _ := user.GetLabel(types.BotScopeLabel)
	if scope != "" {
		return nil, trace.BadParameter("botFromUserAndRole called on user with scope label")
	}

	b := &pb.Bot{
		Kind:    types.KindBot,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name:        botName,
			Expires:     botExpiryFromUser(user),
			Description: user.GetMetadata().Description,
		},
		Status: &pb.BotStatus{
			UserName: user.GetName(),
			RoleName: role.GetName(),
		},
		Spec: &pb.BotSpec{
			Roles:         role.GetImpersonateConditions(types.Allow).Roles,
			MaxSessionTtl: durationpb.New(role.GetOptions().MaxSessionTTL.Duration()),
		},
	}

	// Copy in labels from the user
	b.Metadata.Labels = map[string]string{}
	for k, v := range user.GetMetadata().Labels {
		// We exclude the labels that are implicitly added to the user by the
		// bot service.
		if nonPropagatedLabels.Contains(k) {
			continue
		}
		b.Metadata.Labels[k] = v
	}

	// Copy in traits
	for k, v := range user.GetTraits() {
		if len(v) == 0 {
			continue
		}
		b.Spec.Traits = append(b.Spec.Traits, &pb.Trait{
			Name:   k,
			Values: v,
		})
	}

	return b, nil
}

func scopedBotFromUser(user types.User) (*pb.Bot, error) {
	// User label is canonical source of bot name
	botName, ok := user.GetLabel(types.BotLabel)
	if !ok {
		return nil, trace.BadParameter("user missing bot label")
	}
	scope, ok := user.GetLabel(types.BotScopeLabel)
	if !ok || scope == "" {
		return nil, trace.BadParameter("scopedBotFromUser called on user without scope label")
	}

	b := &pb.Bot{
		Kind:    types.KindBot,
		Version: types.V1,
		Scope:   scope,
		Metadata: &headerv1.Metadata{
			Name:        botName,
			Expires:     botExpiryFromUser(user),
			Description: user.GetMetadata().Description,
		},
		Status: &pb.BotStatus{
			UserName: user.GetName(),
		},
		Spec: &pb.BotSpec{},
	}

	// Copy in labels from the user
	b.Metadata.Labels = map[string]string{}
	for k, v := range user.GetMetadata().Labels {
		// We exclude the labels that are implicitly added to the user by the
		// bot service.
		if nonPropagatedLabels.Contains(k) {
			continue
		}
		b.Metadata.Labels[k] = v
	}

	// TODO(noah): Should we weak validate here? We do not currently validate
	// on read for unscoped Bots.
	return b, nil
}

func botToUserAndRole(bot *pb.Bot, now time.Time, createdBy string) (types.User, types.Role, error) {
	if bot.Scope != "" {
		return nil, nil, trace.BadParameter("botToUserAndRole called on scoped bot")
	}

	// Setup role
	resourceName := BotResourceName(bot.Metadata.Name)

	// Continue to use the legacy max session TTL (12 hours) as the default, but
	// allow overrides via the optional bot spec field.
	maxSessionTTL := defaults.DefaultBotMaxSessionTTL
	if bot.Spec.MaxSessionTtl != nil {
		maxSessionTTL = bot.Spec.MaxSessionTtl.AsDuration()
	}

	role, err := types.NewRole(resourceName, types.RoleSpecV6{
		Options: types.RoleOptions{
			MaxSessionTTL: types.NewDuration(maxSessionTTL),
		},
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				// Bots read certificate authorities to watch for CA rotations
				types.NewRule(types.KindCertAuthority, []string{types.VerbReadNoSecrets}),
			},
			Impersonate: &types.ImpersonateConditions{
				Roles: bot.Spec.Roles,
			},
		},
	})
	if err != nil {
		return nil, nil, trace.Wrap(err, "new role")
	}
	roleMeta := role.GetMetadata()
	roleMeta.Description = fmt.Sprintf(
		"Automatically generated role for bot %s", bot.Metadata.Name,
	)
	roleMeta.Labels = map[string]string{
		types.BotLabel: bot.Metadata.Name,
	}
	roleMeta.Expires = userAndRoleExpiryFromBot(bot)
	role.SetMetadata(roleMeta)

	// Setup user
	user, err := types.NewUser(resourceName)
	if err != nil {
		return nil, nil, trace.Wrap(err, "new user")
	}
	user.SetRoles([]string{resourceName})
	userMeta := user.GetMetadata()

	// First copy in the labels from the Bot resource
	userMeta.Labels = map[string]string{}
	maps.Copy(userMeta.Labels, bot.Metadata.Labels)
	// Then set these labels over the top - we exclude these when converting
	// back.
	userMeta.Labels[types.BotLabel] = bot.Metadata.Name
	// We always set this to zero here - but in Upsert, we copy from the
	// previous user before writing if necessary
	userMeta.Labels[types.BotGenerationLabel] = "0"
	userMeta.Expires = userAndRoleExpiryFromBot(bot)
	// We track the Bot description within the User description field because
	// the Role description already has a message.
	userMeta.Description = bot.Metadata.Description
	user.SetMetadata(userMeta)

	traits := map[string][]string{}
	for _, t := range bot.Spec.Traits {
		if len(t.Values) == 0 {
			continue
		}
		if traits[t.Name] == nil {
			traits[t.Name] = []string{}
		}
		traits[t.Name] = append(traits[t.Name], t.Values...)
	}
	user.SetTraits(traits)
	user.SetCreatedBy(types.CreatedBy{
		User: types.UserRef{Name: createdBy},
		Time: now,
	})

	return user, role, nil
}

func scopedBotToUser(bot *pb.Bot, now time.Time, createdBy string) (types.User, error) {
	if bot.Scope == "" {
		return nil, trace.BadParameter("scopedBotToUser called on unscoped bot")
	}

	// Setup user
	user, err := types.NewUser(BotResourceName(bot.Metadata.Name))
	if err != nil {
		return nil, trace.Wrap(err, "new user")
	}
	userMeta := user.GetMetadata()

	// First copy in the labels from the Bot resource
	userMeta.Labels = map[string]string{}
	maps.Copy(userMeta.Labels, bot.Metadata.Labels)
	// Then set these labels over the top - we exclude these when converting
	// back.
	userMeta.Labels[types.BotLabel] = bot.Metadata.Name
	// We always set this to zero here - but in Upsert, we copy from the
	// previous user before writing if necessary
	userMeta.Labels[types.BotGenerationLabel] = "0"
	userMeta.Labels[types.BotScopeLabel] = bot.Scope
	userMeta.Expires = userAndRoleExpiryFromBot(bot)
	// We track the Bot description within the User description field because
	// the Role description already has a message.
	userMeta.Description = bot.Metadata.Description
	user.SetMetadata(userMeta)

	user.SetCreatedBy(types.CreatedBy{
		User: types.UserRef{Name: createdBy},
		Time: now,
	})

	return user, nil
}

func userAndRoleExpiryFromBot(bot *pb.Bot) *time.Time {
	if bot.Metadata.GetExpires() == nil {
		return nil
	}

	expiry := bot.Metadata.GetExpires().AsTime()
	if expiry.IsZero() || expiry.Unix() == 0 {
		return nil
	}
	return &expiry
}

func botExpiryFromUser(user types.User) *timestamppb.Timestamp {
	userMeta := user.GetMetadata()
	userExpiry := userMeta.Expiry()
	if userExpiry.IsZero() || userExpiry.Unix() == 0 {
		return nil
	}
	return timestamppb.New(userExpiry)
}
