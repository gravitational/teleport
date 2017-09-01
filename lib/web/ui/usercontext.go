package ui

import (
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

type access struct {
	Read   bool `json:"read"`
	Edit   bool `json:"edit"`
	Create bool `json:"create"`
	Delete bool `json:"delete"`
}

type userACL struct {
	// Sessions defines access to recorded sessions
	Sessions access `json:"sessions"`
	// AuthConnectors defines access to auth.connectors
	AuthConnectors access `json:"authConnectors"`
	// Roles defined access to roles
	Roles access `json:"roles"`
	// TrustedClusters defined access to trusted clusters
	TrustedClusters access `json:"trustedClusters"`
	// SSH defined access to servers
	SSHLogins []string `json:"sshLogins"`
}

type userContext struct {
	// Name is this user name
	Name string `json:"userName"`
	// Email is this user email
	Email string `json:"userEmail"`
	// Logins is this user available logins
	Logins []string `json:"userLogins"`
	// ACL contains user access control list
	ACL userACL `json:"userAcl"`
}

func getLogins(roleSet services.RoleSet) []string {
	allLogins := []string{}
	for _, role := range roleSet {
		logins := role.GetLogins(services.Allow)
		allLogins = append(allLogins, logins...)
	}

	return utils.Deduplicate(allLogins)
}

func hasAccess(roleSet services.RoleSet, ctx *services.Context, kind string, verbs ...string) bool {
	for _, verb := range verbs {
		err := roleSet.CheckAccessToRule(ctx, defaults.Namespace, kind, verb)
		if err != nil {
			return false
		}
	}

	return true
}

func newAccess(roleSet services.RoleSet, ctx *services.Context, kind string) access {
	return access{
		Read:   hasAccess(roleSet, ctx, kind, services.VerbList),
		Edit:   hasAccess(roleSet, ctx, kind, services.VerbUpdate),
		Create: hasAccess(roleSet, ctx, kind, services.VerbCreate),
		Delete: hasAccess(roleSet, ctx, kind, services.VerbDelete),
	}
}

// NewUserContext returns userContext
func NewUserContext(user services.User, userRoles services.RoleSet) (*userContext, error) {
	ctx := &services.Context{User: user}
	sessionAccess := newAccess(userRoles, ctx, services.KindSession)
	roleAccess := newAccess(userRoles, ctx, services.KindRole)
	authConnectors := newAccess(userRoles, ctx, services.KindAuthConnector)
	trustedClusterAccess := newAccess(userRoles, ctx, services.KindTrustedCluster)
	logins := getLogins(userRoles)

	acl := userACL{
		AuthConnectors:  authConnectors,
		TrustedClusters: trustedClusterAccess,
		Sessions:        sessionAccess,
		Roles:           roleAccess,
		SSHLogins:       logins,
	}

	return &userContext{
		Name: user.GetName(),
		ACL:  acl,
	}, nil
}
