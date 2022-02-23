package main

import (
	"context"
	"os"

	"github.com/gravitational/teleport/e/tool/teleport/process"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/teleport/common"
)

func init() {
	utils.RegisterPrometheusCollectors(utils.BuildCollector())
}

func main() {
	app, executedCommand, config := common.Run(common.Options{
		Args:     os.Args[1:],
		InitOnly: true,
	})
	startCmd := app.GetCommand("start")
	appStartCmd := app.GetCommand("app").GetCommand("start")
	dbStartCmd := app.GetCommand("db").GetCommand("start")
	switch executedCommand {
	case startCmd.FullCommand(), appStartCmd.FullCommand(), dbStartCmd.FullCommand():
		if err := service.Run(context.TODO(), *config, process.NewTeleport); err != nil {
			utils.FatalError(err)
		}
	}
}
