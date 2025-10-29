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

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/emptypb"

	delegationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/delegation/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
)

// ProfileServiceConfig contains the configuration of the ProfileService.
type ProfileServiceConfig struct {
	// Authorizer is used to authorize the user.
	Authorizer authz.Authorizer

	// Writer is used to create, update, and delete profile resources.
	Writer ProfileWriter

	// Reader is used to read and list profile resources.
	Reader ProfileReader

	// Logger to which errors and messages are written.
	Logger *slog.Logger
}

// ProfileService manages DelegationProfile resources.
type ProfileService struct {
	delegationv1.UnimplementedDelegationProfileServiceServer

	authorizer authz.Authorizer
	writer     ProfileWriter
	reader     ProfileReader
	logger     *slog.Logger
}

// ProfileWriter is the writable part of the services.DelegationProfiles
// interface.
type ProfileWriter interface {
	CreateDelegationProfile(ctx context.Context, delegationProfile *delegationv1.DelegationProfile) (*delegationv1.DelegationProfile, error)
	DeleteDelegationProfile(ctx context.Context, name string) error
	UpdateDelegationProfile(ctx context.Context, delegationProfile *delegationv1.DelegationProfile) (*delegationv1.DelegationProfile, error)
	UpsertDelegationProfile(ctx context.Context, delegationProfile *delegationv1.DelegationProfile) (*delegationv1.DelegationProfile, error)
}

// ProfileReader is the read-only part of the services.DelegationProfiles
// interface.
type ProfileReader interface {
	GetDelegationProfile(ctx context.Context, name string) (*delegationv1.DelegationProfile, error)
	ListDelegationProfiles(ctx context.Context, pageSize int, lastToken string) ([]*delegationv1.DelegationProfile, string, error)
}

// NewProfileService creates a ProfileService with the given configuration.
func NewProfileService(cfg ProfileServiceConfig) (*ProfileService, error) {
	if cfg.Authorizer == nil {
		return nil, trace.BadParameter("missing parameter Authorizer")
	}
	if cfg.Writer == nil {
		return nil, trace.BadParameter("missing parameter Writer")
	}
	if cfg.Reader == nil {
		return nil, trace.BadParameter("missing parameter Reader")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &ProfileService{
		authorizer: cfg.Authorizer,
		writer:     cfg.Writer,
		reader:     cfg.Reader,
		logger:     cfg.Logger,
	}, nil
}

// CreateDelegationProfile creates a delegation profile.
func (s *ProfileService) CreateDelegationProfile(ctx context.Context, req *delegationv1.CreateDelegationProfileRequest) (*delegationv1.DelegationProfile, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindDelegationProfile, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	prof, err := s.writer.CreateDelegationProfile(ctx, req.GetDelegationProfile())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return prof, nil
}

// GetDelegationProfile gets a delegation profile by name.
func (s *ProfileService) GetDelegationProfile(ctx context.Context, req *delegationv1.GetDelegationProfileRequest) (*delegationv1.DelegationProfile, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	adminAccessErr := authCtx.CheckAccessToKind(
		types.KindDelegationProfile,
		types.VerbRead,
	)

	prof, err := s.reader.GetDelegationProfile(ctx, req.GetName())
	switch {
	case trace.IsNotFound(err):
		// Only return NotFound if the user is allowed to access all profiles
		// to avoid leaking information.
		if adminAccessErr != nil {
			return nil, trace.Wrap(adminAccessErr)
		}
		return nil, trace.Wrap(err)
	case err != nil:
		return nil, trace.Wrap(err)
	}

	// Allow the user to read the profile if they have access to all profiles
	// via a resource rule (e.g. for administration).
	//
	// 	rules:
	// 	  - resources: [delegation_profile]
	//	    verbs: [read]
	//
	// Or if they have a label matcher allowing them to use the profile:
	//
	// 	allow:
	// 	  delegation_profile_labels:
	// 	    foo: bar
	labelAccessErr := authCtx.Checker.CheckAccess(
		types.Resource153ToResourceWithLabels(prof),
		services.AccessState{MFAVerified: true},
	)
	if adminAccessErr == nil || labelAccessErr == nil {
		return prof, nil
	}

	return nil, trace.NewAggregate(adminAccessErr, labelAccessErr)
}

// UpdateDelegationProfile updates an existing delegation profile. It will
// refuse to update a delegation profile if one does not already exist with
// the same name.
//
// ConditionalUpdate semantics are applied, e.g, the update will only succeed
// if the revision of the provided DelegationProfile matches the revision of
// the existing DelegationProfile.
func (s *ProfileService) UpdateDelegationProfile(ctx context.Context, req *delegationv1.UpdateDelegationProfileRequest) (*delegationv1.DelegationProfile, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindDelegationProfile, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	prof, err := s.writer.UpdateDelegationProfile(ctx, req.GetDelegationProfile())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return prof, nil
}

// UpsertDelegationProfile creates or updates a delegation profile.
//
// You should prefer to call CreateDelegationProfile or UpdateDelegationProfile instead.
func (s *ProfileService) UpsertDelegationProfile(ctx context.Context, req *delegationv1.UpsertDelegationProfileRequest) (*delegationv1.DelegationProfile, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindDelegationProfile, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	prof, err := s.writer.UpsertDelegationProfile(ctx, req.GetDelegationProfile())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return prof, nil
}

// DeleteDelegationProfile deletes a delegation profile by name.
func (s *ProfileService) DeleteDelegationProfile(ctx context.Context, req *delegationv1.DeleteDelegationProfileRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindDelegationProfile, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.writer.DeleteDelegationProfile(ctx, req.GetName()); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// ListDelegationProfiles returns a list of delegation profiles, pagination
// semantics are applied.
func (s *ProfileService) ListDelegationProfiles(ctx context.Context, req *delegationv1.ListDelegationProfilesRequest) (*delegationv1.ListDelegationProfilesResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindDelegationProfile, types.VerbRead, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	profiles, token, err := s.reader.ListDelegationProfiles(
		ctx,
		int(req.GetPageSize()),
		req.GetPageToken(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &delegationv1.ListDelegationProfilesResponse{
		DelegationProfiles: profiles,
		NextPageToken:      token,
	}, nil
}
