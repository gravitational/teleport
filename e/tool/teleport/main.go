package main

import (
	"os"

	"github.com/gravitational/enterprise/lib/web"
	teleweb "github.com/gravitational/teleport/lib/web"
	"github.com/gravitational/teleport/tool/teleport/common"
)

func main() {
	// injects enterprise web handler plugins
	teleweb.SetPlugin(&web.Plugin{})
	const testRun = false
	common.Run(os.Args[1:], testRun)
}
