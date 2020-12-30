package ui

import (
	"time"

	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

// AccessRequest describes a request's current state.
type AccessRequest struct {
	// ID is the request ID.
	ID string `json:"id"`
	// State is the request state.
	State string `json:"state"`
	// ResolveReason is an optional message on the reason
	// why a request was resolved (approved, denied, etc).
	ResolveReason string `json:"resolveReason"`
	// RequestReason is the reason for request.
	RequestReason string `json:"requestReason"`
	// User is the name of requestor.
	User string `json:"user"`
	// Roles are the list of roles requested.
	Roles []string `json:"roles"`
	// Created is the time the request was made.
	Created time.Time `json:"created"`
	// Expires is when the request will expire.
	Expires time.Time `json:"expires"`
}

// NewAccessRequest creates a UI access request object.
func NewAccessRequest(request services.AccessRequest) (*AccessRequest, error) {
	if request == nil {
		return nil, trace.BadParameter("nil request")
	}

	// Access Request state NONE is its empty value and the empty value
	// is treated internally as an error, so it should return as an error.
	if request.GetState().IsNone() {
		return nil, trace.BadParameter("request %q, state is set to none", request.GetMetadata().Name)
	}

	return &AccessRequest{
		ID:            request.GetMetadata().Name,
		State:         request.GetState().String(),
		ResolveReason: request.GetResolveReason(),
		RequestReason: request.GetRequestReason(),
		User:          request.GetUser(),
		Roles:         request.GetRoles(),
		Created:       request.GetCreationTime(),
		Expires:       request.GetAccessExpiry(),
	}, nil
}
