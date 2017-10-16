package modules

import (
	"fmt"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/trace"
)

// SetModules installs modules that provide custom behavior for the
// enterprise compared to the open-source version
func SetModules() {
	modules.SetModules(&enterpriseModules{})
}

// enterpriseModules implements pluggable enterprise teleport logic
type enterpriseModules struct{}

// EmptyRolesHandler is called when a new trusted cluster with empty roles
// is created, for enterprise it returns an error as roles are mandatory
func (p *enterpriseModules) EmptyRolesHandler() error {
	return trace.BadParameter("missing 'role_map' parameter")
}

// DefaultAllowedLogins returns allowed logins for a new admin role, for
// enterprise it includes "root" as well
func (p *enterpriseModules) DefaultAllowedLogins() []string {
	return []string{teleport.TraitInternalRoleVariable, teleport.Root}
}

// PrintVersion prints teleport version, for enterprise it includes
// "Enterprise" in the output
func (p *enterpriseModules) PrintVersion() {
	ver := fmt.Sprintf("Teleport Enterprise v%s", teleport.Version)
	if teleport.Gitref != "" {
		ver = fmt.Sprintf("%s git:%s", ver, teleport.Gitref)
	}
	fmt.Println(ver)
}
