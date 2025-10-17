package delegationv1

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/emptypb"

	delegationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/delegation/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
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
	if err := authCtx.CheckAccessToKind(types.KindDelegationProfile, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	prof, err := s.reader.GetDelegationProfile(ctx, req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return prof, nil
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
