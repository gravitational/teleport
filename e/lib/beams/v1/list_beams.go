package beamsv1

import (
	"cmp"
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/defaults"
	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/set"
)

func (s *BeamsService) ListBeams(ctx context.Context, req *beamsv1.ListBeamsRequest) (*beamsv1.ListBeamsResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindBeam, types.VerbList, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	authPref, err := s.authPreferenceGetter.GetReadOnlyAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pageSize := int(req.GetPageSize())
	if pageSize <= 0 || pageSize > defaults.DefaultChunkSize {
		pageSize = defaults.DefaultChunkSize
	}
	usersFilter := set.New(req.GetFilters().GetUsers()...)

	var authErr error
	results, nextToken, err := s.beamReader.ListBeamsV2(ctx, pageSize, req.GetPageToken(), &services.ListBeamsRequestOptions{
		SortField:   req.GetSortField(),
		SortOrder:   req.GetSortOrder(),
		FilterUsers: usersFilter,
		FilterFn: func(beam *beamsv1.Beam) bool {
			err := s.checkAccessToBeamWithAuthPreference(ctx, authCtx, beam, authPref)
			switch {
			case trace.IsAccessDenied(err):
				return false
			case err != nil:
				authErr = cmp.Or(authErr, err)
				s.logger.ErrorContext(ctx, "Failed to check access to beam", "error", err)
				return false
			}
			return true
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if authErr != nil {
		return nil, trace.Wrap(authErr)
	}

	return beamsv1.ListBeamsResponse_builder{
		Beams:         results,
		NextPageToken: nextToken,
	}.Build(), nil
}
