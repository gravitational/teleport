package beamsv1

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	beamsv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
)

func TestListBeamsFiltersByUsers(t *testing.T) {
	t.Parallel()

	pack := newBeamServiceTestPack(t, beamServiceTestPackConfig{})

	adminService := pack.service(t, pack.admin(t))
	bobService := pack.service(t, pack.user(t, "bob"))

	adminBeamRsp, err := adminService.CreateBeam(t.Context(), beamsv1pb.CreateBeamRequest_builder{
		Egress: beamsv1pb.EgressMode_EGRESS_MODE_UNRESTRICTED,
	}.Build())
	require.NoError(t, err)
	adminBeam := adminBeamRsp.GetBeam()

	_, err = bobService.CreateBeam(t.Context(), beamsv1pb.CreateBeamRequest_builder{
		Egress: beamsv1pb.EgressMode_EGRESS_MODE_UNRESTRICTED,
	}.Build())
	require.NoError(t, err)

	resp, err := adminService.ListBeams(t.Context(), beamsv1pb.ListBeamsRequest_builder{
		Filters: beamsv1pb.ListBeamsRequest_Filters_builder{
			Users: []string{"admin"},
		}.Build(),
	}.Build())
	require.NoError(t, err)
	require.Empty(t, resp.GetNextPageToken())
	require.Len(t, resp.GetBeams(), 1)
	require.Equal(t,
		adminBeam.GetMetadata().GetName(),
		resp.GetBeams()[0].GetMetadata().GetName(),
	)
}

func TestListBeamsAuthz(t *testing.T) {
	t.Parallel()

	pack := newBeamServiceTestPack(t, beamServiceTestPackConfig{})

	adminService := pack.service(t, pack.admin(t))
	otherUserService := pack.service(t, pack.user(t, "bob"))

	ownBeamRsp, err := adminService.CreateBeam(t.Context(), beamsv1pb.CreateBeamRequest_builder{
		Egress: beamsv1pb.EgressMode_EGRESS_MODE_UNRESTRICTED,
	}.Build())
	require.NoError(t, err)
	ownBeam := ownBeamRsp.GetBeam()

	otherUserBeamRsp, err := otherUserService.CreateBeam(t.Context(), beamsv1pb.CreateBeamRequest_builder{
		Egress: beamsv1pb.EgressMode_EGRESS_MODE_UNRESTRICTED,
	}.Build())
	require.NoError(t, err)
	otherUserBeam := otherUserBeamRsp.GetBeam()

	// Admin should be able to list both beams.
	adminListRsp, err := adminService.ListBeams(t.Context(), &beamsv1pb.ListBeamsRequest{})
	require.NoError(t, err)
	require.Len(t, adminListRsp.GetBeams(), 2)
	require.Empty(t, adminListRsp.GetNextPageToken())

	require.ElementsMatch(t, []string{
		ownBeam.GetMetadata().GetName(),
		otherUserBeam.GetMetadata().GetName(),
	}, beamNames(adminListRsp.GetBeams()))

	// Other user should only be able to list their own.
	otherUserListRsp, err := otherUserService.ListBeams(t.Context(), &beamsv1pb.ListBeamsRequest{})
	require.NoError(t, err)
	require.Len(t, otherUserListRsp.GetBeams(), 1)
	require.Empty(t, otherUserListRsp.GetNextPageToken())
	require.Equal(t,
		otherUserBeam.GetMetadata().GetName(),
		otherUserListRsp.GetBeams()[0].GetMetadata().GetName(),
	)
}

func TestListBeamsPagination(t *testing.T) {
	t.Parallel()

	pack := newBeamServiceTestPack(t, beamServiceTestPackConfig{})

	service := pack.service(t, pack.user(t, "alice"))

	firstBeamRsp, err := service.CreateBeam(t.Context(), beamsv1pb.CreateBeamRequest_builder{
		Egress: beamsv1pb.EgressMode_EGRESS_MODE_UNRESTRICTED,
	}.Build())
	require.NoError(t, err)
	firstBeam := firstBeamRsp.GetBeam()

	secondBeamRsp, err := service.CreateBeam(t.Context(), beamsv1pb.CreateBeamRequest_builder{
		Egress: beamsv1pb.EgressMode_EGRESS_MODE_UNRESTRICTED,
	}.Build())
	require.NoError(t, err)
	secondBeam := secondBeamRsp.GetBeam()

	for _, tt := range []struct {
		name                 string
		pageSize             int32
		expectedFirstPageLen int
		expectNextPage       bool
	}{
		{
			name:                 "page size less than number of items",
			pageSize:             1,
			expectedFirstPageLen: 1,
			expectNextPage:       true,
		},
		{
			name:                 "page size equals number of items",
			pageSize:             2,
			expectedFirstPageLen: 2,
		},
		{
			name:                 "page size exceeds number of items",
			pageSize:             3,
			expectedFirstPageLen: 2,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			listRsp, err := service.ListBeams(t.Context(), beamsv1pb.ListBeamsRequest_builder{
				PageSize: tt.pageSize,
			}.Build())
			require.NoError(t, err)
			require.Len(t, listRsp.GetBeams(), tt.expectedFirstPageLen)

			listedBeamNames := beamNames(listRsp.GetBeams())

			if tt.expectNextPage {
				require.NotEmpty(t, listRsp.GetNextPageToken())

				listRsp, err = service.ListBeams(t.Context(), beamsv1pb.ListBeamsRequest_builder{
					PageSize:  tt.pageSize,
					PageToken: listRsp.GetNextPageToken(),
				}.Build())
				require.NoError(t, err)
				require.Len(t, listRsp.GetBeams(), 1)
				require.Empty(t, listRsp.GetNextPageToken())

				listedBeamNames = append(listedBeamNames, beamNames(listRsp.GetBeams())...)
			} else {
				require.Empty(t, listRsp.GetNextPageToken())
			}

			require.ElementsMatch(t, []string{
				firstBeam.GetMetadata().GetName(),
				secondBeam.GetMetadata().GetName(),
			}, listedBeamNames)
		})
	}
}

func TestListBeamsAccessDenied(t *testing.T) {
	t.Parallel()
	pack := newBeamServiceTestPack(t, beamServiceTestPackConfig{})

	aliceService := pack.service(t, pack.user(t, "alice"))
	_, err := aliceService.CreateBeam(t.Context(), beamsv1pb.CreateBeamRequest_builder{
		Egress: beamsv1pb.EgressMode_EGRESS_MODE_UNRESTRICTED,
	}.Build())
	require.NoError(t, err)

	nonBeamUserService := pack.service(t, pack.nonBeamUser(t))
	_, err = nonBeamUserService.ListBeams(t.Context(), &beamsv1pb.ListBeamsRequest{})
	require.True(t, trace.IsAccessDenied(err))
}

func beamNames(beams []*beamsv1pb.Beam) []string {
	names := make([]string, 0, len(beams))
	for _, beam := range beams {
		names = append(names, beam.GetMetadata().GetName())
	}
	return names
}
