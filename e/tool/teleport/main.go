package main

import (
	"os"

	enterpriseUI "github.com/gravitational/teleport/e/lib/web"
	"github.com/gravitational/teleport/e/tool/modules"

	"github.com/gravitational/teleport/lib/web"
	"github.com/gravitational/teleport/tool/teleport/common"
)

func main() {
	// injects enterprise web handler plugins
	web.SetPlugin(&enterpriseUI.Plugin{})
	modules.SetModules()
	const testRun = false
	common.Run(os.Args[1:], testRun)
}
