package plugins

import (
	"fmt"

	enterpriseUI "github.com/gravitational/teleport/e/lib/web"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/plugins"
	"github.com/gravitational/teleport/lib/web"
	"github.com/gravitational/trace"
)

// SetPlugins installs plugins that provide custom behavior for the
// enterprise compared to the open-source version
func SetPlugins() {
	// injects enterprise web handler plugins
	web.SetPlugin(&enterpriseUI.Plugin{})

	// role map is mandatory for trusted clusters in enterprise mode
	plugins.SetEmptyRolesHandler(func() error {
		return trace.BadParameter("missing 'role_map' parameter")
	})

	// the default role also has "root" for enterprise users
	plugins.SetDefaultAllowedLogins(func() []string {
		return []string{teleport.TraitInternalRoleVariable, teleport.Root}
	})

	// version specifies that this is an enterprise build
	plugins.SetVersionPrinter(func() {
		ver := fmt.Sprintf("Teleport Enterprise v%s", teleport.Version)
		if teleport.Gitref != "" {
			ver = fmt.Sprintf("%s git:%s", ver, teleport.Gitref)
		}
		fmt.Println(ver)
	})
}
