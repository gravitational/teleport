package beamsv1

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	beamsv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	compute "github.com/gravitational/teleport/e/api/beamservice/v1"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

// timeToFirstCollection is the time until the first beam garbage collection
// should have run, allowing for jitter.
const timeToFirstCollection = beamTTL + time.Duration(float64(gcDefaultCollectionInterval)*1.5)

func TestGarbageCollectorDeletesExpiredBeam(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		pack := newBeamServiceTestPack(t, beamServiceTestPackConfig{})

		gc := pack.newGarbageCollector(t)
		go gc.Run(t.Context())

		service := pack.service(t, pack.user(t, "alice"))
		createResp, err := service.CreateBeam(t.Context(), beamsv1pb.CreateBeamRequest_builder{
			Egress: beamsv1pb.EgressMode_EGRESS_MODE_UNRESTRICTED,
		}.Build())
		require.NoError(t, err)

		time.Sleep(timeToFirstCollection)
		synctest.Wait()

		require.Len(t, pack.compute.getDestroyRequests(), 1)
		_, err = pack.beam.GetBeam(t.Context(), createResp.GetBeam().GetMetadata().GetName())
		require.True(t, trace.IsNotFound(err))
	})
}

func TestGarbageCollectorDoesNotDeleteNonExpiredBeam(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		pack := newBeamServiceTestPack(t, beamServiceTestPackConfig{})

		service := pack.service(t, pack.user(t, "alice"))
		createResp, err := service.CreateBeam(t.Context(), beamsv1pb.CreateBeamRequest_builder{
			Egress: beamsv1pb.EgressMode_EGRESS_MODE_UNRESTRICTED,
		}.Build())
		require.NoError(t, err)

		gc := pack.newGarbageCollector(t)
		go gc.Run(t.Context())

		time.Sleep(time.Duration(float64(gcDefaultCollectionInterval) * 1.5))
		synctest.Wait()

		require.Empty(t, pack.compute.getDestroyRequests())
		_, err = pack.beam.GetBeam(t.Context(), createResp.GetBeam().GetMetadata().GetName())
		require.NoError(t, err)
	})
}

func TestGarbageCollectorRetriesDeleteAfterCompareFailure(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		pack := newBeamServiceTestPack(t, beamServiceTestPackConfig{})

		gc := pack.newGarbageCollector(t)
		go gc.Run(t.Context())

		service := pack.service(t, pack.user(t, "alice"))
		createResp, err := service.CreateBeam(t.Context(), beamsv1pb.CreateBeamRequest_builder{
			Egress: beamsv1pb.EgressMode_EGRESS_MODE_UNRESTRICTED,
		}.Build())
		require.NoError(t, err)

		staleBeam, err := pack.beam.GetBeam(t.Context(), createResp.GetBeam().GetMetadata().GetName())
		require.NoError(t, err)

		freshBeam := proto.CloneOf(staleBeam)
		freshBeam.GetSpec().SetPublish(beamsv1pb.PublishSpec_builder{
			Port:     8080,
			Protocol: beamsv1pb.Protocol_PROTOCOL_HTTP,
		}.Build())
		updateResp, err := service.UpdateBeam(t.Context(), beamsv1pb.UpdateBeamRequest_builder{
			Beam: freshBeam,
		}.Build())
		require.NoError(t, err)
		require.NotEmpty(t, updateResp.GetBeam().GetStatus().GetAppName())

		time.Sleep(timeToFirstCollection)
		synctest.Wait()

		require.Len(t, pack.compute.getDestroyRequests(), 1)

		_, err = pack.beam.GetBeam(t.Context(), staleBeam.GetMetadata().GetName())
		require.True(t, trace.IsNotFound(err))

		_, err = pack.app.GetApp(t.Context(), updateResp.GetBeam().GetStatus().GetAppName())
		require.True(t, trace.IsNotFound(err))
	})

}

func TestGarbageCollectorLeavesBeamOnComputeFailure(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		pack := newBeamServiceTestPack(t, beamServiceTestPackConfig{
			computeClient: &fakeComputeService{
				provisionResponse: &compute.ProvisionBeamResponse{
					SshAddr:     "127.0.0.1:3022",
					AppAddrHttp: "127.0.0.1:8443",
					AppAddrTcp:  "127.0.0.1:8444",
				},
				destroyError: status.Error(codes.Internal, "boom"),
			},
		})

		gc := pack.newGarbageCollector(t)
		go gc.Run(t.Context())

		service := pack.service(t, pack.user(t, "alice"))
		createResp, err := service.CreateBeam(t.Context(), beamsv1pb.CreateBeamRequest_builder{
			Egress: beamsv1pb.EgressMode_EGRESS_MODE_UNRESTRICTED,
		}.Build())
		require.NoError(t, err)

		time.Sleep(timeToFirstCollection)
		synctest.Wait()

		require.NotEmpty(t, pack.compute.getDestroyRequests())
		_, err = pack.beam.GetBeam(t.Context(), createResp.GetBeam().GetMetadata().GetName())
		require.NoError(t, err)
	})
}

func (p *beamServiceTestPack) newGarbageCollector(t *testing.T) *GarbageCollector {
	t.Helper()

	gc, err := NewGarbageCollector(GarbageCollectorConfig{
		Cache:       p.beam,
		Backend:     p.beam,
		BeamService: p.service(t, p.admin(t)),
		Semaphores:  p.presence,
		HostID:      "test-auth",
		Logger:      logtest.NewLogger(),
	})
	require.NoError(t, err)

	return gc
}
