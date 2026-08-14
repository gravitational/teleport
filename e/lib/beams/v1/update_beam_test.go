package beamsv1

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	beamsv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	"github.com/gravitational/teleport/api/types"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
)

func TestUpdateBeamPublish(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		protocol           beamsv1pb.Protocol
		expectedURI        string
		updateProtocol     beamsv1pb.Protocol
		expectedUpdatedURI string
	}{
		{
			name:        "http",
			protocol:    beamsv1pb.Protocol_PROTOCOL_HTTP,
			expectedURI: "https://127.0.0.1:8443",
		},
		{
			name:        "tcp",
			protocol:    beamsv1pb.Protocol_PROTOCOL_TCP,
			expectedURI: "tls://127.0.0.1:8444",
		},
		{
			name:               "http to tcp",
			protocol:           beamsv1pb.Protocol_PROTOCOL_HTTP,
			expectedURI:        "https://127.0.0.1:8443",
			updateProtocol:     beamsv1pb.Protocol_PROTOCOL_TCP,
			expectedUpdatedURI: "tls://127.0.0.1:8444",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			recorder := &recordingUsageReporter{}
			pack := newBeamServiceTestPack(t, beamServiceTestPackConfig{usageReporter: recorder})

			service := pack.service(t, pack.user(t, "alice"))
			createResp, err := service.CreateBeam(t.Context(), beamsv1pb.CreateBeamRequest_builder{
				Egress: beamsv1pb.EgressMode_EGRESS_MODE_UNRESTRICTED,
			}.Build())
			require.NoError(t, err)

			beam := proto.CloneOf(createResp.GetBeam())
			beam.GetSpec().SetPublish(beamsv1pb.PublishSpec_builder{
				Port:     8080,
				Protocol: tt.protocol,
			}.Build())

			resp, err := service.UpdateBeam(t.Context(), beamsv1pb.UpdateBeamRequest_builder{
				Beam: beam,
			}.Build())
			require.NoError(t, err)
			require.NotEmpty(t, resp.GetBeam().GetStatus().GetAppName())

			storedBeam, err := pack.beam.GetBeam(t.Context(), resp.GetBeam().GetMetadata().GetName())
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(resp.GetBeam(), storedBeam, protocmp.Transform()))

			app, err := pack.app.GetApp(t.Context(), resp.GetBeam().GetStatus().GetAppName())
			require.NoError(t, err)
			require.Equal(t, resp.GetBeam().GetStatus().GetAppName(), app.GetName())
			requirePublishedBeamApp(t, app, resp.GetBeam(), tt.expectedURI)

			// Verify BeamsPublishedEvent was emitted.
			var publishedEvents []*usagereporter.BeamsPublishedEvent
			for _, e := range recorder.recorded() {
				if ev, ok := e.(*usagereporter.BeamsPublishedEvent); ok {
					publishedEvents = append(publishedEvents, ev)
				}
			}
			require.Len(t, publishedEvents, 1)
			require.Equal(t, createResp.GetBeam().GetMetadata().GetName(), publishedEvents[0].BeamId)
			require.Equal(t, protocolString(tt.protocol), publishedEvents[0].Protocol)

			if tt.updateProtocol == beamsv1pb.Protocol_PROTOCOL_UNSPECIFIED {
				return
			}

			updatedBeam := proto.CloneOf(resp.GetBeam())
			updatedBeam.GetSpec().GetPublish().SetProtocol(tt.updateProtocol)

			updateResp, err := service.UpdateBeam(t.Context(), beamsv1pb.UpdateBeamRequest_builder{
				Beam: updatedBeam,
			}.Build())
			require.NoError(t, err)
			require.Equal(t, tt.updateProtocol, updateResp.GetBeam().GetSpec().GetPublish().GetProtocol())
			require.Equal(t, resp.GetBeam().GetStatus().GetAppName(), updateResp.GetBeam().GetStatus().GetAppName())

			storedBeam, err = pack.beam.GetBeam(t.Context(), updateResp.GetBeam().GetMetadata().GetName())
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(updateResp.GetBeam(), storedBeam, protocmp.Transform()))

			app, err = pack.app.GetApp(t.Context(), updateResp.GetBeam().GetStatus().GetAppName())
			require.NoError(t, err)
			requirePublishedBeamApp(t, app, updateResp.GetBeam(), tt.expectedUpdatedURI)

			// Verify a second BeamsPublishedEvent was emitted for the protocol change.
			publishedEvents = nil
			for _, e := range recorder.recorded() {
				if ev, ok := e.(*usagereporter.BeamsPublishedEvent); ok {
					publishedEvents = append(publishedEvents, ev)
				}
			}
			require.Len(t, publishedEvents, 2)
			require.Equal(t, protocolString(tt.updateProtocol), publishedEvents[1].Protocol)
		})
	}
}

