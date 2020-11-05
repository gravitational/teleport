package ui

// AccessRequest describes a request's current state.
type AccessRequest struct {
	// ID is the request ID.
	ID string `json:"id"`
	// State is the request state.
	State string `json:"state"`
	// Reason is currently only used for why request was denied.
	Reason string `json:"reason"`
}
