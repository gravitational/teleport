package ui

import (
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

type access struct {
	List   bool `json:"list"`
	Read   bool `json:"read"`
	Edit   bool `json:"edit"`
	Create bool `json:"create"`
	Delete bool `json:"remove"`
}

type userACL struct {
	// Sessions defines access to recorded sessions
	Sessions access `json:"sessions"`
	// AuthConnectors defines access to auth.connectors
	AuthConnectors access `json:"authConnectors"`
	// Roles defines access to roles
	Roles access `json:"roles"`
	// TrustedClusters defines access to trusted clusters
	TrustedClusters access `json:"trustedClusters"`
	// SSH defines access to servers
	SSHLogins []string `json:"sshLogins"`
}

type userContext struct {
	// Name is this user name
	Name string `json:"userName"`
	// ACL contains user access control list
	ACL userACL `json:"userAcl"`
}

func getLogins(roleSet services.RoleSet) []string {
	allowed := []string{}
	denied := []string{}
	for _, role := range roleSet {
		denied = append(role.GetLogins(services.Deny), denied...)
		allowed = append(role.GetLogins(services.Allow), allowed...)
	}

	allowed = utils.Deduplicate(allowed)
	denied = utils.Deduplicate(denied)
	userLogins := []string{}
	for _, login := range allowed {
		if services.MatchLogin(denied, login) == false {
			userLogins = append(userLogins, login)
		}
	}

	return userLogins
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
		List:   hasAccess(roleSet, ctx, kind, services.VerbList),
		Read:   hasAccess(roleSet, ctx, kind, services.VerbRead),
		Edit:   hasAccess(roleSet, ctx, kind, services.VerbUpdate),
		Create: hasAccess(roleSet, ctx, kind, services.VerbCreate),
		Delete: hasAccess(roleSet, ctx, kind, services.VerbDelete),
	}
}

// NewUserContext constructs user context from roles assigned to user
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
