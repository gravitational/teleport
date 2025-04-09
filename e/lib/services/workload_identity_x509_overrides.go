package services

import (
	"github.com/gravitational/trace"

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	apitypes "github.com/gravitational/teleport/api/types"
)

type ParsedWorkloadIdentityX509IssuerOverride struct{}

func ParseWorkloadIdentityX509IssuerOverride(resource *workloadidentityv1pb.X509IssuerOverride) (*ParsedWorkloadIdentityX509IssuerOverride, error) {
	if expected, actual := apitypes.KindWorkloadIdentityX509IssuerOverride, resource.GetKind(); expected != actual {
		return nil, trace.BadParameter("expected kind %v, got %q", expected, actual)
	}
	if expected, actual := apitypes.V1, resource.GetVersion(); expected != actual {
		return nil, trace.BadParameter("expected version %v, got %q", expected, actual)
	}
	if expected, actual := "", resource.GetSubKind(); expected != actual {
		return nil, trace.BadParameter("expected sub_kind %v, got %q", expected, actual)
	}
	if name := resource.GetMetadata().GetName(); name == "" {
		return nil, trace.BadParameter("missing name")
	}
	if name := resource.GetMetadata().GetName(); name == "none" {
		return nil, trace.BadParameter("got reserved name \"none\"")
	}

	// TODO(espadolini): get rid of this limitation once the story around
	// multiple independent overrides and trust domains is more defined
	if name := resource.GetMetadata().GetName(); name != "default" {
		return nil, trace.BadParameter("expected name \"default\", got %q", name)
	}

	return &ParsedWorkloadIdentityX509IssuerOverride{}, nil
}
