package lister

import (
	"context"

	"github.com/gravitational/trace"

	scimpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/e/lib/scim/service/common"
	scimfilter "github.com/gravitational/teleport/e/lib/scim/service/filter"
)

// GroupLister provides SCIM-compatible listing of Teleport Access Lists (SCIM Groups).
//
// It supports SCIM filtering and pagination, and converts access lists into SCIM resource representations.
type GroupLister struct {
	common.Config
	// Predicate determines which access lists should be included in SCIM results.
	Predicate func(*accesslist.AccessList) bool
	// AccessListToResource converts a Teleport access list into a SCIM resource.
	AccessListToResource func(*accesslist.AccessList) (*scimpb.Resource, error)
}

// ListResources returns a paginated list of SCIM group resources (Teleport Access Lists).
//
// It applies the configured predicate and SCIM filter, and returns only the
// matching subset defined by the pagination parameters in the request.
func (l *GroupLister) ListResources(ctx context.Context, req *scimpb.ListSCIMResourcesRequest) (*scimpb.ResourceList, error) {
	filter, err := scimfilter.ParseFilter(req.GetFilter())
	if err != nil {
		return nil, trace.Wrap(err, "parsing filter")
	}

	var (
		startIndex       = int(req.GetPage().GetStartIndex())
		count            = int(req.GetPage().GetCount())
		currentIndex     = 0
		totalMatched     = 0
		scimGroupResults []*scimpb.Resource
	)

	// Iterate over all access lists in Teleport and filter SCIM-compatible ones
	for acl, err := range clientutils.Resources(ctx, l.AccessListsService.ListAccessLists) {
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if !l.Predicate(acl) {
			continue
		}

		filterAttrs := map[string]string{
			common.GroupNameAttribute:        acl.GetName(),
			common.GroupDisplayNameAttribute: acl.Spec.Title,
		}
		if err := scimfilter.EvaluateFilter(filter, filterAttrs); err != nil {
			continue
		}

		currentIndex++
		if currentIndex < startIndex {
			continue
		}
		if len(scimGroupResults) < count {
			resource, err := l.AccessListToResource(acl)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			scimGroupResults = append(scimGroupResults, resource)
		}
		totalMatched++
	}
	return &scimpb.ResourceList{
		TotalResults: int32(totalMatched),
		StartIndex:   int32(startIndex),
		ItemsPerPage: int32(count),
		Resources:    scimGroupResults,
	}, nil
}
