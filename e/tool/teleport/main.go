package main

import (
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
	modules.SetModules()
	_, config := common.Run(common.Options{
		Args:     os.Args[1:],
		InitOnly: true,
	})
	if err := run(config); err != nil {
		utils.FatalError(err)
	}
}

func run(config *service.Config) error {
	teleport, err := pro.NewTeleport(config)
	if err != nil {
		return trace.Wrap(err)
	}

	auth.InitPlugin(teleport.Enforcer)

	if err := teleport.Start(); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(teleport.Wait())
}
