/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

package dynamicwindowsv1

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport"
	dynamicwindowspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/dynamicwindows/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
)

// maxDynamicWindowsDesktopsPerUpsert is the maximum number of desktops accepted
// in a single UpsertDynamicWindowsDesktops request. The caller will need to
// batch requests. The limit for a gRPC request is 4MB, so this allows for a
// record size of ~4KB per desktop, which should be more than enough.
const maxDynamicWindowsDesktopsPerUpsert = 1000

// Service implements the teleport.trust.v1.TrustService RPC service.
type Service struct {
	dynamicwindowspb.UnimplementedDynamicWindowsServiceServer

	authorizer authz.Authorizer
	backend    Backend
	cache      Cache
	logger     *slog.Logger
}

// ServiceConfig holds configuration options for Service
type ServiceConfig struct {
	// Authorizer is the authorizer service which checks access to resources.
	Authorizer authz.Authorizer
	// Backend will be used for writing the dynamic Windows desktop resources.
	Backend Backend
	// Cache will be used for reading and writing the dynamic Windows desktop resources.
	Cache Cache
	// Logger is the logger instance to use.
	Logger *slog.Logger
}

// Backend is the interface used for writing dynamic Windows desktops
type Backend interface {
	CreateDynamicWindowsDesktop(context.Context, types.DynamicWindowsDesktop) (types.DynamicWindowsDesktop, error)
	UpdateDynamicWindowsDesktop(context.Context, types.DynamicWindowsDesktop) (types.DynamicWindowsDesktop, error)
	UpsertDynamicWindowsDesktop(context.Context, types.DynamicWindowsDesktop) (types.DynamicWindowsDesktop, error)
	DeleteDynamicWindowsDesktop(ctx context.Context, name string) error
}

// Cache is the interface used for reading dynamic Windows desktops
type Cache interface {
	GetDynamicWindowsDesktop(ctx context.Context, name string) (types.DynamicWindowsDesktop, error)
	ListDynamicWindowsDesktops(ctx context.Context, pageSize int, pageToken string) ([]types.DynamicWindowsDesktop, string, error)
}

// NewService creates new dynamic Windows desktop service
func NewService(cfg ServiceConfig) (*Service, error) {
	switch {
	case cfg.Backend == nil:
		return nil, trace.BadParameter("backend service is required")
	case cfg.Cache == nil:
		return nil, trace.BadParameter("cache service is required")
	case cfg.Authorizer == nil:
		return nil, trace.BadParameter("authorizer is required")
	}

	if cfg.Logger == nil {
		cfg.Logger = slog.With(teleport.ComponentKey, "dynamicwindows")
	}

	return &Service{
		authorizer: cfg.Authorizer,
		backend:    cfg.Backend,
		cache:      cfg.Cache,
		logger:     cfg.Logger,
	}, nil
}

