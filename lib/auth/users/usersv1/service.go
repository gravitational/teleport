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

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	userspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/okta"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
)

// Cache is the subset of the cached resources that the Service queries.
type Cache interface {
	// GetUser returns a user by name.
	GetUser(ctx context.Context, user string, withSecrets bool) (types.User, error)
	// ListUsersExt returns a page of users.
	ListUsersExt(ctx context.Context, req *userspb.ListUsersRequest) (*userspb.ListUsersResponse, error)
	// GetRole returns a role by name.
	GetRole(ctx context.Context, name string) (types.Role, error)
}

// Backend is the subset of the backend resources that the Service modifies.
type Backend interface {
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

// ServiceConfig holds configuration options for
// the users gRPC service.
type ServiceConfig struct {
	Authorizer authz.Authorizer
	Cache      Cache
	Backend    Backend
	Logger     logrus.FieldLogger
	Emitter    apievents.Emitter
	Reporter   usagereporter.UsageReporter
	Clock      clockwork.Clock
}

// Service implements the teleport.users.v1.UsersService RPC service.
type Service struct {
	userspb.UnimplementedUsersServiceServer

	authorizer authz.Authorizer
	cache      Cache
	backend    Backend
	logger     logrus.FieldLogger
	emitter    apievents.Emitter
	reporter   usagereporter.UsageReporter
	clock      clockwork.Clock
}

// NewService returns a new users gRPC service.
func NewService(cfg ServiceConfig) (*Service, error) {
	switch {
	case cfg.Cache == nil:
		return nil, trace.BadParameter("cache service is required")
	case cfg.Backend == nil:
		return nil, trace.BadParameter("backend service is required")
	case cfg.Authorizer == nil:
		return nil, trace.BadParameter("authorizer is required")
	case cfg.Emitter == nil:
		return nil, trace.BadParameter("emitter is required")
	case cfg.Reporter == nil:
		return nil, trace.BadParameter("reporter is required")
	}

	if cfg.Logger == nil {
		cfg.Logger = logrus.WithField(teleport.ComponentKey, "users.service")
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}

	return &Service{
		logger:     cfg.Logger,
		authorizer: cfg.Authorizer,
		cache:      cfg.Cache,
		backend:    cfg.Backend,
		emitter:    cfg.Emitter,
		reporter:   cfg.Reporter,
		clock:      cfg.Clock,
	}, nil
}

// currentUserAction is a special checker that allows certain actions for users
// even if they are not admins, e.g. update their own passwords,
// or generate certificates, otherwise it will require admin privileges
func currentUserAction(authzContext authz.Context, username string) error {
	if authz.IsLocalUser(authzContext) && username == authzContext.User.GetName() {
		return nil
	}
	return authzContext.Checker.CheckAccessToRule(&services.Context{User: authzContext.User},
		apidefaults.Namespace, types.KindUser, types.VerbCreate)
}

func (s *Service) getCurrentUser(ctx context.Context, authCtx *authz.Context) (*types.UserV2, error) {
	// check access to roles
	for _, role := range authCtx.User.GetRoles() {
		_, err := s.cache.GetRole(ctx, role)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	withoutSecrets := authCtx.User.WithoutSecrets()
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
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if req.Name == "" && req.CurrentUser {
		user, err := s.getCurrentUser(ctx, authCtx)
		return &userspb.GetUserResponse{User: user}, trace.Wrap(err)
	}

	if req.WithSecrets {
		// TODO(fspmarshall): replace admin requirement with VerbReadWithSecrets once we've
		// migrated to that model.
		if !authz.HasBuiltinRole(*authCtx, string(types.RoleAdmin)) {
			err := trace.AccessDenied("user %q requested access to user %q with secrets", authCtx.User.GetName(), req.Name)
			s.logger.Warn(err)
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
				s.logger.WithError(err).Warn("Failed to emit local login failure event.")
			}
			return nil, trace.AccessDenied("this request can be only executed by an admin")
		}
	} else {
		// if secrets are not being accessed, let users always read
		// their own info.
		if err := currentUserAction(*authCtx, req.Name); err != nil {
			// not current user, perform normal permission check.
			if err := authCtx.CheckAccessToKind(types.KindUser, types.VerbRead); err != nil {
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
		s.logger.Warnf("expected type UserV2, got %T for user %q", user, user.GetName())
		return nil, trace.BadParameter("encountered unexpected user type")
	}

	return &userspb.GetUserResponse{User: v2}, nil
}

func (s *Service) CreateUser(ctx context.Context, req *userspb.CreateUserRequest) (*userspb.CreateUserResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindUser, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	// Support reused MFA for bulk tctl create requests and chained invite commands (CreateResetPasswordToken).
	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err = okta.CheckOrigin(authCtx, req.User); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := services.ValidateUser(req.User); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := services.ValidateUserRoles(ctx, req.User, s.cache); err != nil {
		return nil, trace.Wrap(err)
	}

	if req.User.GetCreatedBy().IsEmpty() {
		req.User.SetCreatedBy(types.CreatedBy{
			User: types.UserRef{Name: authCtx.User.GetName()},
			Time: s.clock.Now().UTC(),
		})
	}

	created, err := s.backend.CreateUser(ctx, req.User)
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
		s.logger.WithError(err).Warn("Failed to emit user create event.")
	}

	usagereporter.EmitEditorChangeEvent(created.GetName(), nil, created.GetRoles(), s.reporter.AnonymizeAndSubmit)

	v2, ok := created.(*types.UserV2)
	if !ok {
		s.logger.Warnf("expected type UserV2, got %T for user %q", created, created.GetName())
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
		s.logger.WithError(err).Warn("Failed getting previous user during update")
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
		s.logger.WithError(err).Warn("Failed to emit user update event.")
	}

	if !omitEditorEvent {
		usagereporter.EmitEditorChangeEvent(updated.GetName(), prevUser.GetRoles(), updated.GetRoles(), s.reporter.AnonymizeAndSubmit)
	}

	v2, ok := updated.(*types.UserV2)
	if !ok {
		s.logger.Warnf("expected type UserV2, got %T for user %q", updated, updated.GetName())
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
		s.logger.WithError(err).Warn("Failed getting previous user during update")
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
		s.logger.WithError(err).Warn("Failed to emit user upsert event.")
	}

	if !omitEditorEvent {
		usagereporter.EmitEditorChangeEvent(upserted.GetName(), prevUser.GetRoles(), upserted.GetRoles(), s.reporter.AnonymizeAndSubmit)
	}

	v2, ok := upserted.(*types.UserV2)
	if !ok {
		s.logger.Warnf("expected type UserV2, got %T for user %q", upserted, upserted.GetName())
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

	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	prevUser, err := s.cache.GetUser(ctx, req.Name, false)
	var omitEditorEvent bool
	if err != nil && !trace.IsNotFound(err) {
		// don't return error here, delete may still succeed
		s.logger.WithError(err).Warn("Failed getting previous user during delete operation")
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
		s.logger.WithError(err).Warn("Failed to emit user delete event.")
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
			s.logger.Warn(err)
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
				s.logger.WithError(err).Warn("Failed to emit local login failure event.")
			}
			return nil, trace.AccessDenied("this request can be only executed by an admin")
		}
	} else {
		if err := authCtx.CheckAccessToKind(types.KindUser, types.VerbList, types.VerbRead); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	rsp, err := s.cache.ListUsersExt(ctx, req)
	return rsp, trace.Wrap(err)
}
