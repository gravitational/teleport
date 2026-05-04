package beamsv1

import (
	"context"

	"github.com/gravitational/trace"

	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	"github.com/gravitational/teleport/api/types"
)

func (s *BeamsService) GetBeam(ctx context.Context, req *beamsv1.GetBeamRequest) (*beamsv1.GetBeamResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindBeam, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	var beam *beamsv1.Beam
	switch v := req.GetId().(type) {
	case *beamsv1.GetBeamRequest_Name:
		if v.Name == "" {
			return nil, trace.BadParameter("name is required")
		}
		if beam, err = s.beamReader.GetBeam(ctx, v.Name); err != nil {
			return nil, trace.Wrap(err)
		}
	case *beamsv1.GetBeamRequest_Alias:
		if v.Alias == "" {
			return nil, trace.BadParameter("alias is required")
		}
		if beam, err = s.beamReader.GetBeamByAlias(ctx, v.Alias); err != nil {
			return nil, trace.Wrap(err)
		}
	default:
		return nil, trace.BadParameter("id is required")
	}

	if err := s.checkAccessToBeam(ctx, authCtx, beam); err != nil {
		return nil, trace.Wrap(err)
	}

	return &beamsv1.GetBeamResponse{
		Beam: beam,
	}, nil
}
