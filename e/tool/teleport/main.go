package main

import (
	"context"
	"os"

	"github.com/gravitational/teleport/e/lib/auth"
	"github.com/gravitational/teleport/e/lib/pro"
	"github.com/gravitational/teleport/e/lib/web"
	"github.com/gravitational/teleport/e/tool/modules"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/teleport/common"

	"github.com/gravitational/trace"
)

func main() {
	web.InitPlugin()
	auth.InitPlugin()

	executedCommand, config := common.Run(common.Options{
		Args:     os.Args[1:],
		InitOnly: true,
	})
	if executedCommand == "start" {
		if err := run(config); err != nil {
			utils.FatalError(err)
		}
	}
}

func run(config *service.Config) error {
	newTeleport := func(cfg *service.Config) (service.Process, error) {
		teleport, err := pro.NewTeleport(cfg)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		modules.SetModules(teleport.License)
		auth.SetEnforcer(teleport.Enforcer)
		return teleport, nil
	}
	return service.Run(context.TODO(), *config, newTeleport)
}
