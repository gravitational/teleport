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
	"github.com/gravitational/teleport/lib/auth/internal"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

// SessionService manages DelegationSession resources.
type SessionService struct {
	delegationv1.UnimplementedDelegationSessionServiceServer

	authorizer        authz.Authorizer
	profileReader     ProfileReader
	sessionReader     SessionReader
	sessionWriter     SessionWriter
	resourceLister    ResourceLister
	roleGetter        services.RoleGetter
	userGetter        services.UserGetter
	certGenerator     CertGenerator
	clusterNameGetter ClusterNameGetter
	appSessionCreator AppSessionCreator
	lockWriter        LockWriter
	logger            *slog.Logger
}

// SessionServiceConfig contains the configuration of the SessionService.
type SessionServiceConfig struct {
	// Authorizer is used to authorize the user.
	Authorizer authz.Authorizer

	// ProfileReader is used to read and list profile resources.
	ProfileReader ProfileReader

	// SessionReader is used to read delegation session resources.
	SessionReader SessionReader

	// SessionWriter is used to write session resources.
	SessionWriter SessionWriter

	// ResourceLister is used to list resources when checking permissions,
	ResourceLister ResourceLister

	// RoleGetter is used to read roles.
	RoleGetter services.RoleGetter

	// UserGetter is used to read users.
	UserGetter services.UserGetter

	// CertGenerator is used to generate delegation certificates.
	CertGenerator CertGenerator

	// ClusterNameGetter is used to get the local cluster name.
	ClusterNameGetter ClusterNameGetter

	// AppSessionCreator is used to create web sessions for application access.
	AppSessionCreator AppSessionCreator

	// LockWriter is used to create locks targeting delegation sessions.
	LockWriter LockWriter

	// Logger to which errors and messages are written.
	Logger *slog.Logger
}

// SessionReader is used to read delegation session resources.
type SessionReader interface {
	GetDelegationSession(ctx context.Context, id string) (*delegationv1.DelegationSession, error)
}

// SessionWriter is used to write delegation session resources.
type SessionWriter interface {
	CreateDelegationSession(ctx context.Context, session *delegationv1.DelegationSession) (*delegationv1.DelegationSession, error)
}

// ResourceLister is used to list resources when checking permissions.
type ResourceLister interface {
	ListResources(ctx context.Context, req proto.ListResourcesRequest) (*types.ListResourcesResponse, error)
}

// CertGenerator is used to generate delegation certificates.
type CertGenerator interface {
	Generate(ctx context.Context, req internal.CertRequest) (*proto.Certs, error)
}

// CertGeneratorFunc allows you to use a function as a CertGenerator.
type CertGeneratorFunc func(context.Context, internal.CertRequest) (*proto.Certs, error)

// Generate satisfies the CertGenerator interface.
func (fn CertGeneratorFunc) Generate(ctx context.Context, req internal.CertRequest) (*proto.Certs, error) {
	return fn(ctx, req)
}

// ClusterNameGetter is used to get the local cluster name.
type ClusterNameGetter interface {
	GetClusterName(context.Context) (types.ClusterName, error)
}

// AppSessionCreator is used to create web sessions for application access.
type AppSessionCreator interface {
	CreateAppSession(context.Context, internal.NewAppSessionRequest) (types.WebSession, error)
}

// AppSessionCreatorFunc allows you to use a function as an AppSessionCreator.
type AppSessionCreatorFunc func(context.Context, internal.NewAppSessionRequest) (types.WebSession, error)

// CreateAppSession satisfies the AppSessionCreator interface.
func (fn AppSessionCreatorFunc) CreateAppSession(ctx context.Context, req internal.NewAppSessionRequest) (types.WebSession, error) {
	return fn(ctx, req)
}

// LockWrite is used to create locks targeting delegation sessions.
type LockWriter interface {
	UpsertLock(ctx context.Context, lock types.Lock) error
}

// NewSessionService creates a SessionService with the given configuration.
func NewSessionService(cfg SessionServiceConfig) (*SessionService, error) {
	if cfg.Authorizer == nil {
		return nil, trace.BadParameter("missing parameter Authorizer")
	}
	if cfg.ProfileReader == nil {
		return nil, trace.BadParameter("missing parameter ProfileReader")
	}
	if cfg.SessionReader == nil {
		return nil, trace.BadParameter("missing parameter SessionReader")
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
	if cfg.CertGenerator == nil {
		return nil, trace.BadParameter("missing parameter CertGenerator")
	}
	if cfg.ClusterNameGetter == nil {
		return nil, trace.BadParameter("missing parameter ClusterNameGetter")
	}
	if cfg.AppSessionCreator == nil {
		return nil, trace.BadParameter("missing parameter AppSessionCreator")
	}
	if cfg.LockWriter == nil {
		return nil, trace.BadParameter("missing parameter LockWriter")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &SessionService{
		authorizer:        cfg.Authorizer,
		profileReader:     cfg.ProfileReader,
		sessionReader:     cfg.SessionReader,
		sessionWriter:     cfg.SessionWriter,
		resourceLister:    cfg.ResourceLister,
		roleGetter:        cfg.RoleGetter,
		userGetter:        cfg.UserGetter,
		certGenerator:     cfg.CertGenerator,
		appSessionCreator: cfg.AppSessionCreator,
		clusterNameGetter: cfg.ClusterNameGetter,
		lockWriter:        cfg.LockWriter,
		logger:            cfg.Logger,
	}, nil
}
