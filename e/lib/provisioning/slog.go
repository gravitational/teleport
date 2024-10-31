package provisioning

import (
	"log/slog"

	provisioningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/provisioning/v1"
)

func principalStateAttr(s *provisioningv1.PrincipalState) slog.Attr {
	return slog.Any("principal_state", principalStateValuer{state: s})
}

type principalStateValuer struct {
	state *provisioningv1.PrincipalState
}

func (psv principalStateValuer) LogValue() slog.Value {
	state := psv.state
	spec := state.GetSpec()
	return slog.GroupValue(
		slog.String("id", state.GetMetadata().GetName()),
		slog.String("principal_id", spec.GetPrincipalId()),
		slog.String("principal_type", spec.GetPrincipalType().String()),
		slog.String("external_id", state.GetStatus().GetExternalId()))
}

func externalIDAttr(id ExternalID) slog.Attr {
	return slog.String("external_id", string(id))
}
