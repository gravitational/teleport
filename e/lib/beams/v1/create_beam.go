package beamsv1

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	delegationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/delegation/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	workloadidentityv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	compute "github.com/gravitational/teleport/e/api/beamservice/v1"
	"github.com/gravitational/teleport/lib/auth/machineid/machineidv1"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// beamTTL is the maximum amount of time a beam may run for before we shut
	// it down. Having this limit avoids the need to manage upgrades, etc.
	beamTTL = 24 * time.Hour

	// maxCreateRetries is the maximum number of times we'll try to generate a
	// unique beam alias before giving up and returning an error.
	maxCreateRetries = 5

	// beamBotRoleName is the name of the role all beam bots are assigned, so
	// they can generate SSH host certificates, etc.
	beamBotRoleName = "beam"
)

func (s *BeamsService) CreateBeam(ctx context.Context, req *beamsv1.CreateBeamRequest) (*beamsv1.CreateBeamResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindBeam, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	// Create the initial beam record before calling the compute service, so
	// that if a subsequent step fails, we've got a record of the beam and the
	// garbage collector can clean it up later.
	var (
		beam      *beamsv1.Beam
		regSecret string
	)
retry:
	for range maxCreateRetries {
		beam, regSecret, err = s.createBeam(ctx, authCtx.User.GetName(), req)
		switch {
		case errors.Is(err, backend.ErrConditionFailed):
			// Beam alias is already in-use, generate another and try again.
		case err != nil:
			return nil, trace.Wrap(err)
		default:
			break retry
		}
	}
	if beam == nil {
		s.logger.ErrorContext(ctx, "Failed to find a unique beam alias", "attempt_count", maxCreateRetries)
		return nil, trace.Errorf("failed to create beam after %d attempts, please try again later", maxCreateRetries)
	}

	logger := s.logger.With("beam_id", beam.GetMetadata().GetName())

	// Call the compute service to provision the beam microVM.
	provisionRsp, err := s.computeService.ProvisionBeam(ctx, &compute.ProvisionBeamRequest{
		BeamId:    beam.GetMetadata().GetName(),
		BeamAlias: beam.GetStatus().GetAlias(),
		Tbot: &compute.TbotConfig{
			JoinToken:            beam.GetStatus().GetJoinTokenName(),
			RegistrationSecret:   regSecret,
			DelegationSessionId:  beam.GetStatus().GetDelegationSessionId(),
			WorkloadIdentityName: beam.GetStatus().GetWorkloadIdentityName(),
			NodeId:               beam.GetMetadata().GetName(),
		},
	})
	if err != nil {
		logger.ErrorContext(ctx, "Failed to provision beam compute", "error", err)

		// Create a detached context for the cleanup in case the call failed
		// because the outer request was canceled.
		cleanupCtx, cancel := context.WithTimeout(
			context.WithoutCancel(ctx),
			5*time.Second,
		)
		defer cancel()

		limitExceeded := status.Code(err) == codes.ResourceExhausted

		// Call DestroyBeam to clean up any resources left over if ProvisionBeam
		// partially failed (e.g. the service died part-way through serving our
		// request).
		//
		// Do not call DestroyBeam if we explicitly got a ResourceExhausted error,
		// as this means no resources were created, and we should not put undue
		// load on the compute service.
		//
		// It doesn't matter if this cleanup fails because the garbage collector
		// will eventually pick it back up.
		var destroyErr error
		if !limitExceeded {
			destroyErr = s.destroyBeamCompute(cleanupCtx, beam.GetMetadata().GetName())
		}
		if destroyErr == nil {
			if deleteErr := s.deleteBeam(cleanupCtx, beam); deleteErr != nil {
				logger.ErrorContext(ctx, "Failed to delete local beam record", "error", deleteErr)
			}
		} else {
			logger.ErrorContext(cleanupCtx, "Failed to destroy failed beam compute", "error", destroyErr)
		}

		if limitExceeded {
			return nil, trace.LimitExceeded("limit exceeded; please try again later")
		}
		return nil, trace.Errorf("failed to provision beam compute")
	}

	// Create a node so we can SSH into the beam, and update the beam's status.
	node, err := types.NewNode(
		beam.GetMetadata().GetName(),
		types.SubKindOpenSSHNode,
		types.ServerSpecV2{
			Addr:     provisionRsp.GetSshAddr(),
			Hostname: beamResourceName(beam),
		},
		beam.GetMetadata().GetLabels(),
	)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to create node for beam", "error", err)
		return nil, trace.Errorf("failed to create node for beam")
	}

	beam.Status.ComputeStatus = beamsv1.ComputeStatus_COMPUTE_STATUS_PROVISION_COMPLETE
	beam.Status.SshAddr = provisionRsp.GetSshAddr()
	beam.Status.AppAddrHttp = provisionRsp.GetAppAddrHttp()
	beam.Status.AppAddrTcp = provisionRsp.GetAppAddrTcp()
	beam.Status.NodeId = node.GetName()

	actions, err := s.nodeWriter.AppendPutNodeActions(
		nil, /* actions */
		node,
		backend.Whatever(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	actions, err = s.beamWriter.AppendPutBeamActions(
		actions,
		beam,
		backend.Revision(beam.GetMetadata().GetRevision()),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	revision, err := s.storageBackend.AtomicWrite(ctx, actions)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to create node and update beam", "error", err)
		return nil, trace.Wrap(err)
	}
	beam.Metadata.Revision = revision

	return &beamsv1.CreateBeamResponse{
		Beam: beam,
	}, nil
}

func (s *BeamsService) createBeam(ctx context.Context, user string, req *beamsv1.CreateBeamRequest) (*beamsv1.Beam, string, error) {
	id := uuid.NewString()
	expires := time.Now().Add(beamTTL)

	alias, err := s.generateAlias()
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	beam := &beamsv1.Beam{
		Kind:    types.KindBeam,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: id,
		},
		Spec: &beamsv1.BeamSpec{
			Egress:         req.GetEgress(),
			AllowedDomains: req.GetAllowedDomains(),
			Expires:        timestamppb.New(expires),
		},
		Status: &beamsv1.BeamStatus{
			User:          user,
			Alias:         alias,
			ComputeStatus: beamsv1.ComputeStatus_COMPUTE_STATUS_PROVISION_PENDING,
		},
	}
	beam.Metadata.Labels = beamResourceLabels(beam)

	// Create bot user and role.
	bot, botUser, botRole, err := beamBotUserAndRole(beam)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	beam.Status.BotName = bot.GetMetadata().GetName()

	actions, err := s.userWriter.AppendPutUserParamsActions(
		nil, /* actions */
		botUser,
		backend.NotExists(),
	)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	actions, err = s.roleWriter.AppendPutRoleActions(
		actions,
		botRole,
		backend.NotExists(),
	)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	// Create provision token.
	token, secret, err := beamJoinTokenAndSecret(beam)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	beam.Status.JoinTokenName = token.GetName()

	actions, err = s.provisionTokenWriter.AppendPutProvisionTokenActions(
		actions,
		token,
		backend.NotExists(),
	)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	// Create workload identity.
	workloadIdentity, err := beamWorkloadIdentity(beam)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	beam.Status.WorkloadIdentityName = workloadIdentity.GetMetadata().GetName()

	actions, err = s.workloadIdentityWriter.AppendPutWorkloadIdentityActions(
		actions,
		workloadIdentity,
		backend.NotExists(),
	)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	// Create delegation session.
	delegationSession, err := beamDelegationSession(beam)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	beam.Status.DelegationSessionId = delegationSession.GetMetadata().GetName()

	actions, err = s.delegationSessionWriter.AppendPutDelegationSessionActions(
		actions,
		delegationSession,
		backend.NotExists(),
	)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	// Create the beam itself.
	//
	// Note: this includes a "Put if NotExists" action on the beam alias to
	// ensure its uniqueness.
	actions, err = s.beamWriter.AppendPutBeamActions(
		actions,
		beam,
		backend.NotExists(),
	)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	revision, err := s.storageBackend.AtomicWrite(ctx, actions)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	beam.Metadata.Revision = revision

	return beam, secret, nil
}

