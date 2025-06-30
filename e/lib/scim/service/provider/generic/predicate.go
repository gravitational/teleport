package generic

import "github.com/gravitational/teleport/api/types"

const originSCIM = "scim"

type labelsGetter interface {
	GetMetadata() types.Metadata
	GetSubKind() string
}

func isSCIMResource(resource labelsGetter) bool {
	if resource.GetSubKind() == originSCIM {
		return true
	}
	labels := resource.GetMetadata().Labels
	if _, ok := labels[originSCIM]; ok {
		return true
	}
	return labels[types.OriginLabel] == originSCIM
}
