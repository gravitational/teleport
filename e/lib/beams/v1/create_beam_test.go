package beamsv1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	beamsv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	delegationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/delegation/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	presencev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/presence/v1"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/scopes"
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
	resp, err := service.CreateBeam(t.Context(), beamsv1pb.CreateBeamRequest_builder{
		Egress:         beamsv1pb.EgressMode_EGRESS_MODE_RESTRICTED,
		AllowedDomains: []string{"example.com."},
	}.Build())
	require.NoError(t, err)

	beam := resp.GetBeam()
	require.NotNil(t, beam)

	expectedLabels := map[string]string{
		types.BeamIDLabel:    beam.GetMetadata().GetName(),
		types.BeamOwnerLabel: "alice",
		types.BeamAliasLabel: "steady-river",
	}

	expectedBeam := beamsv1pb.Beam_builder{
		Kind:    types.KindBeam,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name:   beam.GetMetadata().GetName(),
			Labels: expectedLabels,
		}.Build(),
		Spec: beamsv1pb.BeamSpec_builder{
			Egress:         beamsv1pb.EgressMode_EGRESS_MODE_RESTRICTED,
			AllowedDomains: []string{"example.com."},
			Expires:        beam.GetSpec().GetExpires(),
		}.Build(),
		Status: beamsv1pb.BeamStatus_builder{
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
		}.Build(),
	}.Build()
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
	require.Equal(t, map[string]string{
		types.BeamIDLabel:                  beam.GetMetadata().GetName(),
		types.BeamOwnerLabel:               expectedLabels[types.BeamOwnerLabel],
		types.BeamAliasLabel:               expectedLabels[types.BeamAliasLabel],
		types.TeleportInternalResourceType: types.SystemResource,
	}, token.GetMetadata().Labels)
	require.True(t, types.IsSystemResource(token), "beam join token must be a system resource")
	require.Equal(t, computeReq.GetTbot().GetRegistrationSecret(), token.GetBoundKeypairStatus().RegistrationSecret)

	botResourceName, err := services.BotResourceName(scopes.QualifiedName{Name: beam.GetStatus().GetBotName()})
	require.NoError(t, err)
	user, err := pack.identity.GetUser(t.Context(), botResourceName, false)
	require.NoError(t, err)
	require.Equal(t, botResourceName, user.GetName())
	require.Equal(t, []string{
		beam.GetMetadata().GetName(),
	}, user.GetTraits()["teleport.internal/beams/id"])
	require.True(t, types.IsSystemResource(user), "beam bot user must be a system resource")

	role, err := pack.role.GetRole(t.Context(), botResourceName)
	require.NoError(t, err)
	require.Equal(t, botResourceName, role.GetName())
	require.True(t, types.IsSystemResource(role), "beam bot role must be a system resource")

	workloadIdentity, err := pack.workloadIdentity.GetWorkloadIdentity(t.Context(), workloadidentityv1pb.GetWorkloadIdentityRequest_builder{Name: beam.GetStatus().GetWorkloadIdentityName()}.Build())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(
		workloadidentityv1pb.WorkloadIdentity_builder{
			Kind:    types.KindWorkloadIdentity,
			Version: types.V1,
			Metadata: headerv1.Metadata_builder{
				Name:    beam.GetStatus().GetWorkloadIdentityName(),
				Labels:  expectedLabels,
				Expires: beam.GetSpec().GetExpires(),
			}.Build(),
			Spec: workloadidentityv1pb.WorkloadIdentitySpec_builder{
				Rules: workloadidentityv1pb.WorkloadIdentityRules_builder{
					Allow: []*workloadidentityv1pb.WorkloadIdentityRule{
						workloadidentityv1pb.WorkloadIdentityRule_builder{Expression: `user.bot_name == "` + beam.GetStatus().GetBotName() + `"`}.Build(),
					},
				}.Build(),
				Spiffe: workloadidentityv1pb.WorkloadIdentitySPIFFE_builder{
					Id: "/_teleport-cloud/beams/" + beam.GetMetadata().GetName(),
				}.Build(),
			}.Build(),
		}.Build(),
		workloadIdentity,
		protocmp.Transform(),
		protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
	))

	delegationSession, err := pack.delegationSession.GetDelegationSession(t.Context(), beam.GetStatus().GetDelegationSessionId())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(
		delegationv1.DelegationSession_builder{
			Kind:    types.KindDelegationSession,
			Version: types.V1,
			Metadata: headerv1.Metadata_builder{
				Name:    beam.GetStatus().GetDelegationSessionId(),
				Labels:  expectedLabels,
				Expires: beam.GetSpec().GetExpires(),
			}.Build(),
			Spec: delegationv1.DelegationSessionSpec_builder{
				User: "alice",
				Resources: []*delegationv1.DelegationResourceSpec{
					delegationv1.DelegationResourceSpec_builder{Kind: types.Wildcard, Name: types.Wildcard}.Build(),
				},
				AuthorizedUsers: []*delegationv1.DelegationUserSpec{
					delegationv1.DelegationUserSpec_builder{
						Kind:    types.KindBot,
						BotName: proto.String(beam.GetStatus().GetBotName()),
					}.Build(),
				},
			}.Build(),
		}.Build(),
		delegationSession,
		protocmp.Transform(),
		protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
	))

	node, err := pack.presence.GetSSHServer(t.Context(), presencev1.GetSSHServerRequest_builder{Name: beam.GetStatus().GetNodeId()}.Build())
	require.NoError(t, err)
	require.Equal(t, beam.GetStatus().GetNodeId(), node.GetName())
	require.Equal(t, beam.GetStatus().GetSshAddr(), node.GetAddr())
	require.Equal(t, expectedLabels, node.GetStaticLabels())
	require.Equal(t, storedBeam.GetMetadata().GetName(), node.GetMetadata().Name)
}

