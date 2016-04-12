package teleport

import (
	"fmt"
	"strings"

	"github.com/gravitational/trace"
)

// Role identifies the role of SSH server connection
type Role string

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
	case RoleAuth, RoleUser, RoleWeb, RoleNode, RoleAdmin, RoleProvisionToken, RoleSignup, RoleProxy:
		return nil
	}
	return trace.BadParameter("role %v is not supported", *r)
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
)
