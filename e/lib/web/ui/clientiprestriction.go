package ui

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	clientiprestrictionv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/clientiprestriction/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
)

// ClientIPRestriction is the UI representation of the client IP restriction
// resource returned by the singular GET and PUT endpoints. It mirrors the RFD 153
// resource surfaced by the ClientIPRestriction gRPC service.
type ClientIPRestriction struct {
	// Cidrs is the list of allowed ingress CIDR blocks.
	Cidrs []string `json:"cidrs"`
	// Mode is the user-controlled operational mode ("draft" or "enforced"; empty
	// is treated as enforced for backward compatibility).
	Mode string `json:"mode,omitempty"`
	// Expires, when set, is the deadline on enforcement. Mode is unchanged when it
	// elapses; the status becomes "expired".
	Expires *time.Time `json:"expires,omitempty"`
	// Status is the server-derived enforcement state ("draft", "pending", "active",
	// "expired").
	Status string `json:"status,omitempty"`
	// Revision is the resource revision, echoed back so writes can guard against
	// acting on a stale view.
	Revision string `json:"revision,omitempty"`
}

// PutClientIPRestrictionRequest is the body accepted by the singular PUT endpoint.
type PutClientIPRestrictionRequest struct {
	Cidrs    []string   `json:"cidrs"`
	Mode     string     `json:"mode,omitempty"`
	Expires  *time.Time `json:"expires,omitempty"`
	Revision string     `json:"revision,omitempty"`
}

// ToProto builds the gRPC resource from the PUT request body.
func (req PutClientIPRestrictionRequest) ToProto() *clientiprestrictionv1pb.ClientIPRestriction {
	var expires *timestamppb.Timestamp
	if req.Expires != nil {
		expires = timestamppb.New(*req.Expires)
	}
	return clientiprestrictionv1pb.ClientIPRestriction_builder{
		Metadata: headerv1.Metadata_builder{
			Revision: req.Revision,
		}.Build(),
		Spec: clientiprestrictionv1pb.ClientIPRestrictionSpec_builder{
			AllowedCidrs: req.Cidrs,
			Mode:         req.Mode,
			Expires:      expires,
		}.Build(),
	}.Build()
}

// ToClientIPRestriction converts the gRPC resource into its UI representation.
func ToClientIPRestriction(cir *clientiprestrictionv1pb.ClientIPRestriction) ClientIPRestriction {
	out := ClientIPRestriction{
		Cidrs:    cir.GetSpec().GetAllowedCidrs(),
		Mode:     cir.GetSpec().GetMode(),
		Status:   cir.GetStatus().GetState(),
		Revision: cir.GetMetadata().GetRevision(),
	}
	if ts := cir.GetSpec().GetExpires(); ts != nil {
		expires := ts.AsTime()
		out.Expires = &expires
	}
	return out
}
