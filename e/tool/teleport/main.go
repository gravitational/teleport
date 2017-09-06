package main

import (
	"os"

	enterpriseUI "github.com/gravitational/teleport/e/lib/web"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/web"
	"github.com/gravitational/teleport/tool/teleport/common"
)

func main() {
	// injects enterprise web handler plugins
	web.SetPlugin(&enterpriseUI.Plugin{})
	const testRun = false
	common.Run(os.Args[1:], teleport.DistroTypeEnterprise, testRun)
}
