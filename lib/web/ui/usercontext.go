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
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
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

type accessStrategy struct {
	// Type determines how a user should access teleport resources.
	// ie: does the user require a request to access resources?
	Type types.RequestStrategy `json:"type"`
	// Prompt is the optional dialogue shown to user,
	// when the access strategy type requires a reason.
	Prompt string `json:"prompt"`
}

// AccessCapabilities defines allowable access request rules defined in a user's roles.
type AccessCapabilities struct {
	// RequestableRoles is a list of roles that the user can select when requesting access.
	RequestableRoles []string `json:"requestableRoles"`
	// SuggestedReviewers is a list of reviewers that the user can select when creating a request.
	SuggestedReviewers []string `json:"suggestedReviewers"`
}

type userACL struct {
	// Sessions defines access to recorded sessions
	Sessions access `json:"sessions"`
	// AuthConnectors defines access to auth.connectors
	AuthConnectors access `json:"authConnectors"`
	// Roles defines access to roles
	Roles access `json:"roles"`
	// Users defines access to users.
	Users access `json:"users"`
	// TrustedClusters defines access to trusted clusters
	TrustedClusters access `json:"trustedClusters"`
	// Events defines access to audit logs
	Events access `json:"events"`
	// Tokens defines access to tokens.
	Tokens access `json:"tokens"`
	// Nodes defines access to nodes.
	Nodes access `json:"nodes"`
	// AppServers defines access to application servers
	AppServers access `json:"appServers"`
	// DBServers defines access to database servers.
	DBServers access `json:"dbServers"`
	// KubeServers defines access to kubernetes servers.
	KubeServers access `json:"kubeServers"`
	// SSH defines access to servers
	SSHLogins []string `json:"sshLogins"`
	// AccessRequests defines access to access requests
	AccessRequests access `json:"accessRequests"`
	// Billing defines access to billing information
	Billing access `json:"billing"`
}

type authType string

const (
	authLocal authType = "local"
	authSSO   authType = "sso"
)

// UserContext describes user settings and access to various resources.
type UserContext struct {
	// AuthType is auth method of this user.
	AuthType authType `json:"authType"`
	// Name is this user name.
	Name string `json:"userName"`
	// ACL contains user access control list.
	ACL userACL `json:"userAcl"`
	// Cluster contains cluster detail for this user's context.
	Cluster *Cluster `json:"cluster"`
	// AccessStrategy describes how a user should access teleport resources.
	AccessStrategy accessStrategy `json:"accessStrategy"`
	// AccessCapabilities defines allowable access request rules defined in a user's roles.
	AccessCapabilities AccessCapabilities `json:"accessCapabilities"`
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
		if !loginMatch {
			userLogins = append(userLogins, login)
		}
	}

	return userLogins
}

func hasAccess(roleSet services.RoleSet, ctx *services.Context, kind string, verbs ...string) bool {
	for _, verb := range verbs {
		// Since this check occurs often and it does not imply the caller is trying
		// to access any resource, silence any logging done on the proxy.
		err := roleSet.CheckAccessToRule(ctx, defaults.Namespace, kind, verb, true)
		if err != nil {
			return false
		}
	}

	return true
}

func newAccess(roleSet services.RoleSet, ctx *services.Context, kind string) access {
	return access{
		List:   hasAccess(roleSet, ctx, kind, types.VerbList),
		Read:   hasAccess(roleSet, ctx, kind, types.VerbRead),
		Edit:   hasAccess(roleSet, ctx, kind, types.VerbUpdate),
		Create: hasAccess(roleSet, ctx, kind, types.VerbCreate),
		Delete: hasAccess(roleSet, ctx, kind, types.VerbDelete),
	}
}

func getAccessStrategy(roleset services.RoleSet) accessStrategy {
	strategy := types.RequestStrategyOptional
	prompt := ""

	for _, role := range roleset {
		options := role.GetOptions()

		if options.RequestAccess == types.RequestStrategyReason {
			strategy = types.RequestStrategyReason
			prompt = options.RequestPrompt
			break
		}

		if options.RequestAccess == types.RequestStrategyAlways {
			strategy = types.RequestStrategyAlways
		}
	}

	return accessStrategy{
		Type:   strategy,
		Prompt: prompt,
	}
}

// NewUserContext returns user context
func NewUserContext(user types.User, userRoles services.RoleSet, features proto.Features) (*UserContext, error) {
	ctx := &services.Context{User: user}
	sessionAccess := newAccess(userRoles, ctx, types.KindSession)
	roleAccess := newAccess(userRoles, ctx, types.KindRole)
	authConnectors := newAccess(userRoles, ctx, types.KindAuthConnector)
	trustedClusterAccess := newAccess(userRoles, ctx, types.KindTrustedCluster)
	eventAccess := newAccess(userRoles, ctx, types.KindEvent)
	userAccess := newAccess(userRoles, ctx, types.KindUser)
	tokenAccess := newAccess(userRoles, ctx, types.KindToken)
	nodeAccess := newAccess(userRoles, ctx, types.KindNode)
	appServerAccess := newAccess(userRoles, ctx, types.KindAppServer)
	dbServerAccess := newAccess(userRoles, ctx, types.KindDatabaseServer)
	kubeServerAccess := newAccess(userRoles, ctx, types.KindKubeService)
	requestAccess := newAccess(userRoles, ctx, types.KindAccessRequest)

	var billingAccess access
	if features.Cloud {
		billingAccess = newAccess(userRoles, ctx, types.KindBilling)
	}

	logins := getLogins(userRoles)
	accessStrategy := getAccessStrategy(userRoles)

	acl := userACL{
		AccessRequests:  requestAccess,
		AppServers:      appServerAccess,
		DBServers:       dbServerAccess,
		KubeServers:     kubeServerAccess,
		AuthConnectors:  authConnectors,
		TrustedClusters: trustedClusterAccess,
		Sessions:        sessionAccess,
		Roles:           roleAccess,
		Events:          eventAccess,
		SSHLogins:       logins,
		Users:           userAccess,
		Tokens:          tokenAccess,
		Nodes:           nodeAccess,
		Billing:         billingAccess,
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

	return &UserContext{
		Name:           user.GetName(),
		ACL:            acl,
		AuthType:       authType,
		AccessStrategy: accessStrategy,
	}, nil
}