func requirePublishedBeamApp(t *testing.T, app types.Application, beam *beamsv1pb.Beam, expectedURI string) {
	t.Helper()

	require.Equal(t, expectedURI, app.GetURI())
	require.Equal(t, &types.AppTLS{
		Mode:           types.AppTLSModeVerifySpiffeID,
		ServerSpiffeId: fmt.Sprintf("spiffe://dunder-mifflin.beams.run/_teleport-cloud/beams/%s", beam.GetMetadata().GetName()),
		AllowedCas:     []string{types.AppTLSInternalCAWorkloadIdentity},
		ClientCertMode: types.AppClientCertModeManaged,
	}, app.GetTLS())
	require.Equal(t, types.AppTLSModeVerifySpiffeID, app.GetTLSMode())
	require.Equal(t, types.AppClientCertModeManaged, app.GetClientCertMode())
	require.Equal(t, "ingress", app.GetAllLabels()["teleport.internal/beams/app-type"])
}

func TestUpdateBeamUnpublish(t *testing.T) {
	t.Parallel()

	recorder := &recordingUsageReporter{}
	pack := newBeamServiceTestPack(t, beamServiceTestPackConfig{usageReporter: recorder})

	service := pack.service(t, pack.user(t, "alice"))
	createResp, err := service.CreateBeam(t.Context(), beamsv1pb.CreateBeamRequest_builder{
		Egress: beamsv1pb.EgressMode_EGRESS_MODE_UNRESTRICTED,
	}.Build())
	require.NoError(t, err)

	published := proto.CloneOf(createResp.GetBeam())
	published.GetSpec().SetPublish(beamsv1pb.PublishSpec_builder{
		Port:     8080,
		Protocol: beamsv1pb.Protocol_PROTOCOL_HTTP,
	}.Build())

	publishResp, err := service.UpdateBeam(t.Context(), beamsv1pb.UpdateBeamRequest_builder{
		Beam: published,
	}.Build())
	require.NoError(t, err)
	require.NotEmpty(t, publishResp.GetBeam().GetStatus().GetAppName())

	appName := publishResp.GetBeam().GetStatus().GetAppName()
	unpublished := proto.CloneOf(publishResp.GetBeam())
	unpublished.GetSpec().ClearPublish()

	resp, err := service.UpdateBeam(t.Context(), beamsv1pb.UpdateBeamRequest_builder{
		Beam: unpublished,
	}.Build())
	require.NoError(t, err)
	require.Nil(t, resp.GetBeam().GetSpec().GetPublish())
	require.Empty(t, resp.GetBeam().GetStatus().GetAppName())

	storedBeam, err := pack.beam.GetBeam(t.Context(), resp.GetBeam().GetMetadata().GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(resp.GetBeam(), storedBeam, protocmp.Transform()))

	_, err = pack.app.GetApp(t.Context(), appName)
	require.True(t, trace.IsNotFound(err))

	// Verify BeamsUnpublishedEvent was emitted.
	var unpublishedEvents []*usagereporter.BeamsUnpublishedEvent
	for _, e := range recorder.recorded() {
		if ev, ok := e.(*usagereporter.BeamsUnpublishedEvent); ok {
			unpublishedEvents = append(unpublishedEvents, ev)
		}
	}
	require.Len(t, unpublishedEvents, 1)
	require.Equal(t, createResp.GetBeam().GetMetadata().GetName(), unpublishedEvents[0].BeamId)
}

