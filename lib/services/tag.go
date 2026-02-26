package services

import (
	"github.com/gravitational/trace"

	tagv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/tag/v1"
)

// ValidateTag checks resource-specific invariants.
func ValidateTag(tag *tagv1.Tag) error {
	if tag == nil {
		return trace.BadParameter("tag is nil")
	}
	if tag.GetMetadata() == nil {
		return trace.BadParameter("metadata is required")
	}
	if tag.GetMetadata().GetName() == "" {
		return trace.BadParameter("name is required")
	}
	return nil
}
