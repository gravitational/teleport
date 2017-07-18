package ui

import (
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
)

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
func NewRole(sRole services.Role) (*Role, error) {
	uiRole := Role{
		Name: sRole.GetName(),
	}

	err := uiRole.Access.init(sRole)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &uiRole, nil
}

// ToTeleRole converts UI Role to Storage Role
func (r *Role) ToTeleRole() (services.Role, error) {
	teleRole, err := services.NewRole(r.Name, services.RoleSpecV3{})
	if err != nil {
		return nil, err
	}

	r.Access.Apply(teleRole)
	return teleRole, nil
}