func TestUpdateBeamRejectsLabelChange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*beamsv1pb.Beam)
	}{
		{
			name: "modify label",
			mutate: func(beam *beamsv1pb.Beam) {
				beam.GetMetadata().GetLabels()[types.BeamAliasLabel] = "modified"
			},
		},
		{
			name: "add user label",
			mutate: func(beam *beamsv1pb.Beam) {
				beam.GetMetadata().GetLabels()["user-controlled"] = "label"
			},
		},
		{
			name: "remove label",
			mutate: func(beam *beamsv1pb.Beam) {
				delete(beam.GetMetadata().GetLabels(), types.BeamAliasLabel)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pack := newBeamServiceTestPack(t, beamServiceTestPackConfig{})

			service := pack.service(t, pack.user(t, "alice"))
			createResp, err := service.CreateBeam(t.Context(), beamsv1pb.CreateBeamRequest_builder{
				Egress: beamsv1pb.EgressMode_EGRESS_MODE_UNRESTRICTED,
			}.Build())
			require.NoError(t, err)

			beam := proto.CloneOf(createResp.GetBeam())
			tt.mutate(beam)

			_, err = service.UpdateBeam(t.Context(), beamsv1pb.UpdateBeamRequest_builder{
				Beam: beam,
			}.Build())
			require.True(t, trace.IsBadParameter(err))
			require.ErrorContains(t, err, "metadata.labels: cannot be modified")

			storedBeam, err := pack.beam.GetBeam(t.Context(), createResp.GetBeam().GetMetadata().GetName())
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(createResp.GetBeam(), storedBeam, protocmp.Transform()))
		})
	}
}

func TestUpdateBeamRejectsMetadataExpiryChange(t *testing.T) {
	t.Parallel()

	pack := newBeamServiceTestPack(t, beamServiceTestPackConfig{})

	service := pack.service(t, pack.user(t, "alice"))
	createResp, err := service.CreateBeam(t.Context(), beamsv1pb.CreateBeamRequest_builder{
		Egress: beamsv1pb.EgressMode_EGRESS_MODE_UNRESTRICTED,
	}.Build())
	require.NoError(t, err)

	beam := proto.CloneOf(createResp.GetBeam())
	beam.GetMetadata().SetExpires(createResp.GetBeam().GetSpec().GetExpires())

	_, err = service.UpdateBeam(t.Context(), beamsv1pb.UpdateBeamRequest_builder{
		Beam: beam,
	}.Build())
	require.True(t, trace.IsBadParameter(err))
	require.ErrorContains(t, err, "metadata.expires: must not be set")

	storedBeam, err := pack.beam.GetBeam(t.Context(), createResp.GetBeam().GetMetadata().GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(createResp.GetBeam(), storedBeam, protocmp.Transform()))
}

func TestUpdateBeamRejectsEgressChange(t *testing.T) {
	t.Parallel()

	pack := newBeamServiceTestPack(t, beamServiceTestPackConfig{})

	service := pack.service(t, pack.user(t, "alice"))
	createResp, err := service.CreateBeam(t.Context(), beamsv1pb.CreateBeamRequest_builder{
		Egress: beamsv1pb.EgressMode_EGRESS_MODE_UNRESTRICTED,
	}.Build())
	require.NoError(t, err)

	beam := proto.CloneOf(createResp.GetBeam())
	beam.GetSpec().SetEgress(beamsv1pb.EgressMode_EGRESS_MODE_RESTRICTED)

	_, err = service.UpdateBeam(t.Context(), beamsv1pb.UpdateBeamRequest_builder{
		Beam: beam,
	}.Build())
	require.True(t, trace.IsBadParameter(err))
	require.ErrorContains(t, err, "spec.egress: cannot be modified")

	storedBeam, err := pack.beam.GetBeam(t.Context(), createResp.GetBeam().GetMetadata().GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(createResp.GetBeam(), storedBeam, protocmp.Transform()))
}

