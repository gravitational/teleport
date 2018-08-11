/*
Copyright 2015 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package ui

import (
	"github.com/gravitational/teleport"
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

type authType string

const (
	authLocal authType = "local"
	authSSO   authType = "sso"
)

type userContext struct {
	// AuthType is auth method of this user
	AuthType authType `json:"authType"`
	// Name is this user name
	Name string `json:"userName"`
	// ACL contains user access control list
	ACL userACL `json:"userAcl"`
	// Version is the version of Teleport that is running.
	Version string `json:"version"`
}

func getLogins(roleSet services.RoleSet) []string {
	allowed := []string{}
	denied := []string{}
	for _, role := range roleSet {
		denied = append(denied, role.GetLogins(services.Deny)...)
		allowed = append(allowed, role.GetLogins(services.Allow)...)
	}

	allowed = utils.Deduplicate(allowed)
	denied = utils.Deduplicate(denied)
	userLogins := []string{}
	for _, login := range allowed {
		loginMatch, _ := services.MatchLogin(denied, login)
		if loginMatch == false {
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

	// local user
	authType := authLocal

	// check for any SSO identities
	isSSO := len(user.GetOIDCIdentities()) > 0 ||
		len(user.GetGithubIdentities()) > 0 ||
		len(user.GetSAMLIdentities()) > 0

	if isSSO {
		// SSO user
		authType = authSSO
	}

	return &userContext{
		Name:     user.GetName(),
		ACL:      acl,
		AuthType: authType,
		Version:  teleport.Version,
	}, nil
}
