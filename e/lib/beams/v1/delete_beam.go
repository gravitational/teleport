package beamsv1

import (
	"context"

	"github.com/gravitational/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	"github.com/gravitational/teleport/api/types"
	compute "github.com/gravitational/teleport/e/api/beamservice/v1"
	prehogv1a "github.com/gravitational/teleport/gen/proto/go/prehog/v1alpha"
	"github.com/gravitational/teleport/lib/auth/machineid/machineidv1"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/scopes"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
)

func (s *BeamsService) DeleteBeam(ctx context.Context, req *beamsv1.DeleteBeamRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindBeam, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}
	if req.GetName() == "" {
		return nil, trace.BadParameter("name is required")
	}

	logger := s.logger.With("beam_id", req.GetName())

	beam, err := s.beamReader.GetBeam(ctx, req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := s.checkAccessToBeam(ctx, authCtx, beam); err != nil {
		return nil, trace.Wrap(err)
	}

	// Destroy the compute before the beam record, so that it can be retried if
	// something goes wrong.
	if err := s.destroyBeamCompute(ctx, beam); err != nil {
		logger.ErrorContext(ctx, "Failed to deprovision beam compute", "error", err)
		return nil, trace.Wrap(err)
	}

	if err := s.deleteBeam(ctx, beam); err != nil {
		logger.ErrorContext(ctx, "Failed to delete beam record", "error", err)
		return nil, trace.Wrap(err)
	}

	s.usageReporter.AnonymizeAndSubmit(&usagereporter.BeamsDestroyedEvent{
		BeamId: req.GetName(),
		Reason: prehogv1a.BeamDestroyReason_BEAM_DESTROY_REASON_USER_DELETED,
		Region: beam.GetStatus().GetRegion(),
	})

	return &emptypb.Empty{}, nil
}

func (s *BeamsService) destroyBeamCompute(ctx context.Context, beam *beamsv1.Beam) error {
	region := beam.GetStatus().GetRegion()
	client, err := s.clientForRegion(region)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = client.DestroyBeam(ctx, &compute.DestroyBeamRequest{
		BeamId: beam.GetMetadata().GetName(),
		Region: region,
	})
	switch status.Code(err) {
	case codes.OK, codes.NotFound:
		return nil
	default:
		return trace.Wrap(err, "failed to deprovision beam compute")
	}
}

func (s *BeamsService) deleteBeam(ctx context.Context, beam *beamsv1.Beam) error {
	// Delete the beam itself.
	actions, err := s.beamWriter.AppendDeleteBeamActions(
		nil, /* actions */
		beam,
		backend.Revision(beam.GetMetadata().GetRevision()),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	// Delete the bot user and role.
	actions, err = s.userWriter.AppendDeleteUserParamsActions(
		actions,
		machineidv1.BotResourceName(beam.GetStatus().GetBotName()),
		backend.Whatever(),
	)
	if err != nil {
		return trace.Wrap(err)
	}
	actions, err = s.roleWriter.AppendDeleteRoleActions(
		actions,
		machineidv1.BotResourceName(beam.GetStatus().GetBotName()),
		backend.Whatever(),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	// Delete the provision token.
	actions, err = s.provisionTokenWriter.AppendDeleteProvisionTokenActions(
		actions,
		beam.GetStatus().GetJoinTokenName(),
		backend.Whatever(),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	// Delete the workload identity.
	actions, err = s.workloadIdentityWriter.AppendDeleteWorkloadIdentityActions(
		actions,
		scopes.QualifiedName{Name: beam.GetStatus().GetWorkloadIdentityName()},
		backend.Whatever(),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	// Delete the delegation session.
	actions, err = s.delegationSessionWriter.AppendDeleteDelegationSessionActions(
		actions,
		beam.GetStatus().GetDelegationSessionId(),
		backend.Whatever(),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	// Delete the node if there is one.
	if nodeID := beam.GetStatus().GetNodeId(); nodeID != "" {
		actions, err = s.nodeWriter.AppendDeleteNodeActions(
			actions,
			apidefaults.Namespace,
			nodeID,
			backend.Whatever(),
		)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	// Delete the app if there is one.
	if appName := beam.GetStatus().GetAppName(); appName != "" {
		actions, err = s.appWriter.AppendDeleteAppActions(
			actions,
			appName,
			backend.Whatever(),
		)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	_, err = s.storageBackend.AtomicWrite(ctx, actions)
	return trace.Wrap(err)
}