func beamBotUserAndRole(beam *beamsv1.Beam) (*machineidv1pb.Bot, types.User, types.Role, error) {
	bot := &machineidv1pb.Bot{
		Kind:    types.KindBot,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name:    beamResourceName(beam),
			Labels:  beamResourceLabels(beam),
			Expires: proto.CloneOf(beam.GetSpec().GetExpires()),
		},
		Spec: &machineidv1pb.BotSpec{
			Roles: []string{beamBotRoleName},
			Traits: []*machineidv1pb.Trait{
				// Add the beam's ID as a trait so we can use it in bot role template.
				{
					Name:   types.BeamIDLabel,
					Values: []string{beam.GetMetadata().GetName()},
				},
			},
		},
	}
	user, role, err := machineidv1.BotToUserAndRole(
		bot,
		beam.GetStatus().GetUser(),
	)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}
	return bot, user, role, nil
}

func beamJoinTokenAndSecret(beam *beamsv1.Beam) (types.ProvisionToken, string, error) {
	secret, err := utils.CryptoRandomHex(defaults.TokenLenBytes)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	expires := beam.GetSpec().GetExpires().AsTime()
	return &types.ProvisionTokenV2{
		Metadata: types.Metadata{
			Name:    beamResourceName(beam),
			Expires: &expires,
			Labels:  beamResourceLabels(beam),
		},
		Spec: types.ProvisionTokenSpecV2{
			Roles:      []types.SystemRole{types.RoleBot},
			JoinMethod: types.JoinMethodBoundKeypair,
			BotName:    beam.GetStatus().GetBotName(),
			BoundKeypair: &types.ProvisionTokenSpecV2BoundKeypair{
				Onboarding: &types.ProvisionTokenSpecV2BoundKeypair_OnboardingSpec{
					RegistrationSecret: secret,
				},
				Recovery: &types.ProvisionTokenSpecV2BoundKeypair_RecoverySpec{
					Limit: 1,
				},
			},
		},
	}, secret, nil
}

