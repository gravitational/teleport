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

package usersv1

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	userspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/okta"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/forproto"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// Cache is the subset of the cached resources that the Service queries.
type Cache interface {
	// GetUser returns a user by name.
	GetUser(ctx context.Context, user string, withSecrets bool) (types.User, error)
	// ListUsersForProto is from [authclient.Cache].
	ListUsersForProto(ctx context.Context, req *userspb.ListUsersRequest) (_ []forproto.User, nextPageToken string, _ error)
	// GetRole returns a role by name.
	GetRole(ctx context.Context, name string) (types.Role, error)
}

// Backend is the subset of the backend resources that the Service modifies.
type Backend interface {
	// GetUser returns a user by name.
	GetUser(ctx context.Context, user string, withSecrets bool) (types.User, error)
	// CreateUser creates user, only if the user entry does not exist
	CreateUser(ctx context.Context, user types.User) (types.User, error)
	// UpdateUser updates an existing user if revisions match.
	UpdateUser(ctx context.Context, user types.User) (types.User, error)
	// UpsertUser creates a new user or forcefully updates an existing user.
	UpsertUser(ctx context.Context, user types.User) (types.User, error)
	// DeleteRole deletes a role by name.
	DeleteRole(ctx context.Context, name string) error
	// DeleteUser deletes a user and all associated objects.
	DeleteUser(ctx context.Context, user string) error
}

type AuthServer interface {
	// CreateResetPasswordToken resets a local user's credentials and creates a reset password token.
	CreateResetPasswordToken(ctx context.Context, req authclient.CreateUserTokenRequest) (types.UserToken, error)
	// ResetMFADevices removes all of the user's MFA devices and returns how many were removed.
	ResetMFADevices(ctx context.Context, user string) (int, error)
}

// ServiceConfig holds configuration options for
// the users gRPC service.
type ServiceConfig struct {
	Authorizer       authz.Authorizer
	ScopedAuthorizer authz.ScopedAuthorizer
	Cache            Cache
	Backend          Backend
	AuthServer       AuthServer
	Logger           *slog.Logger
	Emitter          apievents.Emitter
	Reporter         usagereporter.UsageReporter
	Clock            clockwork.Clock
}

// Service implements the teleport.users.v1.UsersService RPC service.
type Service struct {
	userspb.UnimplementedUsersServiceServer

	authorizer       authz.Authorizer
	scopedAuthorizer authz.ScopedAuthorizer
	cache            Cache
	backend          Backend
	authServer       AuthServer
	logger           *slog.Logger
	emitter          apievents.Emitter
	reporter         usagereporter.UsageReporter
	clock            clockwork.Clock
}

// NewService returns a new users gRPC service.
func NewService(cfg ServiceConfig) (*Service, error) {
	switch {
	case cfg.Cache == nil:
		return nil, trace.BadParameter("cache service is required")
	case cfg.Backend == nil:
		return nil, trace.BadParameter("backend service is required")
	case cfg.AuthServer == nil:
		return nil, trace.BadParameter("auth server is required")
	case cfg.Authorizer == nil:
		return nil, trace.BadParameter("authorizer is required")
	case cfg.ScopedAuthorizer == nil:
		return nil, trace.BadParameter("scoped authorizer is required")
	case cfg.Emitter == nil:
		return nil, trace.BadParameter("emitter is required")
	case cfg.Reporter == nil:
		return nil, trace.BadParameter("reporter is required")
	}

	if cfg.Logger == nil {
		cfg.Logger = slog.With(teleport.ComponentKey, "users.service")
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}

	return &Service{
		logger:           cfg.Logger,
		authorizer:       cfg.Authorizer,
		scopedAuthorizer: cfg.ScopedAuthorizer,
		cache:            cfg.Cache,
		backend:          cfg.Backend,
		authServer:       cfg.AuthServer,
		emitter:          cfg.Emitter,
		reporter:         cfg.Reporter,
		clock:            cfg.Clock,
	}, nil
}

