package ui

import (
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
)

// User contains data needed by the web UI to display locally saved users.
type User struct {
	// Name is the user name.
	Name string `json:"name"`
	// Roles is the list of roles user belongs to.
	Roles []string `json:"roles"`
	// AuthType is the type of auth service
	// that the user was authenticated through.
	AuthType string `json:"authType"`
}

// NewUser creates UI user object
func NewUser(teleUser services.User) (*User, error) {
	if teleUser == nil {
		return nil, trace.BadParameter("missing teleUser")
	}

	authType := "local"
	if teleUser.GetCreatedBy().Connector != nil {
		authType = teleUser.GetCreatedBy().Connector.Type
	}

	return &User{
		Name:     teleUser.GetName(),
		Roles:    teleUser.GetRoles(),
		AuthType: authType,
	}, nil
}
