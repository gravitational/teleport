package ui

import (
	"time"

	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
)

const (
	inviteStatus            = "invited"
	activeStatus            = "active"
	userTypeToHide          = "agent"
	roleDefaultAllowedLogin = "!invalid!"
)

var adminRules = []string{
	services.KindRole,
	services.KindUser,
	services.KindOIDC,
	services.KindCertAuthority,
	services.KindReverseTunnel,
	services.KindTrustedCluster,
	services.KindNode,
}

// AdminAccess describes admin access
type AdminAccess struct {
	// Enabled indicates if access is enabled
	Enabled bool `json:"enabled"`
}

// SSHAccess describes shh access
type SSHAccess struct {
	// Logins is a list of allowed logins
	Logins []string `json:"logins"`
	// MaxSessionTTL is max session TLL
	MaxSessionTTL time.Duration `json:"maxTtl"`
	// NodeLabels
	NodeLabels map[string]string `json:"nodeLabels"`
}

// RoleAccess describes a set of role permissions
type RoleAccess struct {
	// Admin describes admin access
	Admin AdminAccess `json:"admin"`
	// SSH describes SSH access
	SSH SSHAccess `json:"ssh"`
}

// MergeAccessSet merges a set of roles by strongest permission
func MergeAccessSet(accessList []RoleAccess) RoleAccess {
	uiAccess := RoleAccess{}
	for _, item := range accessList {
		uiAccess.SSH.Logins = utils.Deduplicate(append(uiAccess.SSH.Logins, item.SSH.Logins...))
		uiAccess.Admin.Enabled = item.Admin.Enabled || uiAccess.Admin.Enabled
	}

	return uiAccess
}

// Apply applies this role access to Teleport Role
func (a *RoleAccess) Apply(teleRole services.Role) {
	a.applyAdmin(teleRole)
	a.applySSH(teleRole)
}

func (a *RoleAccess) init(teleRole services.Role) error {
	a.initAdmin(teleRole)

	err := a.initSSH(teleRole)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (a *RoleAccess) initSSH(teleRole services.Role) error {
	maxSessionTTL, err := teleRole.GetOptions().GetDuration(services.MaxSessionTTL)
	if err != nil {
		return trace.Wrap(err)
	}

	a.SSH.MaxSessionTTL = maxSessionTTL.Duration
	a.SSH.NodeLabels = teleRole.GetNodeLabels(services.Allow)

	// FIXME: this is a workaround for #1623
	filteredLogins := []string{}
	for _, login := range teleRole.GetLogins(services.Allow) {
		if login != roleDefaultAllowedLogin {
			filteredLogins = append(filteredLogins, login)
		}
	}
	a.SSH.Logins = filteredLogins

	return nil
}

func (a *RoleAccess) initAdmin(teleRole services.Role) {
	hasAllNamespaces := services.MatchNamespace(
		teleRole.GetNamespaces(services.Allow),
		services.Wildcard)

	rules := teleRole.GetRules(services.Allow)
	a.Admin.Enabled = hasFullAccess(rules, adminRules) && hasAllNamespaces
}

func (a *RoleAccess) applyAdmin(role services.Role) {
}

func (a *RoleAccess) applySSH(teleRole services.Role) {
	// FIXME: this is a workaround for #1623
	if len(a.SSH.Logins) == 0 {
		a.SSH.Logins = append(a.SSH.Logins, roleDefaultAllowedLogin)
	}

	roleOptions := teleRole.GetOptions()
	roleOptions[services.MaxSessionTTL] = services.NewDuration(a.SSH.MaxSessionTTL)
	teleRole.SetOptions(roleOptions)

	teleRole.SetLogins(services.Allow, a.SSH.Logins)
	teleRole.SetNodeLabels(services.Allow, a.SSH.NodeLabels)
}

func all() []string {
	return []string{services.Wildcard}
}

func allowAllNamespaces(teleRole services.Role) {
	newNamespaces := utils.Deduplicate(append(teleRole.GetNamespaces(services.Allow), all()...))
	teleRole.SetNamespaces(services.Allow, newNamespaces)
}

func none() []string {
	return nil
}

func hasFullAccess(rules []services.Rule, resources []string) bool {
	set := services.MakeRuleSet(rules)
	for _, resource := range resources {
		hasRead := set.Match(resource, services.ActionRead)
		hasWrite := set.Match(resource, services.ActionWrite)

		if !(hasRead && hasWrite) {
			return false
		}
	}

	return true
}
