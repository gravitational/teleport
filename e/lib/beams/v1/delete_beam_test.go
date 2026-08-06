package beamsv1

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	beamsv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	compute "github.com/gravitational/teleport/e/api/beamservice/v1"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/services"
)

func TestDeleteBeam(t *testing.T) {
	t.Parallel()

	pack := newBeamServiceTestPack(t, beamServiceTestPackConfig{})

	// Create the beam.
	service := pack.service(t, pack.user(t, "alice"))
	createResp, err := service.CreateBeam(t.Context(), beamsv1pb.CreateBeamRequest_builder{
		Egress: beamsv1pb.EgressMode_EGRESS_MODE_UNRESTRICTED,
	}.Build())
	require.NoError(t, err)
	beam := proto.CloneOf(createResp.GetBeam())

	// Publish the beam.
	beam.GetSpec().SetPublish(beamsv1pb.PublishSpec_builder{
		Port:     8080,
		Protocol: beamsv1pb.Protocol_PROTOCOL_HTTP,
	}.Build())

	updateResp, err := service.UpdateBeam(t.Context(), beamsv1pb.UpdateBeamRequest_builder{
		Beam: beam,
	}.Build())
	require.NoError(t, err)

	beam = updateResp.GetBeam()
	require.NotEmpty(t, beam.GetStatus().GetAppName())

	// Delete the beam.
	resp, err := service.DeleteBeam(t.Context(), beamsv1pb.DeleteBeamRequest_builder{
		Name: beam.GetMetadata().GetName(),
	}.Build())
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Check the compute was destroyed.
	require.Len(t, pack.compute.getDestroyRequests(), 1)
	require.Equal(t,
		beam.GetMetadata().GetName(),
		pack.compute.getDestroyRequests()[0].GetBeamId(),
	)

	// Check the beam and all of its resources are deleted.
	_, err = pack.beam.GetBeam(t.Context(), beam.GetMetadata().GetName())
	require.True(t, trace.IsNotFound(err))

	_, err = pack.beam.GetBeamByAlias(t.Context(), beam.GetStatus().GetAlias())
	require.True(t, trace.IsNotFound(err))

	_, err = pack.token.GetToken(t.Context(), beam.GetStatus().GetJoinTokenName())
	require.True(t, trace.IsNotFound(err))

	botResourceName, err := services.BotResourceName(scopes.QualifiedName{Name: beam.GetStatus().GetBotName()})
	require.NoError(t, err)
	_, err = pack.identity.GetUser(t.Context(), botResourceName, false)
	require.True(t, trace.IsNotFound(err))

	_, err = pack.role.GetRole(t.Context(), botResourceName)
	require.True(t, trace.IsNotFound(err))

	_, err = pack.workloadIdentity.GetWorkloadIdentity(t.Context(), workloadidentityv1pb.GetWorkloadIdentityRequest_builder{Name: beam.GetStatus().GetWorkloadIdentityName()}.Build())
	require.True(t, trace.IsNotFound(err))

	_, err = pack.delegationSession.GetDelegationSession(t.Context(), beam.GetStatus().GetDelegationSessionId())
	require.True(t, trace.IsNotFound(err))

	_, err = pack.presence.GetNode(t.Context(), apidefaults.Namespace, beam.GetStatus().GetNodeId())
	require.True(t, trace.IsNotFound(err))

	_, err = pack.app.GetApp(t.Context(), beam.GetStatus().GetAppName())
	require.True(t, trace.IsNotFound(err))
}

func TestDeleteBeamUsesStoredRegion(t *testing.T) {
	t.Parallel()

	pack := newBeamServiceTestPack(t, beamServiceTestPackConfig{
		computeClient: &fakeComputeService{
			getInfoResponse: &compute.GetInfoResponse{
				Region: "eu-central-1",
			},
			provisionResponse: &compute.ProvisionBeamResponse{
				SshAddr: "127.0.0.1:3022",
			},
		},
		validRegions: []string{"us-east-1", "eu-west-1"},
	})
	service := pack.service(t, pack.user(t, "alice"))

	createResp, err := service.CreateBeam(t.Context(), beamsv1pb.CreateBeamRequest_builder{
		Egress:      beamsv1pb.EgressMode_EGRESS_MODE_UNRESTRICTED,
		ProxyRegion: "eu-west-1",
	}.Build())
	require.NoError(t, err)

	_, err = service.DeleteBeam(t.Context(), beamsv1pb.DeleteBeamRequest_builder{
		Name: createResp.GetBeam().GetMetadata().GetName(),
	}.Build())
	require.NoError(t, err)

	require.Len(t, pack.compute.getDestroyRequests(), 1)
	require.Equal(t, "eu-central-1", pack.compute.getDestroyRequests()[0].GetRegion())
}

