package beamsv1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	beamsv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
)

func TestGetBeamByName(t *testing.T) {
	t.Parallel()

	pack := newBeamServiceTestPack(t, beamServiceTestPackConfig{})

	service := pack.service(t, pack.user(t, "alice"))
	createResp, err := service.CreateBeam(t.Context(), beamsv1pb.CreateBeamRequest_builder{
		Egress: beamsv1pb.EgressMode_EGRESS_MODE_UNRESTRICTED,
	}.Build())
	require.NoError(t, err)

	resp, err := service.GetBeam(t.Context(), beamsv1pb.GetBeamRequest_builder{
		Name: proto.String(createResp.GetBeam().GetMetadata().GetName()),
	}.Build())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(createResp.GetBeam(), resp.GetBeam(), protocmp.Transform()))
}

func TestGetBeamByAlias(t *testing.T) {
	t.Parallel()

	pack := newBeamServiceTestPack(t, beamServiceTestPackConfig{})

	service := pack.service(t, pack.user(t, "alice"))
	createResp, err := service.CreateBeam(t.Context(), beamsv1pb.CreateBeamRequest_builder{
		Egress: beamsv1pb.EgressMode_EGRESS_MODE_UNRESTRICTED,
	}.Build())
	require.NoError(t, err)

	resp, err := service.GetBeam(t.Context(), beamsv1pb.GetBeamRequest_builder{
		Alias: proto.String(createResp.GetBeam().GetStatus().GetAlias()),
	}.Build())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(createResp.GetBeam(), resp.GetBeam(), protocmp.Transform()))
}

func TestGetBeamMissingID(t *testing.T) {
	t.Parallel()

	pack := newBeamServiceTestPack(t, beamServiceTestPackConfig{})

	service := pack.service(t, pack.user(t, "alice"))
	_, err := service.GetBeam(t.Context(), &beamsv1pb.GetBeamRequest{})
	require.ErrorContains(t, err, "id is required")
}

func TestGetBeamMissingName(t *testing.T) {
	t.Parallel()

	pack := newBeamServiceTestPack(t, beamServiceTestPackConfig{})

	service := pack.service(t, pack.user(t, "alice"))
	_, err := service.GetBeam(t.Context(), beamsv1pb.GetBeamRequest_builder{
		Name: proto.String(""),
	}.Build())
	require.ErrorContains(t, err, "name is required")
}

func TestGetBeamMissingAlias(t *testing.T) {
	t.Parallel()

	pack := newBeamServiceTestPack(t, beamServiceTestPackConfig{})

	service := pack.service(t, pack.user(t, "alice"))
	_, err := service.GetBeam(t.Context(), beamsv1pb.GetBeamRequest_builder{
		Alias: proto.String(""),
	}.Build())
	require.ErrorContains(t, err, "alias is required")
}

func TestGetBeamAccessDenied(t *testing.T) {
	t.Parallel()

	pack := newBeamServiceTestPack(t, beamServiceTestPackConfig{})

	aliceService := pack.service(t, pack.user(t, "alice"))
	createResp, err := aliceService.CreateBeam(t.Context(), beamsv1pb.CreateBeamRequest_builder{
		Egress: beamsv1pb.EgressMode_EGRESS_MODE_UNRESTRICTED,
	}.Build())
	require.NoError(t, err)

	nonBeamUserService := pack.service(t, pack.nonBeamUser(t))
	_, err = nonBeamUserService.GetBeam(t.Context(), beamsv1pb.GetBeamRequest_builder{
		Name: proto.String(createResp.GetBeam().GetMetadata().GetName()),
	}.Build())
	require.True(t, trace.IsAccessDenied(err))

	bobService := pack.service(t, pack.user(t, "bob"))
	_, err = bobService.GetBeam(t.Context(), beamsv1pb.GetBeamRequest_builder{
		Name: proto.String(createResp.GetBeam().GetMetadata().GetName()),
	}.Build())
	require.True(t, trace.IsAccessDenied(err))

	adminService := pack.service(t, pack.admin(t))
	_, err = adminService.GetBeam(t.Context(), beamsv1pb.GetBeamRequest_builder{
		Name: proto.String(createResp.GetBeam().GetMetadata().GetName()),
	}.Build())
	require.NoError(t, err)
}
