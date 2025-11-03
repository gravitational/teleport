/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package delegationv1

import (
	"context"
	"log/slog"

	"github.com/gravitational/teleport/api/client/proto"
	delegationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/delegation/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

// SessionService manages DelegationSession resources.
type SessionService struct {
	delegationv1.UnimplementedDelegationSessionServiceServer

	authorizer     authz.Authorizer
	profileReader  ProfileReader
	sessionWriter  SessionWriter
	resourceLister ResourceLister
	roleGetter     services.RoleGetter
	userGetter     services.UserGetter
	logger         *slog.Logger
}

// SessionServiceConfig contains the configuration of the SessionService.
type SessionServiceConfig struct {
	// Authorizer is used to authorize the user.
	Authorizer authz.Authorizer

	// ProfileReader is used to read and list profile resources.
	ProfileReader ProfileReader

	// SessionWriter is used to write session resources.
	SessionWriter SessionWriter

	// ResourceLister is used to list resources when checking permissions,
	ResourceLister ResourceLister

	// RoleGetter is used to read roles.
	RoleGetter services.RoleGetter

	// UserGetter is used to read users.
	UserGetter services.UserGetter

	// Logger to which errors and messages are written.
	Logger *slog.Logger
}

// SessionWriter is used to write delegation session resources.
type SessionWriter interface {
	CreateDelegationSession(ctx context.Context, session *delegationv1.DelegationSession) (*delegationv1.DelegationSession, error)
}

// ResourceLister is used to list resources when checking permissions.
type ResourceLister interface {
	ListResources(ctx context.Context, req proto.ListResourcesRequest) (*types.ListResourcesResponse, error)
}

// NewSessionService creates a SessionService with the given configuration.
func NewSessionService(cfg SessionServiceConfig) (*SessionService, error) {
	if cfg.Authorizer == nil {
		return nil, trace.BadParameter("missing parameter Authorizer")
	}
	if cfg.ProfileReader == nil {
		return nil, trace.BadParameter("missing parameter ProfileReader")
	}
	if cfg.SessionWriter == nil {
		return nil, trace.BadParameter("missing parameter SessionWriter")
	}
	if cfg.ResourceLister == nil {
		return nil, trace.BadParameter("missing parameter ResourceLister")
	}
	if cfg.RoleGetter == nil {
		return nil, trace.BadParameter("missing parameter RoleGetter")
	}
	if cfg.UserGetter == nil {
		return nil, trace.BadParameter("missing parameter UserGetter")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &SessionService{
		authorizer:     cfg.Authorizer,
		profileReader:  cfg.ProfileReader,
		sessionWriter:  cfg.SessionWriter,
		resourceLister: cfg.ResourceLister,
		roleGetter:     cfg.RoleGetter,
		userGetter:     cfg.UserGetter,
		logger:         cfg.Logger,
	}, nil
}