func TestUpdateBeamRejectsSpecExpiryChange(t *testing.T) {
	t.Parallel()

	pack := newBeamServiceTestPack(t, beamServiceTestPackConfig{})

	service := pack.service(t, pack.user(t, "alice"))
	createResp, err := service.CreateBeam(t.Context(), beamsv1pb.CreateBeamRequest_builder{
		Egress: beamsv1pb.EgressMode_EGRESS_MODE_UNRESTRICTED,
	}.Build())
	require.NoError(t, err)

	beam := proto.CloneOf(createResp.GetBeam())
	beam.GetSpec().GetExpires().Seconds++

	_, err = service.UpdateBeam(t.Context(), beamsv1pb.UpdateBeamRequest_builder{
		Beam: beam,
	}.Build())
	require.True(t, trace.IsBadParameter(err))
	require.ErrorContains(t, err, "spec.expires: cannot be modified")

	storedBeam, err := pack.beam.GetBeam(t.Context(), createResp.GetBeam().GetMetadata().GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(createResp.GetBeam(), storedBeam, protocmp.Transform()))
}

func TestUpdateBeamAccessDenied(t *testing.T) {
	t.Parallel()

	pack := newBeamServiceTestPack(t, beamServiceTestPackConfig{})

	aliceService := pack.service(t, pack.user(t, "alice"))
	createResp, err := aliceService.CreateBeam(t.Context(), beamsv1pb.CreateBeamRequest_builder{
		Egress: beamsv1pb.EgressMode_EGRESS_MODE_UNRESTRICTED,
	}.Build())
	require.NoError(t, err)

	beam := proto.CloneOf(createResp.GetBeam())
	beam.GetSpec().SetPublish(beamsv1pb.PublishSpec_builder{
		Port:     8080,
		Protocol: beamsv1pb.Protocol_PROTOCOL_HTTP,
	}.Build())

	bobService := pack.service(t, pack.user(t, "bob"))
	_, err = bobService.UpdateBeam(t.Context(), beamsv1pb.UpdateBeamRequest_builder{
		Beam: beam,
	}.Build())
	require.True(t, trace.IsAccessDenied(err))
}

func TestUpdateBeamNoPublishChange(t *testing.T) {
	t.Parallel()

	pack := newBeamServiceTestPack(t, beamServiceTestPackConfig{})

	service := pack.service(t, pack.user(t, "alice"))
	createResp, err := service.CreateBeam(t.Context(), beamsv1pb.CreateBeamRequest_builder{
		Egress: beamsv1pb.EgressMode_EGRESS_MODE_UNRESTRICTED,
	}.Build())
	require.NoError(t, err)

	beam := proto.CloneOf(createResp.GetBeam())

	resp, err := service.UpdateBeam(t.Context(), beamsv1pb.UpdateBeamRequest_builder{
		Beam: beam,
	}.Build())
	require.NoError(t, err)

	require.Empty(t, cmp.Diff(
		createResp.GetBeam(),
		resp.GetBeam(),
		protocmp.Transform(),
		protocmp.IgnoreFields(createResp.GetBeam().GetMetadata(), "revision"),
	))

	storedBeam, err := pack.beam.GetBeam(t.Context(), resp.GetBeam().GetMetadata().GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(resp.GetBeam(), storedBeam, protocmp.Transform()))
}
