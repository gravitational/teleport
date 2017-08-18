package ui

import (
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
)

type userContext struct {
	// Name is this user name
	Name string `json:"userName"`
	// Email is this user email
	Email string `json:"userEmail"`
	// ACL is this user access control list
	ACL RoleAccess `json:"userAcl"`
}

// NewUserContext returns userContext
func NewUserContext(user services.User, allRoles []services.Role) (*userContext, error) {
	userRoles := user.GetRoles()
	roleNamesMap := map[string]bool{}
	for _, name := range userRoles {
		roleNamesMap[name] = true
	}

	accessSet := []RoleAccess{}
	for _, item := range allRoles {
		if roleNamesMap[item.GetName()] {
			uiRole, err := NewRole(item)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			accessSet = append(accessSet, uiRole.Access)
		}
	}

	userACL := MergeAccessSet(accessSet)
	return &userContext{
		Name: user.GetName(),
		ACL:  userACL,
	}, nil
}
