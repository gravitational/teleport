package modules

import (
	"bytes"
	"fmt"
	"runtime"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

func init() {
	// Set the modules to Enterprise but with no license information.
	modules.SetModules(&enterpriseModules{})
}

// SetModules installs modules that provide custom behavior for the
// enterprise compared to the open-source version
func SetModules(license services.License) {
	modules.SetModules(&enterpriseModules{license: license})
}

// enterpriseModules implements pluggable enterprise teleport logic
type enterpriseModules struct {
	license services.License
}

// EmptyRolesHandler is called when a new trusted cluster with empty roles
// is created, for enterprise it returns an error as roles are mandatory
func (p *enterpriseModules) EmptyRolesHandler() error {
	return trace.BadParameter("missing 'role_map' parameter")
}

// SupportsKubernetes returns true if this cluster supports kubernetes
func (p *enterpriseModules) SupportsKubernetes() bool {
	return p.license.GetSupportsKubernetes().Value()
}

// DefaultKubeGroups returns default kuberentes groups for a new admin role
func (p *enterpriseModules) DefaultKubeGroups() []string {
	return []string{teleport.TraitInternalKubeGroupsVariable}
}

// DefaultAllowedLogins returns allowed logins for a new admin role, for
// enterprise it includes "root" as well
func (p *enterpriseModules) DefaultAllowedLogins() []string {
	return []string{teleport.TraitInternalLoginsVariable, teleport.Root}
}

// PrintVersion prints the Teleport version. For enterprise it includes
// "Enterprise" in the output.
func (p *enterpriseModules) PrintVersion() {
	var buf bytes.Buffer

	buf.WriteString(fmt.Sprintf("Teleport Enterprise v%s", teleport.Version))
	buf.WriteString(fmt.Sprintf("git:%s ", teleport.Gitref))
	buf.WriteString(runtime.Version())

	fmt.Println(buf.String())
}

// RolesFromLogins returns roles for external user based on the logins
// extracted from the connector
//
// For Enterprise edition "logins" are used as role names
func (p *enterpriseModules) RolesFromLogins(logins []string) []string {
	return logins
}

// TraitsFromLogins returns traits for external user based on the logins
// extracted from the connector
//
// For Enterprise edition "logins" are used as role names so traits are empty
func (p *enterpriseModules) TraitsFromLogins(logins []string, kubeGroups []string) map[string][]string {
	return nil
}
