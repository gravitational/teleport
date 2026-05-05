package ui

// ScopedRoleListItem holds a subset of the scoped role definition needed by the UI.
type ScopedRoleListItem struct {
	Name             string   `json:"name"`
	Scope            string   `json:"scope"`
	AssignableScopes []string `json:"assignableScopes,omitempty"`
}

// ListScopedRolesResponse is a paginated response type for listed scoped roles.
type ListScopedRolesResponse struct {
	Roles    []ScopedRoleListItem `json:"roles"`
	StartKey string               `json:"startKey,omitempty"`
}
