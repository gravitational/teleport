package beamsv1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	beamsv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	delegationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/delegation/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	compute "github.com/gravitational/teleport/e/api/beamservice/v1"
	"github.com/gravitational/teleport/lib/services"
)

func TestCreateBeam(t *testing.T) {
	t.Parallel()

	computeClient := &fakeComputeService{
		provisionResponse: &compute.ProvisionBeamResponse{
			SshAddr:     "127.0.0.1:3022",
			AppAddrHttp: "127.0.0.1:8080",
			AppAddrTcp:  "127.0.0.1:10080",
		},
	}

	pack := newBeamServiceTestPack(t, beamServiceTestPackConfig{
		aliasGenerator: sequenceAliasGenerator("steady-river"),
		computeClient:  computeClient,
	})

	service := pack.service(t, pack.user(t, "alice"))
	resp, err := service.CreateBeam(t.Context(), &beamsv1pb.CreateBeamRequest{
		Egress:         beamsv1pb.EgressMode_EGRESS_MODE_RESTRICTED,
		AllowedDomains: []string{"example.com."},
	})
	require.NoError(t, err)

	beam := resp.GetBeam()
	require.NotNil(t, beam)

	expectedLabels := map[string]string{
		types.BeamIDLabel:    beam.GetMetadata().GetName(),
		types.BeamOwnerLabel: "alice",
		types.BeamAliasLabel: "steady-river",
	}

	expectedBeam := &beamsv1pb.Beam{
		Kind:    types.KindBeam,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name:   beam.GetMetadata().GetName(),
			Labels: expectedLabels,
		},
		Spec: &beamsv1pb.BeamSpec{
			Egress:         beamsv1pb.EgressMode_EGRESS_MODE_RESTRICTED,
			AllowedDomains: []string{"example.com."},
			Expires:        beam.GetSpec().GetExpires(),
		},
		Status: &beamsv1pb.BeamStatus{
			User:                 "alice",
			Alias:                "steady-river",
			BotName:              beamResourceName(beam),
			JoinTokenName:        beamResourceName(beam),
			DelegationSessionId:  beam.GetStatus().GetDelegationSessionId(),
			WorkloadIdentityName: beam.GetStatus().GetWorkloadIdentityName(),
			ComputeStatus:        beamsv1pb.ComputeStatus_COMPUTE_STATUS_PROVISION_COMPLETE,
			SshAddr:              "127.0.0.1:3022",
			AppAddrHttp:          "127.0.0.1:8080",
			AppAddrTcp:           "127.0.0.1:10080",
			NodeId:               beam.GetStatus().GetNodeId(),
		},
	}
	require.Empty(t, cmp.Diff(
		expectedBeam,
		beam,
		protocmp.Transform(),
		protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
	))

	require.Len(t, computeClient.getProvisionRequests(), 1)
	computeReq := computeClient.getProvisionRequests()[0]
	require.Empty(t, cmp.Diff(
		&compute.ProvisionBeamRequest{
			BeamId:    beam.GetMetadata().GetName(),
			BeamAlias: beam.GetStatus().GetAlias(),
			Tbot: &compute.TbotConfig{
				JoinToken:            beam.GetStatus().GetJoinTokenName(),
				DelegationSessionId:  beam.GetStatus().GetDelegationSessionId(),
				WorkloadIdentityName: beam.GetStatus().GetWorkloadIdentityName(),
				NodeId:               beam.GetStatus().GetNodeId(),
				RegistrationSecret:   computeReq.GetTbot().GetRegistrationSecret(),
			},
		},
		computeReq,
		protocmp.Transform(),
	))
	require.NotEmpty(t, computeReq.GetTbot().GetRegistrationSecret())

	storedBeam, err := pack.beam.GetBeam(t.Context(), beam.GetMetadata().GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(
		expectedBeam,
		storedBeam,
		protocmp.Transform(),
		protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
	))

	storedByAlias, err := pack.beam.GetBeamByAlias(t.Context(), beam.GetStatus().GetAlias())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(
		expectedBeam,
		storedByAlias,
		protocmp.Transform(),
		protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
	))

	token, err := pack.token.GetToken(t.Context(), beam.GetStatus().GetJoinTokenName())
	require.NoError(t, err)
	require.Equal(t, beam.GetStatus().GetJoinTokenName(), token.GetName())
	require.Equal(t, expectedLabels, token.GetMetadata().Labels)
	require.Equal(t, computeReq.GetTbot().GetRegistrationSecret(), token.GetBoundKeypairStatus().RegistrationSecret)

	botResourceName := services.BotResourceName(beam.GetStatus().GetBotName())
	user, err := pack.identity.GetUser(t.Context(), botResourceName, false)
	require.NoError(t, err)
	require.Equal(t, botResourceName, user.GetName())
	require.Equal(t, []string{
		beam.GetMetadata().GetName(),
	}, user.GetTraits()["teleport.internal/beams/id"])

	role, err := pack.role.GetRole(t.Context(), botResourceName)
	require.NoError(t, err)
	require.Equal(t, botResourceName, role.GetName())

	workloadIdentity, err := pack.workloadIdentity.GetWorkloadIdentity(t.Context(), beam.GetStatus().GetWorkloadIdentityName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(
		&workloadidentityv1pb.WorkloadIdentity{
			Kind:    types.KindWorkloadIdentity,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name:    beam.GetStatus().GetWorkloadIdentityName(),
				Labels:  expectedLabels,
				Expires: beam.GetSpec().GetExpires(),
			},
			Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
				Rules: &workloadidentityv1pb.WorkloadIdentityRules{
					Allow: []*workloadidentityv1pb.WorkloadIdentityRule{
						{Expression: `user.bot_name == "` + beam.GetStatus().GetBotName() + `"`},
					},
				},
				Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
					Id: "/_teleport-cloud/beams/" + beam.GetMetadata().GetName(),
				},
			},
		},
		workloadIdentity,
		protocmp.Transform(),
		protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
	))

	delegationSession, err := pack.delegationSession.GetDelegationSession(t.Context(), beam.GetStatus().GetDelegationSessionId())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(
		&delegationv1.DelegationSession{
			Kind:    types.KindDelegationSession,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name:    beam.GetStatus().GetDelegationSessionId(),
				Labels:  expectedLabels,
				Expires: beam.GetSpec().GetExpires(),
			},
			Spec: &delegationv1.DelegationSessionSpec{
				User: "alice",
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
		},
		delegationSession,
		protocmp.Transform(),
		protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
	))

	node, err := pack.presence.GetNode(t.Context(), apidefaults.Namespace, beam.GetStatus().GetNodeId())
	require.NoError(t, err)
	require.Equal(t, beam.GetStatus().GetNodeId(), node.GetName())
	require.Equal(t, beam.GetStatus().GetSshAddr(), node.GetAddr())
	require.Equal(t, expectedLabels, node.GetStaticLabels())
	require.Equal(t, storedBeam.GetMetadata().GetName(), node.GetMetadata().Name)
}

