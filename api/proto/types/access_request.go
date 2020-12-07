package types

import (
	fmt "fmt"

	"github.com/gravitational/trace"
)

func (r *AccessRequestV3) String() string {
	return fmt.Sprintf("AccessRequest(user=%v,roles=%+v)", r.Spec.User, r.Spec.Roles)
}

// IsNone checks if the status is None
func (s RequestState) IsNone() bool {
	return s == RequestState_NONE
}

// IsPending checks if the status is Pending
func (s RequestState) IsPending() bool {
	return s == RequestState_PENDING
}

// IsApproved checks if the status is Approved
func (s RequestState) IsApproved() bool {
	return s == RequestState_APPROVED
}

// IsDenied checks if the status is Denied
func (s RequestState) IsDenied() bool {
	return s == RequestState_DENIED
}

// IsResolved checks if the status is Resolved
func (s RequestState) IsResolved() bool {
	return s.IsApproved() || s.IsDenied()
}

// stateVariants allows iteration of the expected variants
// of RequestState.
var stateVariants = [4]RequestState{
	RequestState_NONE,
	RequestState_PENDING,
	RequestState_APPROVED,
	RequestState_DENIED,
}

// Parse attempts to interpret a value as a string representation
// of a RequestState.
func (s *RequestState) Parse(val string) error {
	for _, state := range stateVariants {
		if state.String() == val {
			*s = state
			return nil
		}
	}
	return trace.BadParameter("unknown request state: %q", val)
}

// Equals compares two AccessRequestSpecV3s
func (s *AccessRequestSpecV3) Equals(other *AccessRequestSpecV3) bool {
	if s.User != other.User {
		return false
	}
	if len(s.Roles) != len(other.Roles) {
		return false
	}
	for i, role := range s.Roles {
		if role != other.Roles[i] {
			return false
		}
	}
	if s.Created != other.Created {
		return false
	}
	if s.Expires != other.Expires {
		return false
	}
	return s.State == other.State
}
