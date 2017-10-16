package main

import (
	"os"

	enterpriseUI "github.com/gravitational/teleport/e/lib/web"
	"github.com/gravitational/teleport/e/tool/plugins"

	"github.com/gravitational/teleport/lib/web"
	"github.com/gravitational/teleport/tool/teleport/common"
)

func main() {
	web.SetPlugin(&enterpriseUI.Plugin{})
	plugins.SetPlugins()
	const testRun = false
	common.Run(os.Args[1:], testRun)
}
