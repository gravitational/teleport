// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package linuxdesktopv1

import (
	"context"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/emptypb"

	linuxdesktopv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/linuxdesktop/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
)

// ServiceConfig holds configuration options for the LinuxDesktop gRPC service.
type ServiceConfig struct {
	// Authorizer is the authorizer to use.
	Authorizer authz.Authorizer

	// Backend is the backend for storing LinuxDesktop.
	Backend services.LinuxDesktops

	// Reader is the cache for storing LinuxDesktop.
	Reader Reader

	// Emitter is the event emitter.
	Emitter events.Emitter
}

// CheckAndSetDefaults checks the ServiceConfig fields and returns an error if
// a required param is not provided.
// Authorizer, Cache and Backend are required params
func (s *ServiceConfig) CheckAndSetDefaults() error {
	switch {
	case s.Authorizer == nil:
		return trace.BadParameter("authorizer is required")
	case s.Backend == nil:
		return trace.BadParameter("backend is required")
	case s.Reader == nil:
		return trace.BadParameter("cache is required")
	case s.Emitter == nil:
		return trace.BadParameter("emitter is required")
	}
	return nil
}

type Reader interface {
	ListLinuxDesktops(ctx context.Context, pageSize int, nextToken string) ([]*linuxdesktopv1.LinuxDesktop, string, error)
	GetLinuxDesktop(ctx context.Context, name string) (*linuxdesktopv1.LinuxDesktop, error)
}

// Service implements the teleport.LinuxDesktop.v1.LinuxDesktopService RPC service.
type Service struct {
	linuxdesktopv1.UnimplementedLinuxDesktopServiceServer

	authorizer authz.Authorizer
	backend    services.LinuxDesktops
	reader     Reader
	emitter    events.Emitter
}

// NewService returns a new LinuxDesktop gRPC service.
func NewService(cfg ServiceConfig) (*Service, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Service{
		authorizer: cfg.Authorizer,
		backend:    cfg.Backend,
		reader:     cfg.Reader,
		emitter:    cfg.Emitter,
	}, nil
}

// CreateLinuxDesktop creates Linux desktop resource.
func (s *Service) CreateLinuxDesktop(ctx context.Context, req *linuxdesktopv1.CreateLinuxDesktopRequest) (rec *linuxdesktopv1.LinuxDesktop, err error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindLinuxDesktop, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	desktop := req.GetLinuxDesktop()
	if err := ValidateLinuxDesktop(desktop); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToResource153(desktop, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	rsp, err := s.backend.CreateLinuxDesktop(ctx, desktop)

	return rsp, trace.Wrap(err)
}

// ListLinuxDesktops returns a list of Linux desktops.
func (s *Service) ListLinuxDesktops(ctx context.Context, req *linuxdesktopv1.ListLinuxDesktopsRequest) (*linuxdesktopv1.ListLinuxDesktopsResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindLinuxDesktop, types.VerbRead, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	rsp, nextToken, err := s.reader.ListLinuxDesktops(ctx, int(req.PageSize), req.PageToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var allowed []*linuxdesktopv1.LinuxDesktop
	for _, desktop := range rsp {
		if err := authCtx.CheckAccessToResource153(desktop, types.VerbList); err != nil {
			continue
		}
		allowed = append(allowed, desktop)
	}

	return &linuxdesktopv1.ListLinuxDesktopsResponse{
		LinuxDesktops: allowed,
		NextPageToken: nextToken,
	}, nil
}

// GetLinuxDesktop returns Linux desktop resource.
func (s *Service) GetLinuxDesktop(ctx context.Context, req *linuxdesktopv1.GetLinuxDesktopRequest) (*linuxdesktopv1.LinuxDesktop, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindLinuxDesktop, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	rsp, err := s.reader.GetLinuxDesktop(ctx, req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToResource153(rsp, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	return rsp, nil
}

// UpdateLinuxDesktop updates Linux desktop resource.
func (s *Service) UpdateLinuxDesktop(ctx context.Context, req *linuxdesktopv1.UpdateLinuxDesktopRequest) (*linuxdesktopv1.LinuxDesktop, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindLinuxDesktop, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	desktop := req.GetLinuxDesktop()
	if err := ValidateLinuxDesktop(desktop); err != nil {
		return nil, trace.Wrap(err)
	}

	existing, err := s.reader.GetLinuxDesktop(ctx, desktop.GetMetadata().GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToResource153(existing, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToResource153(desktop, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	rsp, err := s.backend.UpdateLinuxDesktop(ctx, desktop)

	return rsp, trace.Wrap(err)
}

// UpsertLinuxDesktop upserts Linux desktop resource.
func (s *Service) UpsertLinuxDesktop(ctx context.Context, req *linuxdesktopv1.UpsertLinuxDesktopRequest) (*linuxdesktopv1.LinuxDesktop, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindLinuxDesktop, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	desktop := req.GetLinuxDesktop()
	if err := ValidateLinuxDesktop(desktop); err != nil {
		return nil, trace.Wrap(err)
	}

	existing, err := s.reader.GetLinuxDesktop(ctx, desktop.GetMetadata().GetName())
	if !trace.IsNotFound(err) {
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if err := authCtx.CheckAccessToResource153(existing, types.VerbUpdate); err != nil {
			return nil, trace.Wrap(err)
		}
		if err := authCtx.CheckAccessToResource153(desktop, types.VerbUpdate); err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		if err := authCtx.CheckAccessToResource153(desktop, types.VerbCreate); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	rsp, err := s.backend.UpsertLinuxDesktop(ctx, desktop)

	return rsp, trace.Wrap(err)
}

// DeleteLinuxDesktop deletes Linux desktop resource.
func (s *Service) DeleteLinuxDesktop(ctx context.Context, req *linuxdesktopv1.DeleteLinuxDesktopRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindLinuxDesktop, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	existing, err := s.reader.GetLinuxDesktop(ctx, req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToResource153(existing, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	if err = s.backend.DeleteLinuxDesktop(ctx, req.GetName()); err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}