func TestCreateBeamRegions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                       string
		proxyRegion                string
		overrideRegion             string
		defaultRegion              string
		infoRegion                 string
		wantDefaultProvisionCount  int
		wantRegionalProvisionCount int
		wantProviderRegions        []string
		wantRequestedRegion        string
		wantStatusRegion           string
		wantRegionLabel            string
	}{
		{
			name:                      "uses default client",
			wantDefaultProvisionCount: 1,
		},
		{
			name:                       "uses default region",
			defaultRegion:              "us-east-1",
			wantRegionalProvisionCount: 1,
			wantProviderRegions:        []string{"us-east-1"},
			wantRequestedRegion:        "us-east-1",
		},
		{
			name:                       "uses returned default region",
			defaultRegion:              "us-east-1",
			infoRegion:                 "us-east-1",
			wantRegionalProvisionCount: 1,
			wantProviderRegions:        []string{"us-east-1"},
			wantRequestedRegion:        "us-east-1",
			wantStatusRegion:           "us-east-1",
			wantRegionLabel:            "us-east-1",
		},
		{
			name:                       "uses override region",
			proxyRegion:                "us-east-1",
			overrideRegion:             "eu-west-1",
			infoRegion:                 "eu-central-1",
			wantRegionalProvisionCount: 1,
			wantProviderRegions:        []string{"eu-west-1"},
			wantRequestedRegion:        "eu-west-1",
			wantStatusRegion:           "eu-central-1",
			wantRegionLabel:            "eu-central-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			defaultClient := &fakeComputeService{
				provisionResponse: &compute.ProvisionBeamResponse{
					SshAddr:     "127.0.0.1:3022",
					AppAddrHttp: "127.0.0.1:8080",
					AppAddrTcp:  "127.0.0.1:10080",
				},
			}
			regionalClient := &fakeComputeService{
				getInfoResponse: &compute.GetInfoResponse{
					Region: tt.infoRegion,
				},
				provisionResponse: &compute.ProvisionBeamResponse{
					SshAddr:     "127.0.0.1:3022",
					AppAddrHttp: "127.0.0.1:8080",
					AppAddrTcp:  "127.0.0.1:10080",
				},
			}
			useRegionalClient := tt.defaultRegion != "" || tt.proxyRegion != "" || tt.overrideRegion != ""
			if useRegionalClient {
				defaultClient.provisionError = trace.Errorf("default client should not be used")
			} else {
				regionalClient.provisionError = trace.Errorf("regional provider should not be used")
			}
			provider := &fakeComputeServiceProvider{client: regionalClient}
			pack := newBeamServiceTestPack(t, beamServiceTestPackConfig{
				computeClient:   defaultClient,
				computeProvider: provider,
				validRegions:    []string{"us-east-1", "eu-west-1"},
				defaultRegion:   tt.defaultRegion,
			})
			service := pack.service(t, pack.user(t, "alice"))

			resp, err := service.CreateBeam(t.Context(), beamsv1pb.CreateBeamRequest_builder{
				Egress:         beamsv1pb.EgressMode_EGRESS_MODE_UNRESTRICTED,
				ProxyRegion:    tt.proxyRegion,
				OverrideRegion: tt.overrideRegion,
			}.Build())
			require.NoError(t, err)

			beam := resp.GetBeam()
			require.Equal(t, tt.wantRequestedRegion, beam.GetSpec().GetRequestedRegion())
			require.Equal(t, tt.wantStatusRegion, beam.GetStatus().GetRegion())
			require.Equal(t, tt.wantRegionLabel, beam.GetMetadata().GetLabels()[types.BeamRegionLabel])
			require.Len(t, defaultClient.getProvisionRequests(), tt.wantDefaultProvisionCount)
			require.Equal(t, tt.wantProviderRegions, provider.getRegions())
			require.Len(t, regionalClient.getProvisionRequests(), tt.wantRegionalProvisionCount)
			if tt.wantRegionalProvisionCount > 0 {
				require.Len(t, regionalClient.getGetInfoRequests(), 1)
				require.Equal(t, tt.wantStatusRegion, regionalClient.getProvisionRequests()[0].GetRegion())
			}

			storedBeam, err := pack.beam.GetBeam(t.Context(), beam.GetMetadata().GetName())
			require.NoError(t, err)
			require.Equal(t, tt.wantRequestedRegion, storedBeam.GetSpec().GetRequestedRegion())
			require.Equal(t, tt.wantStatusRegion, storedBeam.GetStatus().GetRegion())
			require.Equal(t, tt.wantRegionLabel, storedBeam.GetMetadata().GetLabels()[types.BeamRegionLabel])

			node, err := pack.presence.GetSSHServer(t.Context(), presencev1.GetSSHServerRequest_builder{Name: beam.GetStatus().GetNodeId()}.Build())
			require.NoError(t, err)
			require.Equal(t, tt.wantRegionLabel, node.GetStaticLabels()[types.BeamRegionLabel])
		})
	}
}

