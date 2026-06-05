package beamsv1

import (
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

	var (
		results   []*beamsv1.Beam
		nextToken string
	)
	for beam, err := range s.beamReader.IterateBeamsV2(ctx, req.GetPageToken(), &services.ListBeamsRequestOptions{
		SortField:   req.GetSortField(),
		SortOrder:   req.GetSortOrder(),
		FilterUsers: usersFilter,
	}) {
		if err != nil {
			s.logger.ErrorContext(ctx, "Failed to iterate beams", "error", err)
			return nil, trace.Wrap(err)
		}

		err := s.checkAccessToBeamWithAuthPreference(ctx, authCtx, beam, authPref)
		switch {
		case trace.IsAccessDenied(err):
			continue
		case err != nil:
			s.logger.ErrorContext(ctx, "Failed to check access to beam", "error", err)
			return nil, trace.Wrap(err)
		}

		// Read one more than pageSize results, so we can point nextToken at the
		// next result in the set.
		if len(results) == pageSize {
			nextToken = beam.GetMetadata().GetName()
			break
		}

		results = append(results, beam)
	}

	return beamsv1.ListBeamsResponse_builder{
		Beams:         results,
		NextPageToken: nextToken,
	}.Build(), nil
}