func TestCreateBeamRetriesAliasCollision(t *testing.T) {
	t.Parallel()

	pack := newBeamServiceTestPack(t, beamServiceTestPackConfig{
		aliasGenerator: sequenceAliasGenerator("steady-river", "steady-river", "brisk-otter"),
	})

	service := pack.service(t, pack.user(t, "alice"))
	firstResp, err := service.CreateBeam(t.Context(), &beamsv1pb.CreateBeamRequest{
		Egress:         beamsv1pb.EgressMode_EGRESS_MODE_RESTRICTED,
		AllowedDomains: []string{"example.com."},
	})
	require.NoError(t, err)
	require.Equal(t, "steady-river", firstResp.GetBeam().GetStatus().GetAlias())

	resp, err := service.CreateBeam(t.Context(), &beamsv1pb.CreateBeamRequest{
		Egress:         beamsv1pb.EgressMode_EGRESS_MODE_RESTRICTED,
		AllowedDomains: []string{"example.com."},
	})
	require.NoError(t, err)
	require.Equal(t, "brisk-otter", resp.GetBeam().GetStatus().GetAlias())

	storedOriginal, err := pack.beam.GetBeamByAlias(t.Context(), "steady-river")
	require.NoError(t, err)
	require.Equal(t, firstResp.GetBeam().GetMetadata().GetName(), storedOriginal.GetMetadata().GetName())

	storedNew, err := pack.beam.GetBeamByAlias(t.Context(), "brisk-otter")
	require.NoError(t, err)
	require.Equal(t, resp.GetBeam().GetMetadata().GetName(), storedNew.GetMetadata().GetName())

	require.Len(t, pack.compute.getProvisionRequests(), 2)
	require.Equal(t, "brisk-otter", resp.GetBeam().GetStatus().GetAlias())
}

