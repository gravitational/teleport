package ui

import (
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

type access struct {
	Read bool `json:"read"`
}

type sshAccess struct {
	Logins []string `json:"logins"`
}

type userACL struct {
	Sessions        access    `json:"sessions"`
	AuthConnectors  access    `json:"authConnectors"`
	Roles           access    `json:"roles"`
	TrustedClusters access    `json:"trustedClusters"`
	SSH             sshAccess `json:"ssh"`
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

func canReadTrustedClusters(roleSet services.RoleSet, ctx *services.Context) bool {
	return checkAccess(roleSet, ctx, services.KindTrustedCluster, services.VerbList) &&
		checkAccess(roleSet, ctx, services.KindTrustedCluster, services.VerbRead)
}

func canReadRoles(roleSet services.RoleSet, ctx *services.Context) bool {
	return checkAccess(roleSet, ctx, services.KindSession, services.VerbList)
}

func canReadSessions(roleSet services.RoleSet, ctx *services.Context) bool {
	return checkAccess(roleSet, ctx, services.KindSession, services.VerbList)
}

func checkAccess(roleSet services.RoleSet, ctx *services.Context, kind string, verb string) bool {
	err := roleSet.CheckAccessToRule(ctx, defaults.Namespace, kind, verb)
	return err == nil
}

// NewUserContext returns userContext
func NewUserContext(user services.User, userRoles services.RoleSet) (*userContext, error) {
	ctx := &services.Context{User: user}

	sessionAccess := access{
		Read: canReadSessions(userRoles, ctx),
	}

	roleAccess := access{
		Read: canReadRoles(userRoles, ctx),
	}

	trustedClusterAccess := access{
		Read: canReadTrustedClusters(userRoles, ctx),
	}

	logins := getLogins(userRoles)
	ssh := sshAccess{
		Logins: logins,
	}

	acl := userACL{
		TrustedClusters: trustedClusterAccess,
		Sessions:        sessionAccess,
		Roles:           roleAccess,
		SSH:             ssh,
	}

	return &userContext{
		Name: user.GetName(),
		ACL:  acl,
	}, nil
}
