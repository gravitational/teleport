package plugins

import (
	"fmt"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/plugins"
	"github.com/gravitational/trace"
)

// SetPlugins installs plugins that provide custom behavior for the
// enterprise compared to the open-source version
func SetPlugins() {
	plugins.SetPlugins(&enterprisePlugins{})
}

// enterprisePlugins implements pluggable enterprise teleport logic
type enterprisePlugins struct{}

// EmptyRolesHandler is called when a new trusted cluster with empty roles
// is created, for enterprise it returns an error as roles are mandatory
func (p *enterprisePlugins) EmptyRolesHandler() error {
	return trace.BadParameter("missing 'role_map' parameter")
}

// DefaultAllowedLogins returns allowed logins for a new admin role, for
// enterprise it includes "root" as well
func (p *enterprisePlugins) DefaultAllowedLogins() []string {
	return []string{teleport.TraitInternalRoleVariable, teleport.Root}
}

// PrintVersion prints teleport version, for enterprise it includes
// "Enterprise" in the output
func (p *enterprisePlugins) PrintVersion() {
	ver := fmt.Sprintf("Teleport Enterprise v%s", teleport.Version)
	if teleport.Gitref != "" {
		ver = fmt.Sprintf("%s git:%s", ver, teleport.Gitref)
	}
	fmt.Println(ver)
}