func TestClientForRegionRequiresRegionWithoutDefaultClient(t *testing.T) {
	t.Parallel()

	pack := newBeamServiceTestPack(t, beamServiceTestPackConfig{
		noComputeClient: true,
		computeProvider: &fakeComputeServiceProvider{err: trace.BadParameter("beam region is required")},
		validRegions:    []string{"us-east-1"},
	})
	service := pack.service(t, pack.user(t, "alice"))

	_, err := service.clientForRegion("")
	require.True(t, trace.IsBadParameter(err), "got %v", err)
	require.ErrorContains(t, err, "beam region is required")
	require.NotContains(t, err.Error(), "TELEPORT_BEAM_SERVICE")
}

func TestCreateBeamRejectsInvalidRegion(t *testing.T) {
	t.Parallel()

	computeClient := &fakeComputeService{}
	pack := newBeamServiceTestPack(t, beamServiceTestPackConfig{
		computeClient: computeClient,
	})
	service := pack.service(t, pack.user(t, "alice"))

	_, err := service.CreateBeam(t.Context(), beamsv1pb.CreateBeamRequest_builder{
		Egress:      beamsv1pb.EgressMode_EGRESS_MODE_UNRESTRICTED,
		ProxyRegion: "us-east-1.teleport-beam",
	}.Build())
	require.True(t, trace.IsBadParameter(err), "got %v", err)
	require.Empty(t, computeClient.getProvisionRequests())
}

func TestCreateBeamRejectsRegionOutsideAllowList(t *testing.T) {
	t.Parallel()

	computeClient := &fakeComputeService{}
	pack := newBeamServiceTestPack(t, beamServiceTestPackConfig{
		computeClient: computeClient,
		validRegions:  []string{"us-east-1"},
	})
	service := pack.service(t, pack.user(t, "alice"))

	_, err := service.CreateBeam(t.Context(), beamsv1pb.CreateBeamRequest_builder{
		Egress:      beamsv1pb.EgressMode_EGRESS_MODE_UNRESTRICTED,
		ProxyRegion: "eu-west-1",
	}.Build())
	require.True(t, trace.IsBadParameter(err), "got %v", err)
	require.Empty(t, computeClient.getProvisionRequests())

	beams, _, err := pack.beam.ListBeams(t.Context(), 10, "")
	require.NoError(t, err)
	require.Empty(t, beams)
}

func TestCreateBeamRetriesAliasCollision(t *testing.T) {
	t.Parallel()

	pack := newBeamServiceTestPack(t, beamServiceTestPackConfig{
		aliasGenerator: sequenceAliasGenerator("steady-river", "steady-river", "brisk-otter"),
	})

	service := pack.service(t, pack.user(t, "alice"))
	firstResp, err := service.CreateBeam(t.Context(), beamsv1pb.CreateBeamRequest_builder{
		Egress:         beamsv1pb.EgressMode_EGRESS_MODE_RESTRICTED,
		AllowedDomains: []string{"example.com."},
	}.Build())
	require.NoError(t, err)
	require.Equal(t, "steady-river", firstResp.GetBeam().GetStatus().GetAlias())

	resp, err := service.CreateBeam(t.Context(), beamsv1pb.CreateBeamRequest_builder{
		Egress:         beamsv1pb.EgressMode_EGRESS_MODE_RESTRICTED,
		AllowedDomains: []string{"example.com."},
	}.Build())
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
	_, err := service.CreateBeam(t.Context(), beamsv1pb.CreateBeamRequest_builder{
		Egress:         beamsv1pb.EgressMode_EGRESS_MODE_RESTRICTED,
		AllowedDomains: []string{"example.com."},
	}.Build())
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
	_, err := service.CreateBeam(t.Context(), beamsv1pb.CreateBeamRequest_builder{
		Egress: beamsv1pb.EgressMode_EGRESS_MODE_UNRESTRICTED,
	}.Build())
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
	_, err := service.CreateBeam(t.Context(), beamsv1pb.CreateBeamRequest_builder{
		Egress: beamsv1pb.EgressMode_EGRESS_MODE_UNRESTRICTED,
	}.Build())
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
	_, err := service.CreateBeam(t.Context(), beamsv1pb.CreateBeamRequest_builder{
		Egress: beamsv1pb.EgressMode_EGRESS_MODE_UNRESTRICTED,
	}.Build())
	require.ErrorContains(t, err, "failed to provision beam compute")
	require.Len(t, computeClient.getDestroyRequests(), 1)

	_, err = pack.beam.GetBeamByAlias(t.Context(), "stranded-compute")
	require.NoError(t, err)
}
