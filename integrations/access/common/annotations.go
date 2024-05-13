package common

import (
	"slices"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// GetServiceNamesFromAnnotations reads systems annotations from an access
// requests and returns the services to notify/use for approval.
// The list is sorted and duplicates are removed.
func GetServiceNamesFromAnnotations(req types.AccessRequest, annotationKey string) ([]string, error) {
	serviceNames, ok := req.GetSystemAnnotations()[annotationKey]
	if !ok {
		return nil, trace.NotFound("request annotation %s is missing", annotationKey)
	}
	if len(serviceNames) == 0 {
		return nil, trace.BadParameter("request annotation %s is present but empty", annotationKey)
	}
	slices.Sort(serviceNames)
	return slices.Compact(serviceNames), nil
}
