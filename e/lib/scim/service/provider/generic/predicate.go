package generic

import (
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/common"
)

type labelsGetter interface {
	GetMetadata() types.Metadata
}

func hasSCIMOrigin(resource labelsGetter) bool {
	labels := resource.GetMetadata().Labels
	return labels[types.OriginLabel] == common.OriginSCIM

}

func accessListPredicate(item *accesslist.AccessList) bool {
	if item == nil {
		return false
	}
	return hasSCIMOrigin(item) || item.Spec.Type == accesslist.SCIM
}
