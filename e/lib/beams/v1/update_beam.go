package beamsv1

import (
	"context"
	"fmt"
	"maps"

	"github.com/gravitational/trace"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"google.golang.org/protobuf/proto"

	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
)

func (s *BeamsService) UpdateBeam(ctx context.Context, req *beamsv1.UpdateBeamRequest) (*beamsv1.UpdateBeamResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindBeam, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	newBeam := req.GetBeam()
	oldBeam, err := s.beamReader.GetBeam(
		ctx,
		newBeam.GetMetadata().GetName(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Checking access against the old and new beams should be the same thing
	// because we don't allow users to modify labels, but we take a belt and
	// braces approach.
	if err := s.checkAccessToBeam(ctx, authCtx, oldBeam); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := s.checkAccessToBeam(ctx, authCtx, newBeam); err != nil {
		return nil, trace.Wrap(err)
	}

	// Do not allow users to set the record expiry, because we must keep it
	// around so we can clean up the actual compute.
	if newBeam.GetMetadata().GetExpires() != nil {
		return nil, trace.BadParameter("metadata.expires: must not be set")
	}

	// These fields are currently immutable.
	if oldBeam.GetSpec().GetEgress() != newBeam.GetSpec().GetEgress() {
		return nil, trace.BadParameter("spec.egress: cannot be modified")
	}
	if !oldBeam.GetSpec().GetExpires().AsTime().Equal(newBeam.GetSpec().GetExpires().AsTime()) {
		return nil, trace.BadParameter("spec.expires: cannot be modified")
	}
	if oldBeam.GetSpec().GetRequestedRegion() != newBeam.GetSpec().GetRequestedRegion() {
		return nil, trace.BadParameter("spec.requested_region: cannot be modified")
	}

	// Labels are managed by the service and used for RBAC. Do not allow users
	// to modify, add, or remove any labels (at least until we figure out how
	// custom labels should work).
	if labelsChanged(oldBeam, newBeam) {
		return nil, trace.BadParameter("metadata.labels: cannot be modified")
	}

	// Prevent user from changing status.
	newBeam.SetStatus(proto.CloneOf(oldBeam.GetStatus()))

	var actions []backend.ConditionalAction

	// If `spec.publish` was changed, create/update/delete the backing application.
	oldPublish := oldBeam.GetSpec().GetPublish()
	newPublish := newBeam.GetSpec().GetPublish()

	if publishChanged(oldPublish, newPublish) {
		if newPublish == nil {
			newBeam.GetStatus().SetAppName("")

			actions, err = s.appWriter.AppendDeleteAppActions(
				actions,
				oldBeam.GetStatus().GetAppName(),
				backend.Whatever(),
			)
			if err != nil {
				return nil, trace.Wrap(err)
			}
		} else {
			app, err := s.publishBeamApp(newBeam)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			newBeam.GetStatus().SetAppName(app.GetName())

			actions, err = s.appWriter.AppendPutAppActions(
				actions,
				app,
				backend.Whatever(),
			)
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}
	}

	actions, err = s.beamWriter.AppendPutBeamActions(
		actions,
		newBeam,
		backend.Revision(newBeam.GetMetadata().GetRevision()),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	revision, err := s.storageBackend.AtomicWrite(ctx, actions)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	newBeam.GetMetadata().SetRevision(revision)

	if publishChanged(oldPublish, newPublish) {
		beamID := newBeam.GetMetadata().GetName()
		if newPublish == nil {
			s.usageReporter.AnonymizeAndSubmit(&usagereporter.BeamsUnpublishedEvent{
				BeamId: beamID,
			})
		} else {
			s.usageReporter.AnonymizeAndSubmit(&usagereporter.BeamsPublishedEvent{
				BeamId:   beamID,
				Protocol: protocolString(newPublish.GetProtocol()),
			})
		}
	}

	return beamsv1.UpdateBeamResponse_builder{
		Beam: newBeam,
	}.Build(), nil
}

func publishChanged(before, after *beamsv1.PublishSpec) bool {
	if before == nil || after == nil {
		return before != after
	}

	return before.GetPort() != after.GetPort() ||
		before.GetProtocol() != after.GetProtocol()
}

func labelsChanged(before, after *beamsv1.Beam) bool {
	return !maps.Equal(before.GetMetadata().GetLabels(), after.GetMetadata().GetLabels())
}

// protocolString renders a Protocol as the product-facing lowercase value
// (matching the web API's beamProtocolString) for usage reporting.
func protocolString(p beamsv1.Protocol) string {
	switch p {
	case beamsv1.Protocol_PROTOCOL_HTTP:
		return "http"
	case beamsv1.Protocol_PROTOCOL_TCP:
		return "tcp"
	default:
		return ""
	}
}

func (s *BeamsService) publishBeamApp(beam *beamsv1.Beam) (types.Application, error) {
	trustDomain, err := spiffeid.TrustDomainFromString(s.clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	spiffeID, err := spiffeid.FromPath(trustDomain, beamSPIFFEPath(beam))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	spec := types.AppSpecV3{
		TLS: &types.AppTLS{
			Mode:           types.AppTLSModeVerifySpiffeID,
			ServerSpiffeId: spiffeID.String(),
			AllowedCas:     []string{types.AppTLSInternalCAWorkloadIdentity},
			ClientCertMode: types.AppClientCertModeManaged,
		},
	}

	protocol := beam.GetSpec().GetPublish().GetProtocol()
	switch protocol {
	case beamsv1.Protocol_PROTOCOL_HTTP:
		spec.URI = fmt.Sprintf("https://%s", beam.GetStatus().GetAppAddrHttp())
	case beamsv1.Protocol_PROTOCOL_TCP:
		spec.URI = fmt.Sprintf("tls://%s", beam.GetStatus().GetAppAddrTcp())
	default:
		return nil, trace.BadParameter("unsupported protocol: %s", protocol)
	}

	labels := beamResourceLabels(beam, false)

	// This label matches the selector in the beams app service config.
	//
	// TODO(boxofrad): Extract this into a constant in the api/types package.
	labels[types.TeleportInternalLabelPrefix+"beams/app-type"] = "ingress"

	expires := beam.GetSpec().GetExpires().AsTime()
	return types.NewAppV3(
		types.Metadata{
			Name: fmt.Sprintf(
				"%s-%s",
				beam.GetStatus().GetAlias(),
				beam.GetMetadata().GetName()[0:4],
			),
			Labels:  labels,
			Expires: &expires,
		},
		spec,
	)
}
