package ui

import (
	"time"

	teleservices "github.com/gravitational/teleport/lib/services"
	teleutils "github.com/gravitational/teleport/lib/utils"
)

const (
	inviteStatus            = "invited"
	activeStatus            = "active"
	userTypeToHide          = "agent"
	roleDefaultAllowedLogin = "!invalid!"
)

var adminResources = []string{
	teleservices.KindRole,
	teleservices.KindUser,
	teleservices.KindOIDC,
	teleservices.KindCertAuthority,
	teleservices.KindReverseTunnel,
	teleservices.KindTrustedCluster,
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
func MergeAccessSet(accessList []*RoleAccess) *RoleAccess {
	uiAccess := RoleAccess{}
	for _, item := range accessList {
		uiAccess.SSH.Logins = teleutils.Deduplicate(append(uiAccess.SSH.Logins, item.SSH.Logins...))
		uiAccess.Admin.Enabled = item.Admin.Enabled || uiAccess.Admin.Enabled
	}

	return &uiAccess
}

// Apply applies this role access to Teleport Role
func (a *RoleAccess) Apply(teleRole teleservices.Role) {
	a.applyAdmin(teleRole)
	a.applySSH(teleRole)
}

func (a *RoleAccess) init(teleRole teleservices.Role) {
	a.initAdmin(teleRole)
	a.initSSH(teleRole)
}

func (a *RoleAccess) initSSH(teleRole teleservices.Role) {
	a.SSH.MaxSessionTTL = teleRole.GetMaxSessionTTL().Duration
	a.SSH.NodeLabels = teleRole.GetNodeLabels()
	// FIXME: this is a workaround for #1623
	filteredLogins := []string{}
	for _, login := range teleRole.GetLogins() {
		if login != roleDefaultAllowedLogin {
			filteredLogins = append(filteredLogins, login)
		}
	}

	a.SSH.Logins = filteredLogins
}

func (a *RoleAccess) initAdmin(teleRole teleservices.Role) {
	hasAllNamespaces := teleservices.MatchNamespace(
		teleRole.GetNamespaces(),
		teleservices.Wildcard)

	resources := teleRole.GetResources()
	a.Admin.Enabled = hasFullAccess(resources, adminResources) && hasAllNamespaces
}

func (a *RoleAccess) applyAdmin(teleRole teleservices.Role) {
	if a.Admin.Enabled {
		allowAllNamespaces(teleRole)
		applyResourceAccess(teleRole, adminResources, teleservices.RW())
	} else {
		teleRole.RemoveResource(teleservices.Wildcard)
		applyResourceAccess(teleRole, adminResources, teleservices.RO())
	}
}

func (a *RoleAccess) applySSH(teleRole teleservices.Role) {
	// FIXME: this is a workaround for #1623
	if len(a.SSH.Logins) == 0 {
		a.SSH.Logins = append(a.SSH.Logins, roleDefaultAllowedLogin)
	}

	teleRole.SetMaxSessionTTL(a.SSH.MaxSessionTTL)
	teleRole.SetLogins(a.SSH.Logins)
	teleRole.SetNodeLabels(a.SSH.NodeLabels)
}

func all() []string {
	return []string{teleservices.Wildcard}
}

func allowAllNamespaces(teleRole teleservices.Role) {
	newNamespaces := teleutils.Deduplicate(append(teleRole.GetNamespaces(), all()...))
	teleRole.SetNamespaces(newNamespaces)
}

func none() []string {
	return nil
}

func hasFullAccess(resources map[string][]string, kinds []string) bool {
	for _, kind := range kinds {
		hasRead := teleservices.MatchResourceAction(
			resources,
			kind,
			teleservices.ActionRead)

		hasWrite := teleservices.MatchResourceAction(
			resources,
			kind,
			teleservices.ActionWrite)

		if !(hasRead && hasWrite) {
			return false
		}
	}

	return true
}

func applyResourceAccess(teleRole teleservices.Role, kinds []string, actions []string) {
	for _, kind := range kinds {
		teleRole.SetResource(kind, actions)
	}
}