func TestCreateBeamProvisionFailureCleansUp(t *testing.T) {
	t.Parallel()

	computeClient := &fakeComputeService{
		provisionError: trace.ConnectionProblem(nil, "provision failed"),
	}

	pack := newBeamServiceTestPack(t, beamServiceTestPackConfig{
		aliasGenerator: sequenceAliasGenerator("failing-beam"),
		computeClient:  computeClient,
	})

	service := pack.service(t, pack.user(t, "alice"))
	_, err := service.CreateBeam(t.Context(), &beamsv1pb.CreateBeamRequest{
		Egress:         beamsv1pb.EgressMode_EGRESS_MODE_RESTRICTED,
		AllowedDomains: []string{"example.com."},
	})
	require.ErrorContains(t, err, "failed to provision beam compute")
	require.Len(t, computeClient.getProvisionRequests(), 1)
	require.Len(t, computeClient.getDestroyRequests(), 1)
	require.Equal(t,
		computeClient.getProvisionRequests()[0].GetBeamId(),
		computeClient.getDestroyRequests()[0].GetBeamId(),
	)

	_, err = pack.beam.GetBeamByAlias(t.Context(), "failing-beam")
	require.True(t, trace.IsNotFound(err))

	beams, nextKey, err := pack.beam.ListBeams(t.Context(), 10, "")
	require.NoError(t, err)
	require.Empty(t, nextKey)
	require.Empty(t, beams)
}

func TestCreateBeamProvisionResourceExhaustedReturnsLimitExceeded(t *testing.T) {
	t.Parallel()

	computeClient := &fakeComputeService{
		provisionError: status.Error(codes.ResourceExhausted, "capacity exhausted"),
	}

	pack := newBeamServiceTestPack(t, beamServiceTestPackConfig{
		aliasGenerator: sequenceAliasGenerator("capacity-limited"),
		computeClient:  computeClient,
	})

	service := pack.service(t, pack.user(t, "alice"))
	_, err := service.CreateBeam(t.Context(), &beamsv1pb.CreateBeamRequest{
		Egress: beamsv1pb.EgressMode_EGRESS_MODE_UNRESTRICTED,
	})
	require.True(t, trace.IsLimitExceeded(err), "expected limit exceeded error, got %v", err)
	require.ErrorContains(t, err, "limit exceeded; please try again later")
	require.Empty(t, computeClient.getDestroyRequests())

	_, err = pack.beam.GetBeamByAlias(t.Context(), "capacity-limited")
	require.True(t, trace.IsNotFound(err))
}

func TestCreateBeamProvisionFailureDestroyNotFoundCleansUp(t *testing.T) {
	t.Parallel()

	computeClient := &fakeComputeService{
		provisionError: trace.ConnectionProblem(nil, "provision failed"),
		destroyError:   status.Error(codes.NotFound, "beam missing"),
	}

	pack := newBeamServiceTestPack(t, beamServiceTestPackConfig{
		aliasGenerator: sequenceAliasGenerator("missing-compute"),
		computeClient:  computeClient,
	})

	service := pack.service(t, pack.user(t, "alice"))
	_, err := service.CreateBeam(t.Context(), &beamsv1pb.CreateBeamRequest{
		Egress: beamsv1pb.EgressMode_EGRESS_MODE_UNRESTRICTED,
	})
	require.ErrorContains(t, err, "failed to provision beam compute")
	require.Len(t, computeClient.getDestroyRequests(), 1)

	_, err = pack.beam.GetBeamByAlias(t.Context(), "missing-compute")
	require.True(t, trace.IsNotFound(err))
}

func TestCreateBeamProvisionFailureDestroyFailureLeavesBeam(t *testing.T) {
	t.Parallel()

	computeClient := &fakeComputeService{
		provisionError: trace.ConnectionProblem(nil, "provision failed"),
		destroyError:   status.Error(codes.Internal, "boom"),
	}

	pack := newBeamServiceTestPack(t, beamServiceTestPackConfig{
		aliasGenerator: sequenceAliasGenerator("stranded-compute"),
		computeClient:  computeClient,
	})

	service := pack.service(t, pack.user(t, "alice"))
	_, err := service.CreateBeam(t.Context(), &beamsv1pb.CreateBeamRequest{
		Egress: beamsv1pb.EgressMode_EGRESS_MODE_UNRESTRICTED,
	})
	require.ErrorContains(t, err, "failed to provision beam compute")
	require.Len(t, computeClient.getDestroyRequests(), 1)

	_, err = pack.beam.GetBeamByAlias(t.Context(), "stranded-compute")
	require.NoError(t, err)
}
