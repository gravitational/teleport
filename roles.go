package teleport

import (
	"fmt"
	"strings"

	"github.com/gravitational/trace"
)

// Role identifies the role of SSH server connection
type Role string
type Roles []Role

// ParseRoles takes a comma-separated list of roles and returns a slice
// of roles, or an error if parsing failed
func ParseRoles(str string) (roles Roles, err error) {
	for _, s := range strings.Split(str, ",") {
		r := Role(strings.Title(strings.ToLower(strings.TrimSpace(s))))
		if err = r.Check(); err != nil {
			return nil, trace.Wrap(err)
		}
		roles = append(roles, r)
	}
	return roles, nil
}

// Includes returns 'true' if a given list of roles includes a given role
func (roles Roles) Include(role Role) bool {
	for _, r := range roles {
		if r == role {
			return true
		}
	}
	return false
}

// Equals compares two sets of roles
func (roles Roles) Equals(other Roles) bool {
	if len(roles) != len(other) {
		return false
	}
	for _, r := range roles {
		if !other.Include(r) {
			return false
		}
	}
	return true
}

// Check returns an erorr if the role set is incorrect (contains unknown roles)
func (roles Roles) Check() (err error) {
	for _, role := range roles {
		if err = role.Check(); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (roles Roles) String() string {
	s := make([]string, 0)
	for _, r := range roles {
		s = append(s, string(r))
	}
	return strings.Join(s, ",")
}

// Set sets the value of the role from string, used to integrate with CLI tools
func (r *Role) Set(v string) error {
	val := Role(strings.Title(v))
	if err := val.Check(); err != nil {
		return trace.Wrap(err)
	}
	*r = val
	return nil
}

// String returns debug-friendly representation of this role
func (r *Role) String() string {
	return fmt.Sprintf("%v", strings.ToUpper(string(*r)))
}

// Check checks if this a a valid role value, returns nil
// if it's ok, false otherwise
func (r *Role) Check() error {
	switch *r {
	case RoleAuth, RoleUser, RoleWeb, RoleNode, RoleAdmin, RoleProvisionToken, RoleSignup, RoleProxy, RoleNop:
		return nil
	}
	return trace.BadParameter("role %v is not registered", *r)
}

// Role returns system username associated with it
func (r Role) User() string {
	return fmt.Sprintf("@%v", strings.ToLower(string(r)))
}

// SystemUsernamePrefix is reserved for system users
const SystemUsernamePrefix = "@"

// IsSystemUsername returns true if given username is a reserved system username
func IsSystemUsername(username string) bool {
	return strings.HasPrefix(username, SystemUsernamePrefix)
}

const (
	// RoleAuth is for teleport auth server (authority, authentication and authorization)
	RoleAuth Role = "Auth"
	// RoleUser is a role for teleport SSH user
	RoleUser Role = "User"
	// RoleWeb is for web access users
	RoleWeb Role = "Web"
	// RoleNode is a role for SSH node in the cluster
	RoleNode Role = "Node"
	// RoleProxy is a role for SSH proxy in the cluster
	RoleProxy Role = "Proxy"
	// RoleAdmin is admin role
	RoleAdmin Role = "Admin"
	// RoleProvisionToken is a role for nodes authenticated using provisioning tokens
	RoleProvisionToken Role = "ProvisionToken"
	// RoleSignup is for first time signing up users
	RoleSignup Role = "Signup"
	// RoleNop is used for actions that already using external authz mechanisms
	// e.g. tokens or passwords
	RoleNop Role = "Nop"
)
