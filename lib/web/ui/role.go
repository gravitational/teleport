package ui

import teleservices "github.com/gravitational/teleport/lib/services"

// Role describes user role consumed by web ui
type Role struct {
	// Name is role name
	Name string `json:"name"`
	// Access is a set of attributes describing role permissions
	Access RoleAccess `json:"access"`
	// System is a flag indicating if a role is builtin system role
	System bool `json:"system"`
}

// NewRole creates a new instance of UI Role
func NewRole(sRole teleservices.Role) *Role {
	uiRole := Role{
		Name: sRole.GetName(),
	}

	uiRole.Access.init(sRole)
	return &uiRole
}

// ToTeleRole converts UI Role to Storage Role
func (r *Role) ToTeleRole() (teleservices.Role, error) {
	teleRole, err := teleservices.NewRole(r.Name, teleservices.RoleSpecV3{})
	if err != nil {
		return nil, err
	}

	r.Access.Apply(teleRole)
	return teleRole, nil
}
