package teleport

import (
	"fmt"

	"github.com/gravitational/trace"
)

// Role identifies the role of SSH server connection
type Role string

// Check checks if this a a valid role value, returns nil
// if it's ok, false otherwise
func (r Role) Check() error {
	switch r {
	case RoleAuth, RoleUser, RoleWeb, RoleNode, RoleAdmin, RoleProvisionToken, RoleSignup, RoleHangoutRemoteUser:
		return nil
	}
	return trace.Wrap(BadParameter("role", fmt.Sprintf("%v is not supported", r)))
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
	// RoleAdmin is admin role
	RoleAdmin Role = "Admin"
	// RoleProvisionToken is a role for
	RoleProvisionToken Role = "ProvisionToken"
	// RoleSignup is for first time signing up users
	RoleSignup Role = "Signup"
	// RoleHangoutRemoteUser is for users joining remote hangouts
	RoleHangoutRemoteUser Role = "HangoutRemoteUser"
)