func TestDeleteBeamMissingName(t *testing.T) {
	t.Parallel()

	pack := newBeamServiceTestPack(t, beamServiceTestPackConfig{
		computeClient: &fakeComputeService{},
	})

	service := pack.service(t, pack.user(t, "alice"))
	_, err := service.DeleteBeam(t.Context(), &beamsv1pb.DeleteBeamRequest{})
	require.ErrorContains(t, err, "name is required")
}

func TestDeleteBeamAccessDenied(t *testing.T) {
	t.Parallel()

	pack := newBeamServiceTestPack(t, beamServiceTestPackConfig{})

	// Create a beam.
	aliceService := pack.service(t, pack.user(t, "alice"))
	createResp, err := aliceService.CreateBeam(t.Context(), beamsv1pb.CreateBeamRequest_builder{
		Egress: beamsv1pb.EgressMode_EGRESS_MODE_UNRESTRICTED,
	}.Build())
	require.NoError(t, err)

	bobService := pack.service(t, pack.user(t, "bob"))
	_, err = bobService.DeleteBeam(t.Context(), beamsv1pb.DeleteBeamRequest_builder{
		Name: createResp.GetBeam().GetMetadata().GetName(),
	}.Build())
	require.True(t, trace.IsAccessDenied(err))

	// Check beam wasn't deleted.
	_, err = pack.beam.GetBeam(t.Context(), createResp.GetBeam().GetMetadata().GetName())
	require.NoError(t, err)
}

func TestDeleteBeamDestroyComputeNotFoundStillDeletes(t *testing.T) {
	t.Parallel()

	pack := newBeamServiceTestPack(t, beamServiceTestPackConfig{
		computeClient: &fakeComputeService{
			provisionResponse: &compute.ProvisionBeamResponse{
				SshAddr: "127.0.0.1:3022",
			},
			destroyError: status.Error(codes.NotFound, "beam missing"),
		},
	})

	service := pack.service(t, pack.user(t, "alice"))
	createResp, err := service.CreateBeam(t.Context(), beamsv1pb.CreateBeamRequest_builder{
		Egress: beamsv1pb.EgressMode_EGRESS_MODE_UNRESTRICTED,
	}.Build())
	require.NoError(t, err)

	_, err = service.DeleteBeam(t.Context(), beamsv1pb.DeleteBeamRequest_builder{
		Name: createResp.GetBeam().GetMetadata().GetName(),
	}.Build())
	require.NoError(t, err)
	require.Len(t, pack.compute.getDestroyRequests(), 1)

	// Check the local record was still deleted.
	_, err = pack.beam.GetBeam(t.Context(), createResp.GetBeam().GetMetadata().GetName())
	require.True(t, trace.IsNotFound(err))
}

func TestDeleteBeamDestroyComputeFailureLeavesBeam(t *testing.T) {
	t.Parallel()

	pack := newBeamServiceTestPack(t, beamServiceTestPackConfig{
		computeClient: &fakeComputeService{
			provisionResponse: &compute.ProvisionBeamResponse{
				SshAddr: "127.0.0.1:3022",
			},
			destroyError: status.Error(codes.Internal, "boom"),
		},
	})

	service := pack.service(t, pack.user(t, "alice"))
	createResp, err := service.CreateBeam(t.Context(), beamsv1pb.CreateBeamRequest_builder{
		Egress: beamsv1pb.EgressMode_EGRESS_MODE_UNRESTRICTED,
	}.Build())
	require.NoError(t, err)

	_, err = service.DeleteBeam(t.Context(), beamsv1pb.DeleteBeamRequest_builder{
		Name: createResp.GetBeam().GetMetadata().GetName(),
	}.Build())
	require.ErrorContains(t, err, "failed to deprovision beam compute")
	require.Len(t, pack.compute.getDestroyRequests(), 1)

	// Check the beam was not deleted.
	_, err = pack.beam.GetBeam(t.Context(), createResp.GetBeam().GetMetadata().GetName())
	require.NoError(t, err)
}
