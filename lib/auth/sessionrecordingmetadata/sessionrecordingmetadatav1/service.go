/*
 * *
 *  * Teleport
 *  * Copyright (C) 2024 Gravitational, Inc.
 *  *
 *  * This program is free software: you can redistribute it and/or modify
 *  * it under the terms of the GNU Affero General Public License as published by
 *  * the Free Software Foundation, either version 3 of the License, or
 *  * (at your option) any later version.
 *  *
 *  * This program is distributed in the hope that it will be useful,
 *  * but WITHOUT ANY WARRANTY; without even the implied warranty of
 *  * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 *  * GNU Affero General Public License for more details.
 *  *
 *  * You should have received a copy of the GNU Affero General Public License
 *  * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package sessionrecordingmetadatav1

import (
	"context"
	"log/slog"

	"github.com/gravitational/teleport"
	sessionrecordingmetadatapb "github.com/gravitational/teleport/api/gen/proto/go/teleport/sessionrecordingmetatada/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/emptypb"
)

type Backend interface {
	CreateSessionRecordingMetadata(ctx context.Context, metadata *sessionrecordingmetadatapb.SessionRecordingMetadata) (*sessionrecordingmetadatapb.SessionRecordingMetadata, error)
	UpdateSessionRecordingMetadata(ctx context.Context, metadata *sessionrecordingmetadatapb.SessionRecordingMetadata) (*sessionrecordingmetadatapb.SessionRecordingMetadata, error)
	GetSessionRecordingMetadata(ctx context.Context, sessionID string) (*sessionrecordingmetadatapb.SessionRecordingMetadata, error)
	DeleteSessionRecordingMetadata(ctx context.Context, sessionID string) error
	ListSessionRecordingMetadata(ctx context.Context, pageSize int, nextToken string, sessionIDs []string, withSummary bool) ([]*sessionrecordingmetadatapb.SessionRecordingMetadata, string, error)
}

type ServiceConfig struct {
	Authorizer authz.Authorizer
	Backend    Backend
	Logger     *slog.Logger
}

type Service struct {
	sessionrecordingmetadatapb.UnimplementedSessionRecordingMetadataServiceServer
	authorizer authz.Authorizer
	backend    Backend
	logger     *slog.Logger
}

func NewService(config ServiceConfig) (*Service, error) {
	if config.Authorizer == nil {
		return nil, trace.BadParameter("authorizer is required")
	}
	if config.Backend == nil {
		return nil, trace.BadParameter("backend service is required")
	}
	if config.Logger == nil {
		config.Logger = slog.With(teleport.ComponentKey, "sessionrecordingmetadata.service")
	}
	return &Service{
		authorizer: config.Authorizer,
		backend:    config.Backend,
		logger:     config.Logger,
	}, nil
}

func (s *Service) CreateSessionRecordingMetadata(ctx context.Context, req *sessionrecordingmetadatapb.CreateSessionRecordingMetadataRequest) (*sessionrecordingmetadatapb.SessionRecordingMetadata, error) {
	auth, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.CheckAccessToKind(types.KindSessionRecordingMetadata, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}
	return s.backend.CreateSessionRecordingMetadata(ctx, req.GetSessionRecordingMetadata())
}

func (s *Service) UpdateSessionRecordingMetadata(ctx context.Context, req *sessionrecordingmetadatapb.UpdateSessionRecordingMetadataRequest) (*sessionrecordingmetadatapb.SessionRecordingMetadata, error) {
	auth, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.CheckAccessToKind(types.KindSessionRecordingMetadata, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}
	return s.backend.UpdateSessionRecordingMetadata(ctx, req.GetSessionRecordingMetadata())
}

func (s *Service) DeleteSessionRecordingMetadata(ctx context.Context, req *sessionrecordingmetadatapb.DeleteSessionRecordingMetadataRequest) (*emptypb.Empty, error) {
	auth, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.CheckAccessToKind(types.KindSessionRecordingMetadata, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}
	return nil, s.backend.DeleteSessionRecordingMetadata(ctx, req.GetSessionId())
}

func (s *Service) GetSessionRecordingMetadata(ctx context.Context, req *sessionrecordingmetadatapb.GetSessionRecordingMetadataRequest) (*sessionrecordingmetadatapb.SessionRecordingMetadata, error) {
	auth, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.CheckAccessToKind(types.KindSessionRecordingMetadata, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return s.backend.GetSessionRecordingMetadata(ctx, req.GetSessionId())
}

func (s *Service) ListSessionRecordingMetadata(ctx context.Context, req *sessionrecordingmetadatapb.ListSessionRecordingMetadataRequest) (*sessionrecordingmetadatapb.ListSessionRecordingMetadataResponse, error) {
	auth, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.CheckAccessToKind(types.KindSessionRecordingMetadata, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	metadata, nextToken, err := s.backend.ListSessionRecordingMetadata(ctx, int(req.GetPageSize()), req.PageToken, req.SessionIds, req.WithSummary)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &sessionrecordingmetadatapb.ListSessionRecordingMetadataResponse{
		SessionRecordingMetadata: metadata,
		NextPageToken:            nextToken,
	}, err
}