func beamWorkloadIdentity(beam *beamsv1.Beam) (*workloadidentityv1.WorkloadIdentity, error) {
	return &workloadidentityv1.WorkloadIdentity{
		Kind:    types.KindWorkloadIdentity,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name:    beamResourceName(beam),
			Labels:  beamResourceLabels(beam),
			Expires: proto.CloneOf(beam.GetSpec().GetExpires()),
		},
		Spec: &workloadidentityv1.WorkloadIdentitySpec{
			Rules: &workloadidentityv1.WorkloadIdentityRules{
				Allow: []*workloadidentityv1.WorkloadIdentityRule{
					{Expression: fmt.Sprintf("user.bot_name == %q", beam.GetStatus().GetBotName())},
				},
			},
			Spiffe: &workloadidentityv1.WorkloadIdentitySPIFFE{
				Id: beamSPIFFEPath(beam),
			},
		},
	}, nil
}

func beamDelegationSession(beam *beamsv1.Beam) (*delegationv1.DelegationSession, error) {
	return &delegationv1.DelegationSession{
		Kind:    types.KindDelegationSession,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name:    uuid.NewString(),
			Labels:  beamResourceLabels(beam),
			Expires: proto.CloneOf(beam.GetSpec().GetExpires()),
		},
		Spec: &delegationv1.DelegationSessionSpec{
			User: beam.GetStatus().GetUser(),
			Resources: []*delegationv1.DelegationResourceSpec{
				{Kind: types.Wildcard, Name: types.Wildcard},
			},
			AuthorizedUsers: []*delegationv1.DelegationUserSpec{
				{
					Kind: types.KindBot,
					Matcher: &delegationv1.DelegationUserSpec_BotName{
						BotName: beam.GetStatus().GetBotName(),
					},
				},
			},
		},
	}, nil
}

func beamResourceLabels(beam *beamsv1.Beam) map[string]string {
	return map[string]string{
		types.BeamIDLabel:    beam.GetMetadata().GetName(),
		types.BeamOwnerLabel: beam.GetStatus().GetUser(),
		types.BeamAliasLabel: beam.GetStatus().GetAlias(),
	}
}

func beamResourceName(beam *beamsv1.Beam) string {
	return fmt.Sprintf("beam-%s", beam.GetMetadata().GetName())
}

func beamSPIFFEPath(beam *beamsv1.Beam) string {
	return fmt.Sprintf("/_teleport-cloud/beams/%s", beam.GetMetadata().GetName())
}
