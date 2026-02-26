package services

import (
	"github.com/gravitational/trace"

	cookiev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/cookie/v1"
)

// ValidateCookie checks resource-specific invariants.
func ValidateCookie(cookie *cookiev1.Cookie) error {
	if cookie == nil {
		return trace.BadParameter("cookie is nil")
	}
	if cookie.GetMetadata() == nil {
		return trace.BadParameter("metadata is required")
	}
	if cookie.GetMetadata().GetName() == "" {
		return trace.BadParameter("name is required")
	}
	if cookie.GetSpec() == nil {
		return trace.BadParameter("spec is required")
	}
	if cookie.GetSpec().GetDomain() == "" {
		return trace.BadParameter("domain is required")
	}
	return nil
}
