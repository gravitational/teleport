package main

import (
	"context"
	"os"

	emodules "github.com/gravitational/teleport/e/tool/modules"
	"github.com/gravitational/teleport/e/tool/teleport/process"
	_ "github.com/gravitational/teleport/lib/fipscheck"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/session/reexec"
	"github.com/gravitational/teleport/tool/teleport/common"
)

func init() {
	metrics.RegisterPrometheusCollectors(metrics.BuildCollector())
}

func main() {
	reexec.MaybeReexec()

	// Set the modules to a default [emodules.EnterpriseModules] so that commands like
	// teleport version output the appropriate information. The modules will
	// be specified appropriately and populated with licensing and feature
	// entitlements by the start commands.
	modules.SetModules(&emodules.EnterpriseModules{})

	app, executedCommand, config := common.Run(common.Options{
		Args:     os.Args[1:],
		InitOnly: true,
	})
	startCmd := app.GetCommand("start")
	appStartCmd := app.GetCommand("app").GetCommand("start")
	dbStartCmd := app.GetCommand("db").GetCommand("start")
	switch executedCommand {
	case startCmd.FullCommand(), appStartCmd.FullCommand(), dbStartCmd.FullCommand():
		if err := service.Run(context.Background(), *config, process.NewTeleport); err != nil {
			utils.FatalError(err)
		}
	}
}
