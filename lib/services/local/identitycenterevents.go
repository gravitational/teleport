package local

import (
	"github.com/gravitational/trace"

	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
)

type identityCenterAccountParser struct {
	baseParser
}

func newIdentityCenterAccountParser() *identityCenterAccountParser {
	return &identityCenterAccountParser{
		baseParser: newBaseParser(backend.NewKey(awsICPrefix, awsAccountPrefix)),
	}
}

func (p *identityCenterAccountParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		return resourceHeader(event, types.KindProvisioningState, types.V1, 0)
	case types.OpPut:
		r, err := services.UnmarshalProtoResource[*identitycenterv1.Account](
			event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return types.Resource153ToLegacy(r), nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

type identityCenterPrincipalAssignmentParser struct {
	baseParser
}

func newIdentityCenterPrincipalAssignmentParser() *identityCenterPrincipalAssignmentParser {
	return &identityCenterPrincipalAssignmentParser{
		baseParser: newBaseParser(backend.NewKey(awsPrincipalAssignmentPrefix)),
	}
}

func (p *identityCenterPrincipalAssignmentParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		return resourceHeader(event, types.KindProvisioningState, types.V1, 0)

	case types.OpPut:
		r, err := services.UnmarshalPrincipalAssignment(event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return types.Resource153ToLegacy(r), nil

	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}
