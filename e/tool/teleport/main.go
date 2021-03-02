package main

import (
	"context"
	"os"

	"github.com/gravitational/teleport/e/tool/teleport/process"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/teleport/common"
)

func main() {
	executedCommand, config := common.Run(common.Options{
		Args:     os.Args[1:],
		InitOnly: true,
	})
	if executedCommand == "start" {
		if err := service.Run(context.TODO(), *config, process.NewTeleport); err != nil {
			utils.FatalError(err)
		}
	}
}
