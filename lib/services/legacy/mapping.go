package legacy

// RoleMap is a list of mappings
type RoleMap []RoleMapping

// RoleMappping provides mapping of remote roles to local roles
// for trusted clusters
type RoleMapping struct {
	// Remote specifies remote role name to map from
	Remote string `json:"remote"`
	// Local specifies local roles to map to
	Local []string `json:"local"`
}