// GetDynamicWindowsDesktop returns registered dynamic Windows desktop by name.
func (s *Service) GetDynamicWindowsDesktop(ctx context.Context, request *dynamicwindowspb.GetDynamicWindowsDesktopRequest) (*types.DynamicWindowsDesktopV1, error) {
	auth, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.CheckAccessToKind(types.KindDynamicWindowsDesktop, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	if request.GetName() == "" {
		return nil, trace.BadParameter("dynamic windows desktop name is required")
	}

	d, err := s.cache.GetDynamicWindowsDesktop(ctx, request.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := checkAccess(auth, d); err != nil {
		return nil, trace.Wrap(err)
	}

	desktop, ok := d.(*types.DynamicWindowsDesktopV1)
	if !ok {
		return nil, trace.BadParameter("unexpected type %T", d)
	}

	return desktop, nil
}

// ListDynamicWindowsDesktops returns list of dynamic Windows desktops.
func (s *Service) ListDynamicWindowsDesktops(ctx context.Context, request *dynamicwindowspb.ListDynamicWindowsDesktopsRequest) (*dynamicwindowspb.ListDynamicWindowsDesktopsResponse, error) {
	auth, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.CheckAccessToKind(types.KindDynamicWindowsDesktop, types.VerbRead, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	desktops, next, err := s.cache.ListDynamicWindowsDesktops(ctx, int(request.GetPageSize()), request.GetPageToken())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response := dynamicwindowspb.ListDynamicWindowsDesktopsResponse_builder{
		NextPageToken: next,
	}.Build()
	for _, d := range desktops {
		if err := checkAccess(auth, d); err != nil {
			continue
		}
		desktop, ok := d.(*types.DynamicWindowsDesktopV1)
		if !ok {
			return nil, trace.BadParameter("unexpected type %T", d)
		}
		response.SetDesktops(append(response.GetDesktops(), desktop))
	}

	return response, nil
}

// CreateDynamicWindowsDesktop registers a new dynamic Windows desktop.
func (s *Service) CreateDynamicWindowsDesktop(ctx context.Context, req *dynamicwindowspb.CreateDynamicWindowsDesktopRequest) (*types.DynamicWindowsDesktopV1, error) {
	auth, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.CheckAccessToKind(types.KindDynamicWindowsDesktop, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := checkAccess(auth, req.GetDesktop()); err != nil {
		return nil, trace.Wrap(err)
	}
	d, err := s.backend.CreateDynamicWindowsDesktop(ctx, types.DynamicWindowsDesktop(req.GetDesktop()))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	createdDesktop, ok := d.(*types.DynamicWindowsDesktopV1)
	if !ok {
		return nil, trace.BadParameter("unexpected type %T", d)
	}

	return createdDesktop, nil
}

func checkAccess(auth *authz.Context, desktop types.DynamicWindowsDesktop) error {
	return auth.Checker.CheckAccess(desktop, services.AccessState{MFAVerified: true})
}

// UpdateDynamicWindowsDesktop updates an existing dynamic Windows desktop.
func (s *Service) UpdateDynamicWindowsDesktop(ctx context.Context, req *dynamicwindowspb.UpdateDynamicWindowsDesktopRequest) (*types.DynamicWindowsDesktopV1, error) {
	auth, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.CheckAccessToKind(types.KindDynamicWindowsDesktop, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	d, err := s.cache.GetDynamicWindowsDesktop(ctx, req.GetDesktop().GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := checkAccess(auth, d); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := checkAccess(auth, req.GetDesktop()); err != nil {
		return nil, trace.Wrap(err)
	}
	d, err = s.backend.UpdateDynamicWindowsDesktop(ctx, req.GetDesktop())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	updatedDesktop, ok := d.(*types.DynamicWindowsDesktopV1)
	if !ok {
		return nil, trace.BadParameter("unexpected type %T", d)
	}

	return updatedDesktop, nil
}

// UpsertDynamicWindowsDesktop updates an existing dynamic Windows desktop or creates one if it doesn't exist.
func (s *Service) UpsertDynamicWindowsDesktop(ctx context.Context, req *dynamicwindowspb.UpsertDynamicWindowsDesktopRequest) (*types.DynamicWindowsDesktopV1, error) {
	auth, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.CheckAccessToKind(types.KindDynamicWindowsDesktop, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	d, err := s.cache.GetDynamicWindowsDesktop(ctx, req.GetDesktop().GetName())
	if !trace.IsNotFound(err) {
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if err := checkAccess(auth, d); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if err := checkAccess(auth, req.GetDesktop()); err != nil {
		return nil, trace.Wrap(err)
	}
	d, err = s.backend.UpsertDynamicWindowsDesktop(ctx, req.GetDesktop())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	updatedDesktop, ok := d.(*types.DynamicWindowsDesktopV1)
	if !ok {
		return nil, trace.BadParameter("unexpected type %T", d)
	}

	return updatedDesktop, nil
}

// UpsertDynamicWindowsDesktops updates existing dynamic Windows desktops or
// creates new ones if they don't exist. The function will attempt to upsert
// each desktop in the request and return a list of results. An empty error
// means a desktop was upserted successfully. Requests must be batched to
// 1000 desktops or fewer at a time.
func (s *Service) UpsertDynamicWindowsDesktops(ctx context.Context, req *dynamicwindowspb.UpsertDynamicWindowsDesktopsRequest) (*dynamicwindowspb.UpsertDynamicWindowsDesktopsResponse, error) {
	auth, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.CheckAccessToKind(types.KindDynamicWindowsDesktop, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	desktops := req.GetDesktops()
	if len(desktops) > maxDynamicWindowsDesktopsPerUpsert {
		return nil, trace.BadParameter("too many desktops in request: got %d, at most %d are allowed", len(desktops), maxDynamicWindowsDesktopsPerUpsert)
	}

	results := make([]*dynamicwindowspb.UpsertDynamicWindowsDesktopResult, 0, len(desktops))
	fail := func(result *dynamicwindowspb.UpsertDynamicWindowsDesktopResult, err error) {
		result.Error = err.Error()
		results = append(results, result)
	}

	for _, desktop := range desktops {
		result := &dynamicwindowspb.UpsertDynamicWindowsDesktopResult{
			Name:  desktop.GetName(),
			Error: "",
		}

		// desktop is user supplied and could have a spoofed type, so make sure it
		// is set to `dynamic_windows_desktop` before calling checkAccess
		if err := desktop.CheckAndSetDefaults(); err != nil {
			fail(result, trace.Wrap(err))
			continue
		}

		d, err := s.cache.GetDynamicWindowsDesktop(ctx, desktop.GetName())
		if !trace.IsNotFound(err) {
			if err != nil {
				fail(result, err)
				continue
			}
			if err := checkAccess(auth, d); err != nil {
				fail(result, err)
				continue
			}
		}
		if err := checkAccess(auth, desktop); err != nil {
			fail(result, err)
			continue
		}
		d, err = s.backend.UpsertDynamicWindowsDesktop(ctx, desktop)
		if err != nil {
			fail(result, err)
			continue
		}

		results = append(results, result)
	}

	response := &dynamicwindowspb.UpsertDynamicWindowsDesktopsResponse{
		Results: results,
	}
	return response, nil
}

// DeleteDynamicWindowsDesktop removes the specified dynamic Windows desktop.
func (s *Service) DeleteDynamicWindowsDesktop(ctx context.Context, req *dynamicwindowspb.DeleteDynamicWindowsDesktopRequest) (*emptypb.Empty, error) {
	auth, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.CheckAccessToKind(types.KindDynamicWindowsDesktop, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}
	d, err := s.cache.GetDynamicWindowsDesktop(ctx, req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := checkAccess(auth, d); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := s.backend.DeleteDynamicWindowsDesktop(ctx, req.GetName()); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}