// currentUserAction is a special checker that allows certain actions for users
// even if they are not admins, e.g. update their own passwords,
// or generate certificates, otherwise it will require admin privileges
func currentUserAction(scopedCtx *authz.ScopedContext, username string) error {
	if authz.ScopedIsCurrentUser(scopedCtx, username) {
		return nil
	}

	unscopedCtx, isUnscoped := scopedCtx.UnscopedContext()
	if !isUnscoped {
		return trace.AccessDenied("scoped users can not create other users")
	}

	if authz.IsCurrentUser(*unscopedCtx, username) {
		return nil
	}

	return unscopedCtx.Checker.CheckAccessToRule(&services.Context{User: unscopedCtx.User},
		apidefaults.Namespace, types.KindUser, types.VerbCreate)
}

func (s *Service) getCurrentUser(ctx context.Context, scopedCtx *authz.ScopedContext) (*types.UserV2, error) {
	// scopedCtx.User can be nil in the case of a scopedCtx built on authz.ScopedBuiltinRole
	if scopedCtx.User == nil {
		return nil, trace.BadParameter("expected a current authenticated user, identity likely a builtin system role")
	}

	// check access to roles
	for _, role := range scopedCtx.User.GetRoles() {
		_, err := s.cache.GetRole(ctx, role)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	withoutSecrets := scopedCtx.User.WithoutSecrets()
	user, ok := withoutSecrets.(types.User)
	if !ok {
		return nil, trace.BadParameter("expected types.User when fetching current user information, got %T", withoutSecrets)
	}

	v2, ok := user.(*types.UserV2)
	if !ok {
		return nil, trace.BadParameter("encountered unexpected user type")
	}

	return v2, nil
}

func (s *Service) GetUser(ctx context.Context, req *userspb.GetUserRequest) (*userspb.GetUserResponse, error) {
	scopedCtx, err := s.scopedAuthorizer.AuthorizeScoped(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if req.GetName() == "" && req.GetCurrentUser() {
		user, err := s.getCurrentUser(ctx, scopedCtx)
		return userspb.GetUserResponse_builder{User: user}.Build(), trace.Wrap(err)
	}

	unscopedCtx, isUnscoped := scopedCtx.UnscopedContext()
	if req.GetWithSecrets() {
		if !isUnscoped {
			return nil, trace.AccessDenied("this request can only be executed by an unscoped admin")
		}
		// TODO(fspmarshall): replace admin requirement with VerbReadWithSecrets once we've
		// migrated to that model.
		if !authz.HasBuiltinRole(*unscopedCtx, string(types.RoleAdmin)) {
			err := trace.AccessDenied("user %q requested access to user %q with secrets", unscopedCtx.User.GetName(), req.GetName())
			s.logger.WarnContext(ctx, "user does not have permission to read user with secrets", "error", err)
			if err := s.emitter.EmitAuditEvent(ctx, &apievents.UserLogin{
				Metadata: apievents.Metadata{
					Type: events.UserLoginEvent,
					Code: events.UserLocalLoginFailureCode,
				},
				Method: events.LoginMethodClientCert,
				Status: apievents.Status{
					Success:     false,
					Error:       trace.Unwrap(err).Error(),
					UserMessage: err.Error(),
				},
			}); err != nil {
				s.logger.WarnContext(ctx, "Failed to emit local login failure event", "error", err)
			}
			return nil, trace.AccessDenied("this request can only be executed by an admin")
		}
	} else {
		// if secrets are not being accessed, let users always read
		// their own info.
		if err := currentUserAction(scopedCtx, req.GetName()); err != nil {
			ruleCtx := scopedCtx.RuleContext()
			if err := scopedCtx.CheckerContext.RiskyAuthorizeUnpinnedRead(ctx, services.UnpinnedReadUser, &ruleCtx); err != nil {
				return nil, trace.Wrap(err)
			}
		}
	}

	user, err := s.cache.GetUser(ctx, req.Name, req.WithSecrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	v2, ok := user.(*types.UserV2)
	if !ok {
		s.logger.WarnContext(ctx, "unexpected user type",
			"got_type", logutils.TypeAttr(user),
			"expected_type", "UserV2",
			"user", user.GetName(),
		)
		return nil, trace.BadParameter("encountered unexpected user type")
	}

	return &userspb.GetUserResponse{User: v2}, nil
}

func (s *Service) CreateUser(ctx context.Context, req *userspb.CreateUserRequest) (*userspb.CreateUserResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	reqUser := req.GetUser()
	if reqUser == nil {
		return nil, trace.BadParameter("user: must be not-nil")
	}

	if len(reqUser.GetName()) > teleport.MaxUsernameLength {
		return nil, trace.BadParameter("username exceeds maximum length of %d characters", teleport.MaxUsernameLength)
	}

	if err := authCtx.CheckAccessToKind(types.KindUser, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	// Support reused MFA for bulk tctl create requests and chained invite commands (CreateResetPasswordToken).
	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err = okta.CheckOrigin(authCtx, reqUser); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := services.ValidateUser(reqUser); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := services.ValidateUserRoles(ctx, reqUser, s.cache); err != nil {
		return nil, trace.Wrap(err)
	}

	if reqUser.GetCreatedBy().IsEmpty() {
		reqUser.SetCreatedBy(types.CreatedBy{
			User: types.UserRef{Name: authCtx.User.GetName()},
			Time: s.clock.Now().UTC(),
		})
	}

	created, err := s.backend.CreateUser(ctx, reqUser)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	connectorName := constants.LocalConnector
	if created.GetCreatedBy().Connector != nil {
		connectorName = created.GetCreatedBy().Connector.ID
	}

	if err := s.emitter.EmitAuditEvent(ctx, &apievents.UserCreate{
		Metadata: apievents.Metadata{
			Type: events.UserCreateEvent,
			Code: events.UserCreateCode,
		},
		UserMetadata: authCtx.GetUserMetadata(),
		ResourceMetadata: apievents.ResourceMetadata{
			Name:    created.GetName(),
			Expires: created.Expiry(),
		},
		Connector:          connectorName,
		Roles:              created.GetRoles(),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
	}); err != nil {
		s.logger.WarnContext(ctx, "Failed to emit user create event", "error", err)
	}

	usagereporter.EmitEditorChangeEvent(created.GetName(), nil, created.GetRoles(), s.reporter.AnonymizeAndSubmit)

	v2, ok := created.(*types.UserV2)
	if !ok {
		s.logger.WarnContext(ctx, "unexpected user type",
			"got_type", logutils.TypeAttr(created),
			"expected_type", "UserV2",
			"user", created.GetName(),
		)
		return nil, trace.BadParameter("encountered unexpected user type")
	}

	return &userspb.CreateUserResponse{User: v2}, nil
}

func (s *Service) UpdateUser(ctx context.Context, req *userspb.UpdateUserRequest) (*userspb.UpdateUserResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindUser, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	// Allow reused MFA responses to allow Updating a user after get (WebUI).
	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err = okta.CheckOrigin(authCtx, req.User); err != nil {
		return nil, trace.Wrap(err)
	}

	// ValidateUser is called a bit later by LegacyUpdateUser. However, it's clearer
	// to do it here like the other verbs, plus it won't break again when we'll
	// get rid of the legacy update function.
	if err := services.ValidateUser(req.User); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := services.ValidateUserRoles(ctx, req.User, s.cache); err != nil {
		return nil, trace.Wrap(err)
	}

	prevUser, err := s.cache.GetUser(ctx, req.User.GetName(), false)
	var omitEditorEvent bool
	if err != nil {
		// don't return error here since this call is for event emitting purposes only
		s.logger.WarnContext(ctx, "Failed getting previous user during update", "error", err)
		omitEditorEvent = true
	}

	if prevUser != nil {
		// Preserve the users' created by information.
		req.User.SetCreatedBy(prevUser.GetCreatedBy())
	}

	if err = okta.CheckAccess(authCtx, prevUser, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	updated, err := s.backend.UpdateUser(ctx, req.User)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	connectorName := constants.LocalConnector
	if updated.GetCreatedBy().Connector != nil {
		connectorName = updated.GetCreatedBy().Connector.ID
	}

	if err := s.emitter.EmitAuditEvent(ctx, &apievents.UserCreate{
		Metadata: apievents.Metadata{
			Type: events.UserUpdatedEvent,
			Code: events.UserUpdateCode,
		},
		UserMetadata: authCtx.GetUserMetadata(),
		ResourceMetadata: apievents.ResourceMetadata{
			Name:    updated.GetName(),
			Expires: updated.Expiry(),
		},
		Connector:          connectorName,
		Roles:              updated.GetRoles(),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
	}); err != nil {
		s.logger.WarnContext(ctx, "Failed to emit user update event", "error", err)
	}

	if !omitEditorEvent {
		usagereporter.EmitEditorChangeEvent(updated.GetName(), prevUser.GetRoles(), updated.GetRoles(), s.reporter.AnonymizeAndSubmit)
	}

	v2, ok := updated.(*types.UserV2)
	if !ok {
		s.logger.WarnContext(ctx, "unexpected user type",
			"got_type", logutils.TypeAttr(updated),
			"expected_type", "UserV2",
			"user", updated.GetName(),
		)
		return nil, trace.BadParameter("encountered unexpected user type")
	}

	return &userspb.UpdateUserResponse{User: v2}, nil
}

func (s *Service) UpsertUser(ctx context.Context, req *userspb.UpsertUserRequest) (*userspb.UpsertUserResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindUser, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	// Support reused MFA for bulk tctl create requests.
	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := services.ValidateUser(req.User); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := services.ValidateUserRoles(ctx, req.User, s.cache); err != nil {
		return nil, trace.Wrap(err)
	}

	if createdBy := req.User.GetCreatedBy(); createdBy.IsEmpty() {
		req.User.SetCreatedBy(types.CreatedBy{
			User: types.UserRef{Name: authCtx.User.GetName()},
		})
	}

	prevUser, err := s.cache.GetUser(ctx, req.User.GetName(), false)
	var omitEditorEvent bool
	if err != nil {
		// don't return error here since this call is for event emitting purposes only
		s.logger.WarnContext(ctx, "Failed getting previous user during update", "error", err)
		omitEditorEvent = true
	}

	verb := types.VerbUpdate
	if prevUser == nil {
		verb = types.VerbCreate
	}

	if err = okta.CheckOrigin(authCtx, req.User); err != nil {
		return nil, trace.Wrap(err)
	}

	if err = okta.CheckAccess(authCtx, prevUser, verb); err != nil {
		return nil, trace.Wrap(err)
	}

	upserted, err := s.backend.UpsertUser(ctx, req.User)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	connectorName := constants.LocalConnector
	if upserted.GetCreatedBy().Connector != nil {
		connectorName = upserted.GetCreatedBy().Connector.ID
	}

	if err := s.emitter.EmitAuditEvent(ctx, &apievents.UserCreate{
		Metadata: apievents.Metadata{
			Type: events.UserCreateEvent,
			Code: events.UserCreateCode,
		},
		UserMetadata: authCtx.GetUserMetadata(),
		ResourceMetadata: apievents.ResourceMetadata{
			Name:    upserted.GetName(),
			Expires: upserted.Expiry(),
		},
		Connector:          connectorName,
		Roles:              upserted.GetRoles(),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
	}); err != nil {
		s.logger.WarnContext(ctx, "Failed to emit user upsert event", "error", err)
	}

	if !omitEditorEvent {
		usagereporter.EmitEditorChangeEvent(upserted.GetName(), prevUser.GetRoles(), upserted.GetRoles(), s.reporter.AnonymizeAndSubmit)
	}

	v2, ok := upserted.(*types.UserV2)
	if !ok {
		s.logger.WarnContext(ctx, "unexpected user type",
			"got_type", logutils.TypeAttr(upserted),
			"expected_type", "UserV2",
			"user", upserted.GetName(),
		)
		return nil, trace.BadParameter("encountered unexpected user type")
	}

	return &userspb.UpsertUserResponse{User: v2}, nil
}

func (s *Service) DeleteUser(ctx context.Context, req *userspb.DeleteUserRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindUser, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	prevUser, err := s.cache.GetUser(ctx, req.Name, false)
	var omitEditorEvent bool
	if err != nil && !trace.IsNotFound(err) {
		// don't return error here, delete may still succeed
		s.logger.WarnContext(ctx, "Failed getting previous user during delete operation", "error", err)
		prevUser = nil
		omitEditorEvent = true
	}

	if err = okta.CheckAccess(authCtx, prevUser, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	role, err := s.cache.GetRole(ctx, services.RoleNameForUser(req.Name))
	if err != nil {
		if !trace.IsNotFound(err) {
			return &emptypb.Empty{}, trace.Wrap(err)
		}
	} else {
		if err := s.backend.DeleteRole(ctx, role.GetName()); err != nil {
			if !trace.IsNotFound(err) {
				return &emptypb.Empty{}, trace.Wrap(err)
			}
		}
	}

	if err := s.backend.DeleteUser(ctx, req.Name); err != nil {
		return &emptypb.Empty{}, trace.Wrap(err)
	}

	// If the user was successfully deleted, emit an event.
	if err := s.emitter.EmitAuditEvent(ctx, &apievents.UserDelete{
		Metadata: apievents.Metadata{
			Type: events.UserDeleteEvent,
			Code: events.UserDeleteCode,
		},
		UserMetadata: authz.ClientUserMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: req.Name,
		},
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
	}); err != nil {
		s.logger.WarnContext(ctx, "Failed to emit user delete event", "error", err)
	}

	if !omitEditorEvent {
		usagereporter.EmitEditorChangeEvent(req.Name, prevUser.GetRoles(), nil, s.reporter.AnonymizeAndSubmit)
	}

	return &emptypb.Empty{}, nil
}

func (s *Service) ListUsers(ctx context.Context, req *userspb.ListUsersRequest) (*userspb.ListUsersResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if req.WithSecrets {
		// TODO(fspmarshall): replace admin requirement with VerbReadWithSecrets once we've
		// migrated to that model.
		if !authz.HasBuiltinRole(*authCtx, string(types.RoleAdmin)) {
			err := trace.AccessDenied("user %q requested access to all users with secrets", authCtx.User.GetName())
			s.logger.WarnContext(ctx, "user does not have permission to read all users with secrets", "error", err)
			if err := s.emitter.EmitAuditEvent(ctx, &apievents.UserLogin{
				Metadata: apievents.Metadata{
					Type: events.UserLoginEvent,
					Code: events.UserLocalLoginFailureCode,
				},
				Method: events.LoginMethodClientCert,
				Status: apievents.Status{
					Success:     false,
					Error:       trace.Unwrap(err).Error(),
					UserMessage: err.Error(),
				},
			}); err != nil {
				s.logger.WarnContext(ctx, "Failed to emit local login failure event", "error", err)
			}
			return nil, trace.AccessDenied("this request can be only executed by an admin")
		}
	} else {
		if err := authCtx.CheckAccessToKind(types.KindUser, types.VerbList, types.VerbRead); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return forproto.ToListUsersResponse(s.cache.ListUsersForProto(ctx, req))
}

func (s *Service) ResetUser(ctx context.Context, req *userspb.ResetUserRequest) (*userspb.ResetUserResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		s.emitUserResetEvent(ctx, req.GetName(), err)
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindUser, types.VerbUpdate); err != nil {
		s.emitUserResetEvent(ctx, req.GetName(), err)
		return nil, trace.Wrap(err)
	}

	if authz.IsLocalOrRemoteService(*authCtx) && !authz.HasBuiltinRole(*authCtx, string(types.RoleAdmin)) {
		err := trace.AccessDenied("access denied")
		s.emitUserResetEvent(ctx, req.GetName(), err)
		return nil, err
	}

	// Don't emit a failure event here since a reused MFA challenge is a normal part of the
	// admin action flow
	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := s.resetUser(ctx, req)
	s.emitUserResetEvent(ctx, req.GetName(), err)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

// resetUser performs the reset once the caller has been authorized, resolving whether the
// user is local, SSO, or already deleted and returning the response for the client.
func (s *Service) resetUser(ctx context.Context, req *userspb.ResetUserRequest) (*userspb.ResetUserResponse, error) {
	var tokenType string
	switch req.GetType() {
	case userspb.ResetUserTokenType_RESET_USER_TOKEN_TYPE_UNSPECIFIED, userspb.ResetUserTokenType_RESET_USER_TOKEN_TYPE_PASSWORD:
		tokenType = authclient.UserTokenTypeResetPassword
	case userspb.ResetUserTokenType_RESET_USER_TOKEN_TYPE_INVITE:
		tokenType = authclient.UserTokenTypeResetPasswordInvite
	default:
		return nil, trace.BadParameter("unknown token type %q", req.GetType())
	}

	tokenReq := authclient.CreateUserTokenRequest{
		Name: req.GetName(),
		TTL:  req.GetTtl().AsDuration(),
		Type: tokenType,
	}
	if err := tokenReq.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	// We read from the backend rather than the cache here so that a newly created local user that
	// hasn't propagated to the cache yet won't be mistaken for a deleted one, which would remove
	// their MFA devices without giving us a reset token.
	user, err := s.backend.GetUser(ctx, tokenReq.Name, false)
	if trace.IsNotFound(err) {
		return s.resetDeletedUser(ctx, tokenReq.Name)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if user.GetUserType() == types.UserTypeSSO {
		removed, err := s.authServer.ResetMFADevices(ctx, user.GetName())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// For SSO users the response is just the number of MFA devices removed
		return userspb.ResetUserResponse_builder{
			UserKind:          userspb.ResetUserKind_RESET_USER_KIND_SSO,
			MfaDevicesRemoved: int32(removed),
		}.Build(), nil
	}

	// CreateResetPasswordToken resets the user's password and also removes their MFA devices
	token, err := s.authServer.CreateResetPasswordToken(ctx, tokenReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tokenV3, ok := token.(*types.UserTokenV3)
	if !ok {
		return nil, trace.BadParameter("unexpected UserToken type %T", token)
	}
	// For local users the response is just the password reset token
	return userspb.ResetUserResponse_builder{
		UserKind:           userspb.ResetUserKind_RESET_USER_KIND_LOCAL,
		PasswordResetToken: tokenV3,
	}.Build(), nil
}

// emitUserResetEvent emits an audit event that the named user's authentication was reset.
func (s *Service) emitUserResetEvent(ctx context.Context, name string, resetErr error) {
	code := events.UserResetCode
	var errorMessage string
	if resetErr != nil {
		code = events.UserResetFailureCode
		errorMessage = resetErr.Error()
	}
	if err := s.emitter.EmitAuditEvent(ctx, &apievents.UserReset{
		Metadata: apievents.Metadata{
			Type: events.UserResetEvent,
			Code: code,
		},
		UserMetadata: authz.ClientUserMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: name,
		},
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status: apievents.Status{
			Success: resetErr == nil,
			Error:   errorMessage,
		},
	}); err != nil {
		s.logger.WarnContext(ctx, "Failed to emit user reset event", "error", err)
	}
}

// resetDeletedUser removes MFA devices for a user that no longer exists (aka an expired SSO user)
func (s *Service) resetDeletedUser(ctx context.Context, name string) (*userspb.ResetUserResponse, error) {
	removed, err := s.authServer.ResetMFADevices(ctx, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if removed == 0 {
		return nil, trace.NotFound("user %q not found", name)
	}
	return userspb.ResetUserResponse_builder{
		UserKind:          userspb.ResetUserKind_RESET_USER_KIND_DELETED,
		MfaDevicesRemoved: int32(removed),
	}.Build(), nil
}
